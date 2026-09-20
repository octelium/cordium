/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cordium

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"google.golang.org/grpc"
)

// Stream identifies one of the two output streams of a command.
type Stream uint8

const (
	// StreamStdout is the command's standard output.
	StreamStdout Stream = iota + 1
	// StreamStderr is the command's standard error.
	StreamStderr
)

// String returns "stdout" or "stderr".
func (s Stream) String() string {
	switch s {
	case StreamStdout:
		return "stdout"
	case StreamStderr:
		return "stderr"
	default:
		return "unknown"
	}
}

// ExecOutput is a chunk of a command's output, as it was produced. The chunks
// are not aligned on line boundaries.
type ExecOutput struct {
	// Stream is the output stream that the chunk was produced on.
	Stream Stream
	// Data is the raw chunk. It is owned by the receiver.
	Data []byte
}

// String returns the chunk as a string.
func (o ExecOutput) String() string { return string(o.Data) }

// ExecResult is the outcome of a command that was executed inside a Workspace.
type ExecResult struct {
	// Command is the command that was executed.
	Command string
	// ExitCode is the status code with which the command exited. It is -1 when
	// the command was terminated by a signal, which is what killing it does.
	ExitCode int
	// Stdout is the captured standard output. It is empty when the capture was
	// turned off with [WithoutCapture].
	Stdout []byte
	// Stderr is the captured standard error.
	Stderr []byte
	// Combined is the captured standard output and standard error, interleaved
	// in the order in which they were produced.
	Combined []byte
	// Truncated reports whether the capture hit the limit set by
	// [WithMaxCaptureBytes] and therefore dropped part of the output.
	Truncated bool
	// Killed reports whether the command was terminated by [ExecSession.Kill]
	// rather than having exited on its own.
	Killed bool
	// StartedAt is when the execution was requested.
	StartedAt time.Time
	// FinishedAt is when the command exited.
	FinishedAt time.Time
}

