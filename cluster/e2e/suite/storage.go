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

func testWorkspaceStorage(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	name := h.Name()
	marker := workspacePath("e2e-persist-" + name)
	homeMarker := workspaceHome("e2e-home-" + name)
	rootMarker := "/e2e-root-" + name

	counter := workspacePath("e2e-runs-" + name)

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
				postStartTask("e2e-count-runs", fmt.Sprintf("echo run >> %s", counter)),
			},
		},
	})

	firstPVC := workspacePVCUID(t, h, ws)

	h.MustExec(t, ws, fmt.Sprintf("echo persisted > %s", marker))
	h.MustExec(t, ws, fmt.Sprintf("echo home > %s", homeMarker))

	h.Exec(t, ws, charness.ExecOpts{
		Command:   fmt.Sprintf("echo layer > %s", rootMarker),
		RunAsRoot: true,
	})

	waitLineCount(t, h, ws, counter, 1)

	h.StopWorkspace(t, ws)
	h.WaitWorkspaceStopped(t, ws)

	h.StartWorkspace(t, ws)
	second := h.WaitWorkspaceRunning(t, ws)

	t.Run("TheWorkspaceDirSurvivesTheSecondRun", func(t *testing.T) {
		require.Equal(t, uint32(2), second.Status.SuccessfulRuns)
		assert.Equal(t, "persisted", h.MustExec(t, ws, "cat "+marker))
	})

	t.Run("ThePersistentVolumeIsReused", func(t *testing.T) {
		assert.Equal(t, firstPVC, workspacePVCUID(t, h, ws),
			"a non-ephemeral Workspace was given a new PersistentVolumeClaim")
	})

	t.Run("TheHomeDirSurvivesTheSecondRun", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + homeMarker})
		assert.Equal(t, int32(0), res.Code,
			"the Workspace home directory was not restored: %s", res.Stderr)
		assert.Equal(t, "home", res.Out())
	})

	t.Run("TheContainerLayerSurvivesTheSecondRun", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + rootMarker})
		assert.Equal(t, int32(0), res.Code,
			"the rootless podman container was rebuilt instead of restarted: %s",
			res.Stderr)
		assert.Equal(t, "layer", res.Out())
	})

	t.Run("ThePostStartTasksRanOnBothRuns", func(t *testing.T) {
		waitLineCount(t, h, ws, counter, 2)
	})

	h.StopWorkspace(t, ws)
	h.WaitWorkspaceStopped(t, ws)

	h.StartWorkspace(t, ws)
	third := h.WaitWorkspaceRunning(t, ws)

	t.Run("TheStorageSurvivesAThirdRun", func(t *testing.T) {
		require.Equal(t, uint32(3), third.Status.SuccessfulRuns)
		assert.Len(t, third.Status.LastRuns, 2)
		assert.Equal(t, "persisted", h.MustExec(t, ws, "cat "+marker))
		waitLineCount(t, h, ws, counter, 3)

		assert.Equal(t, firstPVC, workspacePVCUID(t, h, ws))
	})

	t.Run("TheRunHistoryIsRecorded", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)

		require.NotNil(t, cur.Status.Run)
		assert.NotEmpty(t, cur.Status.Run.Id)
		assert.Nil(t, cur.Status.Run.StoppedAt)

		var ids []string
		for _, run := range cur.Status.LastRuns {
			assert.NotNil(t, run.StoppedAt, "a past run has no stop timestamp")
			assert.NotContains(t, ids, run.Id, "the run ids are not unique")
			ids = append(ids, run.Id)
		}
		assert.NotContains(t, ids, cur.Status.Run.Id)
	})

	t.Run("TheEphemeralWorkspaceStartsFreshOnEveryRun", func(t *testing.T) {
		ephMarker := workspacePath("e2e-eph-" + name)
		onCreate := workspacePath("e2e-eph-oncreate-" + name)

		eph := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
			IsEphemeral: true,
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
					onCreateTask("e2e-eph-on-create",
						fmt.Sprintf("echo run >> %s", onCreate)),
				},
			},
		})

		waitLineCount(t, h, eph, onCreate, 1)
		h.MustExec(t, eph, fmt.Sprintf("echo gone > %s", ephMarker))

		before := workspacePVCUID(t, h, eph)

		h.StopWorkspace(t, eph)
		h.WaitWorkspaceStopped(t, eph)
		waitWorkspacePVCGone(t, h, eph)

		h.StartWorkspace(t, eph)
		h.WaitWorkspaceRunning(t, eph)

		res := h.Exec(t, eph, charness.ExecOpts{Command: "cat " + ephMarker})
		assert.NotEqual(t, int32(0), res.Code,
			"the ephemeral Workspace restored its previous storage")

		assert.NotEqual(t, before, workspacePVCUID(t, h, eph),
			"the ephemeral Workspace reused its previous PersistentVolumeClaim")

		waitLineCount(t, h, eph, onCreate, 1)
	})

	t.Run("TheStorageLimitReachesThePersistentVolumeClaim", func(t *testing.T) {
		sized := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
			Limit: &cordiumv1.Workspace_Spec_Limit{
				Storage: &cordiumv1.Workspace_Spec_Limit_Storage{Megabytes: 3000},
			},
		})

		pvc, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(sized), k8smetav1.GetOptions{})
		require.Nil(t, err)
		assert.Equal(t, int64(3000*1024*1024),
			pvc.Spec.Resources.Requests.Storage().Value())

		cur := h.GetWorkspace(t, sized)
		require.NotNil(t, cur.Status.Limit)
		require.NotNil(t, cur.Status.Limit.Storage)
		assert.Equal(t, uint32(3000), cur.Status.Limit.Storage.Megabytes)
	})
}

func waitWorkspacePVCGone(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) {
	t.Helper()

	h.Eventually(t, "the ephemeral Workspace storage to be removed",
		charness.StopBudget, func(ctx context.Context) error {
			_, err := h.K8sC().CoreV1().
				PersistentVolumeClaims(charness.WorkspaceNamespace).
				Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
			if err == nil {
				return errors.Errorf("the PersistentVolumeClaim %s still exists",
					workspacePVCName(ws))
			}
			if !k8serr.IsNotFound(err) {
				return err
			}

			return nil
		})
}

func workspacePVCUID(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) string {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	pvc, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
		Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
	require.Nil(t, err)

	return string(pvc.UID)
}
