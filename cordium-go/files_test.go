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
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"google.golang.org/grpc"
)

// shellFS stands in for the Workspace filesystem and interprets the two shell
// pipelines that the transfer helpers build, the way a real shell would.
type shellFS struct {
	files map[string][]byte
}

// parseUpload and parseDownload read back the pipelines that the transfer
// helpers build, and shellUnquote undoes the quoting that ShellQuote applied.
func parseUpload(cmd string) (remotePath string, want int, ok bool) {
	const marker = "| base64 -d > "
	idx := strings.LastIndex(cmd, marker)
	if idx < 0 {
		return "", 0, false
	}
	remotePath = shellUnquote(strings.TrimSpace(cmd[idx+len(marker):]))

	prefix := cmd[:idx]
	hidx := strings.LastIndex(prefix, "head -c ")
	if hidx < 0 {
		return "", 0, false
	}
	want, err := strconv.Atoi(strings.TrimSpace(prefix[hidx+len("head -c "):]))
	if err != nil {
		return "", 0, false
	}
	return remotePath, want, true
}

func parseDownload(cmd string) (remotePath string, ok bool) {
	const marker = "base64 < "
	if !strings.HasPrefix(cmd, marker) {
		return "", false
	}
	return shellUnquote(strings.TrimSpace(cmd[len(marker):])), true
}

func shellUnquote(s string) string {
	if !strings.HasPrefix(s, "'") {
		return s
	}

	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case s[i] == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				b.WriteString(s[i+1:])
				return b.String()
			}
			b.WriteString(s[i+1 : i+1+j])
			i += j + 2
		case s[i] == '\\' && i+1 < len(s):
			b.WriteByte(s[i+1])
			i += 2
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

func (s *shellFS) script(req *cordiumv1.ExecRequest_Request,
	srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {

	if path, want, ok := parseUpload(req.GetCommand()); ok {

		// `head -c N` consumes exactly N bytes and then exits on its own.
		var encoded []byte
		for len(encoded) < want {
			msg, err := srv.Recv()
			if err != nil {
				break
			}
			if data := msg.GetWriteData().GetData(); len(data) > 0 {
				encoded = append(encoded, data...)
			}
		}
		if len(encoded) > want {
			encoded = encoded[:want]
		}

		decoded, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			_ = srv.Send(stderr("base64: invalid input"))
			_ = srv.Send(exit(1))
			<-srv.Context().Done()
			return nil
		}

		s.files[path] = decoded
		_ = srv.Send(exit(0))
		<-srv.Context().Done()
		return nil
	}

	if path, ok := parseDownload(req.GetCommand()); ok {
		content, ok := s.files[path]
		if !ok {
			_ = srv.Send(stderr("sh: cannot open " + path + ": No such file or directory"))
			_ = srv.Send(exit(1))
			<-srv.Context().Done()
			return nil
		}

		// Real base64 wraps its output, which the decoder has to tolerate.
		encoded := base64.StdEncoding.EncodeToString(content)
		for i := 0; i < len(encoded); i += 76 {
			end := min(i+76, len(encoded))
			if err := srv.Send(stdout(encoded[i:end] + "\n")); err != nil {
				return err
			}
		}
		_ = srv.Send(exit(0))
		<-srv.Context().Done()
		return nil
	}

	_ = srv.Send(stderr("sh: unexpected command: " + req.GetCommand()))
	_ = srv.Send(exit(127))
	<-srv.Context().Done()
	return nil
}

func fileWorkspace(t *testing.T) (*Workspace, *shellFS) {
	t.Helper()

	fs := &shellFS{files: map[string][]byte{}}
	fake := newFakeCluster()
	fake.execScript = fs.script

	c := startFakeCluster(t, fake)
	return testWorkspace(t, c), fs
}

func TestWriteAndReadFile(t *testing.T) {
	ws, fs := fileWorkspace(t)

	content := []byte("{\n  \"debug\": true\n}\n")
	if err := ws.WriteFile(t.Context(), "/workspace/repo/config.json", content); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got := fs.files["/workspace/repo/config.json"]; !bytes.Equal(got, content) {
		t.Errorf("the Workspace holds %q, want %q", got, content)
	}

	read, err := ws.ReadFile(t.Context(), "/workspace/repo/config.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(read, content) {
		t.Errorf("ReadFile = %q, want %q", read, content)
	}

	str, err := ws.ReadFileString(t.Context(), "/workspace/repo/config.json")
	if err != nil || str != string(content) {
		t.Errorf("ReadFileString = %q, %v", str, err)
	}
}

