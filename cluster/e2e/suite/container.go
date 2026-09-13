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

func testWorkspaceContainer(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	name := h.Name()

	marker := "/tmp/e2e-cmd-" + name

	overridden := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Entrypoint: "/usr/bin/env",
			Cmd:        fmt.Sprintf("echo cmd-ran > %s; sleep infinity", marker),
		},
	})

	t.Run("TheCmdOverrideRunsInTheSandbox", func(t *testing.T) {
		assert.Equal(t, "cmd-ran", h.MustExec(t, overridden, "cat "+marker))
	})

	t.Run("TheEntrypointOverrideReplacesTheImageEntrypoint", func(t *testing.T) {
		cmdlines := sandboxCmdlines(t, h, overridden)

		assert.Contains(t, cmdlines, "/usr/bin/env",
			"the Entrypoint override never reached the container process")
		assert.NotContains(t, cmdlines, "tini",
			"the image entrypoint still runs despite the Entrypoint override")
	})

	capped := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Capabilities: capabilities([]string{"SYS_PTRACE"}, []string{"SYS_CHROOT"}),
		},
	})

	t.Run("TheRequestedCapabilitiesReachTheSandbox", func(t *testing.T) {
		caps := sandboxCapEff(t, h, capped)

		assert.True(t, hasCapability(caps, capSysPtrace),
			"the requested capability did not reach the sandbox")
		assert.False(t, hasCapability(caps, capSysChroot),
			"the dropped capability is still held by the sandbox")
		assert.True(t, hasCapability(caps, capNetAdmin),
			"the sandbox lost NET_ADMIN while adjusting the capabilities")
	})

	t.Run("TheDefaultSandboxKeepsTheImageEntrypoint", func(t *testing.T) {
		assert.Contains(t, sandboxCmdlines(t, h, capped), "tini",
			"the default Workspace does not run the image entrypoint")
	})

	t.Run("TheTimeoutModeIsStored", func(t *testing.T) {
		ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Timeout: &cordiumv1.Workspace_Spec_Runtime_Timeout{
					Mode: cordiumv1.Workspace_Spec_Runtime_Timeout_DISABLED,
				},
			},
		})

		cur := h.GetWorkspace(t, ws)
		require.NotNil(t, cur.Spec.Runtime.Timeout)
		assert.Equal(t, cordiumv1.Workspace_Spec_Runtime_Timeout_DISABLED,
			cur.Spec.Runtime.Timeout.Mode)
	})
}

func testWorkspaceTaskFailure(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	name := h.Name()

	t.Run("AnAbortingTaskFailsTheWorkspace", func(t *testing.T) {
		ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
					{
						Name:      "e2e-abort",
						Run:       "exit 13",
						Type:      cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
						OnFailure: cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
					},
				},
			},
		})

		h.StartWorkspace(t, ws)
		failed := h.WaitWorkspaceFailure(t, ws)

		require.NotNil(t, failed.Status.Failure)
		task := failed.Status.Failure.GetTask()
		require.NotNil(t, task,
			"the Workspace failed with %s", charness.WorkspaceFailure(failed))
		assert.Equal(t, "e2e-abort", task.Name)
		assert.Equal(t, int32(13), task.ExitCode)
	})

	userMarker := workspacePath("e2e-task-user-" + name)
	rootMarker := workspacePath("e2e-task-root-" + name)
	dirMarker := workspacePath("e2e-task-dir-" + name)
	envMarker := workspacePath("e2e-task-env-" + name)
	afterMarker := workspacePath("e2e-continue-" + name)
	lateMarker := workspacePath("e2e-background-" + name)

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
				{
					Name:      "e2e-continue-failing",
					Run:       "exit 5",
					Type:      cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					OnFailure: cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_CONTINUE,
				},
				postStartTask("e2e-continue-after",
					fmt.Sprintf("echo after > %s", afterMarker)),
				postStartTask("e2e-task-user",
					fmt.Sprintf("id -un > %s", userMarker)),
				{
					Name:      "e2e-task-root",
					Run:       fmt.Sprintf("id -un > %s", rootMarker),
					Type:      cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					RunAsRoot: true,
				},
				{
					Name:       "e2e-task-dir",
					Run:        fmt.Sprintf("pwd > %s", dirMarker),
					Type:       cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					WorkingDir: workspaceDir,
				},
				{
					Name: "e2e-task-env",
					Run:  fmt.Sprintf("printenv E2E_TASK > %s", envMarker),
					Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					EnvVars: []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{
						{Key: "E2E_TASK", Value: "task-env-value"},
					},
				},
				{
					Name:         "e2e-background",
					Run:          fmt.Sprintf("sleep 5; echo late > %s", lateMarker),
					Type:         cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					IsBackground: true,
				},
				{
					Name:         "e2e-background-failing",
					Run:          "exit 9",
					Type:         cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					IsBackground: true,
					OnFailure:    cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
				},
			},
		},
	})

	started := h.GetWorkspace(t, ws)
	lateAtStartup := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + lateMarker})

	t.Run("TheBackgroundTasksDoNotBlockOrFailTheStartup", func(t *testing.T) {
		require.Equal(t, cordiumv1.Workspace_Status_RUNNING, started.Status.State)
		require.Nil(t, started.Status.Failure)

		assert.NotEqual(t, int32(0), lateAtStartup.Code,
			"the Workspace only reached RUNNING once the background task had finished")

		waitLineCount(t, h, ws, lateMarker, 1)
	})

	t.Run("AContinuingTaskLetsTheWorkspaceRun", func(t *testing.T) {
		waitLineCount(t, h, ws, afterMarker, 1)

		cur := h.GetWorkspace(t, ws)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, cur.Status.State)
		assert.Nil(t, cur.Status.Failure)
	})

	t.Run("TheTaskOptionsAreHonored", func(t *testing.T) {
		waitLineCount(t, h, ws, userMarker, 1)
		assert.Equal(t, h.MustExec(t, ws, "id -un"),
			h.MustExec(t, ws, "cat "+userMarker))

		waitLineCount(t, h, ws, rootMarker, 1)
		assert.Equal(t, "root", h.MustExec(t, ws, "cat "+rootMarker))

		waitLineCount(t, h, ws, dirMarker, 1)
		assert.Equal(t, workspaceDir, h.MustExec(t, ws, "cat "+dirMarker))

		waitLineCount(t, h, ws, envMarker, 1)
		assert.Equal(t, "task-env-value", h.MustExec(t, ws, "cat "+envMarker))
	})
}

const (
	capNetAdmin  = 12
	capNetRaw    = 13
	capSysChroot = 18
	capSysPtrace = 19
)

func sandboxCapEff(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) uint64 {
	t.Helper()

	res := h.Exec(t, ws, charness.ExecOpts{
		Command:   "grep -m1 '^CapEff:' /proc/self/status | cut -f2",
		RunAsRoot: true,
	})
	require.Equal(t, int32(0), res.Code,
		"could not read the sandbox capabilities: %s", res.Stderr)

	ret, err := strconv.ParseUint(strings.TrimSpace(res.Out()), 16, 64)
	require.Nil(t, err, "unexpected CapEff %q", res.Out())

	return ret
}

func hasCapability(mask uint64, bit int) bool {
	return mask&(uint64(1)<<uint(bit)) != 0
}
