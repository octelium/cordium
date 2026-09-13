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

package suite

import (
	"fmt"
	"strings"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorkspaceFilesystem(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	name := h.Name()

	writable := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})

	t.Run("TheRootFilesystemIsWritableByDefault", func(t *testing.T) {
		res := h.Exec(t, writable, charness.ExecOpts{
			Command:   fmt.Sprintf("touch /e2e-root-%s", name),
			RunAsRoot: true,
		})
		assert.Equal(t, int32(0), res.Code,
			"the root filesystem is not writable: %s", res.Stderr)
	})

	t.Run("TheWorkspaceDirIsWritable", func(t *testing.T) {
		file := workspacePath("e2e-rw-" + name)

		h.MustExec(t, writable, fmt.Sprintf("echo rw > %s", file))
		assert.Equal(t, "rw", h.MustExec(t, writable, "cat "+file))
	})

	t.Run("TheTempDirsAreWritable", func(t *testing.T) {
		for _, dir := range []string{"/tmp", "/var/tmp"} {
			file := fmt.Sprintf("%s/e2e-%s", dir, name)

			res := h.Exec(t, writable, charness.ExecOpts{
				Command: fmt.Sprintf("echo tmp > %s && cat %s", file, file),
			})
			require.Equal(t, int32(0), res.Code,
				"the dir %s is not writable: %s", dir, res.Stderr)
			assert.Equal(t, "tmp", res.Out())
		}
	})

	readOnly := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Filesystem: readOnlyFilesystem(),
		},
	})

	t.Run("TheReadOnlyWorkspaceStillRuns", func(t *testing.T) {
		cur := h.GetWorkspace(t, readOnly)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, cur.Status.State)
		assert.Nil(t, cur.Status.Failure)
		assert.Equal(t, "cordium-e2e", h.MustExec(t, readOnly, "echo cordium-e2e"))
	})

	t.Run("TheReadOnlyRootFilesystemRefusesWrites", func(t *testing.T) {
		res := h.Exec(t, readOnly, charness.ExecOpts{
			Command:   fmt.Sprintf("touch /e2e-root-%s", name),
			RunAsRoot: true,
		})
		assert.NotEqual(t, int32(0), res.Code,
			"the root filesystem of a readOnly Workspace accepted a write")

		res = h.Exec(t, readOnly, charness.ExecOpts{
			Command:   fmt.Sprintf("touch /etc/e2e-%s", name),
			RunAsRoot: true,
		})
		assert.NotEqual(t, int32(0), res.Code,
			"/etc of a readOnly Workspace accepted a write")
	})

	t.Run("TheReadOnlyWorkspaceKeepsItsWritableVolumes", func(t *testing.T) {
		for _, dir := range []string{"/tmp", "/var/tmp"} {
			file := fmt.Sprintf("%s/e2e-ro-%s", dir, name)

			res := h.Exec(t, readOnly, charness.ExecOpts{
				Command: fmt.Sprintf("echo volume > %s && cat %s", file, file),
			})
			require.Equal(t, int32(0), res.Code,
				"the volume %s is not writable in a readOnly Workspace: %s",
				dir, res.Stderr)
			assert.Equal(t, "volume", res.Out())
		}
	})

	t.Run("TheReadOnlyFlagReachesTheRuntime", func(t *testing.T) {
		out := rootMountOptions(t, h, readOnly)
		assert.Contains(t, strings.Split(out, ","), "ro",
			"the root mount of a readOnly Workspace is not mounted read-only: %q", out)
	})

	t.Run("TheDefaultWorkspaceIsNotReadOnly", func(t *testing.T) {
		out := rootMountOptions(t, h, writable)
		assert.Contains(t, strings.Split(out, ","), "rw",
			"the root mount of a default Workspace is not mounted read-write: %q", out)
	})

	t.Run("TheWorkspaceUserOwnsItsHomeAndWorkspaceDirs", func(t *testing.T) {
		assert.Equal(t, "owned",
			h.MustExec(t, writable, "test -O ${HOME} && echo owned"))
		assert.Equal(t, "owned",
			h.MustExec(t, writable, fmt.Sprintf("test -O %s && echo owned", workspaceDir)))
	})

	t.Run("TheWorkspaceUserCanEscalateWithSudo", func(t *testing.T) {
		res := h.Exec(t, writable, charness.ExecOpts{Command: "sudo -n id -u"})
		require.Equal(t, int32(0), res.Code,
			"the Workspace user cannot use sudo: %s", res.Stderr)
		assert.Equal(t, "0", res.Out())
	})
}

func rootMountOptions(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) string {
	t.Helper()

	return h.MustExec(t, ws, "grep -m1 ' / / ' /proc/self/mountinfo | cut -d' ' -f6")
}
