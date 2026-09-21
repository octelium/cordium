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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVolumeMount(t *testing.T) {

	t.Run("a Volume is mounted read-write by default", func(t *testing.T) {
		mount, err := parseVolumeMount("datasets:/data")
		require.Nil(t, err, "%+v", err)

		assert.Equal(t, "datasets", mount.VolumeRef.Name)
		assert.Equal(t, "/data", mount.MountPath)
		assert.False(t, mount.ReadOnly)
	})

	t.Run("a qualified Volume name is kept as is", func(t *testing.T) {
		mount, err := parseVolumeMount("datasets.my-project:/data")
		require.Nil(t, err, "%+v", err)

		assert.Equal(t, "datasets.my-project", mount.VolumeRef.Name)
	})

	t.Run("the mode is parsed", func(t *testing.T) {
		mount, err := parseVolumeMount("datasets:/data:ro")
		require.Nil(t, err, "%+v", err)
		assert.True(t, mount.ReadOnly)

		mount, err = parseVolumeMount("datasets:/data:rw")
		require.Nil(t, err, "%+v", err)
		assert.False(t, mount.ReadOnly)
	})

	t.Run("an invalid value is rejected", func(t *testing.T) {
		for _, raw := range []string{
			"",
			"datasets",
			":/data",
			"datasets:",
			"datasets:/data:readonly",
			"datasets:/data:ro:rw",
		} {
			_, err := parseVolumeMount(raw)
			assert.NotNil(t, err, "%q", raw)
		}
	})
}
