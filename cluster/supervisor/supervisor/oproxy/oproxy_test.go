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

package oproxy

import (
	"context"
	"net"
	"os"
	"path"
	"testing"

	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOcteliumProxy(t *testing.T) *OcteliumProxy {

	p, err := NewOcteliumProxy(&Opts{
		Domain: "example.com",
	})
	require.Nil(t, err, "%+v", err)

	p.socketPath = path.Join(t.TempDir(), "octelium-proxy.socket")

	return p
}

func TestRunProxy(t *testing.T) {
	ctx := context.Background()

	t.Run("a missing socket path", func(t *testing.T) {
		p := newTestOcteliumProxy(t)

		err := p.runProxy(ctx)
		require.Nil(t, err, "%+v", err)
		t.Cleanup(func() {
			p.Close()
		})

		conn, err := net.Dial("unix", p.socketPath)
		require.Nil(t, err, "%+v", err)
		conn.Close()
	})

	t.Run("a stale socket path is replaced", func(t *testing.T) {
		p := newTestOcteliumProxy(t)

		lis, err := net.Listen("unix", p.socketPath)
		require.Nil(t, err, "%+v", err)
		lis.(*net.UnixListener).SetUnlinkOnClose(false)
		lis.Close()

		_, err = os.Stat(p.socketPath)
		require.Nil(t, err, "the stale socket path must survive closing its listener")

		_, err = net.Dial("unix", p.socketPath)
		require.NotNil(t, err, "the stale socket path must not have a listener")

		err = p.runProxy(ctx)
		require.Nil(t, err, "%+v", err)
		t.Cleanup(func() {
			p.Close()
		})

		conn, err := net.Dial("unix", p.socketPath)
		require.Nil(t, err, "%+v", err)
		conn.Close()
	})

	t.Run("the socket path is removed on close", func(t *testing.T) {
		p := newTestOcteliumProxy(t)

		err := p.runProxy(ctx)
		require.Nil(t, err, "%+v", err)

		err = p.Close()
		assert.Nil(t, err, "%+v", err)

		_, err = os.Stat(p.socketPath)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestGetUserAgent(t *testing.T) {
	oldSemVer, oldGitCommit := ldflags.SemVer, ldflags.GitCommit
	t.Cleanup(func() {
		ldflags.SemVer, ldflags.GitCommit = oldSemVer, oldGitCommit
	})

	for _, tc := range []struct {
		semVer    string
		gitCommit string
		userAgent string
	}{
		{semVer: "v0.12.0", gitCommit: "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567", userAgent: "cordium/v0.12.0"},
		{semVer: "v1.2.3-rc.1", userAgent: "cordium/v1.2.3-rc.1"},
		{gitCommit: "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567", userAgent: "cordium/v0.0.0-dev+0a1b2c3d4e5f"},
		{gitCommit: "0a1b2c3", userAgent: "cordium/v0.0.0-dev+0a1b2c3"},
		{gitCommit: "0a1b2c", userAgent: "cordium/v0.0.0-dev"},
		{gitCommit: "not a commit", userAgent: "cordium/v0.0.0-dev"},
		{userAgent: "cordium/v0.0.0-dev"},
	} {
		ldflags.SemVer, ldflags.GitCommit = tc.semVer, tc.gitCommit
		assert.Equal(t, tc.userAgent, getUserAgent(), "%+v", tc)
	}
}
