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
	"strings"
	"testing"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVolumeRootMountArg(t *testing.T) {

	arg := getVolumeRootMountArg()

	parts := strings.Split(arg, ":")
	require.Equal(t, 3, len(parts), arg)

	assert.Equal(t, workspacecommon.VolumeRootDir, parts[0])
	assert.Equal(t, workspacecommon.VolumeRootDir, parts[1],
		"the Volume root must be mounted at the same path inside the Workspace container so that the mount survives a restart")

	for _, opt := range []string{"rbind", "nodev", "nosuid"} {
		assert.Contains(t, strings.Split(parts[2], ","), opt)
	}
}

func TestPrepareVolumeRoots(t *testing.T) {

	uid := os.Getuid()
	gid := os.Getgid()

	t.Run("a missing Volume root dir is tolerated", func(t *testing.T) {
		srv := &Server{octeliumUID: uid, octeliumGID: gid}
		srv.prepareVolumeRoots()
	})

	t.Run("the already owned Volume dirs are left alone", func(t *testing.T) {
		root := t.TempDir()

		oldRoot := volumeRootDir
		t.Cleanup(func() { volumeRootDir = oldRoot })
		volumeRootDir = root

		for i := 0; i < 3; i++ {
			require.Nil(t, os.MkdirAll(path.Join(root, fmt.Sprintf("vol%d", i)), 0755))
		}
		require.Nil(t, os.WriteFile(path.Join(root, "not-a-dir"), []byte("x"), 0644))

		srv := &Server{octeliumUID: uid, octeliumGID: gid}
		srv.prepareVolumeRoots()

		entries, err := os.ReadDir(root)
		require.Nil(t, err, "%+v", err)
		assert.Equal(t, 4, len(entries))
	})

	t.Run("an unownable Volume dir does not abort the initialization", func(t *testing.T) {
		root := t.TempDir()

		oldRoot := volumeRootDir
		t.Cleanup(func() { volumeRootDir = oldRoot })
		volumeRootDir = root

		require.Nil(t, os.MkdirAll(path.Join(root, "vol0"), 0755))

		srv := &Server{octeliumUID: uid + 1, octeliumGID: gid + 1}
		srv.prepareVolumeRoots()

		_, err := os.Stat(path.Join(root, "vol0"))
		assert.Nil(t, err, "%+v", err)
	})
}
