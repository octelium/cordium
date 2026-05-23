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
	"fmt"
	"testing"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestK8s(t *testing.T) {

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

	/*
		storage, err := fakeC.OcteliumC.CordiumC().CreateCloudProvider(ctx, &cordiumv1.CloudProvider{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.CloudProvider_Spec{
				Type: &cordiumv1.CloudProvider_Spec_S3_{
					S3: &cordiumv1.CloudProvider_Spec_S3{},
				},
			},
		})

		cc, err := fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err)
		cc.Spec.StorageCloudProvider = storage.Metadata.Name
		cc, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err)
	*/

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENT)
	assert.Nil(t, err)

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("ws-%s", utilrand.GetRandomStringCanonical(6)),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			State:      cordiumv1.Workspace_Status_INIT_REQUEST,
			UserRef:    umetav1.GetObjectReference(usr.Usr),
			SessionRef: umetav1.GetObjectReference(usr.Session),
			RegionRef:  regionRef,
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	jwkCtl, err := jwkctl.NewJWKController(ctx, fakeC.OcteliumC)
	assert.Nil(t, err)

	ctl, err := NewController(ctx, ctx, fakeC.OcteliumC, fakeC.K8sC, jwkCtl, regionRef)
	assert.Nil(t, err)

	err = ctl.doOnAdd(ctx, ws)
	assert.Nil(t, err)

	err = ctl.doOnDeleteK8s(ctx, ws)
	assert.Nil(t, err)

	err = ctl.doOnDeleteK8s(ctx, ws)
	assert.Nil(t, err)
}
