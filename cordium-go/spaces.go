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

// SpaceClient is the Space API. A Space is Cordium's top-level namespace: it
// groups the Templates, Workspaces, Secrets, GitProviders and Memberships of an
// organizational unit, and its configuration cascades down to every Workspace
// it contains.
type SpaceClient struct {
	c *Client
}

type spaceConfig struct {
	displayName    string
	organization   bool
	spec           *cordiumv1.Space_Spec
	defaultLimit   *cordiumv1.Workspace_Spec_Limit
	maxLimit       *cordiumv1.Workspace_Spec_Limit
	envVars        []*cordiumv1.Workspace_Spec_Runtime_EnvVar
	tasks          []*cordiumv1.Workspace_Spec_Runtime_Task
	addCaps        []string
	dropCaps       []string
	disableSSH     bool
	disableSSHSet  bool
	hasRuntimeSpec bool
}

// SpaceOption configures a Space.
type SpaceOption func(*spaceConfig) error

// AsOrganizationSpace creates a shared, multi-Member ORGANIZATION Space instead
// of a personal one. Only an ORGANIZATION Space can have Members added to it
// and can define its own resource limits.
//
// The Space type follows from its name: a name with no parent lands in the
// calling User's personal namespace, while this option qualifies it with the
// "cordium" parent. A name that is already qualified is left alone.
func AsOrganizationSpace() SpaceOption {
	return func(c *spaceConfig) error {
		c.organization = true
		return nil
	}
}

// WithSpaceDisplayName sets a human readable name for the Space.
func WithSpaceDisplayName(displayName string) SpaceOption {
	return func(c *spaceConfig) error {
		c.displayName = displayName
		return nil
	}
}

// WithSpaceDefaultResources sets the compute allocation of the Space's
// Workspaces that do not define their own. It only applies to ORGANIZATION
// Spaces.
func WithSpaceDefaultResources(resources Resources) SpaceOption {
	return func(c *spaceConfig) error {
		c.defaultLimit = limitFrom(resources)
		return nil
	}
}

// WithSpaceMaxResources sets a hard cap that no Workspace of the Space can
// exceed. It only applies to ORGANIZATION Spaces.
func WithSpaceMaxResources(resources Resources) SpaceOption {
	return func(c *spaceConfig) error {
		c.maxLimit = limitFrom(resources)
		return nil
	}
}

// WithSpaceEnv injects an environment variable into every Workspace of the
// Space, whatever its Template.
func WithSpaceEnv(key, value string) SpaceOption {
	return func(c *spaceConfig) error {
		if key == "" {
			return invalidArgumentf("empty environment variable name")
		}
		c.hasRuntimeSpec = true
		c.envVars = append(c.envVars, &cordiumv1.Workspace_Spec_Runtime_EnvVar{
			Key:  key,
			Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: value},
		})
		return nil
	}
}

// WithSpaceEnvFromSecret injects an environment variable, whose value the
// Cluster reads from one of the Space's Secrets, into every Workspace of the
// Space. Secret-sourced environment variables only apply to ORGANIZATION
// Spaces.
func WithSpaceEnvFromSecret(key, secretName string) SpaceOption {
	return func(c *spaceConfig) error {
		switch {
		case key == "":
			return invalidArgumentf("empty environment variable name")
		case secretName == "":
			return invalidArgumentf("empty Secret name")
		}
		c.hasRuntimeSpec = true
		c.envVars = append(c.envVars, &cordiumv1.Workspace_Spec_Runtime_EnvVar{
			Key:  key,
			Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret{FromSecret: secretName},
		})
		return nil
	}
}

// WithSpaceTask runs a lifecycle Task in every Workspace of the Space. The
// lifecycle point is chosen with the Task options, defaulting to ON_CREATE.
func WithSpaceTask(name, run string, opts ...TaskOption) SpaceOption {
	return spaceTask(cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE, name, run, opts...)
}

