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

package portal

import (
	"fmt"
	"io"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsClusterDomain(t *testing.T) {
	type tstCase struct {
		host   string
		domain string
		valid  bool
	}

	for _, tst := range []tstCase{
		{host: "example.com", domain: "example.com", valid: true},
		{host: "cordium.example.com", domain: "example.com", valid: true},
		{host: "port-3000.cordium.example.com", domain: "example.com", valid: true},
		{host: "evil-example.com", domain: "example.com", valid: false},
		{host: "notexample.com", domain: "example.com", valid: false},
		{host: "example.com.evil.org", domain: "example.com", valid: false},
		{host: "", domain: "example.com", valid: false},
		{host: "example.com", domain: "", valid: false},
	} {
		assert.Equal(t, tst.valid, isClusterDomain(tst.host, tst.domain),
			"host: %s domain: %s", tst.host, tst.domain)
	}
}

func TestHandleIndexHeaders(t *testing.T) {
	s := &Server{
		clusterDomain: "example.com",
		rootURL:       "https://cordium.example.com",
	}

	req := httptest.NewRequest("GET", "https://cordium.example.com/", nil)
	w := httptest.NewRecorder()

	s.handleIndex(w, req)

	resp := w.Result()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.True(t, strings.Contains(resp.Header.Get("Permissions-Policy"), "camera=()"))

	csp := resp.Header.Get("Content-Security-Policy")
	assert.True(t, strings.Contains(csp, "default-src 'none'"), csp)
	assert.True(t, strings.Contains(csp, "frame-ancestors 'none'"), csp)
	assert.True(t, strings.Contains(csp, "object-src 'none'"), csp)
	assert.True(t, strings.Contains(csp, "base-uri 'none'"), csp)
	assert.True(t, strings.Contains(csp, "form-action 'self'"), csp)
	assert.False(t, strings.Contains(csp, "script-src 'self' 'unsafe-inline'"), csp)
	assert.True(t, strings.Contains(csp,
		"connect-src 'self' https://octelium-api.example.com https://*.octelium-api.example.com wss://cordium.example.com"), csp)

	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)

	nonce := regexp.MustCompile(`'nonce-([a-z0-9]+)'`).FindStringSubmatch(csp)
	assert.Equal(t, 2, len(nonce), csp)

	for _, tag := range regexp.MustCompile(`<script[^>]*>`).FindAllString(string(body), -1) {
		assert.True(t, strings.Contains(tag, fmt.Sprintf(`nonce="%s"`, nonce[1])), tag)
	}
}

func TestSetIndexNonce(t *testing.T) {
	blob := []byte(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/assets/logo.svg" />
    <script>
      window.localStorage.getItem("cordium.colorScheme");
    </script>
    <script type="module" crossorigin src="/assets/index.js"></script>
    <link rel="modulepreload" href="/assets/vendor.js" />
    <link rel="stylesheet" crossorigin href="/assets/index.css">
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>`)

	nonce := "abcdef0123456789abcdef01"

	out, err := setIndexNonce(blob, nonce)
	assert.Nil(t, err)

	ret := string(out)

	tags := regexp.MustCompile(`<script[^>]*>`).FindAllString(ret, -1)
	assert.Equal(t, 2, len(tags))
	for _, tag := range tags {
		assert.True(t, strings.Contains(tag, fmt.Sprintf(`nonce="%s"`, nonce)), tag)
	}

	preload := regexp.MustCompile(`<link[^>]*modulepreload[^>]*>`).FindString(ret)
	assert.True(t, strings.Contains(preload, fmt.Sprintf(`nonce="%s"`, nonce)), preload)

	stylesheet := regexp.MustCompile(`<link[^>]*stylesheet[^>]*>`).FindString(ret)
	assert.False(t, strings.Contains(stylesheet, "nonce"), stylesheet)

	assert.True(t, strings.HasPrefix(ret, "<!DOCTYPE html>"))
	assert.True(t, strings.Contains(ret, `id="root"`))
}
