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

package harness

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	workspacePollInterval = 2 * time.Second

	firstDiagnosticsAfter = 45 * time.Second
	diagnosticsEvery      = 60 * time.Second

	restartCheckEvery = 10 * time.Second

	initRequestBudget = 4 * time.Minute
)

func (h *H) CreateWorkspace(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.Workspace {
	t.Helper()

	if ws.Metadata == nil {
		ws.Metadata = &metav1.Metadata{}
	}
	if ws.Spec == nil {
		ws.Spec = &cordiumv1.Workspace_Spec{}
	}

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().CreateWorkspace(ctx, ws)
	if err != nil {
		t.Fatalf("Could not create the Workspace: %+v", err)
	}

	t.Cleanup(func() {
		h.deleteQuietly(t, "Workspace", ret.Metadata.Name, func(ctx context.Context) error {
			_, err := h.CordiumC().DeleteWorkspace(ctx,
				&metav1.DeleteOptions{Uid: ret.Metadata.Uid})
			return err
		})
	})

	zap.L().Debug("Created Workspace fixture", zap.String("name", ret.Metadata.Name))

	return ret
}

func (h *H) NewWorkspace(t *testing.T, spec *cordiumv1.Workspace_Spec) *cordiumv1.Workspace {
	t.Helper()

	return h.CreateWorkspace(t, &cordiumv1.Workspace{Spec: spec})
}

func (h *H) RunWorkspace(t *testing.T, spec *cordiumv1.Workspace_Spec) *cordiumv1.Workspace {
	t.Helper()

	ws := h.NewWorkspace(t, spec)
	h.StartWorkspace(t, ws)

	return h.WaitWorkspaceRunning(t, ws)
}

func (h *H) StartWorkspace(t *testing.T, ws *cordiumv1.Workspace) {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if _, err := h.CordiumC().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	}); err != nil {
		t.Fatalf("Could not start the Workspace %s: %+v", ws.Metadata.Name, err)
	}

	h.StreamWorkspaceLogs(t, ws)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if _, err := h.CordiumC().StopWorkspace(ctx, &cordiumv1.StopWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		}); err != nil {
			zap.L().Debug("Could not stop the Workspace on cleanup",
				zap.String("name", ws.Metadata.Name), zap.Error(err))
		}
	})
}

func (h *H) StopWorkspace(t *testing.T, ws *cordiumv1.Workspace) {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if _, err := h.CordiumC().StopWorkspace(ctx, &cordiumv1.StopWorkspaceRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	}); err != nil {
		t.Fatalf("Could not stop the Workspace %s: %+v", ws.Metadata.Name, err)
	}
}

func (h *H) DeleteWorkspace(t *testing.T, ws *cordiumv1.Workspace) {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if _, err := h.CordiumC().DeleteWorkspace(ctx,
		&metav1.DeleteOptions{Uid: ws.Metadata.Uid}); err != nil {
		t.Fatalf("Could not delete the Workspace %s: %+v", ws.Metadata.Name, err)
	}
}

