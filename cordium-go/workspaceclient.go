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

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
)

// WorkspaceClient is the Workspace (i.e. sandbox) API. It is obtained from
// [Client.Workspaces].
type WorkspaceClient struct {
	c *Client
}

// Create creates a Workspace from a set of spec options. The Workspace is
// created in the STOPPED state and the Cluster assigns it a short randomly
// generated name.
//
//	ws, err := c.Workspaces().Create(ctx,
//		cordium.WithImage("node:20"),
//		cordium.WithRepo("https://github.com/myorg/api", cordium.WithBranch("main")),
//		cordium.WithApp("web", 3000, cordium.AsDefaultApp()),
//	)
//
// Use [WorkspaceClient.Run] to create, start and wait for a Workspace in one
// call.
func (wc *WorkspaceClient) Create(ctx context.Context, opts ...WorkspaceOption) (*Workspace, error) {
	if err := wc.c.ensureOpen(); err != nil {
		return nil, err
	}

	builder, err := newSpecBuilder(opts...)
	if err != nil {
		return nil, err
	}
	if builder.gitProvider != "" {
		return nil, invalidArgumentf(
			"WithGitProvider only applies to Templates; set it on the Template that the Workspace is created from")
	}

	req := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			DisplayName: builder.displayName,
		},
		Spec: builder.spec,
		Status: &cordiumv1.Workspace_Status{
			TemplateRef:          builder.templateRef,
			WorkspaceSnapshotRef: builder.snapshotRef,
		},
	}

	ws, err := wc.c.MainService().CreateWorkspace(ctx, req)
	if err != nil {
		return nil, err
	}

	return newWorkspace(wc.c, ws), nil
}

// Run creates a Workspace, starts it and waits for it to reach the RUNNING
// state. It is the one call that a script or an agent needs in order to obtain
// a usable sandbox:
//
//	ws, err := c.Workspaces().Run(ctx, cordium.WithImage("python:3.11"), cordium.Ephemeral())
//	if err != nil {
//		return err
//	}
//	defer ws.Delete(context.WithoutCancel(ctx))
//
// If the run fails, the Workspace is left in place so that its logs and its
// failure can be inspected, and the returned error is a
// [*WorkspaceFailureError].
//
// The start-time options ([WithRunVar], [WithRegion]) are accepted alongside
// the spec options.
func (wc *WorkspaceClient) Run(ctx context.Context, opts ...RunOption) (*Workspace, error) {
	var (
		specOpts  []WorkspaceOption
		startOpts []StartOption
	)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		specOpt, startOpt := opt.runOption()
		if specOpt != nil {
			specOpts = append(specOpts, specOpt)
		}
		if startOpt != nil {
			startOpts = append(startOpts, startOpt)
		}
	}

	ws, err := wc.Create(ctx, specOpts...)
	if err != nil {
		return nil, err
	}

	if err := ws.Start(ctx, startOpts...); err != nil {
		return nil, err
	}

	if err := ws.WaitUntilRunning(ctx); err != nil {
		return nil, err
	}

	return ws, nil
}

// Start starts an existing Workspace by name and waits for nothing. It is
// shorthand for a Get followed by [Workspace.Start].
func (wc *WorkspaceClient) Start(ctx context.Context, name string, opts ...StartOption) (*Workspace, error) {
	ws, err := wc.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := ws.Start(ctx, opts...); err != nil {
		return nil, err
	}
	return ws, nil
}

// Stop stops an existing Workspace by name.
func (wc *WorkspaceClient) Stop(ctx context.Context, name string) (*Workspace, error) {
	ws, err := wc.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := ws.Stop(ctx); err != nil {
		return nil, err
	}
	return ws, nil
}

// Get retrieves a Workspace by its Cluster-assigned name.
func (wc *WorkspaceClient) Get(ctx context.Context, name string) (*Workspace, error) {
	if err := wc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Workspace name")
	}

	ws, err := wc.c.MainService().GetWorkspace(ctx, getOptionsFor(name))
	if err != nil {
		return nil, err
	}
	return newWorkspace(wc.c, ws), nil
}

// GetByUID retrieves a Workspace by its Cluster-wide unique identifier.
func (wc *WorkspaceClient) GetByUID(ctx context.Context, uid string) (*Workspace, error) {
	if err := wc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if uid == "" {
		return nil, invalidArgumentf("empty Workspace UID")
	}

	ws, err := wc.c.MainService().GetWorkspace(ctx, &metav1.GetOptions{Uid: uid})
	if err != nil {
		return nil, err
	}
	return newWorkspace(wc.c, ws), nil
}

// WorkspaceList is a single page of Workspaces.
type WorkspaceList struct {
	// Items is the page's Workspaces.
	Items []*Workspace
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the Workspaces that the calling User owns.
// [InSpace] and [OfTemplate] narrow the result down.
func (wc *WorkspaceClient) List(ctx context.Context, opts ...ListOption) (*WorkspaceList, error) {
	if err := wc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	req := &cordiumv1.ListWorkspaceOptions{Common: cfg.common()}
	switch {
	case cfg.spaceRef != nil:
		req.Filter = &cordiumv1.ListWorkspaceOptions_SpaceRef{SpaceRef: cfg.spaceRef}
	case cfg.templateRef != nil:
		req.Filter = &cordiumv1.ListWorkspaceOptions_TemplateRef{TemplateRef: cfg.templateRef}
	}

	list, err := wc.c.MainService().ListWorkspace(ctx, req)
	if err != nil {
		return nil, err
	}

	ret := &WorkspaceList{
		Items: make([]*Workspace, 0, len(list.GetItems())),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}
	for _, item := range list.GetItems() {
		ret.Items = append(ret.Items, newWorkspace(wc.c, item))
	}
	return ret, nil
}

// All iterates over every Workspace that the calling User owns, fetching the
// pages as it goes:
//
//	for ws, err := range c.Workspaces().All(ctx) {
//		if err != nil {
//			return err
//		}
//		fmt.Println(ws.Name(), ws.State())
//	}
//
// The iteration stops at the first error, which is yielded with a nil
// Workspace.
func (wc *WorkspaceClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*Workspace, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*Workspace, PageInfo, error) {
			list, err := wc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

// Delete deletes a Workspace by name.
func (wc *WorkspaceClient) Delete(ctx context.Context, name string) error {
	if err := wc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Workspace name")
	}

	_, err := wc.c.MainService().DeleteWorkspace(ctx, deleteOptionsFor(name))
	return err
}

// RunOption is accepted by [WorkspaceClient.Run]. Every [WorkspaceOption] and
// every [StartOption] is one.
type RunOption interface {
	runOption() (WorkspaceOption, StartOption)
}

func (o WorkspaceOption) runOption() (WorkspaceOption, StartOption) { return o, nil }

func (o StartOption) runOption() (WorkspaceOption, StartOption) { return nil, o }
