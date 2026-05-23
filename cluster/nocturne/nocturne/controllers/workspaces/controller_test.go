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

package controller

import (
	"net"
	"testing"
	"time"

	"context"

	snapshotclientfake "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned/fake"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/supervisor/supervisor"
	wssrv "github.com/octelium/cordium/cluster/workspace/workspace"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

type testClient struct {
	sshClient *ssh.Client
	conn      net.Conn
}

func (c *testClient) close() error {
	if c.sshClient != nil {
		c.sshClient.Close()
	}

	if c.conn != nil {
		c.conn.Close()
	}

	return nil
}

func TestServer(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	wsSrv, err := wssrv.NewServer(ctx)
	assert.Nil(t, err)
	defer wsSrv.Close()

	err = wsSrv.Run(ctx)
	assert.Nil(t, err)

	wsSup, err := supervisor.NewServer(ctx)
	assert.Nil(t, err)
	defer wsSup.Close()

	err = wsSup.Run(ctx)
	assert.Nil(t, err)

	go wsSup.WaitForTerm(ctx)

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	org, err := fakeC.OcteliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &cordiumv1.Space_Spec{},
		Status: &cordiumv1.Space_Status{
			Type: cordiumv1.Space_Status_ORGANIZATION,
		},
	})
	assert.Nil(t, err)

	tmpl, err := fakeC.OcteliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &cordiumv1.Template_Spec{},
		Status: &cordiumv1.Template_Status{
			SpaceRef: umetav1.GetObjectReference(org),
			BuildInfo: &cordiumv1.Template_Status_BuildInfo{
				CurrentReadyBuildID: utilrand.GetRandomStringCanonical(8),
			},
		},
	})
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: wsutils.GenWorkspaceName(),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			State:       cordiumv1.Workspace_Status_INIT_REQUEST,
			UserRef:     umetav1.GetObjectReference(usr.Usr),
			SessionRef:  umetav1.GetObjectReference(usr.Session),
			RegionRef:   regionRef,
			SpaceRef:    umetav1.GetObjectReference(org),
			TemplateRef: umetav1.GetObjectReference(tmpl),
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC)
	assert.Nil(t, err)

	ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
	assert.Nil(t, err)

	ctl.snapshotC = snapshotclientfake.NewSimpleClientset()

	err = ctl.startWorkspace(ctx, ws)
	assert.Nil(t, err, "%+v", err)

	time.Sleep(5 * time.Second)

	ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)
	assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, ws.Status.State)

	ws.Status.State = cordiumv1.Workspace_Status_STOPPING_REQUEST
	ws, err = fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws)
	assert.Nil(t, err)

	err = ctl.stopWorkspace(ctx, ws)
	assert.Nil(t, err, "%+v", err)

	time.Sleep(3 * time.Second)

	ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)
	assert.Equal(t, cordiumv1.Workspace_Status_STOPPED, ws.Status.State)

	err = ctl.stopWorkspace(ctx, ws)
	assert.Nil(t, err, "tt %+v", err)

}
