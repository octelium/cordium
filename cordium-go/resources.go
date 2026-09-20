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

/*
 * GitProviders
 */

// GitProviderClient is the GitProvider API. A GitProvider configures OAuth2
// authentication against a git hosting service inside a Space. Once it is
// attached to a Template, the User's stored OAuth2 token is injected into that
// Template's Workspaces, which enables git clone, git push and the other
// authenticated operations with no manual credential configuration.
type GitProviderClient struct {
	c *Client
}

// CreateGitHub creates a GitHub GitProvider. The OAuth2 application's client
// secret is read by the Cluster from a Secret of the same Space.
func (gc *GitProviderClient) CreateGitHub(ctx context.Context, name, clientID, clientSecretName string,
	scopes ...string) (*cordiumv1.GitProvider, error) {

	if clientID == "" || clientSecretName == "" {
		return nil, invalidArgumentf("a GitHub GitProvider needs a client ID and a client secret Secret")
	}

	return gc.create(ctx, name, &cordiumv1.GitProvider_Spec{
		Type: &cordiumv1.GitProvider_Spec_Github_{
			Github: &cordiumv1.GitProvider_Spec_Github{
				ClientID: clientID,
				ClientSecret: &cordiumv1.GitProvider_Spec_Github_ClientSecret{
					Type: &cordiumv1.GitProvider_Spec_Github_ClientSecret_FromSecret{
						FromSecret: clientSecretName,
					},
				},
				Scopes: scopes,
			},
		},
	})
}

// CreateGitLab creates a GitLab GitProvider.
func (gc *GitProviderClient) CreateGitLab(ctx context.Context, name, clientID, clientSecretName string,
	scopes ...string) (*cordiumv1.GitProvider, error) {

	if clientID == "" || clientSecretName == "" {
		return nil, invalidArgumentf("a GitLab GitProvider needs a client ID and a client secret Secret")
	}

	return gc.create(ctx, name, &cordiumv1.GitProvider_Spec{
		Type: &cordiumv1.GitProvider_Spec_Gitlab_{
			Gitlab: &cordiumv1.GitProvider_Spec_Gitlab{
				ClientID: clientID,
				ClientSecret: &cordiumv1.GitProvider_Spec_Gitlab_ClientSecret{
					Type: &cordiumv1.GitProvider_Spec_Gitlab_ClientSecret_FromSecret{
						FromSecret: clientSecretName,
					},
				},
				Scopes: scopes,
			},
		},
	})
}

// OAuth2Provider describes a generic OAuth2 git hosting service, which is what
// the self-hosted and the less common services need.
type OAuth2Provider struct {
	// ClientID is the OAuth2 application's client ID.
	ClientID string
	// ClientSecretName is the name of a Secret in the same Space that holds the
	// OAuth2 application's client secret.
	ClientSecretName string
	// AuthURL is the provider's authorization endpoint URL.
	AuthURL string
	// TokenURL is the provider's token endpoint URL.
	TokenURL string
	// Scopes is the list of the requested OAuth2 scopes. At least one is
	// required.
	Scopes []string
}

// CreateOAuth2 creates a generic OAuth2 GitProvider.
func (gc *GitProviderClient) CreateOAuth2(ctx context.Context, name string,
	provider OAuth2Provider) (*cordiumv1.GitProvider, error) {

	switch {
	case provider.ClientID == "" || provider.ClientSecretName == "":
		return nil, invalidArgumentf("an OAuth2 GitProvider needs a client ID and a client secret Secret")
	case provider.AuthURL == "" || provider.TokenURL == "":
		return nil, invalidArgumentf("an OAuth2 GitProvider needs an authorization URL and a token URL")
	case len(provider.Scopes) == 0:
		return nil, invalidArgumentf("an OAuth2 GitProvider needs at least one scope")
	}

	return gc.create(ctx, name, &cordiumv1.GitProvider_Spec{
		Type: &cordiumv1.GitProvider_Spec_Oauth2{
			Oauth2: &cordiumv1.GitProvider_Spec_OAuth2{
				ClientID: provider.ClientID,
				ClientSecret: &cordiumv1.GitProvider_Spec_OAuth2_ClientSecret{
					Type: &cordiumv1.GitProvider_Spec_OAuth2_ClientSecret_FromSecret{
						FromSecret: provider.ClientSecretName,
					},
				},
				AuthURL:  provider.AuthURL,
				TokenURL: provider.TokenURL,
				Scopes:   provider.Scopes,
			},
		},
	})
}

