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

package wsutils

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcordiumv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCheckVolumeMountPath(t *testing.T) {

	valid := []string{
		"/data",
		"/mnt/datasets",
		"/home/octelium/data",
		"/workspace-data",
		"/opt/models",
		"/var/lib/my-app",
		"/etc/my-app",
	}

	for _, pth := range valid {
		assert.Nil(t, CheckVolumeMountPath(pth), "%s", pth)
	}

	invalid := []string{
		"",
		"data",
		"./data",
		"../data",
		"/",
		"/data/",
		"/data/../etc",
		"/data//cache",
		"/proc",
		"/sys",
		"/dev",
		"/etc",
		"/usr",
		"/var",
		"/var/tmp",
		"/var/run",
		"/var/lib/docker",
		"/tmp",
		"/run",
		"/workspace",
		"/cordium-volumes",
		"/cordium-volumes/abc",
		"/proc/self",
		"/sys/fs/cgroup",
		"/dev/shm",
		"/run/octelium",
		"/var/run/octelium-proxy.sock",
		"/var/lib/docker/overlay",
		"/var/lib/containers/storage",
		"/octelium",
		"/octelium/workspace",
		`/data\x`,
	}

	for _, pth := range invalid {
		assert.NotNil(t, CheckVolumeMountPath(pth), "%s", pth)
	}

	assert.NotNil(t, CheckVolumeMountPath(fmt.Sprintf("/%s", string(make([]byte, 300)))))
}

func TestValidateVolumeMounts(t *testing.T) {

	newMount := func(name, mountPath string) *cordiumv1.Workspace_Spec_Runtime_VolumeMount {
		return &cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			VolumeRef: &metav1.ObjectReference{Name: name},
			MountPath: mountPath,
		}
	}

	t.Run("no mounts is valid", func(t *testing.T) {
		assert.Nil(t, ValidateVolumeMounts(nil))
	})

	t.Run("distinct Volumes at distinct paths are valid", func(t *testing.T) {
		assert.Nil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount("datasets", "/data"),
			newMount("cache", "/cache"),
			newMount("models", "/opt/models"),
		}))
	})

	t.Run("a mount must reference a Volume", func(t *testing.T) {
		assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			{MountPath: "/data"},
		}))

		assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			{VolumeRef: &metav1.ObjectReference{}, MountPath: "/data"},
		}))
	})

	t.Run("the same mountPath cannot be used twice", func(t *testing.T) {
		assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount("datasets", "/data"),
			newMount("cache", "/data"),
		}))
	})

	t.Run("the mountPaths cannot overlap", func(t *testing.T) {
		assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount("datasets", "/data"),
			newMount("cache", "/data/cache"),
		}))

		assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount("cache", "/data/cache"),
			newMount("datasets", "/data"),
		}))

		assert.Nil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount("datasets", "/data"),
			newMount("cache", "/data-cache"),
		}))
	})

	t.Run("the same Volume cannot be mounted twice", func(t *testing.T) {
		assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
			newMount("datasets", "/data"),
			newMount("datasets", "/data2"),
		}))
	})

	t.Run("too many mounts are rejected", func(t *testing.T) {
		var mounts []*cordiumv1.Workspace_Spec_Runtime_VolumeMount
		for i := 0; i < MaxVolumeMountsPerWorkspace+1; i++ {
			mounts = append(mounts, newMount(fmt.Sprintf("vol%d", i), fmt.Sprintf("/data%d", i)))
		}

		assert.NotNil(t, ValidateVolumeMounts(mounts))
		assert.Nil(t, ValidateVolumeMounts(mounts[:MaxVolumeMountsPerWorkspace]))
	})
}

func TestCheckVolumeMountPathBoundaries(t *testing.T) {
	for _, pth := range []string{"/device", "/proc-data", "/sysdata", "/runner", "/var/lib/docker-data", "/cordium-volumes-data", "/etc/application", "/" + strings.Repeat("a", 255)} {
		assert.Nil(t, CheckVolumeMountPath(pth), "%s", pth)
	}
	for _, pth := range []string{"/data\x00file", "/data/.", "//data", "/" + strings.Repeat("a", 256)} {
		assert.NotNil(t, CheckVolumeMountPath(pth), "%s", pth)
	}
	assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{nil}))
	assert.NotNil(t, ValidateVolumeMounts([]*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
		{VolumeRef: &metav1.ObjectReference{Uid: "volume-uid", Name: "first"}, MountPath: "/first"},
		{VolumeRef: &metav1.ObjectReference{Uid: "volume-uid", Name: "second"}, MountPath: "/second"},
	}))
}

type volumeMountClient struct {
	octeliumc.ClientInterface
	resources rcordiumv1.ResourceServiceClient
}

func (c *volumeMountClient) CordiumC() rcordiumv1.ResourceServiceClient { return c.resources }

type volumeMountResources struct {
	rcordiumv1.ResourceServiceClient
	volumes map[string]*cordiumv1.Volume
	err     error
	calls   int
}

func (c *volumeMountResources) GetVolume(ctx context.Context, req *rmetav1.GetOptions, opts ...grpc.CallOption) (*cordiumv1.Volume, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	key := req.Name
	if key == "" {
		key = req.Uid
	}
	if vol := c.volumes[key]; vol != nil {
		return vol, nil
	}
	return nil, status.Error(codes.NotFound, "missing volume")
}

func TestResolveVolumeMountsValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(*ResolveVolumeMountsReq, *volumeMountResources)
		wantError bool
		wantCalls int
	}{
		{name: "no mounts", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { req.Workspace.Spec.Runtime = nil }},
		{name: "build ignores mounts", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { req.Workspace.Status.IsBuild = true }},
		{name: "missing workspace", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { req.Workspace = nil }, wantError: true},
		{name: "missing status", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { req.Workspace.Status = nil }, wantError: true},
		{name: "missing space", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { req.Workspace.Status.SpaceRef = nil }, wantError: true},
		{name: "invalid path", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) {
			req.Workspace.Spec.Runtime.VolumeMounts[0].MountPath = "/proc"
		}, wantError: true},
		{name: "missing volume", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { c.volumes = nil }, wantError: true, wantCalls: 1},
		{name: "other space", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) {
			c.volumes["data"].Status.SpaceRef.Uid = "other"
		}, wantError: true, wantCalls: 1},
		{name: "no volume space", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { c.volumes["data"].Status.SpaceRef = nil }, wantError: true, wantCalls: 1},
		{name: "other region", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) {
			req.RegionRef = &metav1.ObjectReference{Uid: "other"}
		}, wantError: true, wantCalls: 1},
		{name: "unassigned region", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) { c.volumes["data"].Status.RegionRef = nil }, wantError: true, wantCalls: 1},
		{name: "failed storage", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) {
			c.volumes["data"].Status.State = cordiumv1.Volume_Status_STATE_FAILED
		}, wantError: true, wantCalls: 1},
		{name: "resource service error", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) {
			c.err = status.Error(codes.Unavailable, "offline")
		}, wantError: true, wantCalls: 1},
		{name: "valid mount", change: func(req *ResolveVolumeMountsReq, c *volumeMountResources) {}, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := workspaceValidationReq().Workspace
			ws.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{{VolumeRef: &metav1.ObjectReference{Name: "data"}, MountPath: "/data", ReadOnly: true}}}
			req := &ResolveVolumeMountsReq{Workspace: ws, RegionRef: &metav1.ObjectReference{Uid: "region-uid"}}
			resources := &volumeMountResources{volumes: map[string]*cordiumv1.Volume{"data": {Metadata: &metav1.Metadata{Name: "data", Uid: "volume-uid"}, Status: &cordiumv1.Volume_Status{SpaceRef: &metav1.ObjectReference{Uid: "space-uid"}, RegionRef: req.RegionRef}}}}
			tc.change(req, resources)
			got, err := ResolveVolumeMounts(t.Context(), &volumeMountClient{resources: resources}, req)
			assert.Equal(t, tc.wantCalls, resources.calls)
			if tc.wantError {
				assert.NotNil(t, err)
				assert.Nil(t, got)
				if resources.err != nil {
					assert.ErrorIs(t, err, resources.err)
				}
			} else if assert.Nil(t, err) && tc.wantCalls > 0 {
				assert.Equal(t, "volume-uid", got.Mounts[0].VolumeRef.Uid)
				assert.Equal(t, "/data", got.Mounts[0].MountPath)
				assert.True(t, got.Mounts[0].ReadOnly)
			}
		})
	}
}

func TestResolveVolumeMountsSortsPaths(t *testing.T) {
	ws := workspaceValidationReq().Workspace
	ws.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
		{VolumeRef: &metav1.ObjectReference{Name: "z"}, MountPath: "/z"},
		{VolumeRef: &metav1.ObjectReference{Name: "a"}, MountPath: "/a", ReadOnly: true},
	}}
	resources := &volumeMountResources{volumes: map[string]*cordiumv1.Volume{}}
	for _, name := range []string{"a", "z"} {
		resources.volumes[name] = &cordiumv1.Volume{Metadata: &metav1.Metadata{Name: name, Uid: name}, Status: &cordiumv1.Volume_Status{SpaceRef: &metav1.ObjectReference{Uid: "space-uid"}, RegionRef: &metav1.ObjectReference{Uid: "region-uid"}}}
	}
	got, err := ResolveVolumeMounts(t.Context(), &volumeMountClient{resources: resources}, &ResolveVolumeMountsReq{Workspace: ws})
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, "/a", got.Mounts[0].MountPath)
	assert.Equal(t, "/z", got.Mounts[1].MountPath)
	assert.Equal(t, "z", ws.Spec.Runtime.VolumeMounts[0].VolumeRef.Name)
	region, err := got.GetRegionRef()
	assert.Nil(t, err)
	assert.Equal(t, "region-uid", region.Uid)
	got.Volumes[1].Status.RegionRef = &metav1.ObjectReference{Uid: "another"}
	_, err = got.GetRegionRef()
	assert.NotNil(t, err)
	got.Volumes[1].Status.RegionRef = nil
	_, err = got.GetRegionRef()
	assert.NotNil(t, err)
	region, err = (&ResolveVolumeMountsResp{}).GetRegionRef()
	assert.Nil(t, err)
	assert.Nil(t, region)
}

func TestResolveVolumeMountsRejectsAliases(t *testing.T) {
	ws := workspaceValidationReq().Workspace
	ws.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{VolumeMounts: []*cordiumv1.Workspace_Spec_Runtime_VolumeMount{
		{VolumeRef: &metav1.ObjectReference{Name: "data"}, MountPath: "/data"},
		{VolumeRef: &metav1.ObjectReference{Uid: "volume-uid"}, MountPath: "/copy"},
	}}
	vol := &cordiumv1.Volume{Metadata: &metav1.Metadata{Name: "data", Uid: "volume-uid"}, Status: &cordiumv1.Volume_Status{SpaceRef: ws.Status.SpaceRef}}
	resources := &volumeMountResources{volumes: map[string]*cordiumv1.Volume{"data": vol, "volume-uid": vol}}
	_, err := ResolveVolumeMounts(t.Context(), &volumeMountClient{resources: resources}, &ResolveVolumeMountsReq{Workspace: ws})
	assert.NotNil(t, err)
}
