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

// TemplateClient is the Template API. A Template is a reusable Workspace
// configuration inside a Space: every Workspace is created from one and
// inherits its spec.
//
// Templates are described with the same options as Workspaces, minus the few
// that only a single Workspace can carry ([WithApp] and [Ephemeral]).
type TemplateClient struct {
	c *Client
}

// Create creates a Template inside a Space. The name can be short (e.g.
// "ci-runner", which lands in the User's default Space) or qualified with its
// Space (e.g. "ci-runner.my-project"). The caller must be at least an ADMIN
// Member of the Space.
//
//	tpl, err := c.Templates().Create(ctx, "ci-runner.my-project",
//		cordium.WithImage("golang:1.25"),
//		cordium.WithRepo("https://github.com/myorg/api"),
//		cordium.WithTask("deps", "go mod download"),
//	)
func (tc *TemplateClient) Create(ctx context.Context, name string, opts ...WorkspaceOption) (*cordiumv1.Template, error) {
	if err := tc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Template name")
	}

	builder, err := newSpecBuilder(opts...)
	if err != nil {
		return nil, err
	}

	spec, err := templateSpecFrom(builder)
	if err != nil {
		return nil, err
	}

	return tc.c.MainService().CreateTemplate(ctx, &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name:        name,
			DisplayName: builder.displayName,
		},
		Spec:   spec,
		Status: &cordiumv1.Template_Status{},
	})
}

// Update applies spec options to an existing Template, starting from its
// current spec.
func (tc *TemplateClient) Update(ctx context.Context, name string, opts ...WorkspaceOption) (*cordiumv1.Template, error) {
	current, err := tc.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	builder, err := newSpecBuilder(append([]WorkspaceOption{
		FromSpec(workspaceSpecFromTemplate(current.GetSpec())),
	}, opts...)...)

	if err != nil {
		return nil, err
	}
	if builder.gitProvider == "" {
		builder.gitProvider = current.GetSpec().GetGitProvider()
	}

	spec, err := templateSpecFrom(builder)
	if err != nil {
		return nil, err
	}

	current.Spec = spec
	if builder.displayName != "" {
		if current.Metadata == nil {
			current.Metadata = &metav1.Metadata{}
		}
		current.Metadata.DisplayName = builder.displayName
	}

	return tc.c.MainService().UpdateTemplate(ctx, current)
}

// Get retrieves a Template.
func (tc *TemplateClient) Get(ctx context.Context, name string) (*cordiumv1.Template, error) {
	if err := tc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Template name")
	}
	return tc.c.MainService().GetTemplate(ctx, getOptionsFor(name))
}

// Delete deletes a Template. The default Template of a Space cannot be deleted.
func (tc *TemplateClient) Delete(ctx context.Context, name string) error {
	if err := tc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Template name")
	}
	_, err := tc.c.MainService().DeleteTemplate(ctx, deleteOptionsFor(name))
	return err
}

