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
	"fmt"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkspaceVolume(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	name := h.Name()

	spc := h.GetSpace(t, fmt.Sprintf("default.%s", h.UserName(t)))

	vol := h.CreateVolume(t, spc, &cordiumv1.Volume_Spec{
		Size: &cordiumv1.Volume_Spec_Size{Megabytes: 2000},
	})

	t.Run("TheVolumeIsProvisionedAsynchronously", func(t *testing.T) {
		assert.Equal(t, fmt.Sprintf("%s.default.%s", name, h.UserName(t)), vol.Metadata.Name)
		assert.Equal(t, cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, vol.Spec.AccessMode)
		assert.Equal(t, uint32(2000), vol.Spec.Size.Megabytes)
		assert.Equal(t, spc.Metadata.Uid, vol.Status.SpaceRef.Uid)
		require.NotNil(t, vol.Status.RegionRef)
	})

	t.Run("TheVolumeHasItsOwnPersistentVolumeClaim", func(t *testing.T) {
		pvc, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, fmt.Sprintf("vol-%s", vol.Metadata.Uid), k8smetav1.GetOptions{})
		require.Nil(t, err, "the Volume controller did not provision a PVC")

		assert.Equal(t, vol.Metadata.Uid, pvc.Labels["octelium.com/volume-uid"])
	})

	t.Run("TheVolumeIsListedAndRetrieved", func(t *testing.T) {
		cur, err := h.CordiumC().GetVolume(ctx, &metav1.GetOptions{Name: vol.Metadata.Name})
		require.Nil(t, err)
		assert.Equal(t, vol.Metadata.Uid, cur.Metadata.Uid)

		itmList, err := h.CordiumC().ListVolume(ctx, &cordiumv1.ListVolumeOptions{
			SpaceRef: umetav1.GetObjectReference(spc),
		})
		require.Nil(t, err)

		found := false
		for _, itm := range itmList.Items {
			if itm.Metadata.Uid == vol.Metadata.Uid {
				found = true
			}
		}
		assert.True(t, found)
	})

	marker := fmt.Sprintf("/data/e2e-%s", name)

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
				{
					VolumeRef: &metav1.ObjectReference{Name: vol.Metadata.Name},
					MountPath: "/data",
				},
			},
		},
	})

	t.Run("TheVolumeRefIsResolvedInTheStoredSpec", func(t *testing.T) {
		mounts := ws.Spec.Runtime.VolumeMounts
		require.Len(t, mounts, 1)
		assert.Equal(t, vol.Metadata.Uid, mounts[0].VolumeRef.Uid)
		assert.Equal(t, vol.Metadata.Name, mounts[0].VolumeRef.Name)
	})

	t.Run("TheWorkspaceRunsInTheRegionOfTheVolume", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		require.NotNil(t, cur.Status.RegionRef)
		assert.Equal(t, vol.Status.RegionRef.Uid, cur.Status.RegionRef.Uid)
	})

	t.Run("TheVolumeIsWritableInsideTheWorkspace", func(t *testing.T) {
		h.MustExec(t, ws, fmt.Sprintf("echo volume > %s", marker))
		assert.Equal(t, "volume", h.MustExec(t, ws, "cat "+marker))
	})

	t.Run("TheVolumeIsNotTheWorkspaceOwnStorage", func(t *testing.T) {
		assert.Equal(t, fmt.Sprintf("/cordium-volumes/%s", vol.Metadata.Uid),
			h.MustExec(t, ws, "readlink -f /data"))

		assert.NotEqual(t, h.MustExec(t, ws, "stat -c %d /data"),
			h.MustExec(t, ws, "stat -c %d /workspace"))
	})

	t.Run("TheVolumeCannotBeDeletedWhileItIsMounted", func(t *testing.T) {
		_, err := h.CordiumC().DeleteVolume(ctx, &metav1.DeleteOptions{
			Uid: vol.Metadata.Uid,
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("TheVolumeDataSurvivesTheWorkspaceRestart", func(t *testing.T) {
		h.StopWorkspace(t, ws)
		h.WaitWorkspaceStopped(t, ws)

		h.StartWorkspace(t, ws)
		h.WaitWorkspaceRunning(t, ws)

		assert.Equal(t, "volume", h.MustExec(t, ws, "cat "+marker))
	})

	t.Run("TheVolumeDataOutlivesTheWorkspace", func(t *testing.T) {
		other := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
			IsEphemeral: true,
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
					{
						VolumeRef: &metav1.ObjectReference{Name: vol.Metadata.Name},
						MountPath: "/shared",
					},
				},
			},
		})

		assert.Equal(t, "volume",
			h.MustExec(t, other, fmt.Sprintf("cat /shared/e2e-%s", name)))
	})

	t.Run("AVolumeThatDoesNotExistCannotBeMounted", func(t *testing.T) {
		_, err := h.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec: &cordiumv1.Workspace_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
						{
							VolumeRef: &metav1.ObjectReference{
								Name: fmt.Sprintf("e2e-missing-%s", name),
							},
							MountPath: "/data",
						},
					},
				},
			},
			Status: &cordiumv1.Workspace_Status{},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("AReservedMountPathIsRejected", func(t *testing.T) {
		for _, mountPath := range []string{"/", "/proc", "/workspace", "/cordium-volumes"} {
			_, err := h.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{},
				Spec: &cordiumv1.Workspace_Spec{
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
							{
								VolumeRef: &metav1.ObjectReference{Name: vol.Metadata.Name},
								MountPath: mountPath,
							},
						},
					},
				},
				Status: &cordiumv1.Workspace_Status{},
			})
			require.NotNil(t, err, "%s", mountPath)
			assert.True(t, grpcerr.IsInvalidArg(err), "%s: %+v", mountPath, err)
		}
	})

	t.Run("TheVolumeIsDeletedOnceItIsDetached", func(t *testing.T) {
		detached := h.CreateVolume(t, spc, &cordiumv1.Volume_Spec{
			Size: &cordiumv1.Volume_Spec_Size{Megabytes: 2000},
		})

		_, err := h.CordiumC().DeleteVolume(ctx, &metav1.DeleteOptions{
			Uid: detached.Metadata.Uid,
		})
		require.Nil(t, err)

		_, err = h.CordiumC().GetVolume(ctx, &metav1.GetOptions{
			Uid: detached.Metadata.Uid,
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err), "%+v", err)
	})
}
