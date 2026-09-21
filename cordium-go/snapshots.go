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
	"iter"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
)

// SnapshotState is the state of a WorkspaceSnapshot. It is an alias of the
// generated enum, so the generated constants and a SnapshotState are
// interchangeable.
type SnapshotState = cordiumv1.WorkspaceSnapshot_Status_State

// The states of a WorkspaceSnapshot.
const (
	SnapshotStateUnknown  = cordiumv1.WorkspaceSnapshot_Status_STATE_UNKNOWN
	SnapshotStateCreating = cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING
	SnapshotStateReady    = cordiumv1.WorkspaceSnapshot_Status_STATE_READY
	SnapshotStateFailed   = cordiumv1.WorkspaceSnapshot_Status_STATE_FAILED
)

// SnapshotClient is the WorkspaceSnapshot API. A WorkspaceSnapshot is a
// point-in-time checkpoint of the persistent storage of a Workspace out of
// which new Workspaces are created inside the same Space.
//
// It is obtained from [Client.Snapshots].
type SnapshotClient struct {
	c *Client
}

// Create takes a snapshot of the persistent storage of a Workspace. The
// Workspace is neither stopped nor restarted and it remains fully usable while
// the snapshot is being taken.
//
//	snapshot, err := c.Snapshots().Create(ctx, "before-upgrade", ws.Name())
//
// The snapshot is returned in the CREATING state since the storage backend
// takes it asynchronously. Use [SnapshotClient.WaitUntilReady] to wait for it
// to become restorable.
func (sc *SnapshotClient) Create(ctx context.Context,
	name string, workspace string) (*cordiumv1.WorkspaceSnapshot, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty WorkspaceSnapshot name")
	}
	if workspace == "" {
		return nil, invalidArgumentf("empty Workspace name")
	}

	return sc.c.MainService().CreateWorkspaceSnapshot(ctx, &cordiumv1.WorkspaceSnapshot{
		Metadata: &metav1.Metadata{
			Name: name,
		},
		Spec: &cordiumv1.WorkspaceSnapshot_Spec{},
		Status: &cordiumv1.WorkspaceSnapshot_Status{
			WorkspaceRef: &metav1.ObjectReference{Name: workspace},
		},
	})
}

// Get retrieves a WorkspaceSnapshot by name.
func (sc *SnapshotClient) Get(ctx context.Context, name string) (*cordiumv1.WorkspaceSnapshot, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty WorkspaceSnapshot name")
	}

	return sc.c.MainService().GetWorkspaceSnapshot(ctx, getOptionsFor(name))
}

// SnapshotList is a single page of WorkspaceSnapshots.
type SnapshotList struct {
	// Items is the page's WorkspaceSnapshots.
	Items []*cordiumv1.WorkspaceSnapshot
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the WorkspaceSnapshots that the calling User owns.
// [OfWorkspace] and [InSpace] narrow the result down.
func (sc *SnapshotClient) List(ctx context.Context, opts ...ListOption) (*SnapshotList, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	req := &cordiumv1.ListWorkspaceSnapshotOptions{Common: cfg.common()}
	switch {
	case cfg.workspaceRef != nil:
		req.Filter = &cordiumv1.ListWorkspaceSnapshotOptions_WorkspaceRef{WorkspaceRef: cfg.workspaceRef}
	case cfg.spaceRef != nil:
		req.Filter = &cordiumv1.ListWorkspaceSnapshotOptions_SpaceRef{SpaceRef: cfg.spaceRef}
	}

	list, err := sc.c.MainService().ListWorkspaceSnapshot(ctx, req)
	if err != nil {
		return nil, err
	}

	return &SnapshotList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every WorkspaceSnapshot that the calling User owns,
// fetching the pages as it goes. The iteration stops at the first error, which
// is yielded with a nil WorkspaceSnapshot.
func (sc *SnapshotClient) All(ctx context.Context,
	opts ...ListOption) iter.Seq2[*cordiumv1.WorkspaceSnapshot, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.WorkspaceSnapshot, PageInfo, error) {
			list, err := sc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

// Delete deletes a WorkspaceSnapshot along with its underlying storage
// snapshot.
func (sc *SnapshotClient) Delete(ctx context.Context, name string) error {
	if err := sc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty WorkspaceSnapshot name")
	}

	_, err := sc.c.MainService().DeleteWorkspaceSnapshot(ctx, deleteOptionsFor(name))
	return err
}

// WaitUntilReady polls a WorkspaceSnapshot until it becomes restorable. It
// returns a [*SnapshotFailureError] once the snapshot fails.
func (sc *SnapshotClient) WaitUntilReady(ctx context.Context,
	name string) (*cordiumv1.WorkspaceSnapshot, error) {

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		snapshot, err := sc.Get(ctx, name)
		if err != nil {
			return nil, err
		}

		switch snapshot.GetStatus().GetState() {
		case SnapshotStateReady:
			return snapshot, nil
		case SnapshotStateFailed:
			return nil, &SnapshotFailureError{Snapshot: snapshot}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Snapshot takes a snapshot of the Workspace's own persistent storage. It is
// shorthand for [SnapshotClient.Create] with the Workspace's name.
func (w *Workspace) Snapshot(ctx context.Context, name string) (*cordiumv1.WorkspaceSnapshot, error) {
	return w.c.Snapshots().Create(ctx, name, w.Name())
}
