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
	"crypto/x509"
	"log/slog"
	"time"

	octelium "github.com/octelium/octelium/octelium-go"
	"google.golang.org/grpc"
)

const (
	defaultUserAgent = "cordium-go"

	// defaultWaitPollInterval is how often the wait helpers reconcile their
	// view of a Workspace against the Cluster while a watch stream is open.
	defaultWaitPollInterval = 30 * time.Second

	// defaultOutputBuffer is the capacity of the channels that carry the
	// output of the exec sessions, the terminals and the log streams.
	defaultOutputBuffer = 64
)

type clientConfig struct {
	domain string

	octeliumClient  *octelium.Client
	octeliumOptions []octelium.Option

	userAgent string
	logger    *slog.Logger

	authorizedHTTPHosts []string
	allowInsecureHTTP   bool

	waitPollInterval time.Duration
}

// Option configures a [Client].
type Option func(*clientConfig) error

// WithDomain sets the Cluster domain, for example example.com or
// cordium.example.com. It defaults to the CORDIUM_DOMAIN environment variable
// and then to OCTELIUM_DOMAIN.
func WithDomain(domain string) Option {
	return func(c *clientConfig) error {
		c.domain = domain
		return nil
	}
}

// WithOcteliumClient reuses an existing Octelium SDK Client, along with its
// Session, its credentials and its connections. The caller keeps ownership of
// it: [Client.Close] does not close it.
//
// It is the option to use when a process already talks to the same Octelium
// Cluster for other reasons, since it avoids creating a second Session.
//
// It also hands the HTTP destination policy over to that Client, whose default
// rejects the underscore-separated hostnames under which the Cordium portal
// serves the Workspace Applications. A Client that is meant to reach them needs
// an octelium.WithHTTPAuthorizationPolicy of its own.
func WithOcteliumClient(client *octelium.Client) Option {
	return func(c *clientConfig) error {
		if client == nil {
			return invalidArgumentf("nil Octelium client")
		}
		c.octeliumClient = client
		return nil
	}
}

// WithOcteliumOptions passes options straight through to the underlying
// Octelium SDK Client. It is the escape hatch for every Octelium level knob
// that this SDK does not re-export, such as custom authenticators, gRPC
// keepalive parameters or an alternative HTTP transport.
func WithOcteliumOptions(opts ...octelium.Option) Option {
	return func(c *clientConfig) error {
		c.octeliumOptions = append(c.octeliumOptions, opts...)
		return nil
	}
}

// WithAccessToken authenticates with an externally managed Octelium access
// token.
func WithAccessToken(token string) Option {
	return WithOcteliumOptions(octelium.WithAccessToken(token))
}

// WithAccessTokenProvider authenticates with externally managed access tokens
// that are obtained, and refreshed, by the caller.
func WithAccessTokenProvider(provider octelium.AccessTokenProvider) Option {
	return WithOcteliumOptions(octelium.WithAccessTokenProvider(provider))
}

// WithAuthenticationToken authenticates with an Octelium Credential
// authentication token. Authentication tokens are one-time credentials: once
// the resulting Session expires it is not silently recreated.
func WithAuthenticationToken(token string) Option {
	return WithOcteliumOptions(octelium.WithAuthenticator(octelium.AuthenticationToken(token)))
}

// WithAssertion authenticates with a signed assertion that is obtained on every
// authentication attempt, which lets the Client replace an expired Session on
// its own. It suits workload identity federation (e.g. GitHub Actions OIDC).
func WithAssertion(provider func(context.Context) (string, error)) Option {
	return WithOcteliumOptions(octelium.WithAuthenticator(octelium.Assertion(provider)))
}

// WithAssertionFile authenticates with an assertion that is read from path on
// every authentication attempt. It suits Kubernetes projected ServiceAccount
// tokens and the other files that the surrounding platform rotates.
func WithAssertionFile(path string) Option {
	return WithOcteliumOptions(octelium.WithAuthenticator(octelium.AssertionFromFile(path)))
}

