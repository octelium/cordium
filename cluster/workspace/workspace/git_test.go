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
