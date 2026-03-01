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
	"time"

	"context"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsclient"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

	defer srv.Close()

	err = srv.Run(ctx)
	assert.Nil(t, err, "%+v", err)

	started := time.Now()
	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{})
	assert.Nil(t, err)

	c := ccordiumv1.NewWorkspaceServiceClient(grpcConn)

	_, err = c.Prepare(ctx, &ccordiumv1.PrepareRequest{
		Workspace: &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		},
	})
	assert.Nil(t, err)
	zap.L().Debug("elapsed", zap.Duration("elapsed", time.Since(started)))
}

func TestSetEnvExistingKey(t *testing.T) {
	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	env := []string{
		"KEY1=val1=fjkeu=tttt",
		"KEY2=val3",
	}
	assert.True(t, setEnvExistingKey(env, "KEY1", "val2"))
	assert.Equal(t, env[0], "KEY1=val2")
	assert.Equal(t, env[1], "KEY2=val3")
	assert.False(t, setEnvExistingKey(env, "KEY5", "val"))
}