func (gc *GitProviderClient) create(ctx context.Context, name string,
	spec *cordiumv1.GitProvider_Spec) (*cordiumv1.GitProvider, error) {

	if err := gc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty GitProvider name")
	}

	return gc.c.MainService().CreateGitProvider(ctx, &cordiumv1.GitProvider{
		Metadata: &metav1.Metadata{Name: name},
		Spec:     spec,
		Status:   &cordiumv1.GitProvider_Status{},
	})
}

// Update replaces a GitProvider.
func (gc *GitProviderClient) Update(ctx context.Context, provider *cordiumv1.GitProvider) (*cordiumv1.GitProvider, error) {
	if err := gc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, invalidArgumentf("nil GitProvider")
	}
	return gc.c.MainService().UpdateGitProvider(ctx, provider)
}

// Get retrieves a GitProvider.
func (gc *GitProviderClient) Get(ctx context.Context, name string) (*cordiumv1.GitProvider, error) {
	if err := gc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty GitProvider name")
	}
	return gc.c.MainService().GetGitProvider(ctx, getOptionsFor(name))
}

// Delete deletes a GitProvider.
func (gc *GitProviderClient) Delete(ctx context.Context, name string) error {
	if err := gc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty GitProvider name")
	}
	_, err := gc.c.MainService().DeleteGitProvider(ctx, deleteOptionsFor(name))
	return err
}

// GitProviderList is a single page of GitProviders.
type GitProviderList struct {
	// Items is the page's GitProviders.
	Items []*cordiumv1.GitProvider
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the GitProviders of a Space, which is chosen with
// [InSpace].
func (gc *GitProviderClient) List(ctx context.Context, opts ...ListOption) (*GitProviderList, error) {
	if err := gc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := gc.c.MainService().ListGitProvider(ctx, &cordiumv1.ListGitProviderOptions{
		Common:   cfg.common(),
		SpaceRef: cfg.spaceRef,
	})
	if err != nil {
		return nil, err
	}

	return &GitProviderList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every GitProvider of a Space, fetching the pages as it
// goes.
func (gc *GitProviderClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*cordiumv1.GitProvider, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.GitProvider, PageInfo, error) {
			list, err := gc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

/*
 * Memberships
 */

// Role is the level of access that a Member has inside a Space.
type Role uint8

const (
	// RoleUser can use a Space (e.g. create Workspaces in it) but cannot manage
	// it. It is the default.
	RoleUser Role = iota + 1
	// RoleAdmin can manage a Space's Templates, Secrets, GitProviders and
	// Memberships.
	RoleAdmin
	// RoleOwner has full control over a Space, including deleting it and
	// managing its Owners. Granting it requires the caller to be an Owner.
	RoleOwner
)

// String returns the name of the role.
func (r Role) String() string {
	switch r {
	case RoleUser:
		return "USER"
	case RoleAdmin:
		return "ADMIN"
	case RoleOwner:
		return "OWNER"
	default:
		return "UNKNOWN"
	}
}

func (r Role) createRole() (cordiumv1.CreateMembershipRequest_Role, error) {
	switch r {
	case RoleUser:
		return cordiumv1.CreateMembershipRequest_USER, nil
	case RoleAdmin:
		return cordiumv1.CreateMembershipRequest_ADMIN, nil
	case RoleOwner:
		return cordiumv1.CreateMembershipRequest_OWNER, nil
	default:
		return 0, invalidArgumentf("unknown Role %d", r)
	}
}

func (r Role) specRole() (cordiumv1.Membership_Spec_Role, error) {
	switch r {
	case RoleUser:
		return cordiumv1.Membership_Spec_USER, nil
	case RoleAdmin:
		return cordiumv1.Membership_Spec_ADMIN, nil
	case RoleOwner:
		return cordiumv1.Membership_Spec_OWNER, nil
	default:
		return 0, invalidArgumentf("unknown Role %d", r)
	}
}

// MembershipClient is the Membership API. A Membership binds an Octelium User
// to a Space with a Role, and it is what grants that User access to the Space's
// Templates, Workspaces, Secrets and GitProviders.
type MembershipClient struct {
	c *Client
}

// Add adds a User, identified by their email address, as a Member of a Space.
// The User must already exist in the Cluster and the caller must be at least an
// ADMIN Member of the Space. Members can currently only be added to
// ORGANIZATION Spaces.
func (mc *MembershipClient) Add(ctx context.Context, spaceName, email string, role Role) (*cordiumv1.Membership, error) {
	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}
	switch {
	case spaceName == "":
		return nil, invalidArgumentf("empty Space name")
	case email == "":
		return nil, invalidArgumentf("empty User email address")
	}

	pbRole, err := role.createRole()
	if err != nil {
		return nil, err
	}

	return mc.c.MainService().CreateMembership(ctx, &cordiumv1.CreateMembershipRequest{
		Role:     pbRole,
		SpaceRef: nameRef(spaceName),
		UserType: &cordiumv1.CreateMembershipRequest_Email{Email: email},
	})
}

// AddUser adds an Octelium User, identified by their Cluster User name, as a
// Member of a Space.
func (mc *MembershipClient) AddUser(ctx context.Context, spaceName, userName string, role Role) (*cordiumv1.Membership, error) {
	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}
	switch {
	case spaceName == "":
		return nil, invalidArgumentf("empty Space name")
	case userName == "":
		return nil, invalidArgumentf("empty User name")
	}

	pbRole, err := role.createRole()
	if err != nil {
		return nil, err
	}

	return mc.c.MainService().CreateMembership(ctx, &cordiumv1.CreateMembershipRequest{
		Role:     pbRole,
		SpaceRef: nameRef(spaceName),
		UserType: &cordiumv1.CreateMembershipRequest_UserRef{UserRef: nameRef(userName)},
	})
}

// SetRole changes the Role of an existing Membership. Granting the OWNER Role
// requires the caller to be an OWNER themselves.
func (mc *MembershipClient) SetRole(ctx context.Context, name string, role Role) (*cordiumv1.Membership, error) {
	current, err := mc.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	pbRole, err := role.specRole()
	if err != nil {
		return nil, err
	}

	if current.Spec == nil {
		current.Spec = &cordiumv1.Membership_Spec{}
	}
	current.Spec.Role = pbRole

	return mc.c.MainService().UpdateMembership(ctx, current)
}

// Get retrieves a Membership. The caller must be a Member of its Space.
func (mc *MembershipClient) Get(ctx context.Context, name string) (*cordiumv1.Membership, error) {
	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Membership name")
	}
	return mc.c.MainService().GetMembership(ctx, getOptionsFor(name))
}

