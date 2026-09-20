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

package cordium

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
)

// driveToRunning walks a Workspace through a realistic startup once the SDK has
// a watch stream attached.
func driveToRunning(t *testing.T, fake *fakeCluster, name string) {
	t.Helper()

	go func() {
		waitForWatcher(fake)
		for _, state := range []cordiumv1.Workspace_Status_State{
			cordiumv1.Workspace_Status_INITIALIZING,
			cordiumv1.Workspace_Status_PULLING_IMAGE,
			cordiumv1.Workspace_Status_PREPARING,
			cordiumv1.Workspace_Status_RUNNING,
		} {
			fake.setState(name, state)
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

func waitForWatcher(fake *fakeCluster) {
	for range 500 {
		if fake.watcherCount() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRunCreatesStartsAndWaits(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	go func() {
		waitForWatcher(fake)
		fake.setState("ws1", cordiumv1.Workspace_Status_RUNNING)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	ws, err := c.Workspaces().Run(ctx,
		WithImage("python:3.11-slim"),
		Ephemeral(),
		WithRunVar("BRANCH", "main"),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !ws.IsRunning() {
		t.Errorf("state = %s, want RUNNING", ws.State())
	}
	if ws.Name() != "ws1" {
		t.Errorf("name = %q", ws.Name())
	}
	if !ws.IsEphemeral() {
		t.Error("the Workspace is not ephemeral")
	}
	if got, want := ws.URL(), "https://ws1.cordium.example.com"; got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}

	fake.mu.Lock()
	created, starts := fake.lastCreate, fake.startCalls
	fake.mu.Unlock()

	if starts != 1 {
		t.Errorf("StartWorkspace was called %d times", starts)
	}
	if created.GetSpec().GetImage().GetRegistry().GetUrl() != "python:3.11-slim" {
		t.Errorf("image = %v", created.GetSpec().GetImage())
	}
	if !created.GetSpec().GetIsEphemeral() {
		t.Error("isEphemeral was not forwarded")
	}
}

func TestWaitUntilRunningReportsFailure(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context(), WithImage("broken:latest"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	go func() {
		waitForWatcher(fake)
		fake.setState(ws.Name(), cordiumv1.Workspace_Status_BUILDING_IMAGE)
		time.Sleep(10 * time.Millisecond)
		fake.setFailure(ws.Name(), &cordiumv1.Workspace_Status_Failure{
			Message: "could not build the image",
			Type: &cordiumv1.Workspace_Status_Failure_ImageBuild_{
				ImageBuild: &cordiumv1.Workspace_Status_Failure_ImageBuild{},
			},
		})
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	err = ws.WaitUntilRunning(ctx)

	var failure *WorkspaceFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *WorkspaceFailureError", err)
	}
	if got := FailureReason(failure.Failure); got != "ImageBuild" {
		t.Errorf("FailureReason = %q, want ImageBuild", got)
	}
}

func TestWaitUntilStoppedAcceptsACleanStop(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context(), WithAutoStop())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	go func() {
		waitForWatcher(fake)
		fake.setState(ws.Name(), cordiumv1.Workspace_Status_PREPARING)
		time.Sleep(10 * time.Millisecond)
		fake.setState(ws.Name(), cordiumv1.Workspace_Status_STOPPED)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	if err := ws.WaitUntilStopped(ctx); err != nil {
		t.Fatalf("WaitUntilStopped: %v", err)
	}
	if !ws.IsStopped() {
		t.Errorf("state = %s", ws.State())
	}
}

// TestWaitReconcilesBeforeTheStream covers the race in which the Workspace
// reaches the awaited state before the watch stream is even open.
func TestWaitReconcilesStateReachedBeforeTheStream(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	fake.setState(ws.Name(), cordiumv1.Workspace_Status_RUNNING)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	if err := ws.WaitUntilRunning(ctx); err != nil {
		t.Fatalf("WaitUntilRunning: %v", err)
	}
}

func TestWaitDetectsDeletion(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	go func() {
		waitForWatcher(fake)
		_, _ = fake.DeleteWorkspace(context.Background(), deleteOptionsFor(ws.Name()))
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	if err := ws.WaitUntilRunning(ctx); !errors.Is(err, ErrWorkspaceDeleted) {
		t.Fatalf("err = %v, want ErrWorkspaceDeleted", err)
	}
}

func TestWaitHonorsTheContextDeadline(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	if err := ws.WaitUntilRunning(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestWorkspaceLifecycle(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context(), WithDisplayName("my sandbox"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ws.DisplayName() != "my sandbox" {
		t.Errorf("displayName = %q", ws.DisplayName())
	}
	if !ws.IsStopped() {
		t.Errorf("a new Workspace is in the %s state", ws.State())
	}

	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := ws.Refresh(t.Context()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if ws.State() != StateInitRequest {
		t.Errorf("state after Start = %s", ws.State())
	}

	if err := ws.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := ws.Delete(t.Context()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := c.Workspaces().Get(t.Context(), ws.Name()); !IsNotFound(err) {
		t.Fatalf("Get after Delete: %v, want a not-found error", err)
	}
}

func TestStartIsIdempotentOnAlreadyRunning(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Starting an already started Workspace answers ALREADY_EXISTS, which Start
	// absorbs.
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := ws.Start(t.Context()); err != nil {
		t.Fatalf("second Start: %v", err)
	}
}

func TestListPaginates(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	for range 7 {
		if _, err := c.Workspaces().Create(t.Context()); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	page, err := c.Workspaces().List(t.Context(), WithItemsPerPage(3))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 3 {
		t.Errorf("page holds %d items, want 3", len(page.Items))
	}
	if page.Page.TotalCount != 7 || !page.Page.HasMore {
		t.Errorf("page info = %+v", page.Page)
	}

	var names []string
	for ws, err := range c.Workspaces().All(t.Context(), WithItemsPerPage(3)) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		names = append(names, ws.Name())
	}
	if len(names) != 7 {
		t.Errorf("All yielded %d Workspaces, want 7: %v", len(names), names)
	}
}

func TestAllStopsEarly(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	for range 5 {
		if _, err := c.Workspaces().Create(t.Context()); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	count := 0
	for _, err := range c.Workspaces().All(t.Context(), WithItemsPerPage(2)) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("the iteration yielded %d items after the break", count)
	}
}

func TestWatchPublishesEvents(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	watcher, err := c.Workspaces().Watch(t.Context())
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	defer watcher.Close()

	waitForWatcher(fake)
	fake.setState(ws.Name(), cordiumv1.Workspace_Status_RUNNING)

	select {
	case ev := <-watcher.Events():
		if ev.Type != EventUpdated {
			t.Errorf("event type = %s", ev.Type)
		}
		if ev.Workspace.State() != StateRunning {
			t.Errorf("event state = %s", ev.Workspace.State())
		}
		if !ev.StateChanged() {
			t.Error("StateChanged() = false for a state transition")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no event was published")
	}
}

func TestUpdateStartsFromTheCurrentSpec(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	ws, err := c.Workspaces().Create(t.Context(),
		WithImage("node:20"),
		WithEnv("ONE", "1"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := ws.Update(t.Context(), WithEnv("TWO", "2")); err != nil {
		t.Fatalf("Update: %v", err)
	}

	spec := ws.Spec()
	if len(spec.GetRuntime().GetEnvVars()) != 2 {
		t.Errorf("envVars = %v, want the original one plus the new one",
			spec.GetRuntime().GetEnvVars())
	}
	if spec.GetImage().GetRegistry().GetUrl() != "node:20" {
		t.Error("the image was lost by the update")
	}

	if err := ws.Update(t.Context(), WithTemplate("other")); !IsInvalidArgument(err) {
		t.Errorf("changing the Template of a Workspace: %v, want an invalid argument error", err)
	}
}

func TestClosedClientIsRejected(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := c.Workspaces().Create(t.Context()); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("err = %v, want ErrClientClosed", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("a second Close failed: %v", err)
	}
}
