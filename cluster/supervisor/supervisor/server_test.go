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
