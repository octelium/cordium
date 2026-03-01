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

package supervisor

import (
	"context"
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	wssrv "github.com/octelium/cordium/cluster/workspace/workspace"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/stretchr/testify/assert"
)

func TestServerAPI(t *testing.T) {
	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	wsSrv, err := wssrv.NewServer(ctx)
	assert.Nil(t, err)

	defer wsSrv.Close()

	err = wsSrv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	_, err = srv.GetState(ctx, &ccordiumv1.GetStateRequest{})
	assert.Nil(t, err)

	_, err = srv.Initialize(ctx, &ccordiumv1.InitializeRequest{})
	assert.Nil(t, err)
	_, err = srv.GetState(ctx, &ccordiumv1.GetStateRequest{})
	assert.Nil(t, err)
	_, err = srv.Initialize(ctx, &ccordiumv1.InitializeRequest{})
	assert.Nil(t, err, "%+v", err)
	_, err = srv.GetState(ctx, &ccordiumv1.GetStateRequest{})
	assert.Nil(t, err)

	assert.Nil(t, err)
	_, err = srv.GetState(ctx, &ccordiumv1.GetStateRequest{})
	assert.Nil(t, err)

	_, err = srv.Shutdown(ctx, &ccordiumv1.ShutdownRequest{})
	assert.Nil(t, err)
	_, err = srv.GetState(ctx, &ccordiumv1.GetStateRequest{})
	assert.Nil(t, err)
	_, err = srv.Shutdown(ctx, &ccordiumv1.ShutdownRequest{})
	assert.Nil(t, err)
	_, err = srv.GetState(ctx, &ccordiumv1.GetStateRequest{})
	assert.Nil(t, err)
	_, err = srv.ShutdownAck(ctx, &ccordiumv1.ShutdownAckRequest{})
	assert.Nil(t, err)
	_, err = srv.ShutdownAck(ctx, &ccordiumv1.ShutdownAckRequest{})
	assert.Nil(t, err)
}
