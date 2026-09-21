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
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	octelium "github.com/octelium/octelium/octelium-go"
	"google.golang.org/grpc"
)

// Client is an authenticated Cordium Cluster client. It is safe for concurrent
// use and it should normally be created once and shared for the lifetime of the
// process.
//
// The resource accessors ([Client.Workspaces], [Client.Spaces], ...) are the
// high level API. The service accessors ([Client.MainService],
// [Client.WorkspaceService] and [Client.ManagementService]) expose the
// generated gRPC clients for everything the high level API does not cover.
type Client struct {
	cfg clientConfig

	octeliumC     *octelium.Client
	ownsOctelium  bool
	conn          *grpc.ClientConn
	mainC         cordiumv1.MainServiceClient
	workspaceC    cordiumv1.WorkspaceServiceClient
	managementC   cordiumv1.ManagementServiceClient
	closeOnce     sync.Once
	closeErr      error
	closed        chan struct{}
	workspaces    *WorkspaceClient
	spaces        *SpaceClient
	templates     *TemplateClient
	snapshots     *SnapshotClient
	secrets       *SecretClient
	userSecrets   *UserSecretClient
	gitProviders  *GitProviderClient
	memberships   *MembershipClient
	regions       *RegionClient
	userConfig    *UserConfigClient
	managementSvc *ManagementClient
}

// New creates a Client. The caller owns it and must Close it.
//
// With no options at all the Client reads its Cluster domain and its
// credentials from the environment, which is what makes it work unchanged
// inside CI jobs, Kubernetes Pods and Cordium Workspaces. See the package
// documentation for the list of the environment variables.
func New(ctx context.Context, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, invalidArgumentf("nil context")
	}

	cfg := clientConfig{
		userAgent:        defaultUserAgent,
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		waitPollInterval: defaultWaitPollInterval,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if cfg.domain == "" {
		cfg.domain = os.Getenv("CORDIUM_DOMAIN")
	}

	ret := &Client{
		cfg:    cfg,
		closed: make(chan struct{}),
	}

	if cfg.octeliumClient != nil {
		if len(cfg.octeliumOptions) > 0 {
			return nil, invalidArgumentf(
				"WithOcteliumClient and the Octelium level options are mutually exclusive")
		}
		ret.octeliumC = cfg.octeliumClient
	} else {
		octeliumOpts := []octelium.Option{
			octelium.WithUserAgent(cfg.userAgent),
			octelium.WithLogger(cfg.logger),
			// The Cordium portal serves the Workspace Applications under
			// underscore-separated hostnames, which the Octelium SDK's own
			// destination policy rejects. See Client.authorizeHTTPRequest.
			octelium.WithHTTPAuthorizationPolicy(func(req *http.Request) error {
				return ret.authorizeHTTPRequest(req)
			}),
		}
		if cfg.domain != "" {
			octeliumOpts = append(octeliumOpts, octelium.WithDomain(cfg.domain))
		}
		octeliumOpts = append(octeliumOpts, cfg.octeliumOptions...)

		octeliumC, err := octelium.New(ctx, octeliumOpts...)
		if err != nil {
			return nil, err
		}
		ret.octeliumC = octeliumC
		ret.ownsOctelium = true
	}

	ret.cfg.domain = ret.octeliumC.Domain()

	conn, err := ret.octeliumC.Conn(ctx)
	if err != nil {
		if ret.ownsOctelium {
			_ = ret.octeliumC.Close()
		}
		return nil, err
	}

	ret.conn = conn
	ret.mainC = cordiumv1.NewMainServiceClient(conn)
	ret.workspaceC = cordiumv1.NewWorkspaceServiceClient(conn)
	ret.managementC = cordiumv1.NewManagementServiceClient(conn)

	ret.workspaces = &WorkspaceClient{c: ret}
	ret.spaces = &SpaceClient{c: ret}
	ret.templates = &TemplateClient{c: ret}
	ret.snapshots = &SnapshotClient{c: ret}
	ret.secrets = &SecretClient{c: ret}
	ret.userSecrets = &UserSecretClient{c: ret}
	ret.gitProviders = &GitProviderClient{c: ret}
	ret.memberships = &MembershipClient{c: ret}
	ret.regions = &RegionClient{c: ret}
	ret.userConfig = &UserConfigClient{c: ret}
	ret.managementSvc = &ManagementClient{c: ret}

	return ret, nil
}

