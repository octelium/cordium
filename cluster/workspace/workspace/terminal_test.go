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

func TestTerminalCleanup(t *testing.T) {

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
	defer grpcConn.Close()

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
	assert.Equal(t, 1, srv.terminalSrv.len())

	_, err = termC.WriteDataTerminal(ctx, &ccordiumv1.WriteDataTerminalRequest{
		Id:   term.Id,
		Data: []byte("exit\r\n"),
	})
	assert.Nil(t, err, "%+v", err)

	for range 50 {
		if srv.terminalSrv.len() == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.Equal(t, 0, srv.terminalSrv.len())

	termList, err := termC.ListTerminal(ctx, &ccordiumv1.ListTerminalRequest{})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(termList.Items))
}

func TestTerminalPublishMsg(t *testing.T) {

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
	defer grpcConn.Close()

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

	msg := &ccordiumv1.ListenTerminalResponse{
		Type: &ccordiumv1.ListenTerminalResponse_Close_{
			Close: &ccordiumv1.ListenTerminalResponse_Close{},
		},
	}

	t.Run("publishing after close is a no-op", func(t *testing.T) {
		term, err := srv.newTerminal(&ccordiumv1.CreateTerminalRequest{})
		assert.Nil(t, err)

		err = term.run(ctx)
		assert.Nil(t, err)

		sub := term.subscribe()

		err = term.close()
		assert.Nil(t, err)

		term.publishMsg(msg)

		assert.Equal(t, 0, len(term.subscribers.subscribersMap))
		assert.Equal(t, 0, len(sub.msgCh))

		isDone := func() bool {
			select {
			case <-term.doneCh:
				return true
			default:
				return false
			}
		}()
		assert.True(t, isDone)
	})

	t.Run("a lagging subscriber does not block the publisher", func(t *testing.T) {
		term, err := srv.newTerminal(&ccordiumv1.CreateTerminalRequest{})
		assert.Nil(t, err)

		err = term.run(ctx)
		assert.Nil(t, err)

		defer term.close()

		sub := term.subscribe()

		for range cap(sub.msgCh) + 10 {
			term.publishMsg(msg)
		}

		assert.Equal(t, cap(sub.msgCh), len(sub.msgCh))
	})
}

func TestExecCleanup(t *testing.T) {

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
	defer grpcConn.Close()

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

	ectx, cancelFn := context.WithCancel(ctx)

	strm, err := termC.Exec(ectx)
	assert.Nil(t, err)

	err = strm.Send(&cordiumv1.ExecRequest{
		Type: &cordiumv1.ExecRequest_Request_{
			Request: &cordiumv1.ExecRequest_Request{
				Command: "sleep 300",
			},
		},
	})
	assert.Nil(t, err)

	for range 50 {
		if srv.taskManager.runningTasksLen() > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Equal(t, 1, srv.taskManager.runningTasksLen())

	cancelFn()

	for range 50 {
		if srv.taskManager.runningTasksLen() == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Equal(t, 0, srv.taskManager.runningTasksLen())
}
