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

package wsutils

import (
	"fmt"
	"testing"
	"time"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcordiumv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestActivityCtl(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s-%s", usr.Usr.Metadata.Name, utilrand.GetRandomStringCanonical(6)),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			UserRef: umetav1.GetObjectReference(usr.Usr),
			State:   cordiumv1.Workspace_Status_RUNNING,
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	ctl, err := NewActivityCtl(fakeC.OcteliumC)
	assert.Nil(t, err)

	now := time.Now()

	ctl.activityMap[ws.Metadata.Uid] = now

	err = ctl.doCheck(ctx)
	assert.Nil(t, err)

	ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)

	assert.True(t, pbutils.IsEqual(ws.Status.LastActivityAt, pbutils.Timestamp(now)))
	assert.Equal(t, 0, len(ctl.activityMap))
}

type activityResources struct {
	rcordiumv1.ResourceServiceClient
	workspace *cordiumv1.Workspace
	getErr    error
	updateErr error
	updates   int
}

func (c *activityResources) GetWorkspace(ctx context.Context, req *rmetav1.GetOptions, opts ...grpc.CallOption) (*cordiumv1.Workspace, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return proto.Clone(c.workspace).(*cordiumv1.Workspace), nil
}

func (c *activityResources) UpdateWorkspace(ctx context.Context, ws *cordiumv1.Workspace, opts ...grpc.CallOption) (*cordiumv1.Workspace, error) {
	c.updates++
	if c.updateErr != nil {
		return nil, c.updateErr
	}
	c.workspace = proto.Clone(ws).(*cordiumv1.Workspace)
	return ws, nil
}

func TestActivityCtlRetriesAndDeletion(t *testing.T) {
	for _, tc := range []struct {
		name      string
		getErr    error
		updateErr error
		retained  bool
		updates   int
	}{
		{name: "successful update", updates: 1},
		{name: "deleted before read", getErr: status.Error(codes.NotFound, "deleted")},
		{name: "transient read failure", getErr: status.Error(codes.Unavailable, "offline"), retained: true},
		{name: "deleted before update", updateErr: status.Error(codes.NotFound, "deleted"), updates: 1},
		{name: "conflicting update", updateErr: status.Error(codes.Aborted, "conflict"), retained: true, updates: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resources := &activityResources{workspace: &cordiumv1.Workspace{Status: &cordiumv1.Workspace_Status{}}, getErr: tc.getErr, updateErr: tc.updateErr}
			ctl, err := NewActivityCtl(&volumeMountClient{resources: resources})
			assert.Nil(t, err)
			when := time.Unix(1700000000, 123)
			ctl.activityMap["ws-uid"] = when
			assert.Nil(t, ctl.doCheck(t.Context()))
			_, retained := ctl.activityMap["ws-uid"]
			assert.Equal(t, tc.retained, retained)
			assert.Equal(t, tc.updates, resources.updates)
			if tc.retained {
				resources.getErr, resources.updateErr = nil, nil
				assert.Nil(t, ctl.doCheck(t.Context()))
				assert.Empty(t, ctl.activityMap)
				assert.True(t, when.Equal(resources.workspace.Status.LastActivityAt.AsTime()))
			}
		})
	}
}

func TestActivityCtlWorkspaceLoopStops(t *testing.T) {
	ctl, err := NewActivityCtl(nil)
	assert.Nil(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); ctl.startWsLoop(ctx) }()
	ctl.Set("workspace-uid")
	assert.Eventually(t, func() bool {
		ctl.mu.Lock()
		defer ctl.mu.Unlock()
		return !ctl.activityMap["workspace-uid"].IsZero()
	}, time.Second, time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("activity loop did not stop")
	}
}

func TestActivityCtlPreservesNewerActivity(t *testing.T) {
	newer := time.Unix(1700000000, 0)
	resources := &activityResources{workspace: &cordiumv1.Workspace{Status: &cordiumv1.Workspace_Status{LastActivityAt: pbutils.Timestamp(newer)}}}
	ctl, err := NewActivityCtl(&volumeMountClient{resources: resources})
	assert.Nil(t, err)
	ctl.activityMap["workspace-uid"] = newer.Add(-time.Minute)
	assert.Nil(t, ctl.doCheck(t.Context()))
	assert.True(t, newer.Equal(resources.workspace.Status.LastActivityAt.AsTime()))
}
