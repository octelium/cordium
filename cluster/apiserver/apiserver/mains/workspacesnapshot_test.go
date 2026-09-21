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

package mains

import (
	"context"
	"fmt"
	"testing"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestWorkspaceSnapshot(t *testing.T) {

	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err)

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC,
		adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	createWorkspace := func(t *testing.T) *cordiumv1.Workspace {
		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status:   &cordiumv1.Workspace_Status{},
		})
		assert.Nil(t, err, "%+v", err)
		return ws
	}

	setWorkspaceRan := func(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.Workspace {
		ws.Status.LastRegionRef = regionRef
		ws.Status.SuccessfulRuns = 1
		ws, err := fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		return ws
	}

	createSnapshotWithName := func(t *testing.T,
		ws *cordiumv1.Workspace, name string) *cordiumv1.WorkspaceSnapshot {
		snapshot, err := srv.CreateWorkspaceSnapshot(usr.Ctx(), &cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: name,
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				WorkspaceRef: umetav1.GetObjectReference(ws),
			},
		})
		assert.Nil(t, err, "%+v", err)
		return snapshot
	}

	createSnapshot := func(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.WorkspaceSnapshot {
		return createSnapshotWithName(t, ws, utilrand.GetRandomStringCanonical(8))
	}

	setSnapshotReady := func(t *testing.T, snapshot *cordiumv1.WorkspaceSnapshot,
		restoreSizeBytes uint64) *cordiumv1.WorkspaceSnapshot {
		snapshot.Status.State = cordiumv1.WorkspaceSnapshot_Status_STATE_READY
		snapshot.Status.RestoreSizeBytes = restoreSizeBytes
		snapshot, err := fakeC.OcteliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)
		return snapshot
	}

	t.Run("a Workspace that has never run cannot be snapshotted", func(t *testing.T) {
		ws := createWorkspace(t)

		_, err := srv.CreateWorkspaceSnapshot(usr.Ctx(), &cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				WorkspaceRef: umetav1.GetObjectReference(ws),
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a snapshot of a stopped Workspace is clean", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))

		name := utilrand.GetRandomStringCanonical(8)
		snapshot := createSnapshotWithName(t, ws, name)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING, snapshot.Status.State)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CLEAN, snapshot.Status.Consistency)
		assert.Equal(t, ws.Metadata.Uid, snapshot.Status.WorkspaceRef.Uid)
		assert.Equal(t, ws.Status.SpaceRef.Uid, snapshot.Status.SpaceRef.Uid)
		assert.Equal(t, ws.Status.TemplateRef.Uid, snapshot.Status.TemplateRef.Uid)
		assert.Equal(t, regionRef.Uid, snapshot.Status.RegionRef.Uid)
		assert.Equal(t, usr.Usr.Metadata.Uid, snapshot.Status.UserRef.Uid)
		assert.Equal(t, fmt.Sprintf("%s.%s", name, usr.Usr.Metadata.Name), snapshot.Metadata.Name)
	})

	t.Run("a snapshot of a running Workspace is crash consistent", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		ws.Status.State = cordiumv1.Workspace_Status_RUNNING
		ws.Status.RegionRef = regionRef
		ws, err := fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws)
		assert.Nil(t, err)

		snapshot := createSnapshot(t, ws)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CRASH, snapshot.Status.Consistency)

		wsCur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, wsCur.Status.State)
	})

	t.Run("only one snapshot of a Workspace can be taken at a time", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))

		createSnapshot(t, ws)

		_, err := srv.CreateWorkspaceSnapshot(usr.Ctx(), &cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				WorkspaceRef: umetav1.GetObjectReference(ws),
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.AlreadyExists(err), "%+v", err)
	})

	t.Run("a Workspace of another User cannot be snapshotted", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC,
			adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.CreateWorkspaceSnapshot(usr2.Ctx(), &cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				WorkspaceRef: umetav1.GetObjectReference(ws),
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)
	})

	t.Run("get and list", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		name := utilrand.GetRandomStringCanonical(8)
		snapshot := createSnapshotWithName(t, ws, name)

		itm, err := srv.GetWorkspaceSnapshot(usr.Ctx(), &metav1.GetOptions{
			Name: name,
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, snapshot.Metadata.Uid, itm.Metadata.Uid)

		itmList, err := srv.ListWorkspaceSnapshot(usr.Ctx(), &cordiumv1.ListWorkspaceSnapshotOptions{
			Filter: &cordiumv1.ListWorkspaceSnapshotOptions_WorkspaceRef{
				WorkspaceRef: umetav1.GetObjectReference(ws),
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 1, len(itmList.Items))
		assert.Equal(t, snapshot.Metadata.Uid, itmList.Items[0].Metadata.Uid)

		itmList, err = srv.ListWorkspaceSnapshot(usr.Ctx(), &cordiumv1.ListWorkspaceSnapshotOptions{
			Filter: &cordiumv1.ListWorkspaceSnapshotOptions_SpaceRef{
				SpaceRef: ws.Status.SpaceRef,
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.True(t, len(itmList.Items) >= 1)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC,
			adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.GetWorkspaceSnapshot(usr2.Ctx(), &metav1.GetOptions{
			Uid: snapshot.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)
	})

	t.Run("delete", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		name := utilrand.GetRandomStringCanonical(8)
		snapshot := createSnapshotWithName(t, ws, name)

		_, err := srv.DeleteWorkspaceSnapshot(usr.Ctx(), &metav1.DeleteOptions{
			Name: name,
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.GetWorkspaceSnapshot(usr.Ctx(), &metav1.GetOptions{
			Uid: snapshot.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err), "%+v", err)
	})

	t.Run("a Workspace is restored from a ready snapshot", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		snapshot := setSnapshotReady(t, createSnapshot(t, ws), 0)

		restored, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef:          ws.Status.TemplateRef,
				WorkspaceSnapshotRef: umetav1.GetObjectReference(snapshot),
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, snapshot.Metadata.Uid, restored.Status.WorkspaceSnapshotRef.Uid)
		assert.Equal(t, ws.Status.SpaceRef.Uid, restored.Status.SpaceRef.Uid)
	})

	t.Run("a Workspace cannot be restored from a snapshot that is not ready", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		snapshot := createSnapshot(t, ws)

		_, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef:          ws.Status.TemplateRef,
				WorkspaceSnapshotRef: umetav1.GetObjectReference(snapshot),
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a Workspace cannot be restored from a snapshot of another Space", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		snapshot := setSnapshotReady(t, createSnapshot(t, ws), 0)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), usr.Usr.Metadata.Name),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
				},
				WorkspaceSnapshotRef: umetav1.GetObjectReference(snapshot),
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a Workspace cannot be restored into a smaller storage", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		snapshot := setSnapshotReady(t, createSnapshot(t, ws), 100*1000*1000*1000)

		_, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec: &cordiumv1.Workspace_Spec{
				Limit: &cordiumv1.Workspace_Spec_Limit{
					Storage: &cordiumv1.Workspace_Spec_Limit_Storage{
						Megabytes: 1000,
					},
				},
			},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef:          ws.Status.TemplateRef,
				WorkspaceSnapshotRef: umetav1.GetObjectReference(snapshot),
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a snapshot that a Workspace has not been restored from yet cannot be deleted", func(t *testing.T) {
		ws := setWorkspaceRan(t, createWorkspace(t))
		snapshot := setSnapshotReady(t, createSnapshot(t, ws), 0)

		_, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef:          ws.Status.TemplateRef,
				WorkspaceSnapshotRef: umetav1.GetObjectReference(snapshot),
			},
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteWorkspaceSnapshot(usr.Ctx(), &metav1.DeleteOptions{
			Uid: snapshot.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})
}
