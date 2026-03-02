/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package wrks

import (
	"fmt"
	"os"
	"testing"
	"time"

	"context"

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
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestTerminal(t *testing.T) {

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

	srv, err := NewServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err, "%+v", err)

	time.Sleep(3 * time.Second)

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	rgn, err := srv.octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
		Name: "default",
	})
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

	err = srv.supClientMap.Set(ws)
	assert.Nil(t, err)

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

	resp, err := srv.CreateTerminal(usr.Ctx(), &cordiumv1.CreateTerminalRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	})
	assert.Nil(t, err)

	{
		termList, err := srv.ListTerminal(usr.Ctx(), &cordiumv1.ListTerminalRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		assert.Nil(t, err)
		assert.Equal(t, len(termList.Items), 1)
		assert.Equal(t, termList.Items[0].Id, resp.Id)
	}

	_, err = srv.WriteTerminalData(usr.Ctx(), &cordiumv1.WriteTerminalDataRequest{
		Id:   resp.Id,
		Data: []byte("ls- la\r\n"),
	})
	assert.Nil(t, err, "%+v", err)

	time.Sleep(2 * time.Second)

	{
		width := uint32(utilrand.GetRandomRangeMath(100, 200))
		height := uint32(utilrand.GetRandomRangeMath(100, 200))
		_, err = srv.SetTerminalWindowSize(usr.Ctx(), &cordiumv1.SetTerminalWindowSizeRequest{
			Id:     resp.Id,
			Width:  width,
			Height: height,
		})
		assert.Nil(t, err)
	}

	{
		width := uint32(utilrand.GetRandomRangeMath(100, 200))
		height := uint32(utilrand.GetRandomRangeMath(100, 200))
		_, err = srv.SetTerminalWindowSize(usr.Ctx(), &cordiumv1.SetTerminalWindowSizeRequest{
			Id:     fmt.Sprintf("%s-%s", ws.Metadata.Name, utilrand.GetRandomStringLowercase(4)),
			Width:  width,
			Height: height,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	}
}
