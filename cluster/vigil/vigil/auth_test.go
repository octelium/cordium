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

package vigil

import (
	"os"
	"testing"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/cluster/coctovigilv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/cluster/vigil/vigil/modes"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {

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

	svc, err := adminSrv.CreateService(ctx, tests.GenService(""))
	assert.Nil(t, err)

	os.Setenv("OCTELIUM_SVC_UID", svc.Metadata.Uid)

	srv, err := newServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err, "%+v", err)

	{
		req := &modes.PostAuthorizeRequest{}
		resp, err := srv.doPostAuthorize(ctx, req)
		assert.Nil(t, err)
		assert.False(t, resp.IsAuthorized)
	}

	{
		req := &modes.PostAuthorizeRequest{
			Request: &coctovigilv1.DownstreamRequest{
				Request: &corev1.RequestContext_Request{
					Type: &corev1.RequestContext_Request_Ssh{
						Ssh: &corev1.RequestContext_Request_SSH{
							Type: &corev1.RequestContext_Request_SSH_Connect_{
								Connect: &corev1.RequestContext_Request_SSH_Connect{},
							},
						},
					},
				},
			},
		}
		resp, err := srv.doPostAuthorize(ctx, req)
		assert.Nil(t, err)
		assert.False(t, resp.IsAuthorized)
	}

	{
		req := &modes.PostAuthorizeRequest{
			Request: &coctovigilv1.DownstreamRequest{
				Request: &corev1.RequestContext_Request{
					Type: &corev1.RequestContext_Request_Ssh{
						Ssh: &corev1.RequestContext_Request_SSH{
							Type: &corev1.RequestContext_Request_SSH_Connect_{
								Connect: &corev1.RequestContext_Request_SSH_Connect{
									User: utilrand.GetRandomStringCanonical(8),
								},
							},
						},
					},
				},
			},
		}
		resp, err := srv.doPostAuthorize(ctx, req)
		assert.Nil(t, err)
		assert.False(t, resp.IsAuthorized)
	}

	{
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
				UserRef:    umetav1.GetObjectReference(usr.Usr),
				RegionRef:  umetav1.GetObjectReference(rgn),
				SessionRef: umetav1.GetObjectReference(usr.Session),
				State:      cordiumv1.Workspace_Status_RUNNING,
			},
		}

		err = srv.aCache.SetWorkspace(ws)
		assert.Nil(t, err)

		req := &modes.PostAuthorizeRequest{
			Request: &coctovigilv1.DownstreamRequest{
				Request: &corev1.RequestContext_Request{
					Type: &corev1.RequestContext_Request_Ssh{
						Ssh: &corev1.RequestContext_Request_SSH{
							Type: &corev1.RequestContext_Request_SSH_Connect_{
								Connect: &corev1.RequestContext_Request_SSH_Connect{
									User: ws.Metadata.Name,
								},
							},
						},
					},
				},
			},
			Resp: &coctovigilv1.AuthenticateAndAuthorizeResponse{
				RequestContext: &corev1.RequestContext{
					User:    usr.Usr,
					Session: usr.Session,
				},
			},
		}

		{
			resp, err := srv.doPostAuthorize(ctx, req)
			assert.Nil(t, err)
			assert.True(t, resp.IsAuthorized)
		}

		{
			ws.Status.State = cordiumv1.Workspace_Status_STOPPING_REQUEST
			err = srv.aCache.SetWorkspace(ws)
			assert.Nil(t, err)

			resp, err := srv.doPostAuthorize(ctx, req)
			assert.Nil(t, err)
			assert.False(t, resp.IsAuthorized)
		}

		{
			ws.Status.State = cordiumv1.Workspace_Status_PREPARING
			err = srv.aCache.SetWorkspace(ws)
			assert.Nil(t, err)

			resp, err := srv.doPostAuthorize(ctx, req)
			assert.Nil(t, err)
			assert.True(t, resp.IsAuthorized)
		}

		{
			ws.Status.RegionRef = &metav1.ObjectReference{
				Uid:  vutils.UUIDv4(),
				Name: utilrand.GetRandomStringCanonical(8),
			}
			err = srv.aCache.SetWorkspace(ws)
			assert.Nil(t, err)

			resp, err := srv.doPostAuthorize(ctx, req)
			assert.Nil(t, err)
			assert.False(t, resp.IsAuthorized)
		}

	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
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
				UserRef:    umetav1.GetObjectReference(usr.Usr),
				RegionRef:  umetav1.GetObjectReference(rgn),
				SessionRef: umetav1.GetObjectReference(usr.Session),
				State:      cordiumv1.Workspace_Status_RUNNING,
			},
		}

		err = srv.aCache.SetWorkspace(ws)
		assert.Nil(t, err)

		req := &modes.PostAuthorizeRequest{
			Request: &coctovigilv1.DownstreamRequest{
				Request: &corev1.RequestContext_Request{
					Type: &corev1.RequestContext_Request_Ssh{
						Ssh: &corev1.RequestContext_Request_SSH{
							Type: &corev1.RequestContext_Request_SSH_Connect_{
								Connect: &corev1.RequestContext_Request_SSH_Connect{
									User: ws.Metadata.Name,
								},
							},
						},
					},
				},
			},
			Resp: &coctovigilv1.AuthenticateAndAuthorizeResponse{
				RequestContext: &corev1.RequestContext{
					User:    usr2.Usr,
					Session: usr2.Session,
				},
			},
		}
		resp, err := srv.doPostAuthorize(ctx, req)
		assert.Nil(t, err)
		assert.False(t, resp.IsAuthorized)
	}
}
