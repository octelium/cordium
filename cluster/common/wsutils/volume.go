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
	"path"
	"strings"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
)

const MaxVolumeMountsPerWorkspace = 16

var reservedVolumeMountPaths = []string{
	"/",
	"/bin",
	"/boot",
	"/dev",
	"/etc",
	"/lib",
	"/lib32",
	"/lib64",
	"/proc",
	"/root",
	"/run",
	"/sbin",
	"/sys",
	"/tmp",
	"/usr",
	"/var",
	"/var/lib/containers",
	"/var/lib/docker",
	"/var/run",
	"/var/tmp",
	"/workspace",
	workspacecommon.VolumeRootDir,
}

func CheckVolumeMountPath(arg string) error {
	if arg == "" {
		return serr.InvalidArg("Volume mountPath is empty")
	}
	if len(arg) > 256 {
		return serr.InvalidArg("Volume mountPath is too long: %s", arg)
	}
	if !strings.HasPrefix(arg, "/") {
		return serr.InvalidArg("Volume mountPath must be absolute: %s", arg)
	}
	if strings.Contains(arg, `\`) || strings.ContainsRune(arg, 0) {
		return serr.InvalidArg("Invalid Volume mountPath: %s", arg)
	}
	if path.Clean(arg) != arg {
		return serr.InvalidArg(
			"Volume mountPath must be a clean canonical path (e.g. %s): %s", path.Clean(arg), arg)
	}

	for _, reserved := range reservedVolumeMountPaths {
		if arg == reserved {
			return serr.InvalidArg("The Volume mountPath: %s is reserved by the Cluster", arg)
		}
	}

	if isSubPath(workspacecommon.VolumeRootDir, arg) {
		return serr.InvalidArg("The Volume mountPath: %s is reserved by the Cluster", arg)
	}

	return nil
}

func isSubPath(parent, arg string) bool {
	return strings.HasPrefix(arg, parent+"/")
}

func ValidateVolumeMounts(mounts []*cordiumv1.Workspace_Spec_Runtime_VolumeMount) error {
	if len(mounts) == 0 {
		return nil
	}

	if len(mounts) > MaxVolumeMountsPerWorkspace {
		return serr.InvalidArg("Too many Volume mounts")
	}

	var paths []string
	var volumes []string

	for _, mount := range mounts {
		if mount.VolumeRef == nil || (mount.VolumeRef.Name == "" && mount.VolumeRef.Uid == "") {
			return serr.InvalidArg("The volumeRef of a Volume mount is not set")
		}

		if err := CheckVolumeMountPath(mount.MountPath); err != nil {
			return err
		}

		for _, pth := range paths {
			switch {
			case pth == mount.MountPath:
				return serr.InvalidArg("The Volume mountPath: %s is already used", mount.MountPath)
			case isSubPath(pth, mount.MountPath), isSubPath(mount.MountPath, pth):
				return serr.InvalidArg("The Volume mountPaths: %s and %s overlap", pth, mount.MountPath)
			}
		}
		paths = append(paths, mount.MountPath)

		key := mount.VolumeRef.Uid
		if key == "" {
			key = mount.VolumeRef.Name
		}
		if isInList(volumes, key) {
			return serr.InvalidArg("The Volume: %s is mounted more than once", key)
		}
		volumes = append(volumes, key)
	}

	return nil
}