// WithSpacePostStartTask runs a Task on every start of every Workspace of the
// Space.
func WithSpacePostStartTask(name, run string, opts ...TaskOption) SpaceOption {
	return spaceTask(cordiumv1.Workspace_Spec_Runtime_Task_POST_START, name, run, opts...)
}

// WithSpacePreStopTask runs a Task right before every Workspace of the Space is
// stopped.
func WithSpacePreStopTask(name, run string, opts ...TaskOption) SpaceOption {
	return spaceTask(cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP, name, run, opts...)
}

func spaceTask(typ cordiumv1.Workspace_Spec_Runtime_Task_Type,
	name, run string, opts ...TaskOption) SpaceOption {
	return func(c *spaceConfig) error {
		switch {
		case name == "":
			return invalidArgumentf("empty Task name")
		case run == "":
			return invalidArgumentf("empty Task command")
		}

		task := &cordiumv1.Workspace_Spec_Runtime_Task{Name: name, Run: run, Type: typ}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			if err := opt(task); err != nil {
				return err
			}
		}

		c.hasRuntimeSpec = true
		c.tasks = append(c.tasks, task)
		return nil
	}
}

// WithSpaceAddedCapabilities merges Linux capabilities into every Workspace of
// the Space.
func WithSpaceAddedCapabilities(capabilities ...string) SpaceOption {
	return func(c *spaceConfig) error {
		c.hasRuntimeSpec = true
		c.addCaps = append(c.addCaps, capabilities...)
		return nil
	}
}

// WithSpaceDroppedCapabilities drops Linux capabilities from every Workspace of
// the Space.
func WithSpaceDroppedCapabilities(capabilities ...string) SpaceOption {
	return func(c *spaceConfig) error {
		c.hasRuntimeSpec = true
		c.dropCaps = append(c.dropCaps, capabilities...)
		return nil
	}
}

// WithoutSSH denies SSH access to the Workspaces of the Space.
func WithoutSSH() SpaceOption {
	return func(c *spaceConfig) error {
		c.disableSSH, c.disableSSHSet = true, true
		return nil
	}
}

// WithSpaceSpec starts from an existing Space spec, which the later options
// then refine.
func WithSpaceSpec(spec *cordiumv1.Space_Spec) SpaceOption {
	return func(c *spaceConfig) error {
		if spec == nil {
			return invalidArgumentf("nil Space spec")
		}
		c.spec = spec
		return nil
	}
}

func (c *spaceConfig) build() *cordiumv1.Space_Spec {
	ret := c.spec
	if ret == nil {
		ret = &cordiumv1.Space_Spec{}
	}

	if c.defaultLimit != nil || c.maxLimit != nil {
		if ret.Limit == nil {
			ret.Limit = &cordiumv1.Space_Spec_Limit{}
		}
		if c.defaultLimit != nil {
			ret.Limit.DefaultLimit = c.defaultLimit
		}
		if c.maxLimit != nil {
			ret.Limit.MaxLimit = c.maxLimit
		}
	}

	if c.hasRuntimeSpec {
		if ret.Runtime == nil {
			ret.Runtime = &cordiumv1.Space_Spec_Runtime{}
		}
		ret.Runtime.EnvVars = append(ret.Runtime.EnvVars, c.envVars...)
		ret.Runtime.Tasks = append(ret.Runtime.Tasks, c.tasks...)
		if len(c.addCaps) > 0 || len(c.dropCaps) > 0 {
			if ret.Runtime.Capabilities == nil {
				ret.Runtime.Capabilities = &cordiumv1.Workspace_Spec_Runtime_Capabilities{}
			}
			ret.Runtime.Capabilities.Add = append(ret.Runtime.Capabilities.Add, c.addCaps...)
			ret.Runtime.Capabilities.Drop = append(ret.Runtime.Capabilities.Drop, c.dropCaps...)
		}
	}

	if c.disableSSHSet {
		if ret.Authorization == nil {
			ret.Authorization = &cordiumv1.Space_Spec_Authorization{}
		}
		ret.Authorization.DisableSSH = c.disableSSH
	}

	return ret
}

