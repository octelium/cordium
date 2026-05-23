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

package workspace

import (
	"testing"
	"time"

	"context"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsclient"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestTerminal(t *testing.T) {

	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	defer srv.Close()

	err = srv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{})
	assert.Nil(t, err)

	wsC := ccordiumv1.NewWorkspaceServiceClient(grpcConn)

	_, err = wsC.Prepare(ctx, &ccordiumv1.PrepareRequest{
		Workspace: &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: wsutils.GenWorkspaceName(),
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		},
	})
	assert.Nil(t, err, "%+v", err)
	time.Sleep(2 * time.Second)

	termC := ccordiumv1.NewTerminalServiceClient(grpcConn)

	term, err := termC.CreateTerminal(ctx, &ccordiumv1.CreateTerminalRequest{})
	assert.Nil(t, err)

	strm, err := termC.ListenTerminal(ctx, &ccordiumv1.ListenTerminalRequest{
		Id: term.Id,
	})
	assert.Nil(t, err)

	{
		_, err = termC.WriteDataTerminal(ctx, &ccordiumv1.WriteDataTerminalRequest{
			Id:   wsutils.GenWorkspaceName(),
			Data: []byte("ls -la \r\n"),
		})
		assert.NotNil(t, err, "%+v", err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	}

	_, err = termC.WriteDataTerminal(ctx, &ccordiumv1.WriteDataTerminalRequest{
		Id:   term.Id,
		Data: []byte("ls -la \r\n"),
	})
	assert.Nil(t, err, "%+v", err)

	{
		_, err = termC.SetWindowSize(ctx, &ccordiumv1.SetWindowSizeRequest{
			Cols: 100,
			Rows: 100,
		})
		assert.NotNil(t, err)
	}

	{
		_, err = termC.SetWindowSize(ctx, &ccordiumv1.SetWindowSizeRequest{
			Id: term.Id,
		})
		assert.Nil(t, err)
	}

	{
		_, err = termC.SetWindowSize(ctx, &ccordiumv1.SetWindowSizeRequest{
			Id:   term.Id,
			Cols: 100,
			Rows: 100,
		})
		assert.Nil(t, err)
	}

	mctx, cancelFn := context.WithTimeout(ctx, 3*time.Second)
	defer cancelFn()

	go func() {
		for {
			select {
			case <-mctx.Done():
				zap.L().Debug("exiting client test stdout loop")
				return
			default:
				msg, err := strm.Recv()
				if err != nil {
					continue
				}
				zap.L().Debug("New msg", zap.Any("msg", msg))
			}
		}
	}()

	<-mctx.Done()
	err = grpcConn.Close()
	assert.Nil(t, err, "%+v", err)

	zap.S().Debugf("test ended successfully")
}

/*
func TestExec(t *testing.T) {

	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	defer srv.Close()

	err = srv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{})
	assert.Nil(t, err)

	wsC := ccordiumv1.NewWorkspaceServiceClient(grpcConn)

	_, err = wsC.Prepare(ctx, &ccordiumv1.PrepareRequest{
		Workspace: &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: wsutils.GenWorkspaceName(),
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		},
	})
	assert.Nil(t, err, "%+v", err)
	time.Sleep(2 * time.Second)

	termC := ccordiumv1.NewTerminalServiceClient(grpcConn)

	{
		term, err := termC.Exec(ctx, &cordiumv1.ExecRequest{
			Command: "ls -la /",
		})
		assert.Nil(t, err)

		strm, err := termC.ListenExec(ctx, &cordiumv1.ListenExecRequest{
			Id: term.Id,
		})
		assert.Nil(t, err)

		mctx, cancelFn := context.WithTimeout(ctx, 3*time.Second)
		defer cancelFn()

		go func() {
			for {
				select {
				case <-mctx.Done():
					zap.L().Debug("exiting client test stdout loop")
					return
				default:
					msg, err := strm.Recv()
					if err != nil {
						continue
					}

					zap.L().Debug("New msg", zap.Any("msg", msg))
					switch msg.Type.(type) {
					case *cordiumv1.ListenExecResponse_Exit_:
						return
					}

				}
			}
		}()

		<-mctx.Done()
	}

	{
		term, err := termC.Exec(ctx, &cordiumv1.ExecRequest{
			Command: "sleep 20",
		})
		assert.Nil(t, err)

		strm, err := termC.ListenExec(ctx, &cordiumv1.ListenExecRequest{
			Id: term.Id,
		})
		assert.Nil(t, err)

		mctx, cancelFn := context.WithTimeout(ctx, 5*time.Second)
		defer cancelFn()

		go func() {
			for {
				select {
				case <-mctx.Done():
					zap.L().Debug("exiting client test stdout loop")
					return
				default:
					msg, err := strm.Recv()
					if err != nil {
						continue
					}
					zap.L().Debug("New msg", zap.Any("msg", msg))
					switch msg.Type.(type) {
					case *cordiumv1.ListenExecResponse_Exit_:
						return
					}

				}
			}
		}()

		time.Sleep(2 * time.Second)

		_, err = termC.KillExec(ctx, &cordiumv1.KillExecRequest{
			Id: term.Id,
		})
		assert.Nil(t, err)
		<-mctx.Done()

	}

	err = grpcConn.Close()
	assert.Nil(t, err, "%+v", err)

	zap.S().Debugf("test ended successfully")
}
*/

func TestExec(t *testing.T) {

	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	defer srv.Close()

	err = srv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{})
	assert.Nil(t, err)

	wsC := ccordiumv1.NewWorkspaceServiceClient(grpcConn)

	_, err = wsC.Prepare(ctx, &ccordiumv1.PrepareRequest{
		Workspace: &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: wsutils.GenWorkspaceName(),
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		},
	})
	assert.Nil(t, err, "%+v", err)
	time.Sleep(2 * time.Second)

	termC := ccordiumv1.NewTerminalServiceClient(grpcConn)

	{
		strm, err := termC.Exec(ctx)
		assert.Nil(t, err)

		strm.Send(&cordiumv1.ExecRequest{
			Type: &cordiumv1.ExecRequest_Request_{
				Request: &cordiumv1.ExecRequest_Request{
					Command: "ls -la /",
				},
			},
		})

		mctx, cancelFn := context.WithTimeout(ctx, 3*time.Second)
		defer cancelFn()

		go func() {
			for {
				select {
				case <-mctx.Done():
					zap.L().Debug("exiting client test stdout loop")
					return
				default:
					msg, err := strm.Recv()
					if err != nil {
						continue
					}

					zap.L().Debug("New msg", zap.Any("msg", msg))
					switch msg.Type.(type) {
					case *cordiumv1.ExecResponse_Exit_:
						return
					}

				}
			}
		}()

		<-mctx.Done()
	}

	err = grpcConn.Close()
	assert.Nil(t, err, "%+v", err)

	zap.S().Debugf("test ended successfully")
}
