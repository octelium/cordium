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
	"context"
	"testing"
	"time"

	v1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	utils_types "github.com/octelium/octelium/pkg/utils/types"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	k8scorev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type snapshotTest struct {
	fakeC     *otests.FakeClient
	ctl       *Controller
	usr       *tstuser.User
	regionRef *metav1.ObjectReference
}

func newSnapshotTest(ctx context.Context, t *testing.T, fakeC *otests.FakeClient) *snapshotTest {
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil,
		corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	return &snapshotTest{
		fakeC:     fakeC,
		ctl:       newTestController(ctx, t, fakeC, regionRef),
		usr:       usr,
		regionRef: regionRef,
	}
}

func (c *snapshotTest) createWorkspace(ctx context.Context, t *testing.T) *cordiumv1.Workspace {
	ws, err := c.fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: wsutils.GenWorkspaceName(),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			State:     cordiumv1.Workspace_Status_STOPPED,
			UserRef:   umetav1.GetObjectReference(c.usr.Usr),
			RegionRef: c.regionRef,
		},
	})
	assert.Nil(t, err, "%+v", err)

	return ws
}

func (c *snapshotTest) createSnapshot(ctx context.Context, t *testing.T,
	ws *cordiumv1.Workspace, regionRef *metav1.ObjectReference) *cordiumv1.WorkspaceSnapshot {
	snapshot, err := c.fakeC.OcteliumC.CordiumC().CreateWorkspaceSnapshot(ctx,
		&cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				State:        cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING,
				WorkspaceRef: umetav1.GetObjectReference(ws),
				UserRef:      umetav1.GetObjectReference(c.usr.Usr),
				RegionRef:    regionRef,
			},
		})
	assert.Nil(t, err, "%+v", err)

	return snapshot
}

func (c *snapshotTest) createPVC(ctx context.Context, t *testing.T, ws *cordiumv1.Workspace) {
	_, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).Create(ctx,
		&k8scorev1.PersistentVolumeClaim{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      c.ctl.getPVCName(ws),
				Namespace: ns,
			},
		}, k8smetav1.CreateOptions{})
	assert.Nil(t, err, "%+v", err)
}

func (c *snapshotTest) setVolumeSnapshotReady(ctx context.Context, t *testing.T,
	name string, restoreSizeBytes int64) {
	k8sSnapshot, err := c.ctl.snapshotC.SnapshotV1().VolumeSnapshots(ns).
		Get(ctx, name, k8smetav1.GetOptions{})
	assert.Nil(t, err, "%+v", err)

	restoreSize := resource.NewQuantity(restoreSizeBytes, resource.BinarySI)
	creationTime := k8smetav1.NewTime(time.Now())

	k8sSnapshot.Status = &v1.VolumeSnapshotStatus{
		ReadyToUse:   utils_types.BoolToPtr(true),
		CreationTime: &creationTime,
		RestoreSize:  restoreSize,
	}

	_, err = c.ctl.snapshotC.SnapshotV1().VolumeSnapshots(ns).
		Update(ctx, k8sSnapshot, k8smetav1.UpdateOptions{})
	assert.Nil(t, err, "%+v", err)
}

func (c *snapshotTest) getSnapshot(ctx context.Context, t *testing.T,
	snapshot *cordiumv1.WorkspaceSnapshot) *cordiumv1.WorkspaceSnapshot {
	ret, err := c.fakeC.OcteliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
		Uid: snapshot.Metadata.Uid,
	})
	assert.Nil(t, err, "%+v", err)

	return ret
}

func TestReconcileWorkspaceSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	c := newSnapshotTest(ctx, t, tst.C)

	t.Run("a snapshot of another Region is not handled", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		})

		err := c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		_, err = c.ctl.snapshotC.SnapshotV1().VolumeSnapshots(ns).
			Get(ctx, getWorkspaceSnapshotK8sName(snapshot), k8smetav1.GetOptions{})
		assert.True(t, k8serr.IsNotFound(err), "%+v", err)
	})

	t.Run("a clean snapshot of a Workspace that started again is downgraded", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)
		snapshot.Status.Consistency = cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CLEAN
		snapshot, err := c.fakeC.OcteliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		ws.Status.State = cordiumv1.Workspace_Status_RUNNING
		_, err = c.fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws)
		assert.Nil(t, err, "%+v", err)

		assert.Nil(t, c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot))

		cur, err := c.fakeC.OcteliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
			Uid: snapshot.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CRASH,
			cur.Status.Consistency,
			"a clean snapshot was promised while the Workspace was running at the storage cut")
	})

	t.Run("a clean snapshot of a stopped Workspace keeps its guarantee", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)
		snapshot.Status.Consistency = cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CLEAN
		snapshot, err := c.fakeC.OcteliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		assert.Nil(t, c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot))

		cur, err := c.fakeC.OcteliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
			Uid: snapshot.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CLEAN, cur.Status.Consistency)
	})

	t.Run("the restore size falls back to the size of the source volume", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)

		_, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).Create(ctx,
			&k8scorev1.PersistentVolumeClaim{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:      c.ctl.getPVCName(ws),
					Namespace: ns,
				},
				Spec: k8scorev1.PersistentVolumeClaimSpec{
					Resources: k8scorev1.VolumeResourceRequirements{
						Requests: k8scorev1.ResourceList{
							"storage": *resource.NewQuantity(7*1000*1000*1000, resource.BinarySI),
						},
					},
				},
			}, k8smetav1.CreateOptions{})
		assert.Nil(t, err, "%+v", err)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		assert.Nil(t, c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot))

		cur, err := c.fakeC.OcteliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
			Uid: snapshot.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, uint64(7*1000*1000*1000), cur.Status.RestoreSizeBytes,
			"a snapshot whose driver reports no restore size must fall back to the source volume size")
	})

	t.Run("a snapshot without a source volume fails", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		err := c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		cur := c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_FAILED, cur.Status.State)
		assert.NotNil(t, cur.Status.Failure.GetSourceNotFound())
	})

	t.Run("a snapshot becomes ready once the volume snapshot is readyToUse", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		err := c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		k8sSnapshot, err := c.ctl.snapshotC.SnapshotV1().VolumeSnapshots(ns).
			Get(ctx, getWorkspaceSnapshotK8sName(snapshot), k8smetav1.GetOptions{})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, c.ctl.getPVCName(ws), *k8sSnapshot.Spec.Source.PersistentVolumeClaimName)
		assert.Equal(t, snapshot.Metadata.Uid,
			k8sSnapshot.Labels["octelium.com/workspace-snapshot-uid"])
		assert.Equal(t, ws.Metadata.Uid, k8sSnapshot.Labels["octelium.com/workspace-uid"])

		cur := c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING, cur.Status.State)
		assert.False(t, cur.Status.ReadyAt.IsValid())

		err = c.ctl.reconcileWorkspaceSnapshot(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		cur = c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING, cur.Status.State)

		c.setVolumeSnapshotReady(ctx, t, getWorkspaceSnapshotK8sName(snapshot), 4000*1000*1000)

		err = c.ctl.reconcileWorkspaceSnapshot(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		cur = c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_READY, cur.Status.State)
		assert.True(t, cur.Status.ReadyAt.IsValid())
		assert.True(t, cur.Status.SnapshotAt.IsValid())
		assert.Equal(t, uint64(4000*1000*1000), cur.Status.RestoreSizeBytes)

		err = c.ctl.reconcileWorkspaceSnapshot(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		err = c.ctl.OnDeleteWorkspaceSnapshot(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		_, err = c.ctl.snapshotC.SnapshotV1().VolumeSnapshots(ns).
			Get(ctx, getWorkspaceSnapshotK8sName(cur), k8smetav1.GetOptions{})
		assert.True(t, k8serr.IsNotFound(err), "%+v", err)

		err = c.ctl.OnDeleteWorkspaceSnapshot(ctx, cur)
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("a volume snapshot that already exists is adopted", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		_, err := c.ctl.createVolumeSnapshot(ctx, &createVolumeSnapshotReq{
			name:    getWorkspaceSnapshotK8sName(snapshot),
			pvcName: c.ctl.getPVCName(ws),
		})
		assert.Nil(t, err, "%+v", err)

		c.setVolumeSnapshotReady(ctx, t, getWorkspaceSnapshotK8sName(snapshot), 0)

		err = c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		cur := c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_READY, cur.Status.State)
	})

	t.Run("a snapshot that cannot be taken in time fails", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		err := c.ctl.reconcileWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		cur := c.getSnapshot(ctx, t, snapshot)
		cur.Metadata.CreatedAt = pbutils.Timestamp(time.Now().Add(-1 * (snapshotCreationTimeout + time.Hour)))

		err = c.ctl.reconcileWorkspaceSnapshot(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		cur = c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_FAILED, cur.Status.State)
		assert.NotNil(t, cur.Status.Failure.GetStorage())
	})

	t.Run("reconciling all the snapshots", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		c.createPVC(ctx, t, ws)

		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		err := c.ctl.ReconcileWorkspaceSnapshots(ctx)
		assert.Nil(t, err, "%+v", err)

		_, err = c.ctl.snapshotC.SnapshotV1().VolumeSnapshots(ns).
			Get(ctx, getWorkspaceSnapshotK8sName(snapshot), k8smetav1.GetOptions{})
		assert.Nil(t, err, "%+v", err)

		c.setVolumeSnapshotReady(ctx, t, getWorkspaceSnapshotK8sName(snapshot), 0)

		err = c.ctl.ReconcileWorkspaceSnapshots(ctx)
		assert.Nil(t, err, "%+v", err)

		cur := c.getSnapshot(ctx, t, snapshot)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_READY, cur.Status.State)
	})
}

func TestGetPVCDataSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	c := newSnapshotTest(ctx, t, tst.C)

	readySnapshot := func(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.WorkspaceSnapshot {
		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)
		snapshot.Status.State = cordiumv1.WorkspaceSnapshot_Status_STATE_READY
		snapshot, err := c.fakeC.OcteliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		return snapshot
	}

	t.Run("a Workspace without a snapshot has no dataSource", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)

		dataSource, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, dataSource)
	})

	t.Run("a build Workspace has no dataSource", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		ws.Status.IsBuild = true

		dataSource, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, dataSource)
	})

	t.Run("a Workspace that already ran keeps its own storage", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		snapshot := readySnapshot(t, ws)

		ws.Status.WorkspaceSnapshotRef = umetav1.GetObjectReference(snapshot)
		ws.Status.SuccessfulRuns = 1

		dataSource, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, dataSource)
	})

	t.Run("a snapshot that is not ready is a hard error", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		snapshot := c.createSnapshot(ctx, t, ws, c.regionRef)

		ws.Status.WorkspaceSnapshotRef = umetav1.GetObjectReference(snapshot)

		_, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.NotNil(t, err)
	})

	t.Run("a volume snapshot that is not readyToUse is a hard error", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		snapshot := readySnapshot(t, ws)

		ws.Status.WorkspaceSnapshotRef = umetav1.GetObjectReference(snapshot)

		_, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.NotNil(t, err)

		_, err = c.ctl.createVolumeSnapshot(ctx, &createVolumeSnapshotReq{
			name:    getWorkspaceSnapshotK8sName(snapshot),
			pvcName: c.ctl.getPVCName(ws),
		})
		assert.Nil(t, err, "%+v", err)

		_, err = c.ctl.getPVCDataSource(ctx, ws)
		assert.NotNil(t, err)

		c.setVolumeSnapshotReady(ctx, t, getWorkspaceSnapshotK8sName(snapshot), 0)

		dataSource, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.NotNil(t, dataSource)
		assert.Equal(t, volumeSnapshotAPIGroup, *dataSource.APIGroup)
		assert.Equal(t, "VolumeSnapshot", dataSource.Kind)
		assert.Equal(t, getWorkspaceSnapshotK8sName(snapshot), dataSource.Name)
	})

	t.Run("a Template snapshot is only used once it is readyToUse", func(t *testing.T) {
		org, err := c.fakeC.OcteliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err, "%+v", err)

		tmpl, err := c.fakeC.OcteliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
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
		assert.Nil(t, err, "%+v", err)

		ws := c.createWorkspace(ctx, t)
		ws.Status.TemplateRef = umetav1.GetObjectReference(tmpl)

		dataSource, err := c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, dataSource)

		_, err = c.ctl.createVolumeSnapshot(ctx, &createVolumeSnapshotReq{
			name:    c.ctl.getTemplateBuildName(tmpl),
			pvcName: c.ctl.getPVCName(ws),
		})
		assert.Nil(t, err, "%+v", err)

		dataSource, err = c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, dataSource)

		c.setVolumeSnapshotReady(ctx, t, c.ctl.getTemplateBuildName(tmpl), 0)

		dataSource, err = c.ctl.getPVCDataSource(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.NotNil(t, dataSource)
		assert.Equal(t, c.ctl.getTemplateBuildName(tmpl), dataSource.Name)
	})

	t.Run("the PVC is at least as large as the restore size", func(t *testing.T) {
		ws := c.createWorkspace(ctx, t)
		ws.Status.Limit = &cordiumv1.Workspace_Spec_Limit{
			Storage: &cordiumv1.Workspace_Spec_Limit_Storage{
				Megabytes: 1000,
			},
		}

		assert.Equal(t, int64(1000), c.ctl.getPVCStorageMegabytes(ctx, ws))

		snapshot := readySnapshot(t, ws)
		snapshot.Status.RestoreSizeBytes = 8000 * 1000 * 1000
		snapshot, err := c.fakeC.OcteliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot)
		assert.Nil(t, err, "%+v", err)

		ws.Status.WorkspaceSnapshotRef = umetav1.GetObjectReference(snapshot)

		assert.Equal(t, int64(8000), c.ctl.getPVCStorageMegabytes(ctx, ws))
	})
}

