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
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorkspaceIsolation(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	first := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})
	second := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})

	t.Run("TheExecStreamsAreRoutedToTheirWorkspace", func(t *testing.T) {
		assert.Equal(t, first.Metadata.Name,
			h.MustExec(t, first, "printenv CORDIUM_NAME"))
		assert.Equal(t, second.Metadata.Name,
			h.MustExec(t, second, "printenv CORDIUM_NAME"))

		assert.NotEqual(t,
			h.MustExec(t, first, "printenv CORDIUM_HOSTNAME"),
			h.MustExec(t, second, "printenv CORDIUM_HOSTNAME"))
	})

	t.Run("TheFilesystemsAreSeparate", func(t *testing.T) {
		file := workspacePath("e2e-" + h.Name())

		h.MustExec(t, first, fmt.Sprintf("echo only-in-first > %s", file))
		assert.Equal(t, "only-in-first", h.MustExec(t, first, "cat "+file))

		res := h.Exec(t, second, charness.ExecOpts{Command: "cat " + file})
		assert.NotEqual(t, int32(0), res.Code)
	})

	t.Run("TheWorkspaceRuntimesAreCgroupConfined", func(t *testing.T) {
		for _, ws := range []*cordiumv1.Workspace{first, second} {
			res := h.Exec(t, ws, charness.ExecOpts{Command: "cat /sys/fs/cgroup/pids.max"})
			require.Equal(t, int32(0), res.Code,
				"the pids cgroup controller does not reach the Workspace %s: %s",
				ws.Metadata.Name, res.Stderr)
			assert.NotEqual(t, "max", res.Out(),
				"the Workspace %s runs without a cgroup pids limit", ws.Metadata.Name)
		}
	})

	t.Run("TheTerminalsAreScopedToTheirWorkspace", func(t *testing.T) {
		term := h.NewTerminal(t, first)
		t.Cleanup(func() { term.Remove(t) })

		term.Write(t, "expr 20 + 22\n")
		term.WaitOutput(t, "42")

		for _, itm := range h.Terminals(t, second) {
			assert.NotEqual(t, term.ID, itm.Id)
		}

		var found bool
		for _, itm := range h.Terminals(t, first) {
			if itm.Id == term.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("AnotherUserCannotReachTheWorkspace", func(t *testing.T) {
		actor := h.NewActor(t)

		_, err := actor.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Name: first.Metadata.Name})
		assert.NotNil(t, err)

		_, err = actor.CordiumC().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(first),
		})
		assert.NotNil(t, err)

		_, err = actor.CordiumC().StopWorkspace(ctx, &cordiumv1.StopWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(first),
		})
		assert.NotNil(t, err)

		_, err = actor.CordiumC().DeleteWorkspace(ctx,
			&metav1.DeleteOptions{Uid: first.Metadata.Uid})
		assert.NotNil(t, err)

		_, err = h.ExecErr(t, first, charness.ExecOpts{
			Command: "echo should-not-run",
			Conn:    actor.Conn,
		})
		assert.NotNil(t, err)

		_, err = actor.WorkspaceC().CreateTerminal(ctx, &cordiumv1.CreateTerminalRequest{
			WorkspaceRef: umetav1.GetObjectReference(first),
			Cols:         80,
			Rows:         24,
		})
		assert.NotNil(t, err)

		_, err = actor.WorkspaceC().ListTerminal(ctx, &cordiumv1.ListTerminalRequest{
			WorkspaceRef: umetav1.GetObjectReference(first),
		})
		assert.NotNil(t, err)
	})

	t.Run("AnotherUserStillOwnsTheirWorkspaces", func(t *testing.T) {
		actor := h.NewActor(t)

		own, err := actor.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
		})
		require.Nil(t, err)

		t.Cleanup(func() {
			actor.CordiumC().DeleteWorkspace(context.Background(),
				&metav1.DeleteOptions{Uid: own.Metadata.Uid})
		})

		require.NotNil(t, own.Status.UserRef)
		assert.Equal(t, actor.User.Metadata.Uid, own.Status.UserRef.Uid)

		res, err := actor.CordiumC().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{})
		require.Nil(t, err)

		for _, itm := range res.Items {
			assert.NotEqual(t, first.Metadata.Uid, itm.Metadata.Uid)
			assert.NotEqual(t, second.Metadata.Uid, itm.Metadata.Uid)
		}
	})

	t.Run("TheActiveWorkspaceLimitIsEnforced", func(t *testing.T) {
		before := h.ClusterConfig(t)

		t.Cleanup(func() {
			cur := h.ClusterConfig(t)
			cur.Spec = before.Spec
			if _, err := h.ManagementC().UpdateClusterConfig(context.Background(), cur); err != nil {
				t.Errorf("Could not restore the Cordium ClusterConfig: %+v", err)
			}
		})

		cur := h.ClusterConfig(t)
		if cur.Spec.Workspace == nil {
			cur.Spec.Workspace = &cordiumv1.ClusterConfig_Spec_Workspace{}
		}
		cur.Spec.Workspace.Limit = &cordiumv1.ClusterConfig_Spec_Workspace_Limit{
			MaxActivePerUser: 1,
		}

		_, err := h.ManagementC().UpdateClusterConfig(ctx, cur)
		require.Nil(t, err)

		third := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{})

		_, err = h.CordiumC().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(third),
		})
		require.NotNil(t, err)

		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED,
			h.GetWorkspace(t, third).Status.State)
	})
}