func (h *H) GetWorkspace(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.Workspace {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().GetWorkspace(ctx, &metav1.GetOptions{Uid: ws.Metadata.Uid})
	if err != nil {
		t.Fatalf("Could not get the Workspace %s: %+v", ws.Metadata.Name, err)
	}

	return ret
}

func (h *H) WaitWorkspaceRunning(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.Workspace {
	t.Helper()

	return h.WaitWorkspace(t, ws, "the Workspace to run", StartBudget,
		func(cur *cordiumv1.Workspace) (bool, error) {
			if ucordiumv1.ToWorkspace(cur).IsRunning() {
				return true, nil
			}
			if ucordiumv1.ToWorkspace(cur).IsStopped() {
				return false, errors.Errorf(
					"the Workspace stopped while starting up. Failure: %s",
					WorkspaceFailure(cur))
			}

			return false, nil
		})
}

func (h *H) WaitWorkspaceStopped(t *testing.T, ws *cordiumv1.Workspace) *cordiumv1.Workspace {
	t.Helper()

	return h.WaitWorkspace(t, ws, "the Workspace to stop", StopBudget,
		func(cur *cordiumv1.Workspace) (bool, error) {
			return ucordiumv1.ToWorkspace(cur).IsStopped(), nil
		})
}

func (h *H) WaitWorkspaceGone(t *testing.T, ws *cordiumv1.Workspace) {
	t.Helper()

	h.Eventually(t, "the Workspace to be deleted", StopBudget,
		func(ctx context.Context) error {
			_, err := h.CordiumC().GetWorkspace(ctx, &metav1.GetOptions{Uid: ws.Metadata.Uid})
			if err == nil {
				return errors.Errorf("the Workspace %s still exists", ws.Metadata.Name)
			}
			if grpcerr.IsNotFound(err) {
				return nil
			}

			return err
		})
}

func (h *H) WaitWorkspace(t *testing.T, ws *cordiumv1.Workspace, what string,
	budget time.Duration,
	fn func(ws *cordiumv1.Workspace) (bool, error)) *cordiumv1.Workspace {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), budget)
	defer cancel()

	started := time.Now()
	state := ws.GetStatus().GetState()
	reportAt := started.Add(firstDiagnosticsAfter)
	checkAt := started.Add(restartCheckEvery)
	var cur *cordiumv1.Workspace
	var lastErr error

	for {
		cur, lastErr = h.CordiumC().GetWorkspace(ctx, &metav1.GetOptions{Uid: ws.Metadata.Uid})
		if lastErr == nil {
			if cur.Status.State != state {
				state = cur.Status.State
				zap.L().Info("Workspace state",
					zap.String("name", cur.Metadata.Name),
					zap.String("state", state.String()),
					zap.Duration("elapsed", time.Since(started).Truncate(time.Second)))
			}

			done, err := fn(cur)
			if err != nil {
				t.Fatalf("Gave up waiting for %s after %s: %+v\n%s",
					what, time.Since(started).Truncate(time.Millisecond), err,
					h.Diagnostics(ws))
			}
			if done {
				return cur
			}

			if ucordiumv1.ToWorkspace(cur).IsPreRunning() && time.Now().After(checkAt) {
				checkAt = time.Now().Add(restartCheckEvery)

				if err := h.CheckWorkspacePodRestarts(ctx, cur); err != nil {
					t.Fatalf("The Workspace %s cannot start up: %+v\n%s",
						ws.Metadata.Name, err, h.Diagnostics(ws))
				}
			}

			if cur.Status.State == cordiumv1.Workspace_Status_INIT_REQUEST &&
				cur.Status.CurrentStateSetAt.IsValid() &&
				time.Since(cur.Status.CurrentStateSetAt.AsTime()) > initRequestBudget {
				t.Fatalf("The Workspace %s has been at INIT_REQUEST for %s. Its supervisor "+
					"pod never became reachable, so nocturne force-stops it at the 5m mark "+
					"without recording a Failure.\n%s",
					ws.Metadata.Name,
					time.Since(cur.Status.CurrentStateSetAt.AsTime()).Truncate(time.Second),
					h.Diagnostics(ws))
			}
		}

		if time.Now().After(reportAt) {
			reportAt = time.Now().Add(diagnosticsEvery)

			writeLine("--- Still waiting for %s after %s. The Workspace %s is %s ---\n%s",
				what, time.Since(started).Truncate(time.Second),
				ws.Metadata.Name, state, h.Diagnostics(ws))
		}

		select {
		case <-ctx.Done():
			t.Fatalf(
				"Timed out after %s waiting for %s. The Workspace %s is %s. Last error: %+v\n%s",
				time.Since(started).Truncate(time.Millisecond), what,
				ws.Metadata.Name, state, lastErr, h.Diagnostics(ws))
		case <-time.After(workspacePollInterval):
		}
	}
}

func WorkspaceFailure(ws *cordiumv1.Workspace) string {
	if ws.Status == nil || ws.Status.Failure == nil {
		return "none"
	}

	return ws.Status.Failure.String()
}