// Duration returns how long the execution took.
func (r *ExecResult) Duration() time.Duration {
	if r == nil || r.StartedAt.IsZero() || r.FinishedAt.IsZero() {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}

// Success reports whether the command exited with the status code zero.
func (r *ExecResult) Success() bool { return r != nil && r.ExitCode == 0 }

// StdoutString returns the captured standard output as a string.
func (r *ExecResult) StdoutString() string {
	if r == nil {
		return ""
	}
	return string(r.Stdout)
}

// StderrString returns the captured standard error as a string.
func (r *ExecResult) StderrString() string {
	if r == nil {
		return ""
	}
	return string(r.Stderr)
}

// CombinedString returns the captured standard output and standard error,
// interleaved in the order in which they were produced, as a string.
func (r *ExecResult) CombinedString() string {
	if r == nil {
		return ""
	}
	return string(r.Combined)
}

// Output returns the captured standard output with the surrounding whitespace
// removed, which is what reading a single value out of a command needs.
func (r *ExecResult) Output() string {
	return strings.TrimSpace(r.StdoutString())
}

// Err returns a [*ExitError] when the command exited with a non-zero status
// code, and nil otherwise. Exec itself only fails on transport errors, so this
// is how a caller turns an unsuccessful command into an error:
//
//	res, err := ws.Exec(ctx, "make test")
//	if err != nil {
//		return err
//	}
//	if err := res.Err(); err != nil {
//		return err
//	}
func (r *ExecResult) Err() error {
	if r == nil || r.ExitCode == 0 {
		return nil
	}
	return &ExitError{
		Command:  r.Command,
		ExitCode: r.ExitCode,
		Stderr:   r.Stderr,
	}
}

const defaultMaxCaptureBytes = 4 << 20

type execConfig struct {
	workingDir string
	envVars    []*cordiumv1.ExecRequest_Request_EnvVar
	runAsRoot  bool

	stdin               io.Reader
	terminateOnStdinEOF bool

	stdout   io.Writer
	stderr   io.Writer
	combined io.Writer

	capture        bool
	maxCaptureSize int

	timeout    time.Duration
	bufferSize int

	streaming bool
}

// ExecOption configures the execution of a command inside a Workspace.
type ExecOption func(*execConfig) error

func newExecConfig(opts ...ExecOption) (*execConfig, error) {
	ret := &execConfig{
		capture:        true,
		maxCaptureSize: defaultMaxCaptureBytes,
		bufferSize:     defaultOutputBuffer,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(ret); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// WithWorkingDir runs the command in a specific directory inside the Workspace
// (e.g. "/workspace/repo"). It defaults to the Workspace User's home directory.
func WithWorkingDir(dir string) ExecOption {
	return func(c *execConfig) error {
		c.workingDir = dir
		return nil
	}
}

// WithExecEnv sets an environment variable for the command alone.
func WithExecEnv(key, value string) ExecOption {
	return func(c *execConfig) error {
		if key == "" {
			return invalidArgumentf("empty environment variable name")
		}
		c.envVars = append(c.envVars, &cordiumv1.ExecRequest_Request_EnvVar{
			Key:   key,
			Value: value,
		})
		return nil
	}
}

// WithExecEnvs sets a set of environment variables for the command alone. They
// are applied in a stable order.
func WithExecEnvs(envs map[string]string) ExecOption {
	return func(c *execConfig) error {
		for _, key := range sortedKeys(envs) {
			if err := WithExecEnv(key, envs[key])(c); err != nil {
				return err
			}
		}
		return nil
	}
}

// AsRoot runs the command as root instead of as the Workspace User.
func AsRoot() ExecOption {
	return func(c *execConfig) error {
		c.runAsRoot = true
		return nil
	}
}

// WithStdin streams a reader into the command's standard input.
//
// Cordium has no way to half-close a command's standard input, so a command
// that reads until end of file (e.g. cat or psql) does not see the input end on
// its own. [WithTerminateOnStdinEOF] covers that case.
func WithStdin(r io.Reader) ExecOption {
	return func(c *execConfig) error {
		if r == nil {
			return invalidArgumentf("nil stdin reader")
		}
		c.stdin = r
		return nil
	}
}

// WithStdinString streams a string into the command's standard input.
func WithStdinString(s string) ExecOption {
	return WithStdin(strings.NewReader(s))
}

// WithStdinBytes streams a byte slice into the command's standard input.
func WithStdinBytes(b []byte) ExecOption {
	return WithStdin(bytes.NewReader(b))
}

// WithTerminateOnStdinEOF terminates the command once its standard input source
// is exhausted.
//
// Cordium's execution protocol carries no half-close for standard input, so a
// command that reads until end of file would otherwise wait forever. Its
// standard input is closed as the command is terminated, which lets the command
// flush and exit, but a command that needs to do substantial work after
// consuming its input should not be run this way.
func WithTerminateOnStdinEOF() ExecOption {
	return func(c *execConfig) error {
		c.terminateOnStdinEOF = true
		return nil
	}
}

// WithStdout streams the command's standard output into a writer as it is
// produced. It composes with the capture, which stays on unless
// [WithoutCapture] turns it off.
func WithStdout(w io.Writer) ExecOption {
	return func(c *execConfig) error {
		if w == nil {
			return invalidArgumentf("nil stdout writer")
		}
		c.stdout = w
		return nil
	}
}

// WithStderr streams the command's standard error into a writer as it is
// produced.
func WithStderr(w io.Writer) ExecOption {
	return func(c *execConfig) error {
		if w == nil {
			return invalidArgumentf("nil stderr writer")
		}
		c.stderr = w
		return nil
	}
}

// WithCombinedOutput streams both of the command's output streams into a single
// writer, interleaved in the order in which they were produced.
func WithCombinedOutput(w io.Writer) ExecOption {
	return func(c *execConfig) error {
		if w == nil {
			return invalidArgumentf("nil combined output writer")
		}
		c.combined = w
		return nil
	}
}

// WithMaxCaptureBytes caps how much of each captured stream is kept in memory.
// Once the cap is reached the rest is dropped and the result is marked as
// truncated. It defaults to 4 MiB.
func WithMaxCaptureBytes(n int) ExecOption {
	return func(c *execConfig) error {
		if n < 0 {
			return invalidArgumentf("negative capture limit")
		}
		c.maxCaptureSize = n
		return nil
	}
}

// WithoutCapture turns the in-memory capture off, which is what a long running
// or very chatty command that is already being streamed elsewhere wants.
func WithoutCapture() ExecOption {
	return func(c *execConfig) error {
		c.capture = false
		return nil
	}
}

// WithExecTimeout bounds the execution. Once it expires the command is
// terminated and the caller gets [context.DeadlineExceeded].
func WithExecTimeout(timeout time.Duration) ExecOption {
	return func(c *execConfig) error {
		if timeout < 0 {
			return invalidArgumentf("negative exec timeout")
		}
		c.timeout = timeout
		return nil
	}
}

// WithOutputBuffer sizes the channel that [ExecSession.Output] returns. A
// larger buffer absorbs bursts of output before the command is held back by the
// consumer. It defaults to 64 chunks.
func WithOutputBuffer(size int) ExecOption {
	return func(c *execConfig) error {
		if size < 0 {
			return invalidArgumentf("negative output buffer size")
		}
		c.bufferSize = size
		return nil
	}
}

// Exec runs a command inside the Workspace, waits for it to exit and returns
// its captured output along with its exit status code:
//
//	res, err := ws.Exec(ctx, "go test ./...", cordium.WithWorkingDir("/workspace/repo"))
//	if err != nil {
//		return err
//	}
//	fmt.Println(res.ExitCode, res.StdoutString())
//
// The command is interpreted by the Workspace's shell, so pipes, redirections
// and the other shell constructs work. Use [Argv] to build a command out of
// arguments that must not be interpreted.
//
// A command that exits with a non-zero status code is not an error: err only
// reports a failure to run it at all. Use [ExecResult.Err] to turn an
// unsuccessful command into an error.
//
// The Workspace must be in the PREPARING or the RUNNING state.
func (w *Workspace) Exec(ctx context.Context, command string, opts ...ExecOption) (*ExecResult, error) {
	sess, err := w.startExec(ctx, command, false, opts...)
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	return sess.Wait()
}

// ExecStream runs a command inside the Workspace and returns as soon as it has
// started, so that its output can be consumed while it runs:
//
//	sess, err := ws.ExecStream(ctx, "npm run build")
//	if err != nil {
//		return err
//	}
//	defer sess.Close()
//
//	for out := range sess.Output() {
//		fmt.Printf("[%s] %s", out.Stream, out.Data)
//	}
//
//	res, err := sess.Wait()
//
// The output channel must be drained, otherwise the command is held back by the
// consumer and eventually stalls. [ExecSession.Wait] drains it on the caller's
// behalf only when [ExecSession.Output] was never called.
//
// The session also carries the command's standard input: [ExecSession.Write]
// writes to it and [ExecSession.Kill] terminates the command.
func (w *Workspace) ExecStream(ctx context.Context, command string, opts ...ExecOption) (*ExecSession, error) {
	return w.startExec(ctx, command, true, opts...)
}

func (w *Workspace) startExec(ctx context.Context, command string,
	streaming bool, opts ...ExecOption) (*ExecSession, error) {

	if err := w.c.ensureOpen(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(command) == "" {
		return nil, invalidArgumentf("empty command")
	}

	cfg, err := newExecConfig(opts...)
	if err != nil {
		return nil, err
	}
	cfg.streaming = streaming

	var cancel context.CancelFunc
	if cfg.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	strm, err := w.c.WorkspaceService().Exec(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	sess := &ExecSession{
		cfg:       cfg,
		command:   command,
		strm:      strm,
		ctx:       ctx,
		cancel:    cancel,
		output:    make(chan ExecOutput, cfg.bufferSize),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		exitCode:  -1,
	}

	if cfg.capture {
		sess.stdoutBuf = newCaptureBuffer(cfg.maxCaptureSize)
		sess.stderrBuf = newCaptureBuffer(cfg.maxCaptureSize)
		sess.combinedBuf = newCaptureBuffer(cfg.maxCaptureSize)
	}

	if err := sess.send(&cordiumv1.ExecRequest{
		Type: &cordiumv1.ExecRequest_Request_{
			Request: &cordiumv1.ExecRequest_Request{
				WorkspaceRef: w.ref(),
				Command:      command,
				WorkingDir:   cfg.workingDir,
				EnvVars:      cfg.envVars,
				RunAsRoot:    cfg.runAsRoot,
				HasStdin:     cfg.stdin != nil || streaming,
			},
		},
	}); err != nil {
		cancel()
		return nil, err
	}

	go sess.receiveLoop()
	if cfg.stdin != nil {
		go sess.stdinLoop()
	}

	return sess, nil
}

// ExecSession is a command that is running inside a Workspace. It is created by
// [Workspace.ExecStream] and it must be closed once it is no longer needed.
type ExecSession struct {
	cfg     *execConfig
	command string
	strm    grpc.BidiStreamingClient[cordiumv1.ExecRequest, cordiumv1.ExecResponse]

	ctx    context.Context
	cancel context.CancelFunc

	output chan ExecOutput
	done   chan struct{}

	sendMu sync.Mutex

	outputTaken atomic.Bool
	killed      atomic.Bool

	stdoutBuf   *captureBuffer
	stderrBuf   *captureBuffer
	combinedBuf *captureBuffer

	startedAt time.Time

	finishOnce sync.Once
	mu         sync.Mutex
	exitCode   int
	exited     bool
	err        error
	finishedAt time.Time
}

// Command returns the command that the session runs.
func (s *ExecSession) Command() string { return s.command }

// Output returns the channel on which the command's output is published, in the
// order in which it was produced. The channel is closed once the command's
// output is complete, at which point [ExecSession.Wait] returns immediately.
//
// It only carries data for the sessions that [Workspace.ExecStream] created;
// for a plain [Workspace.Exec] it is closed without ever yielding a chunk.
func (s *ExecSession) Output() <-chan ExecOutput {
	s.outputTaken.Store(true)
	return s.output
}

// Done returns a channel that is closed once the command has exited and its
// result is available.
func (s *ExecSession) Done() <-chan struct{} { return s.done }

// Write writes to the command's standard input. It fails once the command has
// exited.
func (s *ExecSession) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	select {
	case <-s.done:
		return 0, ErrExecNotRunning
	default:
	}

	if err := s.send(&cordiumv1.ExecRequest{
		Type: &cordiumv1.ExecRequest_WriteData_{
			WriteData: &cordiumv1.ExecRequest_WriteData{Data: p},
		},
	}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteString writes a string to the command's standard input.
func (s *ExecSession) WriteString(str string) (int, error) {
	return s.Write([]byte(str))
}

// Kill terminates the running command and its process group. The command's
// standard input is closed as part of it, so a command that is waiting on its
// input gets the chance to flush and exit.
//
// A killed command is reported with the exit code -1 and with
// [ExecResult.Killed] set.
func (s *ExecSession) Kill() error {
	select {
	case <-s.done:
		return nil
	default:
	}

	s.killed.Store(true)
	return s.send(&cordiumv1.ExecRequest{
		Type: &cordiumv1.ExecRequest_Kill_{Kill: &cordiumv1.ExecRequest_Kill{}},
	})
}

// Wait blocks until the command has exited and returns its result.
//
// It is safe to call it more than once and from several goroutines: every
// caller gets the same result.
func (s *ExecSession) Wait() (*ExecResult, error) {
	// Safety net: a caller that never consumed the output would otherwise hold
	// the command back forever.
	if s.cfg.streaming && !s.outputTaken.Load() {
		for range s.output {
		}
	}

	<-s.done

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return nil, s.err
	}

	ret := &ExecResult{
		Command:    s.command,
		ExitCode:   s.exitCode,
		Killed:     s.killed.Load(),
		StartedAt:  s.startedAt,
		FinishedAt: s.finishedAt,
	}
	if s.stdoutBuf != nil {
		ret.Stdout = s.stdoutBuf.bytes()
		ret.Stderr = s.stderrBuf.bytes()
		ret.Combined = s.combinedBuf.bytes()
		ret.Truncated = s.stdoutBuf.truncated || s.stderrBuf.truncated || s.combinedBuf.truncated
	}
	return ret, nil
}

// Close releases the session. It terminates the command if it is still running.
// It is safe to call it more than once.
func (s *ExecSession) Close() error {
	s.cancel()
	return nil
}

func (s *ExecSession) send(req *cordiumv1.ExecRequest) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if err := s.strm.Send(req); err != nil {
		if errors.Is(err, io.EOF) {
			// The server closed the stream; the real error, if any, surfaces on
			// the receiving side.
			return ErrExecNotRunning
		}
		return err
	}
	return nil
}

func (s *ExecSession) receiveLoop() {
	defer func() {
		close(s.output)
		s.finish()
		// The Cluster keeps the execution stream open until the client lets it
		// go, so releasing it here is what frees the Workspace side resources.
		s.cancel()
	}()

	for {
		msg, err := s.strm.Recv()
		if err != nil {
			s.setStreamErr(err)
			return
		}

		switch msg.GetType().(type) {
		case *cordiumv1.ExecResponse_Stdout_:
			if !s.deliver(StreamStdout, msg.GetStdout().GetData()) {
				return
			}
		case *cordiumv1.ExecResponse_Stderr_:
			if !s.deliver(StreamStderr, msg.GetStderr().GetData()) {
				return
			}
		case *cordiumv1.ExecResponse_Exit_:
			s.setExit(int(msg.GetExit().GetCode()))
			return
		}
	}
}

// deliver fans a chunk out to the writers, the capture and the output channel.
// It returns false once the session is being torn down.
func (s *ExecSession) deliver(stream Stream, data []byte) bool {
	if len(data) == 0 {
		return true
	}

	var writer io.Writer
	var buf *captureBuffer
	switch stream {
	case StreamStdout:
		writer, buf = s.cfg.stdout, s.stdoutBuf
	default:
		writer, buf = s.cfg.stderr, s.stderrBuf
	}

	if writer != nil {
		if _, err := writer.Write(data); err != nil {
			s.setErr(err)
			return false
		}
	}
	if s.cfg.combined != nil {
		if _, err := s.cfg.combined.Write(data); err != nil {
			s.setErr(err)
			return false
		}
	}
	if buf != nil {
		buf.write(data)
		s.combinedBuf.write(data)
	}

	if !s.cfg.streaming {
		return true
	}

	select {
	case s.output <- ExecOutput{Stream: stream, Data: data}:
		return true
	case <-s.ctx.Done():
		s.setErr(s.ctx.Err())
		return false
	}
}

func (s *ExecSession) stdinLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.cfg.stdin.Read(buf)
		if n > 0 {
			if _, werr := s.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) && s.cfg.terminateOnStdinEOF {
				_ = s.Kill()
			}
			return
		}

		select {
		case <-s.done:
			return
		default:
		}
	}
}

func (s *ExecSession) setExit(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.exited {
		s.exited = true
		s.exitCode = code
		s.finishedAt = time.Now()
	}
}

func (s *ExecSession) setErr(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.exited && s.err == nil {
		s.err = err
		s.finishedAt = time.Now()
	}
}

func (s *ExecSession) setStreamErr(err error) {
	s.mu.Lock()
	exited := s.exited
	s.mu.Unlock()
	if exited {
		return
	}

	// A killed command may never get to report its exit code, since terminating
	// it also tears the Workspace side of the stream down.
	if s.killed.Load() {
		s.setExit(-1)
		return
	}

	// A canceled context and an expired timeout are reported as such, so that
	// the caller can tell them apart from a Cluster side failure.
	if ctxErr := s.ctx.Err(); ctxErr != nil {
		s.setErr(ctxErr)
		return
	}

	if normalized := streamErr(s.ctx, err); normalized != nil {
		s.setErr(normalized)
		return
	}

	s.setErr(errors.New("cordium: the exec stream ended before the command exited"))
}

func (s *ExecSession) finish() {
	s.finishOnce.Do(func() {
		s.mu.Lock()
		if s.finishedAt.IsZero() {
			s.finishedAt = time.Now()
		}
		if !s.exited && s.err == nil {
			s.err = errors.New("cordium: the exec stream ended before the command exited")
		}
		s.mu.Unlock()
		close(s.done)
	})
}

// captureBuffer keeps at most a fixed amount of a stream in memory.
type captureBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCaptureBuffer(limit int) *captureBuffer {
	return &captureBuffer{limit: limit}
}

func (b *captureBuffer) write(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limit > 0 {
		remaining := b.limit - b.buf.Len()
		if remaining <= 0 {
			b.truncated = true
			return
		}
		if len(data) > remaining {
			data = data[:remaining]
			b.truncated = true
		}
	}
	b.buf.Write(data)
}

func (b *captureBuffer) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() == 0 {
		return nil
	}
	return append([]byte(nil), b.buf.Bytes()...)
}
