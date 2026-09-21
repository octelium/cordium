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
	"fmt"
	"testing"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	k8scorev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *snapshotTest) createVolume(ctx context.Context, t *testing.T,
	spaceRef, regionRef *metav1.ObjectReference,
	accessMode cordiumv1.Volume_AccessMode, megabytes uint32) *cordiumv1.Volume {

	vol, err := c.fakeC.OcteliumC.CordiumC().CreateVolume(ctx, &cordiumv1.Volume{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.default.%s",
				utilrand.GetRandomStringCanonical(8), c.usr.Usr.Metadata.Name),
		},
		Spec: &cordiumv1.Volume_Spec{
			Size:       &cordiumv1.Volume_Spec_Size{Megabytes: megabytes},
			AccessMode: accessMode,
		},
		Status: &cordiumv1.Volume_Status{
			State:     cordiumv1.Volume_Status_STATE_PENDING,
			SpaceRef:  spaceRef,
			UserRef:   umetav1.GetObjectReference(c.usr.Usr),
			RegionRef: regionRef,
		},
	})
	assert.Nil(t, err, "%+v", err)

	return vol
}

func (c *snapshotTest) createTemplate(ctx context.Context, t *testing.T,
	spaceRef *metav1.ObjectReference,
	mounts ...*cordiumv1.Workspace_Spec_Runtime_VolumeMount) *cordiumv1.Template {

	tmpl, err := c.fakeC.OcteliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.default.%s",
				utilrand.GetRandomStringCanonical(8), c.usr.Usr.Metadata.Name),
		},
		Spec: &cordiumv1.Template_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				VolumeMounts: mounts,
			},
		},
		Status: &cordiumv1.Template_Status{
			SpaceRef: spaceRef,
			UserRef:  umetav1.GetObjectReference(c.usr.Usr),
		},
	})
	assert.Nil(t, err, "%+v", err)

	return tmpl
}

func (c *snapshotTest) setPVCPhase(ctx context.Context, t *testing.T,
	vol *cordiumv1.Volume, phase k8scorev1.PersistentVolumeClaimPhase, capacityBytes int64) {

	pvc, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
		Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
	assert.Nil(t, err, "%+v", err)

	pvc.Status.Phase = phase
	if capacityBytes > 0 {
		pvc.Status.Capacity = k8scorev1.ResourceList{
			"storage": *resource.NewQuantity(capacityBytes, resource.BinarySI),
		}
	}

	_, err = c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
		Update(ctx, pvc, k8smetav1.UpdateOptions{})
	assert.Nil(t, err, "%+v", err)
}

func (c *snapshotTest) getVolume(ctx context.Context, t *testing.T,
	vol *cordiumv1.Volume) *cordiumv1.Volume {
	ret, err := c.fakeC.OcteliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
		Uid: vol.Metadata.Uid,
	})
	assert.Nil(t, err, "%+v", err)
	return ret
}