// TemplateList is a single page of Templates.
type TemplateList struct {
	// Items is the page's Templates.
	Items []*cordiumv1.Template
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the Templates of a Space that the calling User is a
// Member of. The Space is chosen with [InSpace].
func (tc *TemplateClient) List(ctx context.Context, opts ...ListOption) (*TemplateList, error) {
	if err := tc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := tc.c.MainService().ListTemplate(ctx, &cordiumv1.ListTemplateOptions{
		Common:   cfg.common(),
		SpaceRef: cfg.spaceRef,
	})
	if err != nil {
		return nil, err
	}

	return &TemplateList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every Template of a Space, fetching the pages as it goes.
func (tc *TemplateClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*cordiumv1.Template, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.Template, PageInfo, error) {
			list, err := tc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

// Build starts a pre-build of a Template. A pre-build runs a hidden Workspace
// whose storage is snapshotted once it completes, so that the Template's later
// Workspaces are restored from that snapshot instead of being initialized from
// scratch. It is the single biggest lever on Workspace startup time.
//
// A Template has at most one running pre-build: starting a new one cancels the
// running one. The tags default to "latest".
//
// The pre-build itself is asynchronous; [TemplateClient.WaitForBuild] follows
// it to completion.
func (tc *TemplateClient) Build(ctx context.Context, name string, tags ...string) (*cordiumv1.Template, error) {
	if err := tc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Template name")
	}

	return tc.c.MainService().BuildTemplate(ctx, &cordiumv1.BuildTemplateRequest{
		TemplateRef: nameRef(name),
		Tags:        tags,
	})
}

// CancelBuild cancels the pre-build of a Template that is currently running, if
// any.
func (tc *TemplateClient) CancelBuild(ctx context.Context, name string) (*cordiumv1.Template, error) {
	if err := tc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Template name")
	}

	return tc.c.MainService().CancelBuildTemplate(ctx, &cordiumv1.CancelBuildTemplateRequest{
		TemplateRef: nameRef(name),
	})
}

// WaitForBuild polls a Template until its running pre-build completes and
// returns that pre-build. It fails with a [*WorkspaceFailureError] when the
// pre-build fails, since a pre-build is itself a Workspace run.
//
// The Cluster publishes no pre-build stream, so this polls; the caller's
// context bounds the wait.
func (tc *TemplateClient) WaitForBuild(ctx context.Context, name string) (*cordiumv1.Template_Status_BuildInfo_Build, error) {
	if err := tc.c.ensureOpen(); err != nil {
		return nil, err
	}

	tpl, err := tc.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	buildID := tpl.GetStatus().GetBuildInfo().GetCurrentRunningBuildID()
	if buildID == "" {
		return nil, invalidArgumentf("Template %q has no running pre-build", name)
	}

	backoff := newBackoff()
	for {
		if build := findBuild(tpl, buildID); build != nil {
			switch build.GetState() {
			case cordiumv1.Template_Status_BuildInfo_Build_STATE_READY:
				return build, nil
			case cordiumv1.Template_Status_BuildInfo_Build_STATE_FAILED:
				if failure := build.GetFailure(); failure != nil {
					return build, &WorkspaceFailureError{
						Workspace: "pre-build " + buildID + " of Template " + name,
						Failure:   failure,
					}
				}
				return build, ErrWorkspaceStopped
			}
		}

		if err := backoff.sleep(ctx); err != nil {
			return nil, err
		}

		tpl, err = tc.Get(ctx, name)
		if err != nil {
			return nil, err
		}
	}
}

func findBuild(tpl *cordiumv1.Template, id string) *cordiumv1.Template_Status_BuildInfo_Build {
	for _, build := range tpl.GetStatus().GetBuildInfo().GetBuilds() {
		if build.GetId() == id {
			return build
		}
	}
	return nil
}

func templateSpecFrom(b *specBuilder) (*cordiumv1.Template_Spec, error) {
	switch {
	case len(b.spec.GetApplications()) > 0:
		return nil, invalidArgumentf(
			"a Template cannot declare Applications; set them on the Workspaces that it creates")
	case b.spec.GetIsEphemeral():
		return nil, invalidArgumentf(
			"a Template cannot be ephemeral; set Ephemeral() on the Workspaces that it creates")
	case b.templateRef != nil:
		return nil, invalidArgumentf("a Template cannot be created from another Template")
	case b.snapshotRef != nil:
		return nil, invalidArgumentf(
			"a Template cannot be restored from a WorkspaceSnapshot; set FromSnapshot on the Workspaces that it creates")
	}

	return &cordiumv1.Template_Spec{
		Image:                  b.spec.GetImage(),
		Runtime:                b.spec.GetRuntime(),
		Repository:             b.spec.GetRepository(),
		AdditionalRepositories: b.spec.GetAdditionalRepositories(),
		Limit:                  b.spec.GetLimit(),
		Vars:                   b.spec.GetVars(),
		GitProvider:            b.gitProvider,
	}, nil
}

func workspaceSpecFromTemplate(spec *cordiumv1.Template_Spec) *cordiumv1.Workspace_Spec {
	return &cordiumv1.Workspace_Spec{
		Image:                  spec.GetImage(),
		Runtime:                spec.GetRuntime(),
		Repository:             spec.GetRepository(),
		AdditionalRepositories: spec.GetAdditionalRepositories(),
		Limit:                  spec.GetLimit(),
		Vars:                   spec.GetVars(),
	}
}
