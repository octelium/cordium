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

func TestVolume(t *testing.T) {

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

	_, err = srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{},
		Spec:     &cordiumv1.Workspace_Spec{},
		Status:   &cordiumv1.Workspace_Status{},
	})
	assert.Nil(t, err, "%+v", err)

	createVolumeWithName := func(t *testing.T, name string,
		spec *cordiumv1.Volume_Spec) (*cordiumv1.Volume, error) {
		return srv.CreateVolume(usr.Ctx(), &cordiumv1.Volume{
			Metadata: &metav1.Metadata{
				Name: name,
			},
			Spec:   spec,
			Status: &cordiumv1.Volume_Status{},
		})
	}

	createVolume := func(t *testing.T, spec *cordiumv1.Volume_Spec) *cordiumv1.Volume {
		vol, err := createVolumeWithName(t, utilrand.GetRandomStringCanonical(8), spec)
		assert.Nil(t, err, "%+v", err)
		return vol
	}

	createWorkspace := func(t *testing.T,
		mounts ...*cordiumv1.Workspace_Spec_Runtime_VolumeMount) (*cordiumv1.Workspace, error) {
		return srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec: &cordiumv1.Workspace_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					VolumeMounts: mounts,
				},
			},
			Status: &cordiumv1.Workspace_Status{},
		})
	}

	newMount := func(name, mountPath string) *cordiumv1.Workspace_Spec_Runtime_VolumeMount {
		return &cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			VolumeRef: &metav1.ObjectReference{Name: name},
			MountPath: mountPath,
		}
	}

	t.Run("a Volume is created with defaults inside the default Space", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		assert.Equal(t, fmt.Sprintf("default.%s", usr.Usr.Metadata.Name), vol.Status.SpaceRef.Name)
		assert.Equal(t, usr.Usr.Metadata.Uid, vol.Status.UserRef.Uid)
		assert.Equal(t, cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE, vol.Spec.AccessMode)
		assert.Equal(t, uint32(defaultVolumeMegabytes), vol.Spec.Size.Megabytes)
		assert.Equal(t, cordiumv1.Volume_Status_STATE_PENDING, vol.Status.State)
		assert.NotNil(t, vol.Status.RegionRef)
	})

	t.Run("a SHARED Volume of an explicit size is created", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{
			Size:       &cordiumv1.Volume_Spec_Size{Megabytes: 20000},
			AccessMode: cordiumv1.Volume_ACCESS_MODE_SHARED,
		})

		assert.Equal(t, cordiumv1.Volume_ACCESS_MODE_SHARED, vol.Spec.AccessMode)
		assert.Equal(t, uint32(20000), vol.Spec.Size.Megabytes)
	})

	t.Run("an invalid size is rejected", func(t *testing.T) {
		_, err := createVolumeWithName(t, utilrand.GetRandomStringCanonical(8),
			&cordiumv1.Volume_Spec{
				Size: &cordiumv1.Volume_Spec_Size{Megabytes: 10},
			})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		_, err = createVolumeWithName(t, utilrand.GetRandomStringCanonical(8),
			&cordiumv1.Volume_Spec{
				Size: &cordiumv1.Volume_Spec_Size{Megabytes: maxVolumeMegabytes + 1},
			})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a duplicate name is rejected", func(t *testing.T) {
		name := utilrand.GetRandomStringCanonical(8)

		_, err := createVolumeWithName(t, name, &cordiumv1.Volume_Spec{})
		assert.Nil(t, err, "%+v", err)

		_, err = createVolumeWithName(t, name, &cordiumv1.Volume_Spec{})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a Volume is retrieved and listed", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		cur, err := srv.GetVolume(usr.Ctx(), &metav1.GetOptions{
			Name: vol.Metadata.Name,
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, vol.Metadata.Uid, cur.Metadata.Uid)

		itmList, err := srv.ListVolume(usr.Ctx(), &cordiumv1.ListVolumeOptions{
			SpaceRef: vol.Status.SpaceRef,
		})
		assert.Nil(t, err, "%+v", err)

		found := false
		for _, itm := range itmList.Items {
			if itm.Metadata.Uid == vol.Metadata.Uid {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("a Volume of another User is not accessible", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC,
			adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.GetVolume(usr2.Ctx(), &metav1.GetOptions{
			Name: vol.Metadata.Name,
		})
		assert.NotNil(t, err)

		_, err = srv.DeleteVolume(usr2.Ctx(), &metav1.DeleteOptions{
			Name: vol.Metadata.Name,
		})
		assert.NotNil(t, err)
	})

	t.Run("a Volume can only be grown", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{
			Size: &cordiumv1.Volume_Spec_Size{Megabytes: 5000},
		})

		vol.Spec.Size.Megabytes = 9000
		cur, err := srv.UpdateVolume(usr.Ctx(), vol)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, uint32(9000), cur.Spec.Size.Megabytes)

		cur.Spec.Size.Megabytes = 3000
		_, err = srv.UpdateVolume(usr.Ctx(), cur)
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		cur, err = srv.GetVolume(usr.Ctx(), &metav1.GetOptions{Name: vol.Metadata.Name})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, uint32(9000), cur.Spec.Size.Megabytes)
	})

	t.Run("the accessMode of a Volume is immutable", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		vol.Spec.AccessMode = cordiumv1.Volume_ACCESS_MODE_SHARED
		_, err := srv.UpdateVolume(usr.Ctx(), vol)
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a Workspace mounts a Volume of its own Space", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		ws, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		mounts := ws.Spec.Runtime.VolumeMounts
		assert.Equal(t, 1, len(mounts))
		assert.Equal(t, vol.Metadata.Uid, mounts[0].VolumeRef.Uid)
		assert.Equal(t, vol.Metadata.Name, mounts[0].VolumeRef.Name)
		assert.Equal(t, "/data", mounts[0].MountPath)
	})

	t.Run("a Volume that does not exist cannot be mounted", func(t *testing.T) {
		_, err := createWorkspace(t, newMount(utilrand.GetRandomStringCanonical(8), "/data"))
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("an invalid mountPath is rejected", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		for _, mountPath := range []string{"", "data", "/", "/proc", "/workspace", "/data/../etc"} {
			_, err := createWorkspace(t, newMount(vol.Metadata.Name, mountPath))
			assert.NotNil(t, err, "%s", mountPath)
			assert.True(t, grpcerr.IsInvalidArg(err), "%s: %+v", mountPath, err)
		}
	})

	t.Run("overlapping mountPaths are rejected", func(t *testing.T) {
		vol1 := createVolume(t, &cordiumv1.Volume_Spec{})
		vol2 := createVolume(t, &cordiumv1.Volume_Spec{})

		_, err := createWorkspace(t,
			newMount(vol1.Metadata.Name, "/data"),
			newMount(vol2.Metadata.Name, "/data/cache"))
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a Volume that is mounted by a Workspace cannot be deleted", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		ws, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteVolume(usr.Ctx(), &metav1.DeleteOptions{
			Name: vol.Metadata.Name,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		_, err = srv.DeleteWorkspace(usr.Ctx(), &metav1.DeleteOptions{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteVolume(usr.Ctx(), &metav1.DeleteOptions{
			Name: vol.Metadata.Name,
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.GetVolume(usr.Ctx(), &metav1.GetOptions{Name: vol.Metadata.Name})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err), "%+v", err)
	})

	t.Run("a Template mounts a Volume of its own Space", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		tmpl, err := srv.CreateTemplate(usr.Ctx(), &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s",
					utilrand.GetRandomStringCanonical(8), vol.Status.SpaceRef.Name),
			},
			Spec: &cordiumv1.Template_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
						newMount(vol.Metadata.Name, "/datasets"),
					},
				},
			},
			Status: &cordiumv1.Template_Status{},
		})
		assert.Nil(t, err, "%+v", err)

		mounts := tmpl.Spec.Runtime.VolumeMounts
		assert.Equal(t, 1, len(mounts))
		assert.Equal(t, vol.Metadata.Uid, mounts[0].VolumeRef.Uid)

		_, err = srv.DeleteVolume(usr.Ctx(), &metav1.DeleteOptions{
			Name: vol.Metadata.Name,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		_, err = srv.DeleteTemplate(usr.Ctx(), &metav1.DeleteOptions{
			Uid: tmpl.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteVolume(usr.Ctx(), &metav1.DeleteOptions{
			Name: vol.Metadata.Name,
		})
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("the mounts of a Workspace are updated", func(t *testing.T) {
		vol1 := createVolume(t, &cordiumv1.Volume_Spec{})
		vol2 := createVolume(t, &cordiumv1.Volume_Spec{})

		ws, err := createWorkspace(t, newMount(vol1.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		ws.Spec.Runtime.VolumeMounts = []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount(vol2.Metadata.Name, "/cache"),
		}
		cur, err := srv.UpdateWorkspace(usr.Ctx(), ws)
		assert.Nil(t, err, "%+v", err)

		mounts := cur.Spec.Runtime.VolumeMounts
		assert.Equal(t, 1, len(mounts))
		assert.Equal(t, vol2.Metadata.Uid, mounts[0].VolumeRef.Uid)
		assert.Equal(t, "/cache", mounts[0].MountPath)

		_, err = srv.DeleteVolume(usr.Ctx(), &metav1.DeleteOptions{
			Name: vol1.Metadata.Name,
		})
		assert.Nil(t, err, "%+v", err)

		cur.Spec.Runtime.VolumeMounts = []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount(vol2.Metadata.Name, "/proc"),
		}
		_, err = srv.UpdateWorkspace(usr.Ctx(), cur)
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("the number of mounts per Workspace is capped", func(t *testing.T) {
		cc, err := fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err, "%+v", err)

		cc.Spec.Volume = &cordiumv1.ClusterConfig_Spec_Volume{
			Limit: &cordiumv1.ClusterConfig_Spec_Volume_Limit{
				MaxMountsPerWorkspace: 1,
			},
		}
		_, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err, "%+v", err)

		vol1 := createVolume(t, &cordiumv1.Volume_Spec{})
		vol2 := createVolume(t, &cordiumv1.Volume_Spec{})

		_, err = createWorkspace(t, newMount(vol1.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		_, err = createWorkspace(t,
			newMount(vol1.Metadata.Name, "/data"),
			newMount(vol2.Metadata.Name, "/cache"))
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		cc, err = fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err, "%+v", err)
		cc.Spec.Volume = nil
		_, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("a Workspace whose Volumes span two Regions cannot be started", func(t *testing.T) {
		vol1 := createVolume(t, &cordiumv1.Volume_Spec{})
		vol2 := createVolume(t, &cordiumv1.Volume_Spec{})

		vol2.Status.RegionRef = &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		}
		_, err := fakeC.OcteliumC.CordiumC().UpdateVolume(ctx, vol2)
		assert.Nil(t, err, "%+v", err)

		ws, err := createWorkspace(t,
			newMount(vol1.Metadata.Name, "/data"),
			newMount(vol2.Metadata.Name, "/cache"))
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	})

	t.Run("a Workspace is started in the Region of its Volumes", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		ws, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		assert.Nil(t, err, "%+v", err)

		cur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, vol.Status.RegionRef.Uid, cur.Status.RegionRef.Uid)
	})

	t.Run("an EXCLUSIVE Volume cannot be mounted by two running Workspaces", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{})

		ws1, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		ws2, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws1),
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws2),
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		_, err = srv.StopWorkspace(usr.Ctx(), &cordiumv1.StopWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws1),
		})
		assert.Nil(t, err, "%+v", err)

		cur, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
			Uid: ws1.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
		cur.Status.State = cordiumv1.Workspace_Status_STOPPED
		_, err = fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, cur)
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws2),
		})
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("a SHARED Volume can be mounted by several running Workspaces", func(t *testing.T) {
		vol := createVolume(t, &cordiumv1.Volume_Spec{
			AccessMode: cordiumv1.Volume_ACCESS_MODE_SHARED,
		})

		ws1, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		ws2, err := createWorkspace(t, newMount(vol.Metadata.Name, "/data"))
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws1),
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws2),
		})
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("the Volumes of a Space are deleted along with it", func(t *testing.T) {
		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC,
			adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr2.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err, "%+v", err)

		vol, err := srv.CreateVolume(usr2.Ctx(), &cordiumv1.Volume{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), spc.Metadata.Name),
			},
			Spec:   &cordiumv1.Volume_Spec{},
			Status: &cordiumv1.Volume_Status{},
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteSpace(usr2.Ctx(), &metav1.DeleteOptions{
			Uid: spc.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)

		_, err = fakeC.OcteliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
			Uid: vol.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err), "%+v", err)
	})

	t.Run("the number of Volumes per Space is capped", func(t *testing.T) {
		cc, err := fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err, "%+v", err)

		cc.Spec.Volume = &cordiumv1.ClusterConfig_Spec_Volume{
			Limit: &cordiumv1.ClusterConfig_Spec_Volume_Limit{
				MaxPerSpace: 1,
			},
		}
		_, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err, "%+v", err)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC,
			adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr2.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err, "%+v", err)

		createInSpace := func() error {
			_, err := srv.CreateVolume(usr2.Ctx(), &cordiumv1.Volume{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("%s.%s",
						utilrand.GetRandomStringCanonical(8), spc.Metadata.Name),
				},
				Spec:   &cordiumv1.Volume_Spec{},
				Status: &cordiumv1.Volume_Status{},
			})
			return err
		}

		assert.Nil(t, createInSpace())
		assert.NotNil(t, createInSpace())

		cc, err = fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err, "%+v", err)
		cc.Spec.Volume = nil
		_, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err, "%+v", err)
	})
}