// WithAPIEndpoint overrides the gRPC target of the Cluster API server. It
// accepts any target that grpc.NewClient supports, such as dns:///host:443. It
// is mostly useful behind port forwards and local proxies.
func WithAPIEndpoint(target string) Option {
	return WithOcteliumOptions(octelium.WithAPIEndpoint(target))
}

// WithTLSServerName overrides the name that is verified in the Cluster
// certificate. It pairs with [WithAPIEndpoint] when the endpoint points at an
// address that the certificate does not cover.
func WithTLSServerName(name string) Option {
	return WithOcteliumOptions(octelium.WithTLSServerName(name))
}

// WithRootCAs verifies the Cluster certificate against the supplied pool.
func WithRootCAs(pool *x509.CertPool) Option {
	return WithOcteliumOptions(octelium.WithRootCAs(pool))
}

// WithInsecureSkipVerify disables the verification of the Cluster certificate.
// It should only ever be used for local development.
func WithInsecureSkipVerify() Option {
	return WithOcteliumOptions(octelium.WithInsecureSkipVerify())
}

// WithGRPCDialOptions appends advanced dial options to the Cluster connection.
func WithGRPCDialOptions(opts ...grpc.DialOption) Option {
	return WithOcteliumOptions(octelium.WithGRPCDialOptions(opts...))
}

// WithAuthorizedHTTPHosts lets [Client.HTTPClient] attach the access token to
// the listed exact hostnames, in addition to the Cluster domain and its
// subdomains.
func WithAuthorizedHTTPHosts(hosts ...string) Option {
	return func(c *clientConfig) error {
		for _, host := range hosts {
			normalized := normalizeHTTPHost(host)
			if normalized == "" {
				return invalidArgumentf("empty authorized HTTP host")
			}
			c.authorizedHTTPHosts = append(c.authorizedHTTPHosts, normalized)
		}
		return nil
	}
}

// WithAllowInsecureHTTP lets [Client.HTTPClient] attach the access token over
// plain HTTP. It should be limited to local development.
func WithAllowInsecureHTTP() Option {
	return func(c *clientConfig) error {
		c.allowInsecureHTTP = true
		return nil
	}
}

// WithHTTPAuthorizationPolicy replaces the rule that decides which HTTP
// destinations may receive the Cluster access token. The default authorizes the
// Cluster domain, its subdomains and the hosts added with
// [WithAuthorizedHTTPHosts].
func WithHTTPAuthorizationPolicy(policy octelium.HTTPAuthorizationPolicy) Option {
	return func(c *clientConfig) error {
		if policy == nil {
			return invalidArgumentf("nil HTTP authorization policy")
		}
		c.octeliumOptions = append(c.octeliumOptions,
			octelium.WithHTTPAuthorizationPolicy(policy))
		return nil
	}
}

// WithUserAgent overrides the user agent that is sent to the Cluster.
func WithUserAgent(userAgent string) Option {
	return func(c *clientConfig) error {
		if userAgent == "" {
			return invalidArgumentf("empty user agent")
		}
		c.userAgent = userAgent
		return nil
	}
}

// WithLogger installs a structured logger. The Client is silent by default.
func WithLogger(logger *slog.Logger) Option {
	return func(c *clientConfig) error {
		if logger == nil {
			return invalidArgumentf("nil logger")
		}
		c.logger = logger
		return nil
	}
}

// WithWaitPollInterval sets how often the wait helpers reconcile their view of
// a Workspace against the Cluster while their watch stream is open. It exists
// as a safety net for the environments where long lived streams are silently
// dropped by an intermediary. It defaults to 30 seconds and a non-positive
// value disables the reconciliation.
func WithWaitPollInterval(interval time.Duration) Option {
	return func(c *clientConfig) error {
		c.waitPollInterval = interval
		return nil
	}
}
