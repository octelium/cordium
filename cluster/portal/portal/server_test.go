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

package portal

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"context"

	"github.com/gorilla/websocket"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/supervisor/supervisor"
	"github.com/octelium/cordium/cluster/workspace/workspace"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestServer(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	time.Sleep(2 * time.Second)

	wsSrv, err := workspace.NewServer(ctx)
	assert.Nil(t, err)

	defer wsSrv.Close()

	err = wsSrv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	wsSupSrv, err := supervisor.NewServer(ctx)
	assert.Nil(t, err)

	defer wsSupSrv.Close()

	err = wsSupSrv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	cc, err := fakeC.OcteliumC.CoreV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err)

	svc, err := adminSrv.CreateService(ctx, tests.GenService(""))
	assert.Nil(t, err)

	os.Setenv("OCTELIUM_SVC_UID", svc.Metadata.Uid)

	srv, err := newServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err, "%+v", err)
	err = srv.Run(ctx)
	assert.Nil(t, err)

	time.Sleep(3 * time.Second)

	wsClient := websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	rgn, err := fakeC.OcteliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
		Name: vutils.GetMyRegionName(),
	})
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: wsutils.GenWorkspaceName(),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			UserRef:   umetav1.GetObjectReference(usr.Usr),
			RegionRef: umetav1.GetObjectReference(rgn),
			State:     cordiumv1.Workspace_Status_RUNNING,
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	os.Setenv("CORDIUM_NAME", ws.Metadata.Name)

	err = srv.OnWorkspaceCreate(ctx, ws)
	assert.Nil(t, err)

	/*
		doGetRegistryInfo := func(provider *cordiumv1.CloudProvider) (*ccordiumv1.InitializeRequest_ContainerRegistry, error) {
			spec := provider.Spec.GetContainerRegistry()
			if spec == nil {
				return nil, errors.Errorf("Storage is not ContainerRegistry")
			}

			sec, err := tst.C.OcteliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
				Name: spec.Password.GetFromSecret(),
			})
			if err != nil {
				return nil, err
			}

			return &ccordiumv1.InitializeRequest_ContainerRegistry{
				Server:          spec.Server,
				Namespace:       spec.Namespace,
				User:            spec.Username,
				Password:        ucordiumv1.ToSecret(sec).GetValueStr(),
				InsecureSkipTLS: spec.NoTLS,
			}, nil
		}


			doGetInternalContainerRegistry := func() (*ccordiumv1.InitializeRequest_ContainerRegistry, error) {
				zap.L().Debug("Defaulting to internal container registry")
				provider, err := tst.C.OcteliumC.CordiumC().GetCloudProvider(ctx, &rmetav1.GetOptions{
					Name: "sys:internal-registry",
				})
				if err != nil {
					return nil, err
				}

				return doGetRegistryInfo(provider)

			}

			internalContainerRegistry, err := doGetInternalContainerRegistry()
			assert.Nil(t, err)
	*/

	_, err = wsSupSrv.Initialize(ctx, &ccordiumv1.InitializeRequest{
		Workspace: ws,
		ClientInfo: &ccordiumv1.InitializeRequest_ClientInfo{
			Domain: cc.Status.Domain,
		},

		/*
			LoadContainerRegistry: internalContainerRegistry,
			SaveContainerRegistry: internalContainerRegistry,
		*/
	})
	assert.Nil(t, err)

	time.Sleep(2 * time.Second)

	wsC, _, err := wsClient.DialContext(ctx, "ws://localhost:49999/connect", http.Header{
		"X-Octelium-Session-Uid": []string{
			usr.Session.Metadata.Uid,
		},
		"X-Octelium-Origin": []string{
			fmt.Sprintf("https://cordium.%s",
				(cc.Status.Domain)),
		},
	})
	assert.Nil(t, err, "%+v", err)

	/*
		msg := &cordiumv1.ClientMessage{
			Type: &cordiumv1.ClientMessage_Global_{
				Global: &cordiumv1.ClientMessage_Global{
					Type: &cordiumv1.ClientMessage_Global_CreateTerminal_{
						CreateTerminal: &cordiumv1.ClientMessage_Global_CreateTerminal{
							WorkspaceUID: ws.Metadata.Uid,
						},
					},
				},
			},
		}
	*/

	width := uint32(utilrand.GetRandomRangeMath(100, 1000))
	height := uint32(utilrand.GetRandomRangeMath(100, 1000))

	setWinDone := false
	closeDone := false
	stdoutDone := false
	go func() {
		for {
			_, data, err := wsC.ReadMessage()
			assert.Nil(t, err)
			msg := &cordiumv1.ServerMessage{}
			err = pbutils.Unmarshal(data, msg)
			assert.Nil(t, err)

			switch msg.Type.(type) {
			case *cordiumv1.ServerMessage_ListenTerminalEvent_:
				msgT := msg.GetListenTerminalEvent().ListenTerminalResponse
				switch msgT.Type.(type) {
				case *cordiumv1.ListenTerminalResponse_WindowSize_:
					setWinDone = true
				case *cordiumv1.ListenTerminalResponse_Close_:
					closeDone = true
				case *cordiumv1.ListenTerminalResponse_Stdout_:
					stdoutDone = true
				}
				/*
					switch msg.Get().Type.(type) {
					case *cordiumv1.ServerMessage_Terminal_Create_:
						term := msg.GetTerminal().GetCreate()
						assert.Equal(t, ws.Metadata.Uid, term.WorkspaceUID)
						{
							err = wsC.WriteMessage(websocket.BinaryMessage,
								pbutils.MarshalMust(createMsgTerminalData(msg.GetTerminal().Uid, []byte("ls"))))
							assert.Nil(t, err, "%+v", err)

							err = wsC.WriteMessage(websocket.BinaryMessage,
								pbutils.MarshalMust(createMsgTerminalData(msg.GetTerminal().Uid, []byte("\x09\x09"))))
							assert.Nil(t, err, "%+v", err)

						}
					}
				*/

			}
		}
	}()

	time.Sleep(5 * time.Second)

	zap.L().Debug("Creating terminal...")
	termResp, err := wsSupSrv.CreateTerminal(usr.Ctx(), &ccordiumv1.CreateTerminalRequest{})
	assert.Nil(t, err, "%+v", err)
	zap.L().Debug("Terminal created...")
	time.Sleep(1 * time.Second)

	{
		msg := &cordiumv1.ClientMessage{
			Type: &cordiumv1.ClientMessage_ListenTerminalRequest{
				ListenTerminalRequest: &cordiumv1.ListenTerminalRequest{
					Id: termResp.Id,
				},
			},
		}

		err = wsC.WriteMessage(websocket.BinaryMessage, pbutils.MarshalMust(msg))
		assert.Nil(t, err)
	}

	_, err = wsSupSrv.WriteDataTerminal(usr.Ctx(), &ccordiumv1.WriteDataTerminalRequest{
		Id:   termResp.Id,
		Data: []byte("ls"),
	})
	assert.Nil(t, err, "%+v", err)

	/*
		_, err = wsSupSrv.WriteDataTerminal(usr.Ctx(), &ccordiumv1.WriteDataTerminalRequest{
			Id:   termResp.Id,
			Data: []byte("\r\n"),
		})
		assert.Nil(t, err)
	*/

	time.Sleep(1 * time.Second)
	_, err = wsSupSrv.SetWindowSize(usr.Ctx(), &ccordiumv1.SetWindowSizeRequest{
		Id:   termResp.Id,
		Cols: uint32(width),
		Rows: uint32(height),
	})
	assert.Nil(t, err)
	time.Sleep(1 * time.Second)
	_, err = wsSupSrv.RemoveTerminal(ctx, &ccordiumv1.RemoveTerminalRequest{
		Id: termResp.Id,
	})
	assert.Nil(t, err)
	time.Sleep(1 * time.Second)
	assert.True(t, setWinDone && closeDone && stdoutDone)
}
