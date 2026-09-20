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
	"fmt"
	"net/http"
	"strings"
)

// HTTPClient returns an HTTP client that attaches the Cluster access token to
// the requests that are bound for the Cluster domain and its subdomains. It is
// how a program reaches the Applications that its Workspaces expose:
//
//	resp, err := c.HTTPClient().Get(ws.AppURL("api") + "/healthz")
//
// The token is only ever attached to an authorized destination; every other
// request fails before any network operation takes place. Beyond the Cluster
// domain, [WithAuthorizedHTTPHosts] widens the set and
// [WithHTTPAuthorizationPolicy] replaces the rule outright.
func (c *Client) HTTPClient() *http.Client {
	return c.octeliumC.HTTPClient()
}

// HTTPTransport returns the authenticated transport behind [Client.HTTPClient],
// for the callers that build their own http.Client around it.
func (c *Client) HTTPTransport() http.RoundTripper {
	return c.octeliumC.HTTPTransport()
}

// authorizeHTTPRequest is the SDK's default HTTP destination policy.
//
// It deliberately replaces the Octelium SDK's own policy, which normalizes the
// destination host with IDNA and therefore rejects the hostnames of the
// Workspace Applications: the Cordium portal serves them under
// underscore-separated labels (e.g. "api_abc.cordium.example.com"), which IDNA
// disallows.
func (c *Client) authorizeHTTPRequest(req *http.Request) error {
	if req == nil || req.URL == nil {
		return fmt.Errorf("invalid HTTP request")
	}
	if req.URL.User != nil {
		return fmt.Errorf("URL userinfo is not allowed")
	}

	switch strings.ToLower(req.URL.Scheme) {
	case "https":
	case "http":
		if !c.cfg.allowInsecureHTTP {
			return fmt.Errorf("plain HTTP is disabled")
		}
	default:
		return fmt.Errorf("unsupported URL scheme %q", req.URL.Scheme)
	}

	host := normalizeHTTPHost(req.URL.Hostname())
	if host == "" {
		return fmt.Errorf("empty HTTP host")
	}

	domain := c.cfg.domain
	if domain != "" && (host == domain || strings.HasSuffix(host, "."+domain)) {
		return nil
	}

	for _, allowed := range c.cfg.authorizedHTTPHosts {
		if host == allowed {
			return nil
		}
	}

	return fmt.Errorf("%q is neither the Cluster domain, one of its subdomains nor an authorized host", host)
}

func normalizeHTTPHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}