func TestWriteFileHandlesBinaryAndSizes(t *testing.T) {
	ws, fs := fileWorkspace(t)

	for _, size := range []int{0, 1, 2, 3, 4, 1023, 4096, 300 * 1024} {
		content := make([]byte, size)
		if _, err := rand.Read(content); err != nil {
			t.Fatalf("rand: %v", err)
		}

		path := "/tmp/blob-" + strconv.Itoa(size)
		if err := ws.WriteFile(t.Context(), path, content); err != nil {
			t.Fatalf("WriteFile(%d bytes): %v", size, err)
		}
		if got := fs.files[path]; !bytes.Equal(got, content) {
			t.Fatalf("%d bytes round-tripped to %d bytes", size, len(got))
		}

		read, err := ws.ReadFile(t.Context(), path)
		if err != nil {
			t.Fatalf("ReadFile(%d bytes): %v", size, err)
		}
		if !bytes.Equal(read, content) {
			t.Fatalf("ReadFile(%d bytes) returned %d bytes", size, len(read))
		}
	}
}

func TestWriteFileQuotesPaths(t *testing.T) {
	ws, fs := fileWorkspace(t)

	path := "/workspace/my repo/it's a file.txt"
	if err := ws.WriteFileString(t.Context(), path, "hello"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if string(fs.files[path]) != "hello" {
		t.Errorf("the Workspace holds %q", fs.files[path])
	}

	read, err := ws.ReadFileString(t.Context(), path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if read != "hello" {
		t.Errorf("ReadFile = %q", read)
	}
}

func TestUploadAndDownloadFile(t *testing.T) {
	ws, fs := fileWorkspace(t)
	dir := t.TempDir()

	content := make([]byte, 200*1024)
	if _, err := rand.Read(content); err != nil {
		t.Fatalf("rand: %v", err)
	}

	local := filepath.Join(dir, "artifact.bin")
	if err := os.WriteFile(local, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := ws.UploadFile(t.Context(), local, "/workspace/artifact.bin"); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if !bytes.Equal(fs.files["/workspace/artifact.bin"], content) {
		t.Error("the uploaded file does not match")
	}

	out := filepath.Join(dir, "nested", "copy.bin")
	if err := ws.DownloadFile(t.Context(), "/workspace/artifact.bin", out); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Error("the downloaded file does not match")
	}

	var buf bytes.Buffer
	if err := ws.DownloadFileTo(t.Context(), "/workspace/artifact.bin", &buf); err != nil {
		t.Fatalf("DownloadFileTo: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("DownloadFileTo does not match")
	}
}

func TestDownloadMissingFileFails(t *testing.T) {
	ws, _ := fileWorkspace(t)

	_, err := ws.ReadFile(t.Context(), "/workspace/nope.txt")
	if err == nil {
		t.Fatal("reading a missing file succeeded")
	}
	if !strings.Contains(err.Error(), "No such file") {
		t.Errorf("err = %v, want the shell's own message", err)
	}

	// A failed download must not leave a truncated local file behind.
	out := filepath.Join(t.TempDir(), "out.bin")
	if err := ws.DownloadFile(t.Context(), "/workspace/nope.txt", out); err == nil {
		t.Fatal("DownloadFile succeeded for a missing file")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("a failed download left a local file behind")
	}
}

func TestReadFileRespectsItsSizeCap(t *testing.T) {
	ws, fs := fileWorkspace(t)
	fs.files["/big"] = bytes.Repeat([]byte("x"), 64*1024)

	if _, err := ws.ReadFile(t.Context(), "/big", WithMaxCaptureBytes(1024)); err == nil {
		t.Fatal("ReadFile ignored its size cap")
	}

	// The same file still streams through DownloadFileTo.
	var buf bytes.Buffer
	if err := ws.DownloadFileTo(t.Context(), "/big", &buf); err != nil {
		t.Fatalf("DownloadFileTo: %v", err)
	}
	if buf.Len() != 64*1024 {
		t.Errorf("streamed %d bytes", buf.Len())
	}
}

func TestFileHelpersRejectEmptyPaths(t *testing.T) {
	ws, _ := fileWorkspace(t)

	if err := ws.WriteFile(t.Context(), "", nil); !IsInvalidArgument(err) {
		t.Errorf("an empty remote path: %v", err)
	}
	if _, err := ws.ReadFile(t.Context(), ""); !IsInvalidArgument(err) {
		t.Errorf("an empty remote path: %v", err)
	}
	if err := ws.DownloadFile(t.Context(), "/x", ""); !IsInvalidArgument(err) {
		t.Errorf("an empty local path: %v", err)
	}
	if err := ws.DownloadFileTo(t.Context(), "/x", nil); !IsInvalidArgument(err) {
		t.Errorf("a nil writer: %v", err)
	}
	if err := ws.UploadFile(t.Context(), t.TempDir(), "/x"); !IsInvalidArgument(err) {
		t.Errorf("uploading a directory: %v", err)
	}
	if err := ws.UploadFile(t.Context(), filepath.Join(t.TempDir(), "nope"), "/x"); !os.IsNotExist(err) {
		t.Errorf("uploading a missing file: %v", err)
	}
}
