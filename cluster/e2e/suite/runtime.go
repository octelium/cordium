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
	"context"
	"fmt"
	"strings"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkspaceRuntime(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	const secretValue = "e2e-secret-value"

	spc := h.CreateSpace(t, nil)
	sec := h.CreateSecret(t, spc, secretValue)
	tmpl := h.CreateTemplate(t, spc, nil)

	taskMarker := workspacePath("e2e-task-" + h.Name())

	ws := h.CreateWorkspace(t, &cordiumv1.Workspace{
		Spec: &cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
					envVar("E2E_PLAIN", "plain-value"),
					envVarFromSecret("E2E_SECRET", sec.Metadata.Name),
				},
				Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
					postStartTask("e2e-marker", fmt.Sprintf("echo task-ran > %s", taskMarker)),
					{
						Name:         "e2e-ticker",
						Run:          "while true; do echo cordium-log-tick; sleep 2; done",
						Type:         cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
						IsBackground: true,
					},
				},
			},
			Limit: &cordiumv1.Workspace_Spec_Limit{
				Cpu:     &cordiumv1.Workspace_Spec_Limit_CPU{Millicores: 1500},
				Memory:  &cordiumv1.Workspace_Spec_Limit_Memory{Megabytes: 1024},
				Storage: &cordiumv1.Workspace_Spec_Limit_Storage{Megabytes: 2000},
			},
		},
		Status: &cordiumv1.Workspace_Status{
			TemplateRef: umetav1.GetObjectReference(tmpl),
		},
	})

	h.StartWorkspace(t, ws)
	h.WaitWorkspaceRunning(t, ws)

	t.Run("ExecStreamsStdout", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{Command: "echo cordium-e2e"})
		assert.Equal(t, int32(0), res.Code)
		assert.Equal(t, "cordium-e2e", res.Out())
		assert.Empty(t, strings.TrimSpace(res.Stderr))
	})

	t.Run("ExecStreamsStderr", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{Command: "echo to-stderr >&2"})
		assert.Equal(t, int32(0), res.Code)
		assert.Equal(t, "to-stderr", strings.TrimSpace(res.Stderr))
	})

	t.Run("ExecPropagatesTheExitCode", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{Command: "exit 7"})
		assert.Equal(t, int32(7), res.Code)

		res = h.Exec(t, ws, charness.ExecOpts{Command: "cat /does-not-exist"})
		assert.NotEqual(t, int32(0), res.Code)
		assert.NotEmpty(t, res.Stderr)
	})

	t.Run("ExecRunsInTheWorkingDir", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command:    "pwd",
			WorkingDir: workspaceDir,
		})
		require.Equal(t, int32(0), res.Code)
		assert.Equal(t, workspaceDir, res.Out())
	})

	t.Run("ExecSetsTheRequestedEnvVars", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command: "printenv E2E_EXEC",
			EnvVars: map[string]string{"E2E_EXEC": "exec-value"},
		})
		require.Equal(t, int32(0), res.Code)
		assert.Equal(t, "exec-value", res.Out())
	})

	t.Run("ExecRunsAsTheWorkspaceUser", func(t *testing.T) {
		assert.NotEqual(t, "0", h.MustExec(t, ws, "id -u"))

		res := h.Exec(t, ws, charness.ExecOpts{
			Command:   "id -u",
			RunAsRoot: true,
		})
		require.Equal(t, int32(0), res.Code)
		assert.Equal(t, "0", res.Out())
	})

	t.Run("ExecForwardsStdin", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command: "head -n 1",
			Stdin:   "from-stdin\n",
		})
		require.Equal(t, int32(0), res.Code)
		assert.Equal(t, "from-stdin", res.Out())
	})

	t.Run("TheWorkspaceEnvVarsAreSet", func(t *testing.T) {
		assert.Equal(t, "plain-value", h.MustExec(t, ws, "printenv E2E_PLAIN"))
		assert.Equal(t, secretValue, h.MustExec(t, ws, "printenv E2E_SECRET"))
		assert.Equal(t, ws.Metadata.Name, h.MustExec(t, ws, "printenv CORDIUM_NAME"))
		assert.NotEmpty(t, h.MustExec(t, ws, "printenv CORDIUM_HOSTNAME"))
	})

	t.Run("ThePostStartTaskRuns", func(t *testing.T) {
		h.Eventually(t, "the POST_START task to run", charness.ExecBudget,
			func(ctx context.Context) error {
				res := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + taskMarker})
				if res.Code != 0 {
					return errors.Errorf("the task marker does not exist yet: %s", res.Stderr)
				}
				if res.Out() != "task-ran" {
					return errors.Errorf("the task marker is %q", res.Out())
				}

				return nil
			})
	})

	t.Run("TheHomeDirectoryIsWritable", func(t *testing.T) {
		file := "e2e-" + h.Name()

		h.MustExec(t, ws, fmt.Sprintf("echo home-write > ${HOME}/%s", file))
		assert.Equal(t, "home-write", h.MustExec(t, ws, fmt.Sprintf("cat ${HOME}/%s", file)))
	})

	t.Run("TheLimitsReachThePodAndTheStorage", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		require.NotNil(t, cur.Status.Limit)
		require.NotNil(t, cur.Status.Limit.Cpu)
		require.NotNil(t, cur.Status.Limit.Memory)
		assert.Equal(t, uint32(1500), cur.Status.Limit.Cpu.Millicores)
		assert.Equal(t, uint32(1024), cur.Status.Limit.Memory.Megabytes)

		pods, err := h.WorkspacePods(ctx, ws)
		require.Nil(t, err)
		require.Len(t, pods, 1)
		require.NotEmpty(t, pods[0].Spec.Containers)

		limits := pods[0].Spec.Containers[0].Resources.Limits
		assert.Equal(t, int64(1500), limits.Cpu().MilliValue())
		assert.Equal(t, int64(1024*1024*1024), limits.Memory().Value())

		pvc, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
		require.Nil(t, err)
		assert.Equal(t, int64(2000*1024*1024),
			pvc.Spec.Resources.Requests.Storage().Value())
	})

	t.Run("TheLogStreamDeliversTheTaskOutput", func(t *testing.T) {
		logs := h.ListenLog(t, ws)
		logs.WaitOutput(t, "cordium-log-tick")

		var found bool
		for _, entry := range logs.Entries() {
			if entry.Type == cordiumv1.ListenLogResponse_TYPE_TASK &&
				entry.Mode == cordiumv1.ListenLogResponse_MODE_STDOUT &&
				strings.Contains(string(entry.Data), "cordium-log-tick") {
				found = true
				assert.NotNil(t, entry.CreatedAt)
			}
		}
		assert.True(t, found, "no TASK stdout log entry was delivered")
	})

	t.Run("TheTerminalRunsCommands", func(t *testing.T) {
		term := h.NewTerminal(t, ws)

		term.Write(t, "expr 40 + 2\n")
		term.WaitOutput(t, "42")

		term.SetWindowSize(t, 100, 50)

		terms := h.Terminals(t, ws)
		var found bool
		for _, itm := range terms {
			if itm.Id == term.ID {
				found = true
			}
		}
		assert.True(t, found)

		term.Remove(t)

		h.Eventually(t, "the terminal to be removed", charness.PropagationBudget,
			func(ctx context.Context) error {
				for _, itm := range h.Terminals(t, ws) {
					if itm.Id == term.ID {
						return errors.Errorf("the terminal %s still exists", term.ID)
					}
				}

				return nil
			})
	})

	t.Run("SeveralTerminalsCoexist", func(t *testing.T) {
		first := h.NewTerminal(t, ws)
		second := h.NewTerminal(t, ws)

		assert.NotEqual(t, first.ID, second.ID)

		first.Write(t, "echo first-terminal-ok\n")
		second.Write(t, "echo second-terminal-ok\n")

		first.WaitOutput(t, "first-terminal-ok")
		second.WaitOutput(t, "second-terminal-ok")

		assert.NotContains(t, first.Output(), "second-terminal-ok")

		first.Remove(t)
		second.Remove(t)
	})
}

