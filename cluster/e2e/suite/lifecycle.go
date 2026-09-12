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
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkspaceLifecycle(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{})
	watcher := h.WatchWorkspace(t, ws)

	h.StartWorkspace(t, ws)
	running := h.WaitWorkspaceRunning(t, ws)

	marker := workspacePath("e2e-" + h.Name())

	var startupStates []cordiumv1.Workspace_Status_State

	t.Run("TheRunningWorkspaceIsPlaced", func(t *testing.T) {
		require.NotNil(t, running.Status.RegionRef)
		assert.Equal(t, "default", running.Status.RegionRef.Name)
		assert.NotEmpty(t, running.Status.Hostname)
		assert.NotNil(t, running.Status.LastInitializedAt)
		assert.NotNil(t, running.Status.LastRunningAt)
		assert.Equal(t, uint32(1), running.Status.SuccessfulRuns)
		assert.Nil(t, running.Status.Failure)

		require.NotNil(t, running.Status.Run)
		assert.NotEmpty(t, running.Status.Run.Id)
		assert.Nil(t, running.Status.Run.StoppedAt)
	})

	t.Run("TheWatchStreamReportsTheStartupStates", func(t *testing.T) {
		watcher.WaitState(t, cordiumv1.Workspace_Status_RUNNING, charness.PropagationBudget)

		startupStates = watcher.States()
		assertStateOrder(t, startupStates)

		assert.Contains(t, startupStates, cordiumv1.Workspace_Status_RUNNING)
		assert.True(t, containsAny(startupStates,
			cordiumv1.Workspace_Status_INIT_REQUEST,
			cordiumv1.Workspace_Status_INITIALIZING,
			cordiumv1.Workspace_Status_PULLING_IMAGE,
			cordiumv1.Workspace_Status_STARTING_RUNTIME,
			cordiumv1.Workspace_Status_PREPARING),
			"the Workspace reached RUNNING without any startup state: %v", startupStates)

		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING,
			startupStates[len(startupStates)-1])
	})

	t.Run("TheStatusTimestampsAdvance", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)

		require.NotNil(t, cur.Status.CurrentStateSetAt)
		require.NotNil(t, cur.Status.LastStateSetAt)
		assert.False(t, cur.Status.CurrentStateSetAt.AsTime().
			Before(cur.Status.LastStateSetAt.AsTime()))
		assert.False(t, cur.Status.LastRunningAt.AsTime().
			Before(cur.Status.LastInitializedAt.AsTime()))
		assert.NotEqual(t, cordiumv1.Workspace_Status_UNKNOWN, cur.Status.LastState)
	})

	t.Run("NocturneCreatesTheK8sResources", func(t *testing.T) {
		name := workspaceK8sName(ws)

		dep, err := h.K8sC().AppsV1().Deployments(charness.WorkspaceNamespace).
			Get(ctx, name, k8smetav1.GetOptions{})
		require.Nil(t, err)
		assert.Equal(t, int32(1), dep.Status.ReadyReplicas)
		require.Len(t, dep.OwnerReferences, 1)
		assert.Equal(t, "ConfigMap", dep.OwnerReferences[0].Kind)
		assert.Equal(t, name, dep.OwnerReferences[0].Name)

		svc, err := h.K8sC().CoreV1().Services(charness.WorkspaceNamespace).
			Get(ctx, name, k8smetav1.GetOptions{})
		require.Nil(t, err)
		assert.NotEmpty(t, svc.Spec.Ports)

		pvc, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
		require.Nil(t, err)
		assert.NotEmpty(t, pvc.Spec.Resources.Requests)

		pods, err := h.WorkspacePods(ctx, ws)
		require.Nil(t, err)
		require.Len(t, pods, 1)
		assert.Equal(t, ws.Metadata.Uid, pods[0].Labels["octelium.com/workspace-uid"])
		assert.Equal(t, "user", pods[0].Labels["octelium.com/component-type"])
	})

	t.Run("TheWorkspaceHasASession", func(t *testing.T) {
		require.NotNil(t, running.Status.SessionRef)

		sess, err := h.CoreC().GetSession(ctx,
			&metav1.GetOptions{Uid: running.Status.SessionRef.Uid})
		require.Nil(t, err)
		require.NotNil(t, sess.Status.UserRef)
		assert.Equal(t, running.Status.UserRef.Uid, sess.Status.UserRef.Uid)
	})

	t.Run("TheWorkspaceRunsCommands", func(t *testing.T) {
		assert.Equal(t, "cordium-e2e", h.MustExec(t, ws, "echo cordium-e2e"))

		h.MustExec(t, ws, fmt.Sprintf("echo %s > %s", ws.Metadata.Uid, marker))
		assert.Equal(t, ws.Metadata.Uid, h.MustExec(t, ws, "cat "+marker))
	})

	t.Run("StoppingTheWorkspaceTearsItDown", func(t *testing.T) {
		h.StopWorkspace(t, ws)
		stopped := h.WaitWorkspaceStopped(t, ws)

		assert.Nil(t, stopped.Status.SessionRef)
		assert.Nil(t, stopped.Status.RegionRef)
		assert.Empty(t, stopped.Status.Hostname)
		assert.NotNil(t, stopped.Status.LastStoppedAt)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPING_REASON_API,
			stopped.Status.StoppingReason)

		_, err := h.CoreC().GetSession(ctx,
			&metav1.GetOptions{Uid: running.Status.SessionRef.GetUid()})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))

		watcher.WaitState(t, cordiumv1.Workspace_Status_STOPPED, charness.PropagationBudget)

		shutdownStates := statesAfter(watcher.States(), startupStates)
		assertStateOrder(t, shutdownStates)
		assert.True(t, containsAny(shutdownStates,
			cordiumv1.Workspace_Status_STOPPING_REQUEST,
			cordiumv1.Workspace_Status_STOPPING),
			"the Workspace stopped without any stopping state: %v", shutdownStates)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED,
			shutdownStates[len(shutdownStates)-1])

		assert.NotNil(t, stopped.Status.TotalLastRunsDuration)

		h.Eventually(t, "the Workspace k8s resources to be removed",
			charness.StopBudget, func(ctx context.Context) error {
				_, err := h.K8sC().AppsV1().Deployments(charness.WorkspaceNamespace).
					Get(ctx, workspaceK8sName(ws), k8smetav1.GetOptions{})
				if err == nil {
					return errors.Errorf("the Deployment %s still exists",
						workspaceK8sName(ws))
				}
				if !k8serr.IsNotFound(err) {
					return err
				}

				_, err = h.K8sC().CoreV1().Services(charness.WorkspaceNamespace).
					Get(ctx, workspaceK8sName(ws), k8smetav1.GetOptions{})
				if err == nil {
					return errors.Errorf("the k8s Service %s still exists",
						workspaceK8sName(ws))
				}
				if !k8serr.IsNotFound(err) {
					return err
				}

				return nil
			})
	})

	t.Run("TheStorageIsKeptWhileStopped", func(t *testing.T) {
		_, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
		assert.Nil(t, err)
	})

	t.Run("TheStorageSurvivesARestartInTheRequestedRegion", func(t *testing.T) {
		region, err := h.CoreC().GetRegion(ctx, &metav1.GetOptions{Name: "default"})
		require.Nil(t, err)

		h.StartWorkspaceWithConfig(t, ws, &cordiumv1.StartWorkspaceRequest_Config{
			RegionRef: umetav1.GetObjectReference(region),
		})
		restarted := h.WaitWorkspaceRunning(t, ws)

		require.NotNil(t, restarted.Status.RegionRef)
		assert.Equal(t, region.Metadata.Uid, restarted.Status.RegionRef.Uid)

		assert.Equal(t, uint32(2), restarted.Status.SuccessfulRuns)
		assert.Len(t, restarted.Status.LastRuns, 1)
		assert.NotNil(t, restarted.Status.LastRuns[0].StoppedAt)
		assert.NotEqual(t, restarted.Status.Run.Id, restarted.Status.LastRuns[0].Id)

		assert.Equal(t, ws.Metadata.Uid, h.MustExec(t, ws, "cat "+marker))
	})

	t.Run("DeletingTheWorkspaceRemovesTheStorage", func(t *testing.T) {
		h.DeleteWorkspace(t, ws)
		h.WaitWorkspaceGone(t, ws)

		h.Eventually(t, "the Workspace storage to be removed",
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
	})
}

