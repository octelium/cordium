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
	"time"

	"github.com/octelium/cordium/cluster/common/tests"
	wssrv "github.com/octelium/cordium/cluster/workspace/workspace"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	wsSrv, err := wssrv.NewServer(ctx)
	assert.Nil(t, err)

	err = wsSrv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	assert.Equal(t, srv.octeliumGID, srv.octeliumUID)
	assert.NotEqual(t, 0, srv.octeliumUID)

	srv.setStatus(cordiumv1.Workspace_Status_STOPPING)
	assert.Equal(t, cordiumv1.Workspace_Status_STOPPING, srv.getStatus())

	err = wsSrv.Close()
	assert.Nil(t, err)

	time.Sleep(2 * time.Second)

}