func testWorkspacePorts(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{
		Applications: []*cordiumv1.Workspace_Spec_Application{
			application("web", 3000, true),
			application("api", 8080, false),
		},
	})

	t.Run("TheApplicationsAreStored", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		require.Len(t, cur.Spec.Applications, 2)
		assert.Equal(t, "web", cur.Spec.Applications[0].Name)
		assert.True(t, cur.Spec.Applications[0].IsDefault)
		assert.Empty(t, cur.Status.SharedPorts)
	})

	t.Run("AnApplicationPortIsShareable", func(t *testing.T) {
		_, err := h.CordiumC().ShareWorkspacePort(ctx, &cordiumv1.ShareWorkspacePortRequest{
			WorkspaceRef:    umetav1.GetObjectReference(ws),
			ApplicationName: "web",
			Mode:            cordiumv1.ShareWorkspacePortRequest_MEMBERS,
		})
		require.Nil(t, err)

		cur := h.GetWorkspace(t, ws)
		require.Len(t, cur.Status.SharedPorts, 1)
		assert.Equal(t, "web", cur.Status.SharedPorts[0].ApplicationName)
		assert.Equal(t, cordiumv1.Workspace_Status_SharedPort_MEMBERS,
			cur.Status.SharedPorts[0].Mode)
	})

	t.Run("TheShareModeIsUpdatable", func(t *testing.T) {
		_, err := h.CordiumC().ShareWorkspacePort(ctx, &cordiumv1.ShareWorkspacePortRequest{
			WorkspaceRef:    umetav1.GetObjectReference(ws),
			ApplicationName: "web",
			Mode:            cordiumv1.ShareWorkspacePortRequest_ALL,
		})
		require.Nil(t, err)

		cur := h.GetWorkspace(t, ws)
		require.Len(t, cur.Status.SharedPorts, 1)
		assert.Equal(t, cordiumv1.Workspace_Status_SharedPort_ALL,
			cur.Status.SharedPorts[0].Mode)
	})

	t.Run("AnUnknownApplicationIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().ShareWorkspacePort(ctx, &cordiumv1.ShareWorkspacePortRequest{
			WorkspaceRef:    umetav1.GetObjectReference(ws),
			ApplicationName: h.Name(),
			Mode:            cordiumv1.ShareWorkspacePortRequest_MEMBERS,
		})
		assert.NotNil(t, err)
	})

	t.Run("AnUnsetModeIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().ShareWorkspacePort(ctx, &cordiumv1.ShareWorkspacePortRequest{
			WorkspaceRef:    umetav1.GetObjectReference(ws),
			ApplicationName: "web",
		})
		assert.NotNil(t, err)
	})

	t.Run("TheApplicationPortIsUnshareable", func(t *testing.T) {
		_, err := h.CordiumC().UnshareWorkspacePort(ctx,
			&cordiumv1.UnshareWorkspacePortRequest{
				WorkspaceRef:    umetav1.GetObjectReference(ws),
				ApplicationName: "web",
			})
		require.Nil(t, err)

		cur := h.GetWorkspace(t, ws)
		assert.Empty(t, cur.Status.SharedPorts)
	})
}
