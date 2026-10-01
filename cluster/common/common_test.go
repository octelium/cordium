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

package workspacecommon

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsEmptyVolumeDir(t *testing.T) {

	t.Run("an empty Volume dir is empty", func(t *testing.T) {
		pth := t.TempDir()

		isEmpty, err := IsEmptyVolumeDir(pth)
		assert.Nil(t, err, "%+v", err)
		assert.True(t, isEmpty)
	})

	t.Run("a freshly formatted ext4 Volume dir is empty", func(t *testing.T) {
		pth := t.TempDir()
		require.Nil(t, os.Mkdir(path.Join(pth, "lost+found"), 0700))

		isEmpty, err := IsEmptyVolumeDir(pth)
		assert.Nil(t, err, "%+v", err)
		assert.True(t, isEmpty)
	})

	t.Run("a Volume dir that has content is not empty", func(t *testing.T) {
		pth := t.TempDir()
		require.Nil(t, os.WriteFile(path.Join(pth, "f"), []byte("x"), 0644))

		isEmpty, err := IsEmptyVolumeDir(pth)
		assert.Nil(t, err, "%+v", err)
		assert.False(t, isEmpty)
	})

	t.Run("an ext4 Volume dir that has content is not empty", func(t *testing.T) {
		pth := t.TempDir()
		require.Nil(t, os.Mkdir(path.Join(pth, "lost+found"), 0700))
		require.Nil(t, os.Mkdir(path.Join(pth, "d"), 0755))

		isEmpty, err := IsEmptyVolumeDir(pth)
		assert.Nil(t, err, "%+v", err)
		assert.False(t, isEmpty)
	})

	t.Run("a lost+found that is not a directory is content", func(t *testing.T) {
		pth := t.TempDir()
		require.Nil(t, os.WriteFile(path.Join(pth, "lost+found"), []byte("x"), 0644))

		isEmpty, err := IsEmptyVolumeDir(pth)
		assert.Nil(t, err, "%+v", err)
		assert.False(t, isEmpty)
	})

	t.Run("a missing Volume dir is an error", func(t *testing.T) {
		isEmpty, err := IsEmptyVolumeDir(path.Join(t.TempDir(), "missing"))
		assert.NotNil(t, err)
		assert.False(t, isEmpty)
	})
}
