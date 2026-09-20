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
	"io"
	"sync"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"google.golang.org/grpc"
)

// ErrWorkspaceStopped is returned by [Workspace.WaitUntilRunning] when the
// Workspace stops without reporting a failure, which is what a Workspace that
// was created with [WithAutoStop] does once its Tasks complete.
var ErrWorkspaceStopped = errors.New("cordium: Workspace stopped before it reached the RUNNING state")

// errWatchRetry asks the wait loop to re-establish its watch stream.
var errWatchRetry = errors.New("cordium: watch stream ended")

// WaitCondition is evaluated against every state of a Workspace that the
// Cluster reports. It returns true once the wait is over, or an error to abort
// it.
type WaitCondition func(*cordiumv1.Workspace) (bool, error)

// WaitUntilRunning waits for the Workspace to be fully initialized and ready to
// be used.
//
// It fails with a [*WorkspaceFailureError] when the run fails, and with
// [ErrWorkspaceStopped] when the Workspace stops without reporting a failure.
// A Workspace that was created with [WithAutoStop] is expected to stop on its
// own, so such a Workspace should be awaited with [Workspace.WaitUntilStopped]
// instead.
func (w *Workspace) WaitUntilRunning(ctx context.Context) error {
	return w.WaitFor(ctx, func(ws *cordiumv1.Workspace) (bool, error) {
		switch ws.GetStatus().GetState() {
		case StateRunning:
			return true, nil
		case StateStopped:
			return false, stoppedError(ws)
		default:
			return false, nil
		}
	})
}

// WaitUntilReady waits for the Workspace to reach the earliest state at which
// it accepts command execution and terminals, which is PREPARING. The lifecycle
// setup (i.e. the repository cloning, the ON_CREATE Tasks, the dotfiles and the
// devcontainer Features) may still be running at that point.
func (w *Workspace) WaitUntilReady(ctx context.Context) error {
	return w.WaitFor(ctx, func(ws *cordiumv1.Workspace) (bool, error) {
		switch ws.GetStatus().GetState() {
		case StatePreparing, StateRunning:
			return true, nil
		case StateStopped:
			return false, stoppedError(ws)
		default:
			return false, nil
		}
	})
}

// WaitUntilStopped waits for the Workspace to be fully stopped. It is how a
// one-shot Workspace, such as a CI run or an unattended agent task created with
// [WithAutoStop], is awaited.
//
// A run that failed is reported as a [*WorkspaceFailureError].
func (w *Workspace) WaitUntilStopped(ctx context.Context) error {
	return w.WaitFor(ctx, func(ws *cordiumv1.Workspace) (bool, error) {
		if ws.GetStatus().GetState() != StateStopped {
			return false, nil
		}
		if failure := ws.GetStatus().GetFailure(); failure != nil {
			return false, &WorkspaceFailureError{
				Workspace: ws.GetMetadata().GetName(),
				Failure:   failure,
			}
		}
		return true, nil
	})
}

// WaitForState waits for the Workspace to reach any one of the given states.
func (w *Workspace) WaitForState(ctx context.Context, states ...State) error {
	if len(states) == 0 {
		return invalidArgumentf("no state to wait for")
	}
	return w.WaitFor(ctx, func(ws *cordiumv1.Workspace) (bool, error) {
		for _, state := range states {
			if ws.GetStatus().GetState() == state {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitFor follows the Workspace until cond is satisfied, keeping the handle's
// cached state up to date as it goes.
//
// It follows the Cluster's watch stream and re-establishes it, without losing
// track of the Workspace, whenever an intermediary drops it. The caller's
// context bounds the wait; it normally carries a deadline.
func (w *Workspace) WaitFor(ctx context.Context, cond WaitCondition) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}
	if cond == nil {
		return invalidArgumentf("nil wait condition")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-w.c.closedCh():
			cancel()
		case <-ctx.Done():
		}
	}()

	backoff := newBackoff()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		strm, streamErr := w.c.MainService().WatchWorkspace(ctx, &cordiumv1.WatchWorkspaceRequest{
			WorkspaceRef: w.ref(),
		})

		// Reconcile against the Cluster once the stream is open so that a state
		// that was reached before it is never missed.
		done, err := w.reconcile(ctx, cond)
		if done || err != nil {
			return err
		}

		if streamErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := backoff.sleep(ctx); err != nil {
				return err
			}
			continue
		}

		err = w.consumeWatch(ctx, strm, cond)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, errWatchRetry):
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.c.logger().Debug("Re-establishing the Workspace watch stream",
				"workspace", w.Name())
			if err := backoff.sleep(ctx); err != nil {
				return err
			}
		default:
			return err
		}
	}
}

