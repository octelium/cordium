//go:build e2e

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

package tests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	csuite "github.com/octelium/cordium/cluster/e2e/suite"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/cluster/e2e/suite"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(suite.Bootstrap(m))
}

func TestE2E(t *testing.T) {
	suite.Run(t, csuite.All())
}

func TestE2ERuntimeEdgeCases(t *testing.T) {
	phases := suite.Only(suite.Phases(), "ClusterReady")
	phases = append(phases, csuite.ReadyPhase(),
		suite.Phase{Name: "WorkspaceExecCleanup", Run: testWorkspaceExecCleanup},
		suite.Phase{Name: "WorkspaceTaskRecovery", Run: testWorkspaceTaskRecovery},
	)
	suite.Run(t, phases)
}

func testWorkspaceExecCleanup(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)
	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})

	t.Run("ExecEnvironmentDoesNotLeakBetweenCommands", func(t *testing.T) {
		first := h.Exec(t, ws, charness.ExecOpts{Command: "printf '%s' \"$CORDIUM_E2E_PRIVATE\"", EnvVars: map[string]string{"CORDIUM_E2E_PRIVATE": "command-only"}})
		require.Equal(t, int32(0), first.Code)
		assert.Equal(t, "command-only", first.Stdout)
		second := h.Exec(t, ws, charness.ExecOpts{Command: "printf '%s' \"${CORDIUM_E2E_PRIVATE-unset}\""})
		require.Equal(t, int32(0), second.Code)
		assert.Equal(t, "unset", second.Stdout)
	})

	t.Run("CancelledExecTerminatesItsChildren", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		stream, err := h.WorkspaceC().Exec(ctx)
		require.Nil(t, err)
		marker := "/workspace/e2e-child-" + h.Name()
		require.Nil(t, stream.Send(&cordiumv1.ExecRequest{Type: &cordiumv1.ExecRequest_Request_{Request: &cordiumv1.ExecRequest_Request{
			WorkspaceRef: umetav1.GetObjectReference(ws),
			Command:      fmt.Sprintf("sleep 90 & e2e_child_pid=$!; printf '%%s' \"$e2e_child_pid\" > %s; wait \"$e2e_child_pid\"", marker),
		}}}))
		var pid int
		h.Eventually(t, "the exec child process to start", 15*time.Second, func(context.Context) error {
			result := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + marker})
			if result.Code != 0 {
				return fmt.Errorf("the child PID is not available")
			}
			var err error
			pid, err = strconv.Atoi(strings.TrimSpace(result.Stdout))
			return err
		})
		require.Greater(t, pid, 1)
		cancel()
		_, err = stream.Recv()
		assert.NotNil(t, err)
		h.Eventually(t, "the cancelled exec child process to terminate", 20*time.Second, func(context.Context) error {
			result := h.Exec(t, ws, charness.ExecOpts{Command: fmt.Sprintf("test ! -e /proc/%d/stat || test \"$(cut -d' ' -f3 /proc/%d/stat)\" = Z", pid, pid)})
			if result.Code != 0 {
				return fmt.Errorf("exec child %d is still running", pid)
			}
			return nil
		})
		assert.Equal(t, "responsive", h.MustExec(t, ws, "echo responsive"))
	})

	t.Run("RemovingATerminalDoesNotAffectOtherSessions", func(t *testing.T) {
		first := h.NewTerminal(t, ws)
		second := h.NewTerminal(t, ws)
		t.Cleanup(func() { second.Remove(t) })
		first.Remove(t)
		second.Write(t, "printf 'surviving-%s\\n' terminal\n")
		second.WaitOutput(t, "surviving-terminal")
		for _, terminal := range h.Terminals(t, ws) {
			assert.NotEqual(t, first.ID, terminal.Id)
		}
	})
}

func testWorkspaceTaskRecovery(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)
	marker := "/workspace/e2e-retry-" + h.Name()
	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{Runtime: &cordiumv1.Workspace_Spec_Runtime{Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{{
		Name:      "fail-once",
		Run:       fmt.Sprintf("if test ! -f %s; then touch %s; exit 17; fi; echo recovered", marker, marker),
		Type:      cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
		OnFailure: cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
	}}}})
	h.StartWorkspace(t, ws)
	failed := h.WaitWorkspaceState(t, ws, cordiumv1.Workspace_Status_STOPPED, charness.StartBudget)
	require.NotNil(t, failed.Status.Failure)
	require.NotNil(t, failed.Status.Run)
	firstRunID := failed.Status.Run.Id

	t.Run("TheFailureIdentifiesTheTaskAndExitCode", func(t *testing.T) {
		require.NotNil(t, failed.Status.Failure.GetTask())
		assert.Equal(t, "fail-once", failed.Status.Failure.GetTask().Name)
		assert.Equal(t, int32(17), failed.Status.Failure.GetTask().ExitCode)
	})

	h.StartWorkspace(t, ws)
	running := h.WaitWorkspaceRunning(t, ws)
	t.Run("TheNextRunRecoversAndClearsTheOldFailure", func(t *testing.T) {
		assert.Nil(t, running.Status.Failure)
		require.NotNil(t, running.Status.Run)
		assert.NotEqual(t, firstRunID, running.Status.Run.Id)
		assert.Equal(t, "present", h.MustExec(t, ws, "test -f "+marker+" && echo present"))
		assert.Equal(t, "recovered", h.MustExec(t, ws, "echo recovered"))
	})
}
