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
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"google.golang.org/grpc"
)

func stdout(data string) *cordiumv1.ExecResponse {
	return &cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Stdout_{
			Stdout: &cordiumv1.ExecResponse_Stdout{Data: []byte(data)},
		},
	}
}

func stderr(data string) *cordiumv1.ExecResponse {
	return &cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Stderr_{
			Stderr: &cordiumv1.ExecResponse_Stderr{Data: []byte(data)},
		},
	}
}

func exit(code int32) *cordiumv1.ExecResponse {
	return &cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Exit_{
			Exit: &cordiumv1.ExecResponse_Exit{Code: code},
		},
	}
}

func testWorkspace(t *testing.T, c *Client) *Workspace {
	t.Helper()
	ws, err := c.Workspaces().Create(t.Context())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return ws
}

func TestExecCapturesOutputAndExitCode(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		for _, msg := range []*cordiumv1.ExecResponse{
			stdout("hello "), stderr("warning"), stdout("world\n"), exit(3),
		} {
			if err := srv.Send(msg); err != nil {
				return err
			}
		}
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	res, err := ws.Exec(t.Context(), "echo hello world",
		WithWorkingDir("/workspace/repo"),
		WithExecEnv("FOO", "bar"),
		AsRoot())
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got, want := res.StdoutString(), "hello world\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got, want := res.StderrString(), "warning"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if got, want := res.CombinedString(), "hello warningworld\n"; got != want {
		t.Errorf("combined = %q, want %q", got, want)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", res.ExitCode)
	}
	if res.Success() {
		t.Error("Success() = true for a non-zero exit code")
	}

	var exitErr *ExitError
	if err := res.Err(); !errors.As(err, &exitErr) {
		t.Fatalf("Err() = %v, want an *ExitError", err)
	}
	if exitErr.ExitCode != 3 {
		t.Errorf("ExitError.ExitCode = %d, want 3", exitErr.ExitCode)
	}

	if res.Duration() <= 0 {
		t.Error("Duration() is not positive")
	}

	fake.mu.Lock()
	req := fake.lastExecReq
	fake.mu.Unlock()

	if req.GetWorkingDir() != "/workspace/repo" {
		t.Errorf("workingDir = %q", req.GetWorkingDir())
	}
	if !req.GetRunAsRoot() {
		t.Error("runAsRoot was not forwarded")
	}
	if len(req.GetEnvVars()) != 1 || req.GetEnvVars()[0].GetKey() != "FOO" {
		t.Errorf("envVars = %v", req.GetEnvVars())
	}
	if req.GetWorkspaceRef().GetName() != ws.Name() {
		t.Errorf("workspaceRef = %v", req.GetWorkspaceRef())
	}
}

func TestExecStreamsToWriters(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		_ = srv.Send(stdout("out"))
		_ = srv.Send(stderr("err"))
		_ = srv.Send(exit(0))
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	var outBuf, errBuf, combinedBuf bytes.Buffer
	res, err := ws.Exec(t.Context(), "run",
		WithStdout(&outBuf), WithStderr(&errBuf), WithCombinedOutput(&combinedBuf),
		WithoutCapture())
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if outBuf.String() != "out" || errBuf.String() != "err" {
		t.Errorf("writers got %q / %q", outBuf.String(), errBuf.String())
	}
	if combinedBuf.String() != "outerr" {
		t.Errorf("combined writer got %q", combinedBuf.String())
	}
	if res.Stdout != nil || res.Stderr != nil {
		t.Error("WithoutCapture still captured the output")
	}
	if !res.Success() {
		t.Errorf("exit code = %d", res.ExitCode)
	}
}

func TestExecTruncatesCapture(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		_ = srv.Send(stdout(strings.Repeat("x", 100)))
		_ = srv.Send(exit(0))
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	res, err := ws.Exec(t.Context(), "spew", WithMaxCaptureBytes(10))
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(res.Stdout) != 10 {
		t.Errorf("captured %d bytes, want 10", len(res.Stdout))
	}
	if !res.Truncated {
		t.Error("Truncated was not set")
	}
}

func TestExecStreamOrdersOutput(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		_ = srv.Send(stdout("one"))
		_ = srv.Send(stderr("two"))
		_ = srv.Send(stdout("three"))
		_ = srv.Send(exit(0))
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	sess, err := ws.ExecStream(t.Context(), "run")
	if err != nil {
		t.Fatalf("ExecStream: %v", err)
	}
	defer sess.Close()

	var got []string
	for out := range sess.Output() {
		got = append(got, out.Stream.String()+":"+out.String())
	}

	want := []string{"stdout:one", "stderr:two", "stdout:three"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("output = %v, want %v", got, want)
	}

	res, err := sess.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !res.Success() {
		t.Errorf("exit code = %d", res.ExitCode)
	}
}