func limitFrom(resources Resources) *cordiumv1.Workspace_Spec_Limit {
	ret := &cordiumv1.Workspace_Spec_Limit{}
	if resources.CPUMillicores > 0 {
		ret.Cpu = &cordiumv1.Workspace_Spec_Limit_CPU{Millicores: resources.CPUMillicores}
	}
	if resources.MemoryMB > 0 {
		ret.Memory = &cordiumv1.Workspace_Spec_Limit_Memory{Megabytes: resources.MemoryMB}
	}
	if resources.StorageMB > 0 {
		ret.Storage = &cordiumv1.Workspace_Spec_Limit_Storage{Megabytes: resources.StorageMB}
	}
	return ret
}

// Create creates a Space owned by the calling User, along with a default
// Membership and a default Template. Whether a User may own a Space at all is
// decided by the Space ownership rules of the ClusterConfig.
//
//	spc, err := c.Spaces().Create(ctx, "my-project", cordium.AsOrganizationSpace())
func (sc *SpaceClient) Create(ctx context.Context, name string, opts ...SpaceOption) (*cordiumv1.Space, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Space name")
	}

	cfg := &spaceConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	if cfg.organization {
		name = OrganizationSpaceName(name)
	}

	return sc.c.MainService().CreateSpace(ctx, &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name:        name,
			DisplayName: cfg.displayName,
		},
		Spec:   cfg.build(),
		Status: &cordiumv1.Space_Status{},
	})
}

// Update applies options to an existing Space, starting from its current spec.
// The caller must be the Space's creator or one of its OWNER Members.
func (sc *SpaceClient) Update(ctx context.Context, name string, opts ...SpaceOption) (*cordiumv1.Space, error) {
	current, err := sc.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	cfg := &spaceConfig{spec: current.GetSpec()}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	current.Spec = cfg.build()
	if cfg.displayName != "" {
		if current.Metadata == nil {
			current.Metadata = &metav1.Metadata{}
		}
		current.Metadata.DisplayName = cfg.displayName
	}

	return sc.c.MainService().UpdateSpace(ctx, current)
}

// Get retrieves a Space. The caller must be a Member of it.
func (sc *SpaceClient) Get(ctx context.Context, name string) (*cordiumv1.Space, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Space name")
	}
	return sc.c.MainService().GetSpace(ctx, getOptionsFor(name))
}

// Delete deletes a Space along with all of its Memberships, Templates, Secrets
// and GitProviders.
func (sc *SpaceClient) Delete(ctx context.Context, name string) error {
	if err := sc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Space name")
	}
	_, err := sc.c.MainService().DeleteSpace(ctx, deleteOptionsFor(name))
	return err
}

// Leave removes the calling User's own Membership from a Space. A Space's
// creator cannot leave it, only delete it.
func (sc *SpaceClient) Leave(ctx context.Context, name string) error {
	if err := sc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Space name")
	}
	_, err := sc.c.MainService().LeaveSpace(ctx, &cordiumv1.LeaveSpaceRequest{
		SpaceRef: nameRef(name),
	})
	return err
}

// SpaceList is a single page of Spaces.
type SpaceList struct {
	// Items is the page's Spaces.
	Items []*cordiumv1.Space
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the Spaces that the calling User created. Use
// [MemberSpaces] to list the Spaces they are a Member of instead, and
// [UserSpaces] or [OrganizationSpaces] to restrict the type.
func (sc *SpaceClient) List(ctx context.Context, opts ...ListOption) (*SpaceList, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := sc.c.MainService().ListSpace(ctx, &cordiumv1.ListSpaceOptions{
		Common: cfg.common(),
		Type:   cfg.spaceType,
		Mode:   cfg.spaceMode,
	})
	if err != nil {
		return nil, err
	}

	return &SpaceList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every Space, fetching the pages as it goes.
func (sc *SpaceClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*cordiumv1.Space, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.Space, PageInfo, error) {
			list, err := sc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}