func testWorkspaceEphemeral(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		IsEphemeral: true,
	})

	marker := workspacePath("e2e-" + h.Name())
	h.MustExec(t, ws, fmt.Sprintf("echo %s > %s", ws.Metadata.Uid, marker))

	h.StopWorkspace(t, ws)
	h.WaitWorkspaceStopped(t, ws)

	t.Run("TheStorageIsDiscardedOnStop", func(t *testing.T) {
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
	})

	t.Run("TheWorkspaceStartsAgainWithFreshStorage", func(t *testing.T) {
		h.StartWorkspace(t, ws)
		h.WaitWorkspaceRunning(t, ws)

		res := h.Exec(t, ws, charness.ExecOpts{Command: "cat " + marker})
		assert.NotEqual(t, int32(0), res.Code)

		_, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{})
		assert.Nil(t, err)
	})

	t.Run("NocturneCleansUpADeletedRunningWorkspace", func(t *testing.T) {
		h.DeleteWorkspace(t, ws)
		h.WaitWorkspaceGone(t, ws)

		h.Eventually(t, "the k8s resources of the deleted Workspace to be removed",
			charness.StopBudget, func(ctx context.Context) error {
				if _, err := h.K8sC().AppsV1().
					Deployments(charness.WorkspaceNamespace).
					Get(ctx, workspaceK8sName(ws), k8smetav1.GetOptions{}); err == nil {
					return errors.Errorf("the Deployment %s still exists",
						workspaceK8sName(ws))
				} else if !k8serr.IsNotFound(err) {
					return err
				}

				if _, err := h.K8sC().CoreV1().
					PersistentVolumeClaims(charness.WorkspaceNamespace).
					Get(ctx, workspacePVCName(ws), k8smetav1.GetOptions{}); err == nil {
					return errors.Errorf("the PersistentVolumeClaim %s still exists",
						workspacePVCName(ws))
				} else if !k8serr.IsNotFound(err) {
					return err
				}

				pods, err := h.WorkspacePods(ctx, ws)
				if err != nil {
					return err
				}
				if len(pods) > 0 {
					return errors.Errorf("the Workspace still has %d pod(s)", len(pods))
				}

				return nil
			})
	})
}