// reconcile loads the Workspace and evaluates cond against it. A transient
// failure to load it is not fatal: the wait keeps going.
func (w *Workspace) reconcile(ctx context.Context, cond WaitCondition) (bool, error) {
	ws, err := w.c.MainService().GetWorkspace(ctx, &metav1.GetOptions{Uid: w.UID()})
	if err != nil {
		if IsNotFound(err) {
			return false, ErrWorkspaceDeleted
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		w.c.logger().Debug("Could not reconcile the Workspace state",
			"workspace", w.Name(), "error", err)
		return false, nil
	}

	w.set(ws)
	return cond(ws)
}

func (w *Workspace) consumeWatch(ctx context.Context,
	strm grpc.ServerStreamingClient[cordiumv1.WatchWorkspaceResponse], cond WaitCondition) error {

	type recvResult struct {
		msg *cordiumv1.WatchWorkspaceResponse
		err error
	}

	recvCh := make(chan recvResult, 1)
	go func() {
		defer close(recvCh)
		for {
			msg, err := strm.Recv()
			select {
			case recvCh <- recvResult{msg: msg, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	var tickC <-chan time.Time
	if interval := w.c.cfg.waitPollInterval; interval > 0 {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		tickC = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickC:
			done, err := w.reconcile(ctx, cond)
			if done || err != nil {
				return err
			}
		case res, ok := <-recvCh:
			if !ok {
				return errWatchRetry
			}
			if res.err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return errWatchRetry
			}

			item, deleted := watchedItem(res.msg)
			if deleted {
				return ErrWorkspaceDeleted
			}
			if item == nil {
				continue
			}

			w.set(item)
			done, err := cond(item)
			if done || err != nil {
				return err
			}
		}
	}
}

func watchedItem(msg *cordiumv1.WatchWorkspaceResponse) (item *cordiumv1.Workspace, deleted bool) {
	switch msg.GetType().(type) {
	case *cordiumv1.WatchWorkspaceResponse_Create_:
		return msg.GetCreate().GetItem(), false
	case *cordiumv1.WatchWorkspaceResponse_Update_:
		return msg.GetUpdate().GetNewItem(), false
	case *cordiumv1.WatchWorkspaceResponse_Delete_:
		return nil, true
	default:
		return nil, false
	}
}

func stoppedError(ws *cordiumv1.Workspace) error {
	if failure := ws.GetStatus().GetFailure(); failure != nil {
		return &WorkspaceFailureError{
			Workspace: ws.GetMetadata().GetName(),
			Failure:   failure,
		}
	}
	return ErrWorkspaceStopped
}

/*
 * Watching
 */

// EventType is the type of a Workspace event.
type EventType uint8

const (
	// EventCreated means that a Workspace was created.
	EventCreated EventType = iota + 1
	// EventUpdated means that a Workspace was updated, which is how every
	// state transition is reported.
	EventUpdated
	// EventDeleted means that a Workspace was deleted.
	EventDeleted
)

// String returns the name of the event type.
func (t EventType) String() string {
	switch t {
	case EventCreated:
		return "CREATED"
	case EventUpdated:
		return "UPDATED"
	case EventDeleted:
		return "DELETED"
	default:
		return "UNKNOWN"
	}
}

// WorkspaceEvent is a single event of a Workspace watch.
type WorkspaceEvent struct {
	// Type is the type of the event.
	Type EventType
	// Workspace is the Workspace as it is after the event.
	Workspace *Workspace
	// Previous is the Workspace as it was before an update. Comparing its state
	// against the new one is how an actual state transition is detected. It is
	// nil for the create and the delete events.
	Previous *cordiumv1.Workspace
}

// StateChanged reports whether the event carries an actual state transition.
func (e WorkspaceEvent) StateChanged() bool {
	if e.Type != EventUpdated || e.Previous == nil {
		return e.Type == EventCreated || e.Type == EventDeleted
	}
	return e.Previous.GetStatus().GetState() != e.Workspace.State()
}

// WorkspaceWatcher streams the create, update and delete events of the calling
// User's Workspaces.
type WorkspaceWatcher struct {
	events chan WorkspaceEvent
	cancel context.CancelFunc

	mu   sync.Mutex
	err  error
	done bool
}

// Watch opens a stream of the events of every Workspace that the calling User
// owns:
//
//	watcher, err := c.Workspaces().Watch(ctx)
//	if err != nil {
//		return err
//	}
//	defer watcher.Close()
//
//	for ev := range watcher.Events() {
//		fmt.Println(ev.Type, ev.Workspace.Name(), ev.Workspace.State())
//	}
//	return watcher.Err()
//
// The channel is closed once the stream ends, and [WorkspaceWatcher.Err] then
// reports why. The caller must drain the channel, or cancel the context, in
// order not to hold the stream back.
func (wc *WorkspaceClient) Watch(ctx context.Context) (*WorkspaceWatcher, error) {
	return wc.watch(ctx, nil)
}

// Watch opens a stream of the events of this Workspace alone.
func (w *Workspace) Watch(ctx context.Context) (*WorkspaceWatcher, error) {
	return w.c.Workspaces().watch(ctx, w.ref())
}

func (wc *WorkspaceClient) watch(ctx context.Context, ref *metav1.ObjectReference) (*WorkspaceWatcher, error) {
	if err := wc.c.ensureOpen(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	strm, err := wc.c.MainService().WatchWorkspace(ctx, &cordiumv1.WatchWorkspaceRequest{
		WorkspaceRef: ref,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	ret := &WorkspaceWatcher{
		events: make(chan WorkspaceEvent, defaultOutputBuffer),
		cancel: cancel,
	}

	go func() {
		defer close(ret.events)
		defer cancel()

		for {
			msg, err := strm.Recv()
			if err != nil {
				ret.finish(streamErr(ctx, err))
				return
			}

			ev, ok := workspaceEventFrom(wc.c, msg)
			if !ok {
				continue
			}

			select {
			case ret.events <- ev:
			case <-ctx.Done():
				ret.finish(ctx.Err())
				return
			case <-wc.c.closedCh():
				ret.finish(ErrClientClosed)
				return
			}
		}
	}()

	return ret, nil
}

func workspaceEventFrom(c *Client, msg *cordiumv1.WatchWorkspaceResponse) (WorkspaceEvent, bool) {
	switch msg.GetType().(type) {
	case *cordiumv1.WatchWorkspaceResponse_Create_:
		return WorkspaceEvent{
			Type:      EventCreated,
			Workspace: newWorkspace(c, msg.GetCreate().GetItem()),
		}, true
	case *cordiumv1.WatchWorkspaceResponse_Update_:
		return WorkspaceEvent{
			Type:      EventUpdated,
			Workspace: newWorkspace(c, msg.GetUpdate().GetNewItem()),
			Previous:  msg.GetUpdate().GetOldItem(),
		}, true
	case *cordiumv1.WatchWorkspaceResponse_Delete_:
		return WorkspaceEvent{
			Type:      EventDeleted,
			Workspace: newWorkspace(c, msg.GetDelete().GetItem()),
		}, true
	default:
		return WorkspaceEvent{}, false
	}
}

// Events returns the channel on which the events are published. It is closed
// once the watch ends.
func (w *WorkspaceWatcher) Events() <-chan WorkspaceEvent { return w.events }

// Err returns the error that ended the watch. It returns nil while the watch is
// still running and once it was closed by the caller.
func (w *WorkspaceWatcher) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// Close ends the watch and releases its stream. It is safe to call it more than
// once.
func (w *WorkspaceWatcher) Close() error {
	w.cancel()
	return nil
}

func (w *WorkspaceWatcher) finish(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.done {
		w.done = true
		w.err = err
	}
}

// streamErr normalizes the end of a server stream: a clean end of stream and a
// cancellation by the caller are not failures.
func streamErr(ctx context.Context, err error) error {
	switch {
	case err == nil, errors.Is(err, io.EOF):
		return nil
	case ctx.Err() != nil:
		return nil
	case IsCanceled(err):
		return nil
	default:
		return err
	}
}

type backoff struct {
	current time.Duration
}

func newBackoff() *backoff {
	return &backoff{current: 250 * time.Millisecond}
}

func (b *backoff) sleep(ctx context.Context) error {
	timer := time.NewTimer(b.current)
	defer timer.Stop()

	if b.current < 5*time.Second {
		b.current *= 2
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
