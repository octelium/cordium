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

func testWorkspaceImage(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{
		Image: registryImage(
			fmt.Sprintf("docker.io/library/cordium-e2e-%s:0.0.1", h.Name())),
	})

	watcher := h.WatchWorkspace(t, ws)

	h.StartWorkspace(t, ws)
	failed := h.WaitWorkspaceFailure(t, ws)

	t.Run("TheSupervisorReportsThePullFailure", func(t *testing.T) {
		require.NotNil(t, failed.Status.Failure)
		assert.NotNil(t, failed.Status.Failure.GetImagePull(),
			"the Workspace failed with %s", charness.WorkspaceFailure(failed))

		assert.Zero(t, failed.Status.SuccessfulRuns)
		assert.Nil(t, failed.Status.SessionRef)

		require.NotNil(t, failed.Status.Run)
		assert.NotNil(t, failed.Status.Run.Failure)
	})

	t.Run("TheWorkspaceNeverReachedRunning", func(t *testing.T) {
		states := watcher.States()
		assertStateOrder(t, states)

		assert.NotContains(t, states, cordiumv1.Workspace_Status_RUNNING)
		assert.True(t, containsAny(states, cordiumv1.Workspace_Status_PULLING_IMAGE,
			cordiumv1.Workspace_Status_INITIALIZING),
			"the Workspace never started initializing: %v", states)
	})

	t.Run("NocturneCleansUpAfterTheFailure", func(t *testing.T) {
		h.Eventually(t, "the k8s resources of the failed Workspace to be removed",
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

	t.Run("ARegistryImageRunsOnceTheSpecIsFixed", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		cur.Spec.Image = registryImage(alpineImage)

		if _, err := h.CordiumC().UpdateWorkspace(ctx, cur); err != nil {
			t.Fatalf("Could not update the Workspace image: %+v", err)
		}

		h.StartWorkspace(t, ws)
		running := h.WaitWorkspaceRunning(t, ws)

		assert.Equal(t, uint32(1), running.Status.SuccessfulRuns)
		assert.NotNil(t, running.Status.SessionRef)

		assert.Contains(t, h.MustExec(t, ws, "cat /etc/os-release"), "Alpine")
		assert.Equal(t, ws.Metadata.Name, h.MustExec(t, ws, "printenv CORDIUM_NAME"))
		assert.NotEqual(t, "0", h.MustExec(t, ws, "id -u"))

		file := workspacePath("e2e-" + h.Name())
		h.MustExec(t, ws, fmt.Sprintf("echo alpine-ok > %s", file))
		assert.Equal(t, "alpine-ok", h.MustExec(t, ws, "cat "+file))
	})
}