func TestReconcileVolume(t *testing.T) {
	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	c := newSnapshotTest(ctx, t, tst.C)

	spaceRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	t.Run("a Volume of another Region is ignored", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		}, cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))

		_, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
		assert.True(t, k8serr.IsNotFound(err), "%+v", err)
	})

	t.Run("an EXCLUSIVE Volume is provisioned as a single node volume", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 3000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))

		pvc, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
		assert.Nil(t, err, "%+v", err)

		assert.Equal(t, []k8scorev1.PersistentVolumeAccessMode{k8scorev1.ReadWriteOnce},
			pvc.Spec.AccessModes)
		assert.Equal(t, k8scorev1.PersistentVolumeFilesystem, *pvc.Spec.VolumeMode)
		assert.Equal(t, vol.Metadata.Uid, pvc.Labels["octelium.com/volume-uid"])

		req := pvc.Spec.Resources.Requests["storage"]
		expected := resource.MustParse("3000Mi")
		assert.Equal(t, expected.Value(), req.Value())

		assert.Equal(t, cordiumv1.Volume_Status_STATE_PENDING,
			c.getVolume(ctx, t, vol).Status.State)
	})

	t.Run("a SHARED Volume is provisioned as a multi writer volume", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))

		pvc, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
		assert.Nil(t, err, "%+v", err)

		assert.Equal(t, []k8scorev1.PersistentVolumeAccessMode{k8scorev1.ReadWriteMany},
			pvc.Spec.AccessModes)
	})

	t.Run("a bound PVC sets the Volume to ready", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))
		assert.Equal(t, cordiumv1.Volume_Status_STATE_PENDING,
			c.getVolume(ctx, t, vol).Status.State)

		c.setPVCPhase(ctx, t, vol, k8scorev1.ClaimBound, 4*1000*1000*1000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, c.getVolume(ctx, t, vol)))

		cur := c.getVolume(ctx, t, vol)
		assert.Equal(t, cordiumv1.Volume_Status_STATE_READY, cur.Status.State)
		assert.True(t, cur.Status.ReadyAt.IsValid())
		assert.Equal(t, uint32(4000), cur.Status.Capacity.Megabytes)
	})

	t.Run("a lost PVC fails the Volume", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))
		c.setPVCPhase(ctx, t, vol, k8scorev1.ClaimLost, 0)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, c.getVolume(ctx, t, vol)))

		cur := c.getVolume(ctx, t, vol)
		assert.Equal(t, cordiumv1.Volume_Status_STATE_FAILED, cur.Status.State)
		assert.NotNil(t, cur.Status.Failure.GetStorage())
	})

	t.Run("the reconciliation is idempotent", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		for i := 0; i < 3; i++ {
			assert.Nil(t, c.ctl.reconcileVolume(ctx, c.getVolume(ctx, t, vol)))
		}

		pvcList, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).List(ctx,
			k8smetav1.ListOptions{
				LabelSelector: fmt.Sprintf("octelium.com/volume-uid=%s", vol.Metadata.Uid),
			})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 1, len(pvcList.Items))
	})

	t.Run("growing a Volume grows its PVC", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))

		cur := c.getVolume(ctx, t, vol)
		cur.Spec.Size.Megabytes = 8000
		cur, err := c.fakeC.OcteliumC.CordiumC().UpdateVolume(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, cur))

		pvc, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
		assert.Nil(t, err, "%+v", err)

		req := pvc.Spec.Resources.Requests["storage"]
		expected := resource.MustParse("8000Mi")
		assert.Equal(t, expected.Value(), req.Value())
	})

	t.Run("deleting a Volume deletes its PVC", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		assert.Nil(t, c.ctl.reconcileVolume(ctx, vol))

		assert.Nil(t, c.ctl.OnDeleteVolume(ctx, vol))

		_, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
		assert.True(t, k8serr.IsNotFound(err), "%+v", err)

		assert.Nil(t, c.ctl.OnDeleteVolume(ctx, vol))
	})

	t.Run("the storage class is chosen by the Cluster rules", func(t *testing.T) {
		cc, err := c.fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err, "%+v", err)

		cc.Spec.Volume = &cordiumv1.ClusterConfig_Spec_Volume{
			Storage: &cordiumv1.ClusterConfig_Spec_Volume_Storage{
				StorageClass: &cordiumv1.ClusterConfig_Spec_Volume_Storage_StorageClass{
					Rules: []*cordiumv1.ClusterConfig_Spec_Volume_Storage_StorageClass_Rule{
						{
							StorageClass: "shared-fs",
							Condition: &cordiumv1.Condition{
								Type: &cordiumv1.Condition_Match{
									Match: `ctx.volume.spec.accessMode == "ACCESS_MODE_SHARED"`,
								},
							},
						},
						{
							StorageClass: "block",
							Condition: &cordiumv1.Condition{
								Type: &cordiumv1.Condition_MatchAny{
									MatchAny: true,
								},
							},
						},
					},
				},
			},
		}
		_, err = c.fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err, "%+v", err)

		shared := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)
		assert.Equal(t, "shared-fs", *c.ctl.getVolumeStorageClassName(ctx, shared))

		exclusive := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)
		assert.Equal(t, "block", *c.ctl.getVolumeStorageClassName(ctx, exclusive))

		assert.Nil(t, c.ctl.reconcileVolume(ctx, shared))
		pvc, err := c.ctl.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(shared), k8smetav1.GetOptions{})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, "shared-fs", *pvc.Spec.StorageClassName)

		cc, err = c.fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err, "%+v", err)
		cc.Spec.Volume = nil
		_, err = c.fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err, "%+v", err)
	})
}

