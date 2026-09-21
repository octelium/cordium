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

package supervisor

import (
	"fmt"
	"os"
	"path"
	"syscall"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"go.uber.org/zap"
)

var volumeRootDir = workspacecommon.VolumeRootDir

func (s *Server) prepareVolumeRoots() {

	entries, err := os.ReadDir(volumeRootDir)
	if err != nil {
		if !os.IsNotExist(err) {
			zap.L().Warn("Could not read the Volume root dir", zap.Error(err))
		}
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pth := path.Join(volumeRootDir, entry.Name())

		info, err := os.Stat(pth)
		if err != nil {
			zap.L().Warn("Could not stat the Volume dir", zap.String("path", pth), zap.Error(err))
			continue
		}

		if stat, ok := info.Sys().(*syscall.Stat_t); ok && s.isMappedID(stat.Uid) {
			continue
		}

		if !isEmptyDir(pth) {
			zap.L().Debug("The Volume is already initialized. Leaving its ownership alone",
				zap.String("path", pth))
			continue
		}

		zap.L().Debug("Initializing the ownership of the Volume dir", zap.String("path", pth))

		if err := os.Chown(pth, s.octeliumUID, s.octeliumGID); err != nil {
			zap.L().Debug("Could not chown the Volume dir. It is most probably mounted read-only",
				zap.String("path", pth), zap.Error(err))
		}
	}
}

func (s *Server) isMappedID(id uint32) bool {
	return id >= uint32(s.octeliumUID) &&
		id < uint32(s.octeliumUID)+1+subordinateIDCount
}

func isEmptyDir(pth string) bool {
	entries, err := os.ReadDir(pth)
	if err != nil {
		return false
	}

	return len(entries) == 0
}

func getVolumeRootMountArg() string {
	return fmt.Sprintf("%s:%s:rbind,nodev,nosuid",
		workspacecommon.VolumeRootDir, workspacecommon.VolumeRootDir)
}
