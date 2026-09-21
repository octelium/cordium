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
	"fmt"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/stretchr/testify/assert"
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
