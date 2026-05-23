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

package vigil

import (
	"os"
	"testing"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/stretchr/testify/assert"
)

func TestUpstream(t *testing.T) {

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

		sess := usr.Session
		sess.Status.Connection = &corev1.Session_Status_Connection{
			ESSHEnable: true,
		}
		err = srv.aCache.SetSession(sess)
		assert.Nil(t, err)

		{
			req := &corev1.RequestContext{
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
				User:    usr.Usr,
				Session: sess,
			}

			_, err = srv.doGetUpstream(ctx, nil, req)
			assert.Nil(t, err)
		}

		{
			sess.Status.Connection = nil
			err = srv.aCache.SetSession(sess)
			assert.Nil(t, err)

			req := &corev1.RequestContext{
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
				User:    usr.Usr,
				Session: sess,
			}

			_, err = srv.doGetUpstream(ctx, nil, req)
			assert.NotNil(t, err)
		}

	}

}
