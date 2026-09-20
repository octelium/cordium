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
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// defaultMaxReadFileBytes caps what ReadFile is willing to hold in memory.
const defaultMaxReadFileBytes = 64 << 20

// The file transfer helpers move data through the Workspace's shell, base64
// encoded, over the same execution stream as any other command. They therefore
// need `base64` and `head` inside the Workspace, which every standard base
// image provides through either coreutils or busybox.
//
// They suit source files, patches, configuration and build artifacts. Bulk data
// belongs in a repository, an object store or an SSH/SFTP transfer.

// WriteFile writes data to a file inside the Workspace, creating the file's
// parent directories and replacing the file if it already exists.
//
//	err := ws.WriteFile(ctx, "/workspace/repo/config.json", payload)
//
// The exec options apply, so [AsRoot] writes as root and [WithExecTimeout]
// bounds the transfer.
func (w *Workspace) WriteFile(ctx context.Context, remotePath string, data []byte, opts ...ExecOption) error {
	return w.upload(ctx, remotePath, int64(len(data)), bytes.NewReader(data), opts...)
}

// WriteFileString writes a string to a file inside the Workspace.
func (w *Workspace) WriteFileString(ctx context.Context, remotePath, content string, opts ...ExecOption) error {
	return w.WriteFile(ctx, remotePath, []byte(content), opts...)
}

// UploadFile copies a local file into the Workspace. It streams the file rather
// than holding it in memory, so it is what larger files should use.
//
// The local file must not change while it is being uploaded.
func (w *Workspace) UploadFile(ctx context.Context, localPath, remotePath string, opts ...ExecOption) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return invalidArgumentf("%q is a directory; upload its files one by one", localPath)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return w.upload(ctx, remotePath, info.Size(), file, opts...)
}

func (w *Workspace) upload(ctx context.Context, remotePath string,
	size int64, src io.Reader, opts ...ExecOption) error {

	if remotePath == "" {
		return invalidArgumentf("empty file path")
	}
	if size < 0 {
		return invalidArgumentf("negative file size")
	}
	if size > int64(maxInt/2) {
		return invalidArgumentf("the file is too large to upload: %d bytes", size)
	}

	// The command reads exactly as many base64 bytes as the content encodes to,
	// which is what lets the remote shell finish on its own: Cordium's exec
	// protocol has no way to close a command's standard input.
	encodedLen := int64(base64.StdEncoding.EncodedLen(int(size)))

	command := fmt.Sprintf("head -c %s | base64 -d > %s",
		strconv.FormatInt(encodedLen, 10), ShellQuote(remotePath))
	if dir := path.Dir(remotePath); dir != "." && dir != "/" && dir != "" {
		command = fmt.Sprintf("mkdir -p %s && %s", ShellQuote(dir), command)
	}

	reader, writer := io.Pipe()
	copied := make(chan int64, 1)

	go func() {
		encoder := base64.NewEncoder(base64.StdEncoding, writer)
		n, err := io.Copy(encoder, io.LimitReader(src, size))
		if err == nil {
			err = encoder.Close()
		}
		copied <- n
		_ = writer.CloseWithError(err)
	}()

	// The transfer's own exec options come last so that they always win: the
	// standard input has to be the encoder's pipe, and the capture only ever
	// serves to explain a failure.
	res, err := w.Exec(ctx, command,
		append(append([]ExecOption{}, opts...),
			WithStdin(reader),
			WithMaxCaptureBytes(maxTransferDiagnosticBytes),
		)...)
	_ = reader.Close()
	if err != nil {
		return err
	}

	if n := <-copied; n != size {
		return fmt.Errorf("cordium: could not write %s: the source held %d bytes instead of %d",
			remotePath, n, size)
	}

	if res.ExitCode != 0 {
		return transferError("write", remotePath, res)
	}
	return nil
}

