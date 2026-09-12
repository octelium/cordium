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

package harness

import (
	"context"
	"io"
	"strings"
	"testing"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type ExecOpts struct {
	Command    string
	WorkingDir string
	EnvVars    map[string]string
	RunAsRoot  bool
	Stdin      string

	Conn *grpc.ClientConn
}

type ExecResult struct {
	Stdout string
	Stderr string
	Code   int32
}

func (r *ExecResult) Out() string {
	return strings.TrimSpace(r.Stdout)
}

func (h *H) Exec(t *testing.T, ws *cordiumv1.Workspace, o ExecOpts) *ExecResult {
	t.Helper()

	ret, err := h.ExecErr(t, ws, o)
	if err != nil {
		t.Fatalf("Could not exec the command %q in the Workspace %s: %+v",
			o.Command, ws.Metadata.Name, err)
	}

	return ret
}

func (h *H) ExecErr(t *testing.T, ws *cordiumv1.Workspace, o ExecOpts) (*ExecResult, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), ExecBudget)
	defer cancel()

	conn := o.Conn
	if conn == nil {
		conn = h.Conn()
	}

	strm, err := cordiumv1.NewWorkspaceServiceClient(conn).Exec(ctx, grpc_retry.Disable())
	if err != nil {
		return nil, err
	}

	if err := strm.Send(&cordiumv1.ExecRequest{
		Type: &cordiumv1.ExecRequest_Request_{
			Request: &cordiumv1.ExecRequest_Request{
				WorkspaceRef: umetav1.GetObjectReference(ws),
				Command:      o.Command,
				WorkingDir:   o.WorkingDir,
				EnvVars:      execEnvVars(o.EnvVars),
				RunAsRoot:    o.RunAsRoot,
				HasStdin:     o.Stdin != "",
			},
		},
	}); err != nil {
		return nil, err
	}

	if o.Stdin != "" {
		if err := strm.Send(&cordiumv1.ExecRequest{
			Type: &cordiumv1.ExecRequest_WriteData_{
				WriteData: &cordiumv1.ExecRequest_WriteData{
					Data: []byte(o.Stdin),
				},
			},
		}); err != nil {
			return nil, err
		}
	}

	ret := &ExecResult{}

	for {
		msg, err := strm.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.Errorf(
					"The exec stream ended without an exit code. stdout: %q stderr: %q",
					ret.Stdout, ret.Stderr)
			}
			return nil, err
		}

		switch msg.Type.(type) {
		case *cordiumv1.ExecResponse_Stdout_:
			ret.Stdout = ret.Stdout + string(msg.GetStdout().Data)
		case *cordiumv1.ExecResponse_Stderr_:
			ret.Stderr = ret.Stderr + string(msg.GetStderr().Data)
		case *cordiumv1.ExecResponse_Exit_:
			ret.Code = msg.GetExit().Code

			zap.L().Debug("Exec done",
				zap.String("workspace", ws.Metadata.Name),
				zap.String("cmd", o.Command),
				zap.Int32("code", ret.Code),
				zap.String("stdout", ret.Stdout),
				zap.String("stderr", ret.Stderr))

			return ret, nil
		}
	}
}

func (h *H) MustExec(t *testing.T, ws *cordiumv1.Workspace, cmd string) string {
	t.Helper()

	ret := h.Exec(t, ws, ExecOpts{Command: cmd})
	if ret.Code != 0 {
		t.Fatalf("The command %q in the Workspace %s exited with %d. stdout: %q stderr: %q",
			cmd, ws.Metadata.Name, ret.Code, ret.Stdout, ret.Stderr)
	}

	return ret.Out()
}

func execEnvVars(envVars map[string]string) []*cordiumv1.ExecRequest_Request_EnvVar {
	var ret []*cordiumv1.ExecRequest_Request_EnvVar

	for k, v := range envVars {
		ret = append(ret, &cordiumv1.ExecRequest_Request_EnvVar{
			Key:   k,
			Value: v,
		})
	}

	return ret
}
