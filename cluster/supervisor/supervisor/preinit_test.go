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
	"strings"
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultIfaceAddr(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	/*
		_, err = getDefaultIfaceAddr()
		assert.Nil(t, err)
	*/
}

func TestPodmanVolatileState(t *testing.T) {

	srv := &Server{octeliumUID: 543210, octeliumGID: 543210}

	args := srv.getPodmanVolatileArgs()
	require.Equal(t, 0, len(args)%2, args)

	tmpfsMounts := make(map[string][]string)
	var volumes []string
	for i := 0; i < len(args); i += 2 {
		switch args[i] {
		case "--tmpfs":
			dst, opts, ok := strings.Cut(args[i+1], ":")
			require.True(t, ok, args[i+1])
			tmpfsMounts[dst] = strings.Split(opts, ",")
		case "-v":
			volumes = append(volumes, args[i+1])
		default:
			require.Failf(t, "unexpected podman arg", "%s %s", args[i], args[i+1])
		}
	}

	t.Run("the runtime dir is unchanged for the existing podman databases", func(t *testing.T) {
		assert.Equal(t, "/tmp/storage-run-543210", srv.getPodmanRuntimeDir())
		assert.Equal(t, "/octelium/podman/tmp-storage", podmanRunRoot)
		assert.Contains(t, podmanConfStorage, fmt.Sprintf("\nrunroot = %q\n", podmanRunRoot))
	})

	t.Run("podman resolves its runtime dir to the tmpfs mount", func(t *testing.T) {
		assert.Equal(t, srv.getPodmanRuntimeDir(), srv.getDefaultCmdEnvAsOctelium()["XDG_RUNTIME_DIR"])
	})

	t.Run("the podman volatile state never survives a restart", func(t *testing.T) {
		require.Equal(t, 2, len(tmpfsMounts))

		for _, dir := range []string{podmanRunRoot, srv.getPodmanRuntimeDir()} {
			opts, ok := tmpfsMounts[dir]
			require.True(t, ok, "%s must be a tmpfs mount", dir)

			assert.Contains(t, opts, "notmpcopyup",
				"copying up the persistent dir underneath %s would restore the stale podman state", dir)
			assert.Contains(t, opts, "rw")
			assert.Contains(t, opts, "nosuid")
		}
	})

	t.Run("the tmpfs mounts keep the flags of the dirs they replace", func(t *testing.T) {
		assert.NotContains(t, tmpfsMounts[podmanRunRoot], "noexec")
		assert.Contains(t, tmpfsMounts[podmanRunRoot], "dev")
		assert.Contains(t, tmpfsMounts[podmanRunRoot], "mode=0755")

		assert.Contains(t, tmpfsMounts[srv.getPodmanRuntimeDir()], "noexec")
		assert.Contains(t, tmpfsMounts[srv.getPodmanRuntimeDir()], "dev")
		assert.Contains(t, tmpfsMounts[srv.getPodmanRuntimeDir()], "mode=0700",
			"rootless podman requires its runtime dir to be writable only by its owner")
	})

	t.Run("the outer graph root is not reported as mounted in the inner container", func(t *testing.T) {
		assert.Equal(t, []string{
			fmt.Sprintf("%s:/run/.containerenv:ro", podmanContainerEnvPath),
		}, volumes)
	})
}
