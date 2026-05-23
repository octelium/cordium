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