// TestExecStreamWaitWithoutDraining covers the safety net that keeps a caller
// who never consumed Output from deadlocking.
func TestExecStreamWaitWithoutDraining(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		for range 500 {
			if err := srv.Send(stdout(strings.Repeat("y", 1024))); err != nil {
				return err
			}
		}
		_ = srv.Send(exit(0))
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	sess, err := ws.ExecStream(t.Context(), "spew")
	if err != nil {
		t.Fatalf("ExecStream: %v", err)
	}
	defer sess.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := sess.Wait(); err != nil {
			t.Errorf("Wait: %v", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Wait deadlocked without a consumer of Output")
	}
}

func TestExecStdinAndKill(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		if !req.GetHasStdin() {
			t.Error("hasStdin was not set")
		}
		for {
			msg, err := srv.Recv()
			if err != nil {
				return err
			}
			switch msg.GetType().(type) {
			case *cordiumv1.ExecRequest_WriteData_:
				if err := srv.Send(stdout(string(msg.GetWriteData().GetData()))); err != nil {
					return err
				}
			case *cordiumv1.ExecRequest_Kill_:
				// A killed command reaches the client as a terminated stream
				// rather than as an exit code.
				return nil
			}
		}
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	sess, err := ws.ExecStream(t.Context(), "cat")
	if err != nil {
		t.Fatalf("ExecStream: %v", err)
	}
	defer sess.Close()

	if _, err := sess.WriteString("ping"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}

	out, ok := <-sess.Output()
	if !ok || out.String() != "ping" {
		t.Fatalf("echoed output = %q (ok=%v)", out.String(), ok)
	}

	if err := sess.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	res, err := sess.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !res.Killed {
		t.Error("Killed was not set")
	}
	if res.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1", res.ExitCode)
	}
}

func TestExecTerminateOnStdinEOF(t *testing.T) {
	killed := make(chan struct{})
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		var received []byte
		for {
			msg, err := srv.Recv()
			if err != nil {
				return err
			}
			switch msg.GetType().(type) {
			case *cordiumv1.ExecRequest_WriteData_:
				received = append(received, msg.GetWriteData().GetData()...)
			case *cordiumv1.ExecRequest_Kill_:
				close(killed)
				_ = srv.Send(stdout(string(received)))
				_ = srv.Send(exit(0))
				<-srv.Context().Done()
				return nil
			}
		}
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	res, err := ws.Exec(t.Context(), "cat",
		WithStdinString("piped input"),
		WithTerminateOnStdinEOF())
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	select {
	case <-killed:
	default:
		t.Fatal("the command was not terminated once stdin was exhausted")
	}

	if got := res.StdoutString(); got != "piped input" {
		t.Errorf("stdout = %q", got)
	}
}

func TestExecStreamEndingWithoutExitIsAnError(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		_ = srv.Send(stdout("partial"))
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	if _, err := ws.Exec(t.Context(), "run"); err == nil {
		t.Fatal("Exec succeeded although the command never exited")
	}
}

func TestExecRejectsEmptyCommand(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())
	ws := testWorkspace(t, c)

	_, err := ws.Exec(t.Context(), "   ")
	if !IsInvalidArgument(err) {
		t.Fatalf("err = %v, want an invalid argument error", err)
	}
}

func TestExecTimeout(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	start := time.Now()
	_, err := ws.Exec(t.Context(), "sleep 3600", WithExecTimeout(300*time.Millisecond))
	if !IsDeadlineExceeded(err) {
		t.Fatalf("err = %v, want a deadline error", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("the timeout took %v to fire", elapsed)
	}
}

func TestExecReportsContextCancellation(t *testing.T) {
	fake := newFakeCluster()
	fake.execScript = func(req *cordiumv1.ExecRequest_Request,
		srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
		<-srv.Context().Done()
		return nil
	}

	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	if _, err := ws.Exec(ctx, "sleep 3600"); !IsCanceled(err) {
		t.Fatalf("err = %v, want a cancellation error", err)
	}
}

func TestArgvQuoting(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"ls", "-la"}, "ls -la"},
		{[]string{"echo", "a b"}, "echo 'a b'"},
		{[]string{"echo", "it's"}, `echo 'it'\''s'`},
		{[]string{"echo", ""}, "echo ''"},
		{[]string{"rm", "-rf", "/tmp/x; rm -rf /"}, "rm -rf '/tmp/x; rm -rf /'"},
		{[]string{"echo", "$HOME"}, "echo '$HOME'"},
	} {
		if got := Argv(tc.args...); got != tc.want {
			t.Errorf("Argv(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestExecResultHelpersOnNil(t *testing.T) {
	var res *ExecResult
	if res.Success() || res.Err() != nil || res.StdoutString() != "" || res.Duration() != 0 {
		t.Error("the nil ExecResult helpers are not safe")
	}
}

var _ io.Writer = (*ExecSession)(nil)
