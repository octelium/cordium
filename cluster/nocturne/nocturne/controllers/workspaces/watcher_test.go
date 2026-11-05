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

package controller

import (
	"fmt"
	"testing"
	"time"

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

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC, nil)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws, false)
		assert.Nil(t, err)

		wtchr.run()

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

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC, nil)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws, false)
		assert.Nil(t, err)
		wtchr.run()

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

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC, nil)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws, false)
		assert.Nil(t, err)
		wtchr.run()

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

		jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC, nil)
		assert.Nil(t, err)

		ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
		assert.Nil(t, err)
		ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

		wtchr, err := newStatusWatcher(ctl, ws, false)
		assert.Nil(t, err)
		wtchr.run()

		err = wtchr.onReadyInit(ctx)
		assert.Nil(t, err)

		wtchr.didOnStopping = true
		err = wtchr.onStopped(ctx)
		assert.Nil(t, err, "%+v", err)

		err = wtchr.close()
		assert.Nil(t, err)

	})

}

func newTestController(ctx context.Context, t *testing.T,
	fakeC *otests.FakeClient, regionRef *metav1.ObjectReference) *Controller {

	jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC, nil)
	assert.Nil(t, err)

	ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
	assert.Nil(t, err)
	ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

	return ctl
}

func TestDetach(t *testing.T) {
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

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	ctl := newTestController(ctx, t, fakeC, regionRef)

	lastRunningAt := time.Now().Add(-2 * time.Hour)

	ws, err := fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			State:         cordiumv1.Workspace_Status_RUNNING,
			UserRef:       umetav1.GetObjectReference(usr.Usr),
			SessionRef:    umetav1.GetObjectReference(usr.Session),
			RegionRef:     regionRef,
			LastRunningAt: pbutils.Timestamp(lastRunningAt),
		},
	})
	assert.Nil(t, err)

	wtchr, err := ctl.setWatcher(ws, true)
	assert.Nil(t, err)
	assert.NotNil(t, wtchr)
	assert.NotNil(t, ctl.getWatcher(ws.Metadata.Uid))
	assert.Equal(t, lastRunningAt.Unix(), wtchr.runningStartedAt.Unix())

	err = wtchr.detach()
	assert.Nil(t, err)

	assert.Nil(t, ctl.getWatcher(ws.Metadata.Uid))

	wsCur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)
	assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, wsCur.Status.State)
	assert.Equal(t, ws.Metadata.ResourceVersion, wsCur.Metadata.ResourceVersion)

	_, err = fakeC.OcteliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{Uid: usr.Session.Metadata.Uid})
	assert.Nil(t, err)

	ctl.WaitUntilAllWatchersClosed()
}

func TestUpdateStatusIdempotency(t *testing.T) {
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

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	ctl := newTestController(ctx, t, fakeC, regionRef)

	ws, err := fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			State:          cordiumv1.Workspace_Status_RUNNING,
			UserRef:        umetav1.GetObjectReference(usr.Usr),
			SessionRef:     umetav1.GetObjectReference(usr.Session),
			RegionRef:      regionRef,
			SuccessfulRuns: 1,
		},
	})
	assert.Nil(t, err)

	wtchr, err := ctl.setWatcher(ws, true)
	assert.Nil(t, err)

	t.Run("re-applying the current state is a no-op", func(t *testing.T) {
		err = wtchr.doUpdateStatus(ctx, cordiumv1.Workspace_Status_RUNNING)
		assert.Nil(t, err)

		wsCur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		assert.Equal(t, ws.Metadata.ResourceVersion, wsCur.Metadata.ResourceVersion)
		assert.Equal(t, uint32(1), wsCur.Status.SuccessfulRuns)
	})

	t.Run("a new state is applied", func(t *testing.T) {
		err = wtchr.doUpdateStatus(ctx, cordiumv1.Workspace_Status_STOPPING)
		assert.Nil(t, err)

		wsCur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		assert.NotEqual(t, ws.Metadata.ResourceVersion, wsCur.Metadata.ResourceVersion)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPING, wsCur.Status.State)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, wsCur.Status.LastState)
	})

	err = wtchr.detach()
	assert.Nil(t, err)

	ctl.WaitUntilAllWatchersClosed()
}

func TestUpdateStatusFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	ctl := newTestController(ctx, t, fakeC, regionRef)

	failure := &cordiumv1.Workspace_Status_Failure{
		Type: &cordiumv1.Workspace_Status_Failure_StartupUnknown_{
			StartupUnknown: &cordiumv1.Workspace_Status_Failure_StartupUnknown{},
		},
	}

	for _, tt := range []struct {
		name           string
		state          cordiumv1.Workspace_Status_State
		newState       cordiumv1.Workspace_Status_State
		clearFailure   bool
		successfulRuns uint32
	}{
		{
			name:           "a successful startup clears the previous failure",
			state:          cordiumv1.Workspace_Status_PREPARING,
			newState:       cordiumv1.Workspace_Status_RUNNING,
			clearFailure:   true,
			successfulRuns: 2,
		},
		{
			name:           "a resumed running Workspace clears a stale failure",
			state:          cordiumv1.Workspace_Status_RUNNING,
			newState:       cordiumv1.Workspace_Status_RUNNING,
			clearFailure:   true,
			successfulRuns: 1,
		},
		{
			name:           "startup in progress retains the previous failure",
			state:          cordiumv1.Workspace_Status_INITIALIZING,
			newState:       cordiumv1.Workspace_Status_PREPARING,
			successfulRuns: 1,
		},
		{
			name:           "the stopped state retains a recorded failure",
			state:          cordiumv1.Workspace_Status_STOPPING,
			newState:       cordiumv1.Workspace_Status_STOPPED,
			successfulRuns: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ws, err := fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
				},
				Spec: &cordiumv1.Workspace_Spec{},
				Status: &cordiumv1.Workspace_Status{
					State:             tt.state,
					LastState:         cordiumv1.Workspace_Status_INITIALIZING,
					RegionRef:         regionRef,
					SuccessfulRuns:    1,
					Failure:           failure,
					CurrentStateSetAt: pbutils.Timestamp(time.Now().Add(-time.Hour)),
					LastStateSetAt:    pbutils.Timestamp(time.Now().Add(-2 * time.Hour)),
					LastRunningAt:     pbutils.Timestamp(time.Now().Add(-time.Hour)),
					LastActivityAt:    pbutils.Timestamp(time.Now().Add(-time.Minute)),
					Run: &cordiumv1.Workspace_Status_Run{
						Id: utilrand.GetRandomStringCanonical(8),
					},
					LastRuns: []*cordiumv1.Workspace_Status_Run{
						{
							Id:      utilrand.GetRandomStringCanonical(8),
							Failure: failure,
						},
					},
				},
			})
			assert.Nil(t, err)

			wtchr, err := ctl.setWatcher(ws, true)
			assert.Nil(t, err)
			defer wtchr.detach()

			err = wtchr.doUpdateStatus(ctx, tt.newState)
			assert.Nil(t, err)

			wsCur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
			assert.Nil(t, err)
			assert.Equal(t, tt.newState, wsCur.Status.State)
			assert.Equal(t, tt.successfulRuns, wsCur.Status.SuccessfulRuns)
			assert.Len(t, wsCur.Status.LastRuns, 1)
			assert.True(t, pbutils.IsEqual(ws.Status.LastRuns[0], wsCur.Status.LastRuns[0]))
			assert.Nil(t, wsCur.Status.Run.Failure)
			if tt.clearFailure {
				assert.Nil(t, wsCur.Status.Failure)
			} else {
				assert.True(t, pbutils.IsEqual(failure, wsCur.Status.Failure))
			}

			if tt.state == tt.newState {
				assert.Equal(t, ws.Status.LastState, wsCur.Status.LastState)
				assert.True(t, pbutils.IsEqual(ws.Status.CurrentStateSetAt, wsCur.Status.CurrentStateSetAt))
				assert.True(t, pbutils.IsEqual(ws.Status.LastStateSetAt, wsCur.Status.LastStateSetAt))
				assert.True(t, pbutils.IsEqual(ws.Status.LastRunningAt, wsCur.Status.LastRunningAt))
				assert.True(t, pbutils.IsEqual(ws.Status.LastActivityAt, wsCur.Status.LastActivityAt))
			}

			err = wtchr.doUpdateStatus(ctx, tt.newState)
			assert.Nil(t, err)

			wsAgain, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
			assert.Nil(t, err)
			assert.Equal(t, wsCur.Metadata.ResourceVersion, wsAgain.Metadata.ResourceVersion)
		})
	}

	ctl.WaitUntilAllWatchersClosed()
}

func TestOnStoppedFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	wsSup, err := supervisor.NewServer(ctx)
	assert.Nil(t, err)
	defer wsSup.Close()

	err = wsSup.Run(ctx)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	ctl := newTestController(ctx, t, fakeC, regionRef)

	for _, tt := range []struct {
		name           string
		healthCheckErr bool
	}{
		{name: "a successful shutdown clears the previous failure"},
		{name: "a failed shutdown records the new failure", healthCheckErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			failure := &cordiumv1.Workspace_Status_Failure{
				Type: &cordiumv1.Workspace_Status_Failure_StartupUnknown_{
					StartupUnknown: &cordiumv1.Workspace_Status_Failure_StartupUnknown{},
				},
			}

			ws, err := fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
				},
				Spec: &cordiumv1.Workspace_Spec{},
				Status: &cordiumv1.Workspace_Status{
					State:     cordiumv1.Workspace_Status_STOPPING,
					RegionRef: regionRef,
					Failure:   failure,
					Run: &cordiumv1.Workspace_Status_Run{
						Id: utilrand.GetRandomStringCanonical(8),
					},
					LastRuns: []*cordiumv1.Workspace_Status_Run{
						{
							Id:      utilrand.GetRandomStringCanonical(8),
							Failure: failure,
						},
					},
				},
			})
			assert.Nil(t, err)

			wtchr, err := ctl.setWatcher(ws, true)
			assert.Nil(t, err)
			defer wtchr.detach()

			err = wtchr.waitUntilWorkspaceSupInitialized(ctx)
			assert.Nil(t, err)
			wtchr.healthCheckErr = tt.healthCheckErr

			err = wtchr.onStopped(ctx)
			assert.Nil(t, err)

			wsCur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
			assert.Nil(t, err)
			assert.Len(t, wsCur.Status.LastRuns, 1)
			assert.True(t, pbutils.IsEqual(ws.Status.LastRuns[0], wsCur.Status.LastRuns[0]))
			if tt.healthCheckErr {
				assert.NotNil(t, wsCur.Status.Failure.GetHealthCheck())
				assert.True(t, pbutils.IsEqual(wsCur.Status.Failure, wsCur.Status.Run.Failure))
				assert.Equal(t, cordiumv1.Workspace_Status_STOPPING_REASON_ERROR, wsCur.Status.StoppingReason)
			} else {
				assert.Nil(t, wsCur.Status.Failure)
				assert.Nil(t, wsCur.Status.Run.Failure)
			}
		})
	}

	ctl.WaitUntilAllWatchersClosed()
}