// NewClient is an alias for [New].
func NewClient(ctx context.Context, opts ...Option) (*Client, error) {
	return New(ctx, opts...)
}

// Domain returns the normalized Cluster domain.
func (c *Client) Domain() string { return c.cfg.domain }

// Workspaces returns the Workspace (i.e. sandbox) API.
func (c *Client) Workspaces() *WorkspaceClient { return c.workspaces }

// Spaces returns the Space API.
func (c *Client) Spaces() *SpaceClient { return c.spaces }

// Templates returns the Template API.
func (c *Client) Templates() *TemplateClient { return c.templates }

// Snapshots returns the WorkspaceSnapshot API.
func (c *Client) Snapshots() *SnapshotClient { return c.snapshots }

// Secrets returns the Space scoped Secret API.
func (c *Client) Secrets() *SecretClient { return c.secrets }

// UserSecrets returns the User scoped Secret API.
func (c *Client) UserSecrets() *UserSecretClient { return c.userSecrets }

// GitProviders returns the GitProvider API.
func (c *Client) GitProviders() *GitProviderClient { return c.gitProviders }

// Memberships returns the Space Membership API.
func (c *Client) Memberships() *MembershipClient { return c.memberships }

// Regions returns the read-only Region API.
func (c *Client) Regions() *RegionClient { return c.regions }

// UserConfig returns the per-User configuration API.
func (c *Client) UserConfig() *UserConfigClient { return c.userConfig }

// Management returns the administrative API of the Cluster. Its methods
// require Cluster administrator privileges.
func (c *Client) Management() *ManagementClient { return c.managementSvc }

// MainService returns the generated client of the Cordium MainService. It is
// the lower level escape hatch for everything the high level API does not
// cover.
func (c *Client) MainService() cordiumv1.MainServiceClient { return c.mainC }

// WorkspaceService returns the generated client of the Cordium
// WorkspaceService, which is the API that operates inside a running Workspace.
func (c *Client) WorkspaceService() cordiumv1.WorkspaceServiceClient { return c.workspaceC }

// ManagementService returns the generated client of the Cordium
// ManagementService, which is the administrative API of the Cluster.
func (c *Client) ManagementService() cordiumv1.ManagementServiceClient { return c.managementC }

// Conn returns the shared authenticated gRPC connection to the Cluster. The
// Client owns it and callers must not close it.
func (c *Client) Conn() *grpc.ClientConn { return c.conn }

// Octelium returns the underlying Octelium SDK Client. It gives access to the
// Cluster Session, to the access token and to the authenticated HTTP client
// that reaches the Octelium Services, including the Applications that the
// Workspaces expose.
func (c *Client) Octelium() *octelium.Client { return c.octeliumC }

// AccessToken returns a valid Octelium access token, authenticating or
// refreshing the Session as needed.
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	if err := c.ensureOpen(); err != nil {
		return "", err
	}
	return c.octeliumC.AccessToken(ctx)
}

// Close releases the resources that the Client owns. It does not log out, and
// it does not close an Octelium Client that was supplied with
// [WithOcteliumClient].
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		if c.ownsOctelium {
			c.closeErr = c.octeliumC.Close()
		}
	})
	return c.closeErr
}

func (c *Client) ensureOpen() error {
	select {
	case <-c.closed:
		return ErrClientClosed
	default:
		return nil
	}
}

// closedCh exposes the Client lifetime to the long lived streams so that they
// terminate once the Client is closed.
func (c *Client) closedCh() <-chan struct{} { return c.closed }

func (c *Client) logger() *slog.Logger { return c.cfg.logger }
