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
