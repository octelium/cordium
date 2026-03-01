/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