func TestGetPersistentStateSource(t *testing.T) {

	tmplWithBuild := &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &cordiumv1.Template_Spec{},
		Status: &cordiumv1.Template_Status{
			BuildInfo: &cordiumv1.Template_Status_BuildInfo{
				CurrentReadyBuildID: utilrand.GetRandomStringCanonical(8),
			},
		},
	}

	newWorkspace := func() *cordiumv1.Workspace {
		return &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: wsutils.GenWorkspaceName(),
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		}
	}

	{
		ws := newWorkspace()
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_EMPTY,
			getPersistentStateSource(ws, nil, false))
	}

	{
		ws := newWorkspace()
		ws.Status.IsBuild = true
		ws.Status.SuccessfulRuns = 2
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_EMPTY,
			getPersistentStateSource(ws, tmplWithBuild, true))
	}

	{
		ws := newWorkspace()
		ws.Status.SuccessfulRuns = 1
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_EXISTING,
			getPersistentStateSource(ws, tmplWithBuild, true))
	}

	{
		ws := newWorkspace()
		ws.Spec.IsEphemeral = true
		ws.Status.SuccessfulRuns = 1
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_TEMPLATE_SNAPSHOT,
			getPersistentStateSource(ws, tmplWithBuild, true))
	}

	{
		ws := newWorkspace()
		ws.Status.WorkspaceSnapshotRef = &metav1.ObjectReference{
			Uid: vutils.UUIDv4(),
		}
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_WORKSPACE_SNAPSHOT,
			getPersistentStateSource(ws, tmplWithBuild, true))
	}

	{
		ws := newWorkspace()
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_TEMPLATE_SNAPSHOT,
			getPersistentStateSource(ws, tmplWithBuild, true))
		assert.Equal(t, ccordiumv1.PersistentStateSource_PERSISTENT_STATE_SOURCE_EMPTY,
			getPersistentStateSource(ws, tmplWithBuild, false))
	}
}
