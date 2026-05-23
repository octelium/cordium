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
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/google/uuid"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/suputils"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/portal/portal/acache"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
)

type testWorkspaceTunSrv struct {
	privK wgtypes.Key
	ccordiumv1.UnimplementedWorkspaceSupervisorServiceServer
	netTun  *netTun
	grpcSrv *grpc.Server
}

func (s *testWorkspaceTunSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}

func (s *testWorkspaceTunSrv) close() {
	if s.grpcSrv != nil {
		s.grpcSrv.Stop()
	}
}

func newTestWorkspaceTunSrv(t *testing.T, octeliumC octeliumc.ClientInterface) *testWorkspaceTunSrv {
	ret := &testWorkspaceTunSrv{}

	k, err := wgtypes.GeneratePrivateKey()
	assert.Nil(t, err)
	ret.privK = k

	_, addr, _ := net.ParseCIDR("10.100.100.1/32")

	ret.netTun, err = createNetstackTUN(addr)
	assert.Nil(t, err)

	logger := device.NewLogger(
		device.LogLevelSilent,
		"",
	)

	keySecret, err := octeliumC.CordiumC().GetSecret(context.Background(), &rmetav1.GetOptions{
		Name: "sys:ws-tunnel-wgkey",
	})
	assert.Nil(t, err)

	privKey, err := wgtypes.ParseKey(ucordiumv1.ToSecret(keySecret).GetValueStr())
	assert.Nil(t, err)

	pubK := privKey.PublicKey()

	cfg := func() string {

		var output strings.Builder
		output.WriteString(fmt.Sprintf("private_key=%s\n", wgKeyB64ToHex(ret.privK.String())))
		output.WriteString(fmt.Sprintf("listen_port=%d\n", workspacecommon.GetWorkspaceTunnelPort()))
		output.WriteString("replace_peers=true\n")

		output.WriteString(fmt.Sprintf("public_key=%s\n", wgKeyB64ToHex(pubK.String())))

		output.WriteString("replace_allowed_ips=true\n")

		output.WriteString(fmt.Sprintf("allowed_ip=%s\n", "10.100.100.2/32"))

		return output.String()
	}()

	device := device.NewDevice(ret.netTun, conn.NewDefaultBind(), logger)
	err = device.Up()
	assert.Nil(t, err)

	err = device.IpcSet(cfg)
	assert.Nil(t, err, "%+v", err)

	return ret
}

func (s *testWorkspaceTunSrv) GetTunnelPublicKey(ctx context.Context, req *ccordiumv1.GetTunnelPublicKeyRequest) (*ccordiumv1.GetTunnelPublicKeyResponse, error) {
	return &ccordiumv1.GetTunnelPublicKeyResponse{
		PublicKey: s.privK.PublicKey().String(),
	}, nil
}

func (s *testWorkspaceTunSrv) run() error {

	var lis net.Listener
	var err error

	if err := func() error {
		for i := 0; i < 100; i++ {
			lis, err = net.Listen("tcp", fmt.Sprintf(":%d", suputils.GetWorkspaceSupPort()))
			if err == nil {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
		return err
	}(); err != nil {
		return err
	}

	s.grpcSrv = grpc.NewServer()

	go func() {

		ccordiumv1.RegisterWorkspaceSupervisorServiceServer(s.grpcSrv, s)
		if err := s.grpcSrv.Serve(lis); err != nil {
			return
		}
	}()

	go func() {
		listener, err := s.netTun.GetNetstackNet().ListenTCP(&net.TCPAddr{Port: 3000})
		if err != nil {
			panic(err)
		}
		http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {

			writer.WriteHeader(http.StatusOK)
		})
		http.Serve(listener, nil)

	}()

	return nil
}

func TestParseWorkspaceAppDomain(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	invalids := []string{
		"",
		utilrand.GetRandomStringCanonical(3),
		utilrand.GetRandomStringCanonical(30),
		utilrand.GetRandomStringCanonical(300),
		"port_3000",
		"port_3000_workspace",
		"port_0_workspace.svc",
		"port_70000_workspace.svc",
		"port_num_workspace1.ws",
	}

	for _, invalid := range invalids {
		_, err := parseWorkspaceAppDomain(invalid)
		assert.NotNil(t, err)
	}

	type arg struct {
		str string
		res regexResult
	}
	valids := []arg{
		{
			str: "port_3000_workspace1.ws",
			res: regexResult{
				workspace: "workspace1",
				port:      3000,
				svc:       "ws",
				ns:        "default",
			},
		},

		{
			str: "port_43210_ws2.wssvc_ns1",
			res: regexResult{
				workspace: "ws2",
				port:      43210,
				svc:       "wssvc",
				ns:        "ns1",
			},
		},

		{
			str: "app1_workspace1.ws",
			res: regexResult{
				workspace:   "workspace1",
				application: "app1",
				svc:         "ws",
				ns:          "default",
			},
		},

		{
			str: "workspace1.ws",
			res: regexResult{
				workspace: "workspace1",
				svc:       "ws",
				ns:        "default",
			},
		},
	}

	for _, valid := range valids {
		res, err := parseWorkspaceAppDomain(valid.str)
		assert.Nil(t, err)
		assert.True(t, reflect.DeepEqual(res, &valid.res))
	}
}

func TestTunnelServer(t *testing.T) {
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

	acache, err := acache.NewCache()
	assert.Nil(t, err)

	tstTunSrv := newTestWorkspaceTunSrv(t, fakeC.OcteliumC)
	err = tstTunSrv.run()
	assert.Nil(t, err, "%+v", err)
	defer tstTunSrv.close()

	activityCtly, err := wsutils.NewActivityCtl(fakeC.OcteliumC)
	assert.Nil(t, err)

	region, err := fakeC.OcteliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{Name: "default"})
	assert.Nil(t, err)

	tunSrv, err := newTunnelSrv(ctx, fakeC.OcteliumC, acache, "example.com", activityCtly, umetav1.GetObjectReference(region))
	assert.Nil(t, err)

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  uuid.New().String(),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			UserRef:    umetav1.GetObjectReference(usr.Usr),
			SessionRef: umetav1.GetObjectReference(usr.Session),
			State:      cordiumv1.Workspace_Status_RUNNING,
			RegionRef:  umetav1.GetObjectReference(region),
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	err = acache.SetWorkspace(ws)
	assert.Nil(t, err)

	reqHTTP := httptest.NewRequest("POST", "http://localhost/auth/v1/token", nil)
	reqHTTP.Header.Set("X-Forwarded-Host", fmt.Sprintf("port_3000_%s.%s.example.com", ws.Metadata.Name, "svc"))
	reqCtx := &middlewares.RequestContext{
		Session: usr.Session,
	}

	reqHTTP = reqHTTP.WithContext(context.WithValue(ctx, middlewares.CtxRequestContext, reqCtx))
	w := httptest.NewRecorder()
	tunSrv.ServeHTTP(w, reqHTTP)
	resp := w.Result()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	time.Sleep(2 * time.Second)
}
