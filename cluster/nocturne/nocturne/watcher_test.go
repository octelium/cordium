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

package nocturne

import (
	"fmt"
	"testing"
	"time"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {

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

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s-%s", usr.Usr.Metadata.Name, utilrand.GetRandomStringCanonical(6)),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			UserRef:   umetav1.GetObjectReference(usr.Usr),
			State:     cordiumv1.Workspace_Status_RUNNING,
			RegionRef: regionRef,
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	watcher := newWatcher(fakeC.OcteliumC, fakeC.K8sC, regionRef)

	cc, err := fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err)

	err = watcher.doHandleInactiveRunning(ctx, ws, cc, 0)
	assert.Nil(t, err)

	ws1, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)
	assert.Equal(t, ws.Metadata.ResourceVersion, ws1.Metadata.ResourceVersion)

	ws1.Status.LastActivityAt = pbutils.Timestamp(time.Now().Add(-6 * time.Hour))

	ws1, err = fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws1)
	assert.Nil(t, err)

	err = watcher.doHandleInactiveRunning(ctx, ws1, cc, 4*time.Hour)
	assert.Nil(t, err)

	ws2, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)

	assert.NotEqual(t, ws2.Metadata.ResourceVersion, ws1.Metadata.ResourceVersion)

	assert.Equal(t, cordiumv1.Workspace_Status_STOPPING_REQUEST, ws2.Status.State)
}
