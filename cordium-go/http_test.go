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
	"net/http"
	"testing"
)

func policyClient(domain string, opts ...Option) *Client {
	cfg := clientConfig{domain: domain}
	for _, opt := range opts {
		_ = opt(&cfg)
	}
	return &Client{cfg: cfg}
}

func TestHTTPAuthorizationPolicy(t *testing.T) {
	c := policyClient("example.com", WithAuthorizedHTTPHosts("Partner.Example.NET"))

	for _, url := range []string{
		"https://example.com/api",
		"https://octelium-api.example.com",
		"https://abc.cordium.example.com",
		// The Cordium portal serves the named Applications and the exposed
		// ports under underscore-separated labels, which IDNA rejects and which
		// therefore need this SDK's own policy.
		"https://api_abc.cordium.example.com/healthz",
		"https://port_3000_abc.cordium.example.com",
		"https://partner.example.net",
	} {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("NewRequest(%q): %v", url, err)
		}
		if err := c.authorizeHTTPRequest(req); err != nil {
			t.Errorf("%s was rejected: %v", url, err)
		}
	}

	for _, url := range []string{
		"https://evil.com",
		"https://example.com.evil.com",
		"http://abc.cordium.example.com",
		"ftp://example.com",
		"https://user:pass@example.com",
	} {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("NewRequest(%q): %v", url, err)
		}
		if err := c.authorizeHTTPRequest(req); err == nil {
			t.Errorf("%s was authorized to receive the access token", url)
		}
	}

	if err := c.authorizeHTTPRequest(nil); err == nil {
		t.Error("a nil request was authorized")
	}
}

func TestHTTPAuthorizationPolicyAllowsInsecureWhenAsked(t *testing.T) {
	c := policyClient("example.com", WithAllowInsecureHTTP())

	req, _ := http.NewRequest(http.MethodGet, "http://abc.cordium.example.com", nil)
	if err := c.authorizeHTTPRequest(req); err != nil {
		t.Errorf("plain HTTP was rejected although it was allowed: %v", err)
	}
}

func TestAuthorizedHTTPHostsRejectsEmpty(t *testing.T) {
	cfg := clientConfig{}
	if err := WithAuthorizedHTTPHosts("  ")(&cfg); !IsInvalidArgument(err) {
		t.Errorf("an empty authorized host: %v", err)
	}
	if err := WithHTTPAuthorizationPolicy(nil)(&cfg); !IsInvalidArgument(err) {
		t.Errorf("a nil policy: %v", err)
	}
}

func TestClientHTTPAccessors(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())

	if c.HTTPClient() == nil || c.HTTPTransport() == nil {
		t.Error("the HTTP accessors returned nil")
	}
	if c.Domain() != "example.com" {
		t.Errorf("Domain() = %q", c.Domain())
	}
	if c.Conn() == nil || c.Octelium() == nil {
		t.Error("the connection accessors returned nil")
	}
	if c.MainService() == nil || c.WorkspaceService() == nil || c.ManagementService() == nil {
		t.Error("a service accessor returned nil")
	}
	if c.Spaces() == nil || c.Templates() == nil || c.Secrets() == nil ||
		c.UserSecrets() == nil || c.GitProviders() == nil || c.Memberships() == nil ||
		c.Regions() == nil || c.UserConfig() == nil || c.Management() == nil ||
		c.Workspaces() == nil {
		t.Error("a resource accessor returned nil")
	}

	token, err := c.AccessToken(t.Context())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "test-token" {
		t.Errorf("AccessToken() = %q", token)
	}
}

func TestNewRejectsConflictingOptions(t *testing.T) {
	ctx := t.Context()

	if _, err := New(ctx,
		WithDomain("example.com"),
		WithAccessToken("t"),
		WithUserAgent("")); !IsInvalidArgument(err) {
		t.Errorf("an empty user agent: %v", err)
	}
	if _, err := New(ctx, WithLogger(nil)); !IsInvalidArgument(err) {
		t.Errorf("a nil logger: %v", err)
	}
	if _, err := New(ctx, WithOcteliumClient(nil)); !IsInvalidArgument(err) {
		t.Errorf("a nil Octelium client: %v", err)
	}
	if _, err := New(nil); !IsInvalidArgument(err) { //nolint:staticcheck // deliberately nil
		t.Errorf("a nil context: %v", err)
	}
}