func TestResolveVolumeMounts(t *testing.T) {
	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	c := newSnapshotTest(ctx, t, tst.C)

	spaceRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	newWorkspace := func(mounts ...*cordiumv1.Workspace_Spec_Runtime_VolumeMount) *cordiumv1.Workspace {
		return &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: wsutils.GenWorkspaceName(),
				Uid:  vutils.UUIDv4(),
			},
			Spec: &cordiumv1.Workspace_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					VolumeMounts: mounts,
				},
			},
			Status: &cordiumv1.Workspace_Status{
				SpaceRef:  spaceRef,
				RegionRef: c.regionRef,
			},
		}
	}

	newMount := func(vol *cordiumv1.Volume, mountPath string,
		readOnly bool) *cordiumv1.Workspace_Spec_Runtime_VolumeMount {
		return &cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			VolumeRef: umetav1.GetObjectReference(vol),
			MountPath: mountPath,
			ReadOnly:  readOnly,
		}
	}

	t.Run("a Workspace without mounts resolves to nothing", func(t *testing.T) {
		mounts, err := c.ctl.resolveVolumeMounts(ctx, newWorkspace())
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 0, len(mounts))
	})

	t.Run("the mounts are resolved and ordered by mountPath", func(t *testing.T) {
		vol1 := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)
		vol2 := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)

		mounts, err := c.ctl.resolveVolumeMounts(ctx, newWorkspace(
			newMount(vol2, "/data", false),
			newMount(vol1, "/cache", true),
		))
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 2, len(mounts))

		assert.Equal(t, "/cache", mounts[0].MountPath)
		assert.Equal(t, vol1.Metadata.Uid, mounts[0].VolumeRef.Uid)
		assert.True(t, mounts[0].ReadOnly)

		assert.Equal(t, "/data", mounts[1].MountPath)
		assert.Equal(t, vol2.Metadata.Uid, mounts[1].VolumeRef.Uid)
		assert.False(t, mounts[1].ReadOnly)
	})

	t.Run("a Volume that does not exist is rejected", func(t *testing.T) {
		_, err := c.ctl.resolveVolumeMounts(ctx, newWorkspace(
			&cordiumv1.Workspace_Spec_Runtime_VolumeMount{
				VolumeRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("%s.default.%s",
						utilrand.GetRandomStringCanonical(8), c.usr.Usr.Metadata.Name),
					Uid: vutils.UUIDv4(),
				},
				MountPath: "/data",
			}))
		assert.NotNil(t, err)
		_, ok := err.(*wsutils.VolumeMountError)
		assert.True(t, ok, "%+v", err)
	})

	t.Run("a Volume of another Space is rejected", func(t *testing.T) {
		vol := c.createVolume(ctx, t, &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		}, c.regionRef, cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		_, err := c.ctl.resolveVolumeMounts(ctx, newWorkspace(newMount(vol, "/data", false)))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "another Space")
	})

	t.Run("a Volume of another Region is rejected", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		}, cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		_, err := c.ctl.resolveVolumeMounts(ctx, newWorkspace(newMount(vol, "/data", false)))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "another Region")
	})

	t.Run("a failed Volume is rejected", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)
		vol.Status.State = cordiumv1.Volume_Status_STATE_FAILED
		vol, err := c.fakeC.OcteliumC.CordiumC().UpdateVolume(ctx, vol)
		assert.Nil(t, err, "%+v", err)

		_, err = c.ctl.resolveVolumeMounts(ctx, newWorkspace(newMount(vol, "/data", false)))
		assert.NotNil(t, err)
	})

	t.Run("a build Workspace mounts nothing", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		ws := newWorkspace(newMount(vol, "/data", false))
		ws.Status.IsBuild = true

		mounts, err := c.ctl.resolveVolumeMounts(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 0, len(mounts))
	})

	t.Run("the Template mounts are merged with the Workspace ones", func(t *testing.T) {
		tmplVol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)
		wsVol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		tmpl := c.createTemplate(ctx, t, spaceRef, newMount(tmplVol, "/datasets", true))

		ws := newWorkspace(newMount(wsVol, "/cache", false))
		ws.Status.TemplateRef = umetav1.GetObjectReference(tmpl)

		mounts, err := c.ctl.resolveVolumeMounts(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 2, len(mounts))

		assert.Equal(t, "/cache", mounts[0].MountPath)
		assert.Equal(t, wsVol.Metadata.Uid, mounts[0].VolumeRef.Uid)
		assert.False(t, mounts[0].ReadOnly)

		assert.Equal(t, "/datasets", mounts[1].MountPath)
		assert.Equal(t, tmplVol.Metadata.Uid, mounts[1].VolumeRef.Uid)
		assert.True(t, mounts[1].ReadOnly)
	})

	t.Run("a Workspace that only inherits the Template mounts resolves them", func(t *testing.T) {
		vol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)

		tmpl := c.createTemplate(ctx, t, spaceRef, newMount(vol, "/datasets", true))

		ws := newWorkspace()
		ws.Status.TemplateRef = umetav1.GetObjectReference(tmpl)

		mounts, err := c.ctl.resolveVolumeMounts(ctx, ws)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 1, len(mounts))
		assert.Equal(t, vol.Metadata.Uid, mounts[0].VolumeRef.Uid)
	})

	t.Run("a Workspace mount that collides with a Template one is rejected", func(t *testing.T) {
		tmplVol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)
		wsVol := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		tmpl := c.createTemplate(ctx, t, spaceRef, newMount(tmplVol, "/data", true))

		ws := newWorkspace(newMount(wsVol, "/data", false))
		ws.Status.TemplateRef = umetav1.GetObjectReference(tmpl)

		_, err := c.ctl.resolveVolumeMounts(ctx, ws)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already used")
	})

	t.Run("the mounted Volumes must all be hosted in the same Region", func(t *testing.T) {
		vol1 := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)
		vol2 := c.createVolume(ctx, t, spaceRef, &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		}, cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)

		ws := newWorkspace(newMount(vol1, "/data", false), newMount(vol2, "/cache", false))
		ws.Status.RegionRef = nil

		resolved, err := wsutils.ResolveVolumeMounts(ctx, c.fakeC.OcteliumC,
			&wsutils.ResolveVolumeMountsReq{Workspace: ws})
		assert.Nil(t, err, "%+v", err)

		_, err = resolved.GetRegionRef()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "different Regions")
	})

	t.Run("the Pod mounts every resolved Volume at its own deterministic path", func(t *testing.T) {
		vol1 := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, 2000)
		vol2 := c.createVolume(ctx, t, spaceRef, c.regionRef,
			cordiumv1.Volume_ACCESS_MODE_SHARED, 2000)

		ws := newWorkspace()
		mounts, err := c.ctl.resolveVolumeMounts(ctx, newWorkspace(
			newMount(vol1, "/data", false),
			newMount(vol2, "/cache", true),
		))
		assert.Nil(t, err, "%+v", err)

		podSpec := c.ctl.newPodSpec(ws, mounts)

		getVolume := func(name string) *k8scorev1.Volume {
			for i := range podSpec.Volumes {
				if podSpec.Volumes[i].Name == name {
					return &podSpec.Volumes[i]
				}
			}
			return nil
		}

		getVolumeMount := func(name string) *k8scorev1.VolumeMount {
			for i := range podSpec.Containers[0].VolumeMounts {
				if podSpec.Containers[0].VolumeMounts[i].Name == name {
					return &podSpec.Containers[0].VolumeMounts[i]
				}
			}
			return nil
		}

		for _, itm := range []struct {
			vol      *cordiumv1.Volume
			readOnly bool
		}{
			{vol: vol1, readOnly: false},
			{vol: vol2, readOnly: true},
		} {
			name := fmt.Sprintf("vol-%s", itm.vol.Metadata.Uid)

			k8sVolume := getVolume(name)
			assert.NotNil(t, k8sVolume, name)
			assert.Equal(t, getVolumePVCName(itm.vol), k8sVolume.PersistentVolumeClaim.ClaimName)
			assert.Equal(t, itm.readOnly, k8sVolume.PersistentVolumeClaim.ReadOnly)

			k8sMount := getVolumeMount(name)
			assert.NotNil(t, k8sMount, name)
			assert.Equal(t, workspacecommon.GetVolumePathByUID(itm.vol.Metadata.Uid),
				k8sMount.MountPath)
			assert.Equal(t, itm.readOnly, k8sMount.ReadOnly)
		}

		assert.NotNil(t, getVolume("octelium"))
	})
}
