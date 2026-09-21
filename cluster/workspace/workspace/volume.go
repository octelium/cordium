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
	"slices"
	"strings"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var volumeRootDir = workspacecommon.VolumeRootDir

var volumeStatePath = "/run/octelium/volume-mounts"

func getVolumePath(uid string) string {
	return path.Join(volumeRootDir, uid)
}

func (s *Server) setVolumeMounts() error {

	mounts := s.initReq.GetVolumeMounts()

	zap.L().Debug("Setting the Volume mounts", zap.Int("mounts", len(mounts)))

	if err := s.removeStaleVolumeMounts(mounts); err != nil {
		zap.L().Warn("Could not remove the stale Volume mounts", zap.Error(err))
	}

	for _, mount := range mounts {
		if err := s.setVolumeMount(mount); err != nil {
			return errors.Errorf("Could not mount the Volume at %s: %+v", mount.MountPath, err)
		}
	}

	return s.setVolumeMountState(mounts)
}

func (s *Server) setVolumeMount(mount *ccordiumv1.ResolvedVolumeMount) error {

	src := getVolumePath(mount.VolumeRef.Uid)

	if _, err := os.Stat(src); err != nil {
		return errors.Errorf("The Volume storage is not attached to the Workspace: %+v", err)
	}

	if err := s.initVolumeOwnership(src, mount); err != nil {
		zap.L().Warn("Could not initialize the ownership of the Volume",
			zap.String("mountPath", mount.MountPath), zap.Error(err))
	}

	if cur, err := os.Readlink(mount.MountPath); err == nil {
		if cur == src {
			zap.L().Debug("The Volume is already mounted", zap.String("mountPath", mount.MountPath))
			return nil
		}

		if err := os.Remove(mount.MountPath); err != nil {
			return err
		}
	} else if info, err := os.Lstat(mount.MountPath); err == nil {
		if !info.IsDir() {
			return errors.Errorf("The mountPath already exists inside the Workspace and it is not a directory")
		}

		entries, err := os.ReadDir(mount.MountPath)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return errors.Errorf("The mountPath already exists inside the Workspace and it is not empty")
		}

		if err := os.Remove(mount.MountPath); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(path.Dir(mount.MountPath), 0755); err != nil {
		return err
	}

	zap.L().Debug("Mounting the Volume",
		zap.String("mountPath", mount.MountPath), zap.String("src", src))

	return os.Symlink(src, mount.MountPath)
}

func (s *Server) initVolumeOwnership(src string, mount *ccordiumv1.ResolvedVolumeMount) error {

	if mount.ReadOnly || s.userInfo == nil {
		return nil
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if len(entries) > 0 {
		return nil
	}

	zap.L().Debug("Initializing the ownership of the Volume",
		zap.String("mountPath", mount.MountPath), zap.String("user", s.userInfo.name))

	if err := os.Chown(src, s.userInfo.uid, s.userInfo.gid); err != nil {
		return err
	}

	return os.Chmod(src, 0775)
}

func (s *Server) removeStaleVolumeMounts(mounts []*ccordiumv1.ResolvedVolumeMount) error {

	isCurrent := func(mountPath, target string) bool {
		for _, mount := range mounts {
			if mount.MountPath == mountPath &&
				target == getVolumePath(mount.VolumeRef.Uid) {
				return true
			}
		}
		return false
	}

	for _, mountPath := range getVolumeMountState() {
		target, err := os.Readlink(mountPath)
		if err != nil {
			continue
		}

		if !strings.HasPrefix(target, volumeRootDir+"/") {
			continue
		}

		if isCurrent(mountPath, target) {
			continue
		}

		zap.L().Debug("Unmounting a Volume that is not mounted by the Workspace anymore",
			zap.String("mountPath", mountPath))

		if err := os.Remove(mountPath); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) setVolumeMountState(mounts []*ccordiumv1.ResolvedVolumeMount) error {

	if len(mounts) == 0 {
		if err := os.Remove(volumeStatePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	paths := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		paths = append(paths, mount.MountPath)
	}
	slices.Sort(paths)

	if err := os.MkdirAll(path.Dir(volumeStatePath), 0755); err != nil {
		return err
	}

	return os.WriteFile(volumeStatePath, []byte(strings.Join(paths, "\n")), 0644)
}

func getVolumeMountState() []string {
	content, err := os.ReadFile(volumeStatePath)
	if err != nil {
		return nil
	}

	var ret []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ret = append(ret, line)
		}
	}

	return ret
}
