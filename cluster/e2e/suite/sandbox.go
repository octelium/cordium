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
	"strconv"
	"strings"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorkspaceSandbox(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})

	t.Run("TheSandboxRunsInItsOwnUserNamespace", func(t *testing.T) {
		out := h.MustExec(t, ws, "cat /proc/self/uid_map")
		require.NotEmpty(t, out)

		fields := strings.Fields(strings.Split(out, "\n")[0])
		require.Len(t, fields, 3, "unexpected uid_map %q", out)

		assert.Equal(t, "0", fields[0],
			"the sandbox does not map its own root to a host subordinate uid: %q", out)
		assert.NotEqual(t, "0", fields[1],
			"the sandbox root is mapped to the real host root: %q", out)
	})

	t.Run("TheSandboxHasItsOwnNetworkNamespace", func(t *testing.T) {
		supervisorIP := workspacePodIP(t, h, ws)
		require.NotEmpty(t, supervisorIP)

		out := h.MustExec(t, ws, "ip -4 -o addr show")
		assert.NotContains(t, out, supervisorIP,
			"the sandbox shares the supervisor pod network namespace")
	})

	t.Run("TheSandboxHasItsOwnPidNamespace", func(t *testing.T) {
		out := h.MustExec(t, ws, "ls -d /proc/[0-9]* | wc -l")

		count, err := strconv.Atoi(out)
		require.Nil(t, err, "unexpected process count %q", out)
		assert.Less(t, count, 200,
			"the sandbox appears to see the whole host process table")

		comms := h.MustExec(t, ws, "cat /proc/[0-9]*/comm 2>/dev/null | sort -u")
		assert.NotContains(t, comms, "cordium-supervisor",
			"the supervisor process is visible from inside the sandbox")
		assert.NotContains(t, comms, "conmon",
			"the rootless podman conmon processes are visible inside the sandbox")
	})

	t.Run("TheSandboxCannotSeeTheSupervisorMounts", func(t *testing.T) {
		out := h.MustExec(t, ws, "cat /proc/self/mountinfo")

		assert.NotContains(t, out, "/octelium-net",
			"the network policy socket dir is mounted into the sandbox")
		assert.NotContains(t, out, "/octelium-root",
			"the outer podman storage is mounted into the sandbox")
	})

	t.Run("TheSandboxRunsUnderTheWorkspaceCgroup", func(t *testing.T) {
		out := h.MustExec(t, ws, "cat /proc/self/cgroup")
		assert.Contains(t, out, "0::", "unexpected cgroup format %q", out)

		controllers := strings.Fields(
			h.MustExec(t, ws, "cat /sys/fs/cgroup/cgroup.controllers"))
		for _, controller := range []string{"cpu", "memory", "pids"} {
			assert.Contains(t, controllers, controller)
		}
	})

	t.Run("TheCgroupLimitsMatchTheWorkspaceLimits", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		require.NotNil(t, cur.Status.Limit)
		require.NotNil(t, cur.Status.Limit.Memory)

		out := h.MustExec(t, ws, "cat /sys/fs/cgroup/memory.max")
		require.NotEqual(t, "max", out)

		limit, err := strconv.ParseInt(out, 10, 64)
		require.Nil(t, err, "unexpected memory.max %q", out)
		assert.LessOrEqual(t, limit,
			int64(cur.Status.Limit.Memory.Megabytes)*1024*1024)
	})

	t.Run("TheSandboxHoldsItsNamespacedNetworkCapabilities", func(t *testing.T) {
		caps := sandboxCapEff(t, h, ws)

		assert.True(t, hasCapability(caps, capNetAdmin),
			"the sandbox lost NET_ADMIN, so octelium connect cannot run")
		assert.True(t, hasCapability(caps, capNetRaw), "the sandbox lost NET_RAW")
		assert.False(t, hasCapability(caps, capSysPtrace),
			"the sandbox holds SYS_PTRACE without asking for it")
	})

	t.Run("TheTunDeviceIsAvailableToTheSandbox", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command:   "test -c /dev/net/tun && echo present",
			RunAsRoot: true,
		})
		require.Equal(t, int32(0), res.Code,
			"/dev/net/tun is missing, so octelium connect cannot run: %s", res.Stderr)
		assert.Equal(t, "present", res.Out())
	})

	t.Run("TheSandboxKeepsItsNamespacedNetworkCapabilities", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command:   "ip link set lo up",
			RunAsRoot: true,
		})
		assert.Equal(t, int32(0), res.Code,
			"the sandbox root cannot manage its own network namespace: %s", res.Stderr)
	})

	t.Run("TheSandboxCannotChangeHostWideKernelSettings", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command:   "echo 42 > /proc/sys/vm/swappiness",
			RunAsRoot: true,
		})
		assert.NotEqual(t, int32(0), res.Code,
			"the sandbox root changed a host-wide kernel setting")
	})

	t.Run("TheWorkspaceUserIsNotRoot", func(t *testing.T) {
		assert.NotEqual(t, "0", h.MustExec(t, ws, "id -u"))
		assert.Equal(t, "octelium", h.MustExec(t, ws, "id -un"))
	})

	t.Run("TheSupervisorPodRunsASingleContainer", func(t *testing.T) {
		pods, err := h.WorkspacePods(ctx, ws)
		require.Nil(t, err)
		require.Len(t, pods, 1)
		require.Len(t, pods[0].Spec.Containers, 1)

		assert.Equal(t, "workspace", pods[0].Spec.Containers[0].Name)
		assert.Equal(t, ws.Metadata.Uid, pods[0].Labels["octelium.com/workspace-uid"])

		require.NotNil(t, pods[0].Spec.AutomountServiceAccountToken)
		assert.False(t, *pods[0].Spec.AutomountServiceAccountToken,
			"the Workspace pod mounts a service account token")
	})

	t.Run("TheSandboxHasNoServiceAccountToken", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command:   "test -e /var/run/secrets/kubernetes.io && echo present",
			RunAsRoot: true,
		})
		assert.NotEqual(t, int32(0), res.Code,
			"the sandbox can read a Kubernetes service account token")
	})

	t.Run("TheInitProcessReapsTheSandbox", func(t *testing.T) {
		out := h.MustExec(t, ws, "cat /proc/1/comm")
		assert.NotEqual(t, "sleep", out,
			"pid 1 of the sandbox is the command itself, so zombies are not reaped")
	})

	t.Run("ANestedRootlessContainerRuntimeHasItsStorage", func(t *testing.T) {
		for _, dir := range []string{"/var/lib/docker", "/var/lib/containers"} {
			res := h.Exec(t, ws, charness.ExecOpts{
				Command:   fmt.Sprintf("test -d %s && echo present", dir),
				RunAsRoot: true,
			})
			assert.Equal(t, int32(0), res.Code,
				"the nested runtime storage %s is missing: %s", dir, res.Stderr)
		}
	})

	t.Run("TheOcteliumSocketsAreMountedIntoTheSandbox", func(t *testing.T) {
		for _, sock := range []string{
			"/var/run/octelium-proxy.sock",
			"/var/run/octelium-ssh-agent.sock",
		} {
			res := h.Exec(t, ws, charness.ExecOpts{
				Command:   fmt.Sprintf("test -S %s && echo present", sock),
				RunAsRoot: true,
			})
			assert.Equal(t, int32(0), res.Code,
				"the socket %s is not mounted into the sandbox: %s", sock, res.Stderr)
		}
	})

	t.Run("TheOcteliumBinariesAreMountedReadOnly", func(t *testing.T) {
		for _, bin := range []string{"/bin/octelium", "/bin/cordium-workspace"} {
			assert.Equal(t, "present",
				h.MustExec(t, ws, fmt.Sprintf("test -x %s && echo present", bin)))

			res := h.Exec(t, ws, charness.ExecOpts{
				Command:   fmt.Sprintf("truncate -s 0 %s", bin),
				RunAsRoot: true,
			})
			assert.NotEqual(t, int32(0), res.Code,
				"the mounted binary %s is writable from inside the sandbox", bin)
		}
	})
}
