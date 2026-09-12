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
	"os/exec"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireCordiumCLI(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("cordium"); err != nil {
		t.Skipf("The cordium CLI is not installed: %+v", err)
	}
}

func testCordiumCLI(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	requireCordiumCLI(t)

	t.Run("Version", func(t *testing.T) {
		h.MustRun(t, "cordium version")
		h.MustRun(t, "cordium version -o json")
	})

	t.Run("Status", func(t *testing.T) {
		h.MustRun(t, "cordium status")
	})

	t.Run("List", func(t *testing.T) {
		h.MustRun(t, "cordium get workspaces")
		h.MustRun(t, "cordium get spaces")
		h.MustRun(t, "cordium get templates")
		h.MustRun(t, "cordium get workspaces -o json")
	})

	t.Run("TheCreatedWorkspaceIsVisibleToTheAPI", func(t *testing.T) {
		out := &cordiumv1.Workspace{}
		h.MustOutputProto(t, "cordium create workspace -o json", out)

		require.NotEmpty(t, out.Metadata.Name)

		t.Cleanup(func() {
			h.CordiumC().DeleteWorkspace(context.Background(),
				&metav1.DeleteOptions{Uid: out.Metadata.Uid})
		})

		ws, err := h.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Name: out.Metadata.Name})
		require.Nil(t, err)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED, ws.Status.State)

		fromCLI := &cordiumv1.Workspace{}
		h.MustOutputProto(t,
			fmt.Sprintf("cordium get ws %s -o json", out.Metadata.Name), fromCLI)
		assert.Equal(t, ws.Metadata.Uid, fromCLI.Metadata.Uid)
	})

	t.Run("TheWorkspaceIsDeletable", func(t *testing.T) {
		out := &cordiumv1.Workspace{}
		h.MustOutputProto(t, "cordium create workspace -o json", out)
		require.NotEmpty(t, out.Metadata.Name)

		h.MustRun(t, fmt.Sprintf("cordium delete workspace %s", out.Metadata.Name))

		_, err := h.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Uid: out.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})

	t.Run("AnUnknownWorkspaceFails", func(t *testing.T) {
		h.MustFail(t, fmt.Sprintf("cordium get ws %s", h.Name()))
	})
}

func testCordiumCLIWorkspace(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	requireCordiumCLI(t)

	created := &cordiumv1.Workspace{}
	h.MustOutputProto(t, "cordium create workspace -e E2E_CLI=cli-value -o json", created)
	require.NotEmpty(t, created.Metadata.Name)

	name := created.Metadata.Name
	ws := created

	t.Cleanup(func() {
		h.CordiumC().DeleteWorkspace(context.Background(),
			&metav1.DeleteOptions{Uid: ws.Metadata.Uid})
	})

	h.MustRun(t, fmt.Sprintf("cordium start %s", name))
	h.WaitWorkspaceRunning(t, ws)

	t.Run("Exec", func(t *testing.T) {
		out := h.MustOutput(t,
			fmt.Sprintf("cordium exec %s --no-stdin -- echo cli-exec-ok", name))
		assert.Contains(t, string(out), "cli-exec-ok")

		out = h.MustOutput(t,
			fmt.Sprintf("cordium exec %s --no-stdin -- printenv E2E_CLI", name))
		assert.Contains(t, string(out), "cli-value")
	})

	t.Run("ExecAsRoot", func(t *testing.T) {
		out := h.MustOutput(t,
			fmt.Sprintf("cordium exec %s --no-stdin --root -- id -u", name))
		assert.Contains(t, string(out), "0")
	})

	t.Run("ExecInTheWorkingDir", func(t *testing.T) {
		out := h.MustOutput(t,
			fmt.Sprintf("cordium exec %s --no-stdin -w %s -- pwd", name, workspaceDir))
		assert.Contains(t, string(out), workspaceDir)
	})

	t.Run("ExecPropagatesTheExitCode", func(t *testing.T) {
		h.MustFail(t,
			fmt.Sprintf("cordium exec %s --no-stdin -- sh -c 'exit 3'", name))
	})

	t.Run("SSH", func(t *testing.T) {
		h.Require(t, capRootTUN)

		h.WaitWorkspaceSessionConnected(t, ws)

		out := h.MustOutput(t, fmt.Sprintf("cordium ssh %s --print-config", name))
		assert.Contains(t, string(out), fmt.Sprintf("Host cordium-%s", name))
		assert.Contains(t, string(out), fmt.Sprintf("User %s", name))

		h.Connect(t, harness.ConnectOpts{Root: true})

		out = h.MustOutput(t, fmt.Sprintf("cordium ssh %s -- echo ssh-ok", name))
		assert.Contains(t, string(out), "ssh-ok")

		out = h.MustOutput(t, fmt.Sprintf("cordium ssh %s -- printenv CORDIUM_NAME", name))
		assert.Contains(t, string(out), name)

		h.MustFail(t, fmt.Sprintf("cordium ssh %s -- sh -c 'exit 4'", name))
	})

	t.Run("Stop", func(t *testing.T) {
		h.MustRun(t, fmt.Sprintf("cordium stop %s", name))
		stopped := h.WaitWorkspaceStopped(t, ws)

		assert.Equal(t, cordiumv1.Workspace_Status_STOPPING_REASON_API,
			stopped.Status.StoppingReason)

		_, err := h.ExecErr(t, ws, charness.ExecOpts{Command: "echo nope"})
		assert.NotNil(t, err, "the stopped Workspace still accepted an exec")
	})

	t.Run("Delete", func(t *testing.T) {
		h.MustRun(t, fmt.Sprintf("cordium delete workspace %s", name))

		_, err := h.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Uid: ws.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})
}