func testWorkspaceStopWhileStarting(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{})
	watcher := h.WatchWorkspace(t, ws)

	h.StartWorkspace(t, ws)

	h.WaitWorkspace(t, ws, "the Workspace supervisor to start initializing",
		charness.StartBudget, func(cur *cordiumv1.Workspace) (bool, error) {
			if ucordiumv1.ToWorkspace(cur).IsStopped() {
				return false, errors.Errorf("the Workspace stopped while starting up")
			}

			return ucordiumv1.ToWorkspace(cur).IsPreRunning() &&
				cur.Status.State != cordiumv1.Workspace_Status_INIT_REQUEST, nil
		})

	h.StopWorkspace(t, ws)
	stopped := h.WaitWorkspaceStopped(t, ws)

	t.Run("TheWorkspaceStopsBeforeItEverRan", func(t *testing.T) {
		assert.Zero(t, stopped.Status.SuccessfulRuns)
		assert.Nil(t, stopped.Status.SessionRef)
		assert.Nil(t, stopped.Status.RegionRef)
		assert.Empty(t, stopped.Status.Hostname)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPING_REASON_API,
			stopped.Status.StoppingReason)

		states := watcher.States()
		assertStateOrder(t, states)
		assert.NotContains(t, states, cordiumv1.Workspace_Status_RUNNING)
	})

	t.Run("NocturneCleansUpTheHalfStartedWorkspace", func(t *testing.T) {
		h.Eventually(t, "the k8s resources of the half-started Workspace to be removed",
			charness.StopBudget, func(ctx context.Context) error {
				if _, err := h.K8sC().AppsV1().Deployments(charness.WorkspaceNamespace).
					Get(ctx, workspaceK8sName(ws), k8smetav1.GetOptions{}); err == nil {
					return errors.Errorf("the Deployment %s still exists",
						workspaceK8sName(ws))
				} else if !k8serr.IsNotFound(err) {
					return err
				}

				pods, err := h.WorkspacePods(ctx, ws)
				if err != nil {
					return err
				}
				if len(pods) > 0 {
					return errors.Errorf("the Workspace still has %d pod(s)", len(pods))
				}

				return nil
			})
	})

	t.Run("TheStorageIsKept", func(t *testing.T) {
		_, err := h.K8sC().CoreV1().PersistentVolumeClaims(charness.WorkspaceNamespace).
			Get(t.Context(), workspacePVCName(ws), k8smetav1.GetOptions{})
		assert.Nil(t, err)
	})
}
