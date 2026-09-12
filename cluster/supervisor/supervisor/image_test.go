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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPathWithin(t *testing.T) {

	const baseDir = "/octelium/build"

	t.Run("paths inside the build dir are resolved", func(t *testing.T) {
		for _, tc := range []struct {
			arg      string
			expected string
		}{
			{".", baseDir},
			{"", baseDir},
			{"Dockerfile", "/octelium/build/Dockerfile"},
			{".devcontainer", "/octelium/build/.devcontainer"},
			{"src/../Dockerfile", "/octelium/build/Dockerfile"},
			{"/octelium/build/.devcontainer/Dockerfile", "/octelium/build/.devcontainer/Dockerfile"},
			{baseDir, baseDir},
		} {
			ret, err := checkPathWithin(baseDir, tc.arg)
			assert.Nil(t, err, "path should be accepted: %q", tc.arg)
			assert.Equal(t, tc.expected, ret)
		}
	})

	t.Run("paths escaping the build dir are rejected", func(t *testing.T) {
		for _, arg := range []string{
			"..",
			"../../octelium",
			"../build-other",
			"a/../../../etc/passwd",
			"/octelium",
			"/etc/passwd",
			"/octelium/buildx",
		} {
			_, err := checkPathWithin(baseDir, arg)
			assert.NotNil(t, err, "path should be rejected: %q", arg)
		}
	})
}
