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
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkspaceTasks(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	name := h.Name()
	onCreate := workspacePath("oncreate-" + name)
	postStart := workspacePath("poststart-" + name)
	preStop := workspacePath("prestop-" + name)

	appendTo := func(path string) string {
		return fmt.Sprintf("echo run >> %s", path)
	}

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
				{
					Name: "e2e-on-create",
					Run:  appendTo(onCreate),
					Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
				},
				{
					Name: "e2e-post-start",
					Run:  appendTo(postStart),
					Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
				},
				{
					Name: "e2e-pre-stop",
					Run:  appendTo(preStop),
					Type: cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP,
				},
			},
		},
	})

	t.Run("TheOnCreateAndPostStartTasksRunOnTheFirstRun", func(t *testing.T) {
		waitLineCount(t, h, ws, onCreate, 1)
		waitLineCount(t, h, ws, postStart, 1)

		res := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + preStop})
		assert.NotEqual(t, int32(0), res.Code,
			"the PRE_STOP task ran before the Workspace stopped")
	})

	t.Run("TheTasksRunAsTheWorkspaceUser", func(t *testing.T) {
		assert.Equal(t, "owned",
			h.MustExec(t, ws, fmt.Sprintf("test -O %s && echo owned", postStart)))
	})

	h.StopWorkspace(t, ws)
	h.WaitWorkspaceStopped(t, ws)

	h.StartWorkspace(t, ws)
	h.WaitWorkspaceRunning(t, ws)

	t.Run("ThePreStopTaskRanDuringTheShutdown", func(t *testing.T) {
		waitLineCount(t, h, ws, preStop, 1)
	})

	t.Run("ThePostStartTaskRunsOnEveryRun", func(t *testing.T) {
		waitLineCount(t, h, ws, postStart, 2)
	})

	t.Run("TheOnCreateTaskDoesNotRunAgain", func(t *testing.T) {
		assert.Equal(t, "1", lineCount(t, h, ws, onCreate))
	})
}

func testWorkspaceAutoStop(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			AutoStop: true,
			Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
				postStartTask("e2e-auto-stop", "echo auto-stop-task"),
			},
		},
	})

	watcher := h.WatchWorkspace(t, ws)

	h.StartWorkspace(t, ws)

	stopped := h.WaitWorkspaceState(t, ws,
		cordiumv1.Workspace_Status_STOPPED, charness.StartBudget)

	watcher.WaitState(t, cordiumv1.Workspace_Status_RUNNING, charness.PropagationBudget)

	t.Run("TheWorkspaceStopsItselfAfterTheTasks", func(t *testing.T) {
		assert.Equal(t, uint32(1), stopped.Status.SuccessfulRuns)
		assert.Nil(t, stopped.Status.Failure)
		assert.Nil(t, stopped.Status.SessionRef)
		assert.NotNil(t, stopped.Status.LastStoppedAt)
		assert.NotEqual(t, cordiumv1.Workspace_Status_STOPPING_REASON_API,
			stopped.Status.StoppingReason)

		require.NotNil(t, stopped.Status.Run)
		assert.NotNil(t, stopped.Status.Run.StoppedAt)
	})

	t.Run("TheWatchStreamReportsTheWholeRun", func(t *testing.T) {
		watcher.WaitState(t, cordiumv1.Workspace_Status_STOPPED, charness.PropagationBudget)

		states := watcher.States()
		assertStateOrder(t, states)
		assert.Contains(t, states, cordiumv1.Workspace_Status_RUNNING)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED, states[len(states)-1])
	})

	t.Run("NocturneCleansUpAfterTheAutoStop", func(t *testing.T) {
		h.Eventually(t, "the k8s resources of the auto-stopped Workspace to be removed",
			charness.StopBudget, func(ctx context.Context) error {
				if _, err := h.K8sC().AppsV1().Deployments(charness.WorkspaceNamespace).
					Get(ctx, workspaceK8sName(ws), k8smetav1.GetOptions{}); err == nil {
					return errors.Errorf("the Deployment %s still exists",
						workspaceK8sName(ws))
				} else if !k8serr.IsNotFound(err) {
					return err
				}

				return nil
			})
	})
}

func lineCount(t *testing.T, h *charness.H, ws *cordiumv1.Workspace, path string) string {
	t.Helper()

	return h.MustExec(t, ws, fmt.Sprintf("wc -l < %s", path))
}

func waitLineCount(t *testing.T, h *charness.H,
	ws *cordiumv1.Workspace, path string, want int) {
	t.Helper()

	h.Eventually(t, fmt.Sprintf("the file %s to have %d line(s)", path, want),
		charness.ExecBudget, func(ctx context.Context) error {
			res, err := h.ExecErr(t, ws, charness.ExecOpts{
				Command: fmt.Sprintf("wc -l < %s", path),
			})
			if err != nil {
				return err
			}
			if res.Code != 0 {
				return errors.Errorf("the file %s does not exist yet: %s", path, res.Stderr)
			}
			if res.Out() != fmt.Sprintf("%d", want) {
				return errors.Errorf("the file %s has %s line(s), want %d",
					path, res.Out(), want)
			}

			return nil
		})
}
