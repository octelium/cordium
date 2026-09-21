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

package workspace

import (
	"os"
	"path"
	"testing"

	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/stretchr/testify/assert"
)

type volumeTest struct {
	srv  *Server
	root string
}

func newVolumeTest(t *testing.T) *volumeTest {
	root := t.TempDir()

	oldRootDir, oldStatePath := volumeRootDir, volumeStatePath
	t.Cleanup(func() {
		volumeRootDir, volumeStatePath = oldRootDir, oldStatePath
	})

	volumeRootDir = path.Join(root, "cordium-volumes")
	volumeStatePath = path.Join(root, "run", "volume-mounts")

	assert.Nil(t, os.MkdirAll(volumeRootDir, 0755))

	return &volumeTest{
		srv: &Server{
			initReq: &ccordiumv1.PrepareRequest{},
		},
		root: root,
	}
}

func (c *volumeTest) newVolume(t *testing.T, mountPath string, readOnly bool) *ccordiumv1.ResolvedVolumeMount {
	uid := vutils.UUIDv4()

	assert.Nil(t, os.MkdirAll(path.Join(volumeRootDir, uid), 0755))

	return &ccordiumv1.ResolvedVolumeMount{
		VolumeRef: &metav1.ObjectReference{Uid: uid},
		MountPath: path.Join(c.root, mountPath),
		ReadOnly:  readOnly,
	}
}

func (c *volumeTest) setMounts(mounts ...*ccordiumv1.ResolvedVolumeMount) {
	c.srv.initReq.VolumeMounts = mounts
}

