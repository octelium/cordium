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

package workspace

import (
	"testing"

	"context"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestGitCred(t *testing.T) {

	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	defer srv.Close()

	{
		_, err := srv.GetGitCreds(ctx, &ccordiumv1.GetGitCredsRequest{})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	}

	{

		req := map[string]string{
			"host":     "example.com",
			"username": "user1",
			"password": utilrand.GetRandomStringCanonical(8),
		}
		_, err = srv.StoreGitCreds(ctx, &ccordiumv1.StoreGitCredsRequest{
			Request: req,
		})
		assert.Nil(t, err)
		resp, err := srv.GetGitCreds(ctx, &ccordiumv1.GetGitCredsRequest{
			Request: map[string]string{
				"host": "example.com",
			},
		})
		assert.Nil(t, err)

		assert.Equal(t, req["password"], resp.Response["password"])
	}
}

func TestGitCredProviderHostScope(t *testing.T) {

	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	defer srv.Close()

	providerToken := utilrand.GetRandomStringCanonical(12)

	srv.initReq = &ccordiumv1.PrepareRequest{
		GitProviderInfo: &ccordiumv1.GitProviderInfo{
			Username:    "user1",
			AccessToken: providerToken,
		},
	}
	srv.spec = &cordiumv1.Workspace_Spec{
		Repository: &cordiumv1.Workspace_Spec_Repository{
			Url: "https://github.com/myorg/my-project",
		},
		AdditionalRepositories: []*cordiumv1.Workspace_Spec_AdditionalRepository{
			{
				Name: "lib",
				Repository: &cordiumv1.Workspace_Spec_Repository{
					Url: "https://gitlab.example.com/myorg/lib",
				},
			},
		},
	}

	getFor := func(host string) (*ccordiumv1.GetGitCredsResponse, error) {
		return srv.GetGitCreds(ctx, &ccordiumv1.GetGitCredsRequest{
			Request: map[string]string{
				"protocol": "https",
				"host":     host,
			},
		})
	}

	t.Run("configured hosts get the provider token", func(t *testing.T) {
		for _, host := range []string{
			"github.com",
			"gitlab.example.com",
			"GitHub.com",
		} {
			resp, err := getFor(host)
			assert.Nil(t, err, "host should be allowed: %s", host)
			assert.Equal(t, providerToken, resp.Response["password"])
			assert.Equal(t, "user1", resp.Response["username"])
		}
	})

	t.Run("unconfigured hosts get nothing", func(t *testing.T) {
		for _, host := range []string{
			"attacker.example",
			"",
			"evilgithub.com",
			"github.com.attacker.example",
			"example.com",
		} {
			_, err := getFor(host)
			assert.NotNil(t, err, "host should be refused: %s", host)
			assert.True(t, grpcerr.IsNotFound(err), "host should be refused: %s", host)
		}
	})

	t.Run("creds stored from inside the Workspace are still returned", func(t *testing.T) {
		req := map[string]string{
			"host":     "attacker.example",
			"username": "user1",
			"password": utilrand.GetRandomStringCanonical(8),
		}
		_, err := srv.StoreGitCreds(ctx, &ccordiumv1.StoreGitCredsRequest{
			Request: req,
		})
		assert.Nil(t, err)

		resp, err := getFor("attacker.example")
		assert.Nil(t, err)
		assert.Equal(t, req["password"], resp.Response["password"])
		assert.NotEqual(t, providerToken, resp.Response["password"])
	})
}