// ReadFile reads a file out of the Workspace and returns its content.
//
// It holds the file in memory, capped at 64 MiB by default;
// [WithMaxCaptureBytes] changes the cap and [Workspace.DownloadFile] avoids the
// buffering altogether.
func (w *Workspace) ReadFile(ctx context.Context, remotePath string, opts ...ExecOption) ([]byte, error) {
	cfg, err := newExecConfig(opts...)
	if err != nil {
		return nil, err
	}

	limit := cfg.maxCaptureSize
	if limit == defaultMaxCaptureBytes {
		limit = defaultMaxReadFileBytes
	}

	var buf bytes.Buffer
	if err := w.download(ctx, remotePath, &limitedWriter{w: &buf, remaining: limit}, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReadFileString reads a file out of the Workspace and returns it as a string.
func (w *Workspace) ReadFileString(ctx context.Context, remotePath string, opts ...ExecOption) (string, error) {
	data, err := w.ReadFile(ctx, remotePath, opts...)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DownloadFile copies a file out of the Workspace into a local path. It streams
// the file rather than holding it in memory.
func (w *Workspace) DownloadFile(ctx context.Context, remotePath, localPath string, opts ...ExecOption) error {
	if localPath == "" {
		return invalidArgumentf("empty local file path")
	}

	if dir := localDir(localPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}

	if err := w.download(ctx, remotePath, file, opts...); err != nil {
		_ = file.Close()
		_ = os.Remove(localPath)
		return err
	}
	return file.Close()
}

// DownloadFileTo copies a file out of the Workspace into a writer.
func (w *Workspace) DownloadFileTo(ctx context.Context, remotePath string,
	dst io.Writer, opts ...ExecOption) error {

	if dst == nil {
		return invalidArgumentf("nil destination writer")
	}
	return w.download(ctx, remotePath, dst, opts...)
}

func (w *Workspace) download(ctx context.Context, remotePath string,
	dst io.Writer, opts ...ExecOption) error {

	if remotePath == "" {
		return invalidArgumentf("empty file path")
	}

	reader, writer := io.Pipe()
	decoded := make(chan error, 1)

	go func() {
		_, err := io.Copy(dst, base64.NewDecoder(base64.StdEncoding, reader))
		// Draining the rest keeps the exec from stalling on a short write.
		_, _ = io.Copy(io.Discard, reader)
		decoded <- err
	}()

	// The redirection, rather than an argument, keeps the paths that start with
	// a dash out of base64's own option parsing.
	res, execErr := w.Exec(ctx, "base64 < "+ShellQuote(remotePath),
		append(append([]ExecOption{}, opts...),
			WithStdout(writer),
			WithMaxCaptureBytes(maxTransferDiagnosticBytes),
		)...)

	_ = writer.Close()
	decodeErr := <-decoded

	switch {
	case execErr != nil:
		return execErr
	case res.ExitCode != 0:
		return transferError("read", remotePath, res)
	case decodeErr != nil:
		return fmt.Errorf("cordium: could not read %s: %w", remotePath, decodeErr)
	default:
		return nil
	}
}

// maxTransferDiagnosticBytes is how much of a transfer command's own output is
// kept, which only ever serves to explain a failure.
const maxTransferDiagnosticBytes = 8 << 10

func transferError(verb, remotePath string, res *ExecResult) error {
	message := strings.TrimSpace(res.StderrString())
	if message == "" {
		message = fmt.Sprintf("the command exited with code %d", res.ExitCode)
	}
	return fmt.Errorf("cordium: could not %s %s: %s", verb, remotePath, message)
}

// limitedWriter fails once more than remaining bytes are written to it, which
// is how ReadFile refuses to buffer a file that is larger than its cap.
type limitedWriter struct {
	w         io.Writer
	remaining int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > l.remaining {
		return 0, fmt.Errorf("the file is larger than the %d byte limit; "+
			"raise it with WithMaxCaptureBytes or use DownloadFile", l.remaining)
	}
	n, err := l.w.Write(p)
	l.remaining -= n
	return n, err
}

// localDir is the directory component of a local path, or an empty string when
// there is nothing to create.
func localDir(localPath string) string {
	dir := filepath.Dir(localPath)
	if dir == "." || dir == localPath || dir == string(filepath.Separator) {
		return ""
	}
	return dir
}

const maxInt = int64(^uint(0) >> 1)