func TestSetVolumeMounts(t *testing.T) {

	t.Run("a Workspace without Volumes mounts nothing", func(t *testing.T) {
		c := newVolumeTest(t)

		assert.Nil(t, c.srv.setVolumeMounts())
		assert.Equal(t, 0, len(getVolumeMountState()))
	})

	t.Run("a Volume is mounted at its mountPath", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)
		c.setMounts(mount)

		assert.Nil(t, c.srv.setVolumeMounts())

		target, err := os.Readlink(mount.MountPath)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, path.Join(volumeRootDir, mount.VolumeRef.Uid), target)

		assert.Nil(t, os.WriteFile(path.Join(mount.MountPath, "f"), []byte("x"), 0644))

		content, err := os.ReadFile(path.Join(volumeRootDir, mount.VolumeRef.Uid, "f"))
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, "x", string(content))

		assert.Equal(t, []string{mount.MountPath}, getVolumeMountState())
	})

	t.Run("the missing parent directories of a mountPath are created", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "srv/a/b/data", false)
		c.setMounts(mount)

		assert.Nil(t, c.srv.setVolumeMounts())

		_, err := os.Readlink(mount.MountPath)
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("mounting is idempotent", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)
		c.setMounts(mount)

		for i := 0; i < 3; i++ {
			assert.Nil(t, c.srv.setVolumeMounts())
		}

		target, err := os.Readlink(mount.MountPath)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, path.Join(volumeRootDir, mount.VolumeRef.Uid), target)
	})

	t.Run("an existing empty directory is replaced", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)
		assert.Nil(t, os.MkdirAll(mount.MountPath, 0755))
		c.setMounts(mount)

		assert.Nil(t, c.srv.setVolumeMounts())

		_, err := os.Readlink(mount.MountPath)
		assert.Nil(t, err, "%+v", err)
	})

	t.Run("an existing non empty directory is not replaced", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)
		assert.Nil(t, os.MkdirAll(mount.MountPath, 0755))
		assert.Nil(t, os.WriteFile(path.Join(mount.MountPath, "f"), []byte("x"), 0644))
		c.setMounts(mount)

		assert.NotNil(t, c.srv.setVolumeMounts())
	})

	t.Run("a Volume whose storage is not attached is rejected", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)
		assert.Nil(t, os.RemoveAll(path.Join(volumeRootDir, mount.VolumeRef.Uid)))
		c.setMounts(mount)

		assert.NotNil(t, c.srv.setVolumeMounts())
	})

	t.Run("a detached Volume is unmounted on the next run", func(t *testing.T) {
		c := newVolumeTest(t)
		mount1 := c.newVolume(t, "data", false)
		mount2 := c.newVolume(t, "cache", false)

		c.setMounts(mount1, mount2)
		assert.Nil(t, c.srv.setVolumeMounts())

		_, err := os.Readlink(mount1.MountPath)
		assert.Nil(t, err, "%+v", err)
		_, err = os.Readlink(mount2.MountPath)
		assert.Nil(t, err, "%+v", err)

		c.setMounts(mount1)
		assert.Nil(t, c.srv.setVolumeMounts())

		_, err = os.Readlink(mount1.MountPath)
		assert.Nil(t, err, "%+v", err)

		_, err = os.Lstat(mount2.MountPath)
		assert.True(t, os.IsNotExist(err), "%+v", err)

		assert.Equal(t, []string{mount1.MountPath}, getVolumeMountState())
	})

	t.Run("every Volume is unmounted once they are all detached", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)

		c.setMounts(mount)
		assert.Nil(t, c.srv.setVolumeMounts())

		c.setMounts()
		assert.Nil(t, c.srv.setVolumeMounts())

		_, err := os.Lstat(mount.MountPath)
		assert.True(t, os.IsNotExist(err), "%+v", err)
		assert.Equal(t, 0, len(getVolumeMountState()))
	})

	t.Run("a mountPath that is repointed to another Volume is remounted", func(t *testing.T) {
		c := newVolumeTest(t)
		mount1 := c.newVolume(t, "data", false)

		c.setMounts(mount1)
		assert.Nil(t, c.srv.setVolumeMounts())

		mount2 := c.newVolume(t, "data", false)
		c.setMounts(mount2)
		assert.Nil(t, c.srv.setVolumeMounts())

		target, err := os.Readlink(mount2.MountPath)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, path.Join(volumeRootDir, mount2.VolumeRef.Uid), target)
	})

	t.Run("the ownership of an empty Volume is initialized once", func(t *testing.T) {
		c := newVolumeTest(t)
		c.srv.userInfo = &userInfo{
			name: "octelium",
			uid:  os.Getuid(),
			gid:  os.Getgid(),
		}

		mount := c.newVolume(t, "data", false)
		src := path.Join(volumeRootDir, mount.VolumeRef.Uid)
		assert.Nil(t, os.Chmod(src, 0700))

		c.setMounts(mount)
		assert.Nil(t, c.srv.setVolumeMounts())

		info, err := os.Stat(src)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, os.FileMode(0775), info.Mode().Perm())
	})

	t.Run("the ownership of a Volume that has content is left alone", func(t *testing.T) {
		c := newVolumeTest(t)
		c.srv.userInfo = &userInfo{
			name: "octelium",
			uid:  os.Getuid(),
			gid:  os.Getgid(),
		}

		mount := c.newVolume(t, "data", false)
		src := path.Join(volumeRootDir, mount.VolumeRef.Uid)
		assert.Nil(t, os.WriteFile(path.Join(src, "f"), []byte("x"), 0644))
		assert.Nil(t, os.Chmod(src, 0700))

		c.setMounts(mount)
		assert.Nil(t, c.srv.setVolumeMounts())

		info, err := os.Stat(src)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
	})

	t.Run("the ownership of a read-only Volume is never touched", func(t *testing.T) {
		c := newVolumeTest(t)
		c.srv.userInfo = &userInfo{
			name: "octelium",
			uid:  os.Getuid(),
			gid:  os.Getgid(),
		}

		mount := c.newVolume(t, "data", true)
		src := path.Join(volumeRootDir, mount.VolumeRef.Uid)
		assert.Nil(t, os.Chmod(src, 0555))

		c.setMounts(mount)
		assert.Nil(t, c.srv.setVolumeMounts())

		info, err := os.Stat(src)
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, os.FileMode(0555), info.Mode().Perm())
	})

	t.Run("a path that is not managed by the Cluster is never removed", func(t *testing.T) {
		c := newVolumeTest(t)
		mount := c.newVolume(t, "data", false)

		c.setMounts(mount)
		assert.Nil(t, c.srv.setVolumeMounts())

		assert.Nil(t, os.Remove(mount.MountPath))
		assert.Nil(t, os.MkdirAll(mount.MountPath, 0755))
		assert.Nil(t, os.WriteFile(path.Join(mount.MountPath, "f"), []byte("x"), 0644))

		c.setMounts()
		assert.Nil(t, c.srv.setVolumeMounts())

		_, err := os.Stat(path.Join(mount.MountPath, "f"))
		assert.Nil(t, err, "%+v", err)
	})
}