// Mine retrieves the calling User's own Membership in a Space, which is how an
// application finds out its own Role there.
func (mc *MembershipClient) Mine(ctx context.Context, spaceName string) (*cordiumv1.Membership, error) {
	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if spaceName == "" {
		return nil, invalidArgumentf("empty Space name")
	}
	return mc.c.MainService().GetSpaceMembership(ctx, &cordiumv1.GetSpaceMembershipRequest{
		SpaceRef: nameRef(spaceName),
	})
}

// Remove removes a Member from a Space. A Space's creator cannot be removed.
func (mc *MembershipClient) Remove(ctx context.Context, name string) error {
	if err := mc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Membership name")
	}
	_, err := mc.c.MainService().DeleteMembership(ctx, deleteOptionsFor(name))
	return err
}

// MembershipList is a single page of Memberships.
type MembershipList struct {
	// Items is the page's Memberships.
	Items []*cordiumv1.Membership
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the Memberships of a Space, which is chosen with
// [InSpace].
func (mc *MembershipClient) List(ctx context.Context, opts ...ListOption) (*MembershipList, error) {
	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := mc.c.MainService().ListMembership(ctx, &cordiumv1.ListMembershipOptions{
		Common:   cfg.common(),
		SpaceRef: cfg.spaceRef,
	})
	if err != nil {
		return nil, err
	}

	return &MembershipList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every Membership of a Space, fetching the pages as it goes.
func (mc *MembershipClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*cordiumv1.Membership, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.Membership, PageInfo, error) {
			list, err := mc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

/*
 * Regions
 */

// RegionClient is the read-only Region API. Regions are managed by the Cluster
// administrators; Users only choose among them when starting a Workspace or
// when setting their preferred Region.
type RegionClient struct {
	c *Client
}

// List returns the Regions of the Cluster that are enabled to host Workspaces.
func (rc *RegionClient) List(ctx context.Context, opts ...ListOption) ([]*cordiumv1.Region, error) {
	if err := rc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := rc.c.MainService().ListRegion(ctx, &cordiumv1.ListRegionOptions{
		Common: cfg.common(),
	})
	if err != nil {
		return nil, err
	}
	return list.GetItems(), nil
}

/*
 * UserConfig
 */

// UserConfigClient is the per-User configuration API. A UserConfig applies to
// all of its User's Workspaces whatever their Space or Template, and it is what
// carries the User's dotfiles, their personal environment variables and Tasks
// and their preferred Region.
type UserConfigClient struct {
	c *Client
}

// Get retrieves the calling User's UserConfig. The Cluster creates it
// automatically upon the first call.
func (uc *UserConfigClient) Get(ctx context.Context) (*cordiumv1.UserConfig, error) {
	if err := uc.c.ensureOpen(); err != nil {
		return nil, err
	}
	return uc.c.MainService().GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
}

// Update replaces the calling User's UserConfig.
func (uc *UserConfigClient) Update(ctx context.Context, config *cordiumv1.UserConfig) (*cordiumv1.UserConfig, error) {
	if err := uc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if config == nil {
		return nil, invalidArgumentf("nil UserConfig")
	}
	return uc.c.MainService().UpdateUserConfig(ctx, config)
}

// Modify reads the calling User's UserConfig, hands its spec to fn and writes
// the result back. It is the read-modify-write helper that most changes want:
//
//	_, err := c.UserConfig().Modify(ctx, func(spec *cordiumv1.UserConfig_Spec) error {
//		spec.PreferredRegion = "eu-west"
//		return nil
//	})
func (uc *UserConfigClient) Modify(ctx context.Context,
	fn func(*cordiumv1.UserConfig_Spec) error) (*cordiumv1.UserConfig, error) {

	if fn == nil {
		return nil, invalidArgumentf("nil modify function")
	}

	current, err := uc.Get(ctx)
	if err != nil {
		return nil, err
	}
	if current.Spec == nil {
		current.Spec = &cordiumv1.UserConfig_Spec{}
	}
	if err := fn(current.Spec); err != nil {
		return nil, err
	}

	return uc.Update(ctx, current)
}

// SetPreferredRegion sets the Region in which the User's Workspaces are
// preferably run.
func (uc *UserConfigClient) SetPreferredRegion(ctx context.Context, region string) (*cordiumv1.UserConfig, error) {
	return uc.Modify(ctx, func(spec *cordiumv1.UserConfig_Spec) error {
		spec.PreferredRegion = region
		return nil
	})
}

// SetDotfiles points the User's Workspaces at a dotfiles repository, which is
// cloned at the beginning of the PREPARING phase and whose first install script
// is then executed. An empty branch uses the repository's default one.
func (uc *UserConfigClient) SetDotfiles(ctx context.Context, url, branch string) (*cordiumv1.UserConfig, error) {
	if url == "" {
		return nil, invalidArgumentf("empty dotfiles repository URL")
	}
	return uc.Modify(ctx, func(spec *cordiumv1.UserConfig_Spec) error {
		spec.Dotfiles = &cordiumv1.UserConfig_Spec_Dotfiles{
			Url:    url,
			Branch: branch,
		}
		return nil
	})
}

/*
 * Management
 */

// ManagementClient is the administrative API of the Cluster. Its methods
// require Cluster administrator privileges.
type ManagementClient struct {
	c *Client
}

// GetClusterConfig retrieves the Cluster Configuration, which is the single
// source of truth for the Space ownership policy, the Workspace storage class
// selection, the Cluster-wide resource limits and the Workspace timeouts.
func (mc *ManagementClient) GetClusterConfig(ctx context.Context) (*cordiumv1.ClusterConfig, error) {
	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}
	return mc.c.ManagementService().GetClusterConfig(ctx, &cordiumv1.GetClusterConfigRequest{})
}

// UpdateClusterConfig replaces the Cluster Configuration.
func (mc *ManagementClient) UpdateClusterConfig(ctx context.Context,
	config *cordiumv1.ClusterConfig) (*cordiumv1.ClusterConfig, error) {

	if err := mc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if config == nil {
		return nil, invalidArgumentf("nil ClusterConfig")
	}
	return mc.c.ManagementService().UpdateClusterConfig(ctx, config)
}

// ModifyClusterConfig reads the Cluster Configuration, hands its spec to fn and
// writes the result back.
func (mc *ManagementClient) ModifyClusterConfig(ctx context.Context,
	fn func(*cordiumv1.ClusterConfig_Spec) error) (*cordiumv1.ClusterConfig, error) {

	if fn == nil {
		return nil, invalidArgumentf("nil modify function")
	}

	current, err := mc.GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}
	if current.Spec == nil {
		current.Spec = &cordiumv1.ClusterConfig_Spec{}
	}
	if err := fn(current.Spec); err != nil {
		return nil, err
	}

	return mc.UpdateClusterConfig(ctx, current)
}