func (h *H) deleteQuietly(t *testing.T, kind, name string, fn func(ctx context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := fn(ctx); err != nil {
		if grpcerr.IsNotFound(err) {
			return
		}
		zap.L().Warn("Could not clean up fixture",
			zap.String("kind", kind), zap.String("name", name), zap.Error(err))
	}
}

type WorkspaceWatcher struct {
	h  *H
	ws *cordiumv1.Workspace

	mu     sync.Mutex
	states []cordiumv1.Workspace_Status_State
}

func (h *H) WatchWorkspace(t *testing.T, ws *cordiumv1.Workspace) *WorkspaceWatcher {
	t.Helper()

	strm, err := h.CordiumC().WatchWorkspace(t.Context(), &cordiumv1.WatchWorkspaceRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	})
	if err != nil {
		t.Fatalf("Could not watch the Workspace %s: %+v", ws.Metadata.Name, err)
	}

	ret := &WorkspaceWatcher{
		h:  h,
		ws: ws,
	}

	go func() {
		for {
			msg, err := strm.Recv()
			if err != nil {
				zap.L().Debug("Workspace watch loop ended",
					zap.String("name", ws.Metadata.Name), zap.Error(err))
				return
			}

			var cur *cordiumv1.Workspace
			switch msg.Type.(type) {
			case *cordiumv1.WatchWorkspaceResponse_Create_:
				cur = msg.GetCreate().Item
			case *cordiumv1.WatchWorkspaceResponse_Update_:
				cur = msg.GetUpdate().NewItem
			}

			if cur == nil || cur.Status == nil {
				continue
			}

			ret.mu.Lock()
			if len(ret.states) == 0 || ret.states[len(ret.states)-1] != cur.Status.State {
				ret.states = append(ret.states, cur.Status.State)
			}
			ret.mu.Unlock()
		}
	}()

	return ret
}

func (w *WorkspaceWatcher) States() []cordiumv1.Workspace_Status_State {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.states)
}

func (w *WorkspaceWatcher) WaitState(t *testing.T,
	state cordiumv1.Workspace_Status_State, budget time.Duration) {
	t.Helper()

	w.h.Eventually(t, "the Workspace watch stream to report "+state.String(), budget,
		func(ctx context.Context) error {
			if slices.Contains(w.States(), state) {
				return nil
			}

			return errors.Errorf("the watch stream has only reported %v", w.States())
		})
}

func (h *H) StartWorkspaceWithConfig(t *testing.T, ws *cordiumv1.Workspace,
	cfg *cordiumv1.StartWorkspaceRequest_Config) {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if _, err := h.CordiumC().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
		Config:       cfg,
	}); err != nil {
		t.Fatalf("Could not start the Workspace %s: %+v", ws.Metadata.Name, err)
	}

	h.StreamWorkspaceLogs(t, ws)
}

func (h *H) WaitWorkspaceState(t *testing.T, ws *cordiumv1.Workspace,
	state cordiumv1.Workspace_Status_State, budget time.Duration) *cordiumv1.Workspace {
	t.Helper()

	return h.WaitWorkspace(t, ws, "the Workspace to reach "+state.String(), budget,
		func(cur *cordiumv1.Workspace) (bool, error) {
			return cur.Status.State == state, nil
		})
}

func (h *H) WaitWorkspaceFailure(t *testing.T,
	ws *cordiumv1.Workspace) *cordiumv1.Workspace {
	t.Helper()

	return h.WaitWorkspace(t, ws, "the Workspace to fail and stop", StartBudget,
		func(cur *cordiumv1.Workspace) (bool, error) {
			if !ucordiumv1.ToWorkspace(cur).IsStopped() {
				return false, nil
			}
			if cur.Status.Failure == nil {
				return false, errors.Errorf(
					"the Workspace stopped without reporting a failure")
			}

			return true, nil
		})
}

func (h *H) WaitWorkspaceSessionConnected(t *testing.T, ws *cordiumv1.Workspace) *corev1.Session {
	t.Helper()

	var ret *corev1.Session

	h.Eventually(t, "the Workspace Session to connect with eSSH", StartBudget,
		func(ctx context.Context) error {
			cur, err := h.CordiumC().GetWorkspace(ctx,
				&metav1.GetOptions{Uid: ws.Metadata.Uid})
			if err != nil {
				return err
			}
			if cur.Status.SessionRef == nil {
				return errors.Errorf("the Workspace has no Session")
			}

			sess, err := h.CoreC().GetSession(ctx,
				&metav1.GetOptions{Uid: cur.Status.SessionRef.Uid})
			if err != nil {
				return err
			}
			if sess.Status.Connection == nil {
				return errors.Errorf("the Workspace Session is not connected yet")
			}
			if !sess.Status.Connection.ESSHEnable {
				return errors.Errorf("the Workspace Session does not serve eSSH yet")
			}

			ret = sess
			return nil
		})

	return ret
}
