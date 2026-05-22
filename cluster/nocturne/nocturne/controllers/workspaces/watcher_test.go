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

package controller

import (
	"fmt"
	"testing"

	"context"

	snapshotclientfake "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned/fake"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/supervisor/supervisor"
	wssrv "github.com/octelium/cordium/cluster/workspace/workspace"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestWatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	wsSrv, err := wssrv.NewServer(ctx)
	assert.Nil(t, err)
	defer wsSrv.Close()

	err = wsSrv.Run(ctx)
	assert.Nil(t, err)

	wsSup, err := supervisor.NewServer(ctx)
	assert.Nil(t, err)
	defer wsSup.Close()

	err = wsSup.Run(ctx)
	assert.Nil(t, err)

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	org, err := fakeC.OcteliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &cordiumv1.Space_Spec{},
		Status: &cordiumv1.Space_Status{
			Type: cordiumv1.Space_Status_ORGANIZATION,
		},
	})
	assert.Nil(t, err)

	tmpl, err := fakeC.OcteliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &cordiumv1.Template_Spec{},
		Status: &cordiumv1.Template_Status{
			SpaceRef: umetav1.GetObjectReference(org),
			BuildInfo: &cordiumv1.Template_Status_BuildInfo{
				CurrentReadyBuildID: utilrand.GetRandomStringCanonical(8),
			},
		},
	})
	assert.Nil(t, err)

	t.Run("basic", func(t *testing.T) {
		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				State:       cordiumv1.Workspace_Status_INIT_REQUEST,
				UserRef:     umetav1.GetObjectReference(usr.Usr),
				SessionRef:  umetav1.GetObjectReference(usr.Session),
				RegionRef:   regionRef,
				SpaceRef:    umetav1.GetObjectReference(org),
				TemplateRef: umetav1.GetObjectReference(tmpl),
				Run: &cordiumv1.Workspace_Status_Run{
					Id: utilrand.GetRandomStringCanonical(8),
				},
			},
		}

		ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
		assert.Nil(t, err)

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws)
		assert.Nil(t, err)

		err = wtchr.run(ctx)
		assert.Nil(t, err)

		err = wtchr.onReadyInit(ctx)
		assert.Nil(t, err)

		wtchr.didOnStopping = true
		err = wtchr.onStopped(ctx)
		assert.Nil(t, err)

		err = wtchr.close()
		assert.Nil(t, err)

		_, err = ctl.octeliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{
			Uid: usr.Session.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))

	})

	t.Run("health-check-err", func(t *testing.T) {

		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
		assert.Nil(t, err)

		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				State:       cordiumv1.Workspace_Status_INIT_REQUEST,
				UserRef:     umetav1.GetObjectReference(usr.Usr),
				SessionRef:  umetav1.GetObjectReference(usr.Session),
				RegionRef:   regionRef,
				SpaceRef:    umetav1.GetObjectReference(org),
				TemplateRef: umetav1.GetObjectReference(tmpl),
				Run: &cordiumv1.Workspace_Status_Run{
					Id: utilrand.GetRandomStringCanonical(8),
				},
			},
		}

		ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
		assert.Nil(t, err)

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws)
		assert.Nil(t, err)
		err = wtchr.run(ctx)
		assert.Nil(t, err)

		err = wtchr.onReadyInit(ctx)
		assert.Nil(t, err, "%+v", err)

		wtchr.healthCheckErr = true

		err = wtchr.close()
		assert.Nil(t, err)

		_, err = ctl.octeliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{
			Uid: usr.Session.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})

	{
		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("prebuild-ws-%s", utilrand.GetRandomStringCanonical(6)),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				State:       cordiumv1.Workspace_Status_INIT_REQUEST,
				SpaceRef:    umetav1.GetObjectReference(org),
				IsBuild:     true,
				RegionRef:   regionRef,
				TemplateRef: umetav1.GetObjectReference(tmpl),
			},
		}

		ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
		assert.Nil(t, err)

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws)
		assert.Nil(t, err)
		err = wtchr.run(ctx)
		assert.Nil(t, err)

		err = wtchr.onReadyInit(ctx)
		assert.Nil(t, err)

		wtchr.didOnStopping = true
		err = wtchr.onStopped(ctx)
		assert.Nil(t, err, "%+v", err)

		err = wtchr.close()
		assert.Nil(t, err)
	}

	t.Run("prebuild-template", func(t *testing.T) {

		buildID := utilrand.GetRandomStringCanonical(8)
		tmpl, err := fakeC.OcteliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Template_Spec{},
			Status: &cordiumv1.Template_Status{
				SpaceRef: umetav1.GetObjectReference(org),

				BuildInfo: &cordiumv1.Template_Status_BuildInfo{
					Builds: []*cordiumv1.Template_Status_BuildInfo_Build{
						{
							StartedAt: pbutils.Now(),
							Id:        buildID,
						},
					},
					CurrentRunningBuildID: buildID,
				},
			},
		})
		assert.Nil(t, err)

		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("prebuild-ws-%s", utilrand.GetRandomStringCanonical(6)),
				SystemLabels: map[string]string{
					"build-id": buildID,
				},
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				State:       cordiumv1.Workspace_Status_INIT_REQUEST,
				IsBuild:     true,
				RegionRef:   regionRef,
				TemplateRef: umetav1.GetObjectReference(tmpl),
				SpaceRef:    tmpl.Status.SpaceRef,
			},
		}

		ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
		assert.Nil(t, err)

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws)
		assert.Nil(t, err)
		err = wtchr.run(ctx)
		assert.Nil(t, err)

		err = wtchr.onReadyInit(ctx)
		assert.Nil(t, err)

		wtchr.didOnStopping = true
		err = wtchr.onStopped(ctx)
		assert.Nil(t, err, "%+v", err)

		err = wtchr.close()
		assert.Nil(t, err)

	})

}
