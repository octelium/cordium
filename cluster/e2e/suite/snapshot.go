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

package suite

import (
	"context"
	"fmt"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkspaceSnapshot(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	name := h.Name()

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})

	marker := workspacePath("e2e-" + name)
	h.MustExec(t, ws, fmt.Sprintf("echo %s > %s && sync", ws.Metadata.Uid, marker))

	snapshot := h.CreateWorkspaceSnapshot(t, ws, fmt.Sprintf("e2e-%s", name))

	t.Run("TheSnapshotIsTakenAsynchronously", func(t *testing.T) {
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING, snapshot.Status.State)
		assert.Equal(t, cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CRASH,
			snapshot.Status.Consistency)
		assert.Equal(t, ws.Metadata.Uid, snapshot.Status.WorkspaceRef.Uid)
		assert.Equal(t, ws.Status.SpaceRef.Uid, snapshot.Status.SpaceRef.Uid)
		assert.Equal(t, ws.Status.RegionRef.Uid, snapshot.Status.RegionRef.Uid)
		assert.Equal(t, fmt.Sprintf("e2e-%s.%s", name, h.UserName(t)), snapshot.Metadata.Name)
	})

	t.Run("TheWorkspaceKeepsRunningWhileBeingSnapshotted", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, cur.Status.State)
		assert.Equal(t, "ok", h.MustExec(t, ws, "echo ok"))
	})

	t.Run("TheSnapshotIsListedAndRetrieved", func(t *testing.T) {
		cur, err := h.CordiumC().GetWorkspaceSnapshot(ctx, &metav1.GetOptions{
			Name: snapshot.Metadata.Name,
		})
		require.Nil(t, err)
		assert.Equal(t, snapshot.Metadata.Uid, cur.Metadata.Uid)

		itmList, err := h.CordiumC().ListWorkspaceSnapshot(ctx,
			&cordiumv1.ListWorkspaceSnapshotOptions{
				Filter: &cordiumv1.ListWorkspaceSnapshotOptions_WorkspaceRef{
					WorkspaceRef: umetav1.GetObjectReference(ws),
				},
			})
		require.Nil(t, err)
		require.Len(t, itmList.Items, 1)
		assert.Equal(t, snapshot.Metadata.Uid, itmList.Items[0].Metadata.Uid)
	})

	t.Run("TheWorkspaceCannotBeSnapshottedTwiceAtOnce", func(t *testing.T) {
		_, err := h.CordiumC().CreateWorkspaceSnapshot(ctx, &cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("e2e-%s-second", name),
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				WorkspaceRef: umetav1.GetObjectReference(ws),
			},
		})
		require.NotNil(t, err)
	})

	t.Run("AWorkspaceIsNotRestoredFromANonReadySnapshot", func(t *testing.T) {
		cur, err := h.CordiumC().GetWorkspaceSnapshot(ctx, &metav1.GetOptions{
			Name: snapshot.Metadata.Name,
		})
		require.Nil(t, err)

		if cur.Status.State == cordiumv1.WorkspaceSnapshot_Status_STATE_READY {
			t.Skip("the Cluster storage backend already completed the snapshot")
		}

		_, err = h.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				WorkspaceSnapshotRef: umetav1.GetObjectReference(snapshot),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("AWorkspaceThatHasNeverRunCannotBeSnapshotted", func(t *testing.T) {
		fresh := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{})

		_, err := h.CordiumC().CreateWorkspaceSnapshot(ctx, &cordiumv1.WorkspaceSnapshot{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("e2e-%s-fresh", name),
			},
			Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
			Status: &cordiumv1.WorkspaceSnapshot_Status{
				WorkspaceRef: umetav1.GetObjectReference(fresh),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("AWorkspaceIsRestoredWithTheStorageLimitOfItsSnapshot", func(t *testing.T) {
		ready := waitWorkspaceSnapshotTaken(t, h, snapshot)

		restored := h.CreateWorkspace(t, &cordiumv1.Workspace{
			Status: &cordiumv1.Workspace_Status{
				WorkspaceSnapshotRef: umetav1.GetObjectReference(ready),
			},
		})
		h.StartWorkspace(t, restored)
		h.WaitWorkspaceRunning(t, restored)

		assert.Equal(t, ws.Metadata.Uid, h.MustExec(t, restored, "cat "+marker))

		source := h.GetWorkspace(t, ws)
		cur := h.GetWorkspace(t, restored)
		require.NotNil(t, cur.Status.Limit)
		require.NotNil(t, cur.Status.Limit.Storage)
		assert.Equal(t, source.Status.Limit.Storage.Megabytes, cur.Status.Limit.Storage.Megabytes)

		sourcePVC, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
		require.Nil(t, err)

		restoredPVC, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(restored), k8smetav1.GetOptions{})
		require.Nil(t, err)

		assert.Equal(t,
			max(int64(cur.Status.Limit.Storage.Megabytes)*1000*1000, int64(ready.Status.RestoreSizeBytes)),
			restoredPVC.Spec.Resources.Requests.Storage().Value())

		assert.Equal(t, sourcePVC.Status.Capacity.Storage().Value(),
			restoredPVC.Status.Capacity.Storage().Value(),
			"the restored Workspace storage grew beyond the storage of the snapshotted Workspace")
	})

	t.Run("TheSnapshotIsDeleted", func(t *testing.T) {
		_, err := h.CordiumC().DeleteWorkspaceSnapshot(ctx, &metav1.DeleteOptions{
			Uid: snapshot.Metadata.Uid,
		})
		require.Nil(t, err)

		_, err = h.CordiumC().GetWorkspaceSnapshot(ctx, &metav1.GetOptions{
			Uid: snapshot.Metadata.Uid,
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err), "%+v", err)
	})
}

func waitWorkspaceSnapshotTaken(t *testing.T, h *charness.H,
	snapshot *cordiumv1.WorkspaceSnapshot) *cordiumv1.WorkspaceSnapshot {
	t.Helper()

	var ret *cordiumv1.WorkspaceSnapshot

	h.Eventually(t, "the WorkspaceSnapshot to be taken", charness.StartBudget,
		func(ctx context.Context) error {
			cur, err := h.CordiumC().GetWorkspaceSnapshot(ctx, &metav1.GetOptions{
				Uid: snapshot.Metadata.Uid,
			})
			if err != nil {
				return err
			}

			if cur.Status.State == cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING {
				return errors.Errorf("the WorkspaceSnapshot %s is still being taken",
					cur.Metadata.Name)
			}

			ret = cur

			return nil
		})

	if ret.Status.Failure.GetUnsupported() != nil {
		t.Skip("the Cluster storage backend does not support snapshots")
	}

	require.Equal(t, cordiumv1.WorkspaceSnapshot_Status_STATE_READY, ret.Status.State,
		"the WorkspaceSnapshot %s could not be taken: %s",
		ret.Metadata.Name, ret.Status.Failure.GetMessage())

	return ret
}
