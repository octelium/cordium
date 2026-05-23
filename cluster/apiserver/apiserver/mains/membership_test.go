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

package mains

import (
	"context"
	"fmt"
	"testing"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestMembership(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		org, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		memList, err := srv.ListMembership(usr.Ctx(), &cordiumv1.ListMembershipOptions{
			SpaceRef: umetav1.GetObjectReference(org),
		})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(memList.Items))
		assert.Equal(t, org.Metadata.Uid, memList.Items[0].Status.SpaceRef.Uid)
		assert.Equal(t, usr.Usr.Metadata.Uid, memList.Items[0].Status.UserRef.Uid)
		assert.Equal(t, cordiumv1.Membership_Spec_OWNER, memList.Items[0].Spec.Role)
		memOwner := memList.Items[0]

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		mem, err := srv.CreateMembership(usr.Ctx(), &cordiumv1.CreateMembershipRequest{
			SpaceRef: umetav1.GetObjectReference(org),
			UserType: &cordiumv1.CreateMembershipRequest_UserRef{
				UserRef: umetav1.GetObjectReference(usr2.Usr),
			},
		})
		assert.Nil(t, err)

		assert.Equal(t, cordiumv1.Membership_Spec_USER, mem.Spec.Role)

		_, err = srv.DeleteMembership(usr2.Ctx(), &metav1.DeleteOptions{
			Uid: mem.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		{
			memClone := pbutils.Clone(mem).(*cordiumv1.Membership)
			memClone.Spec.Role = cordiumv1.Membership_Spec_ADMIN
			_, err = srv.UpdateMembership(usr2.Ctx(), memClone)
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsUnauthorized(err))
		}

		{
			memClone := pbutils.Clone(mem).(*cordiumv1.Membership)
			memClone.Spec.Role = cordiumv1.Membership_Spec_OWNER
			_, err = srv.UpdateMembership(usr2.Ctx(), memClone)
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsUnauthorized(err))
		}

		{
			mem.Spec.Role = cordiumv1.Membership_Spec_ADMIN
			mem, err = srv.UpdateMembership(usr.Ctx(), mem)
			assert.Nil(t, err, "%+v", err)
		}

		usr3, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		mem2, err := srv.CreateMembership(usr.Ctx(), &cordiumv1.CreateMembershipRequest{
			SpaceRef: umetav1.GetObjectReference(org),
			UserType: &cordiumv1.CreateMembershipRequest_UserRef{
				UserRef: umetav1.GetObjectReference(usr3.Usr),
			},
		})
		assert.Nil(t, err)

		_, err = srv.DeleteMembership(usr2.Ctx(), &metav1.DeleteOptions{
			Uid: mem2.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteMembership(usr2.Ctx(), &metav1.DeleteOptions{
			Uid: memOwner.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		mem.Spec.Role = cordiumv1.Membership_Spec_OWNER
		mem, err = srv.UpdateMembership(usr.Ctx(), mem)
		assert.Nil(t, err)

		/*
			_, err = srv.DeleteMembership(usr2.Ctx(), &rmetav1.DeleteOptions{
				Uid: memOwner.Metadata.Uid,
			})
			assert.Nil(t, err)
		*/

		/*
			_, err = srv.DeleteSpace(usr.Ctx(), &rmetav1.DeleteOptions{
				Uid: org.Metadata.Uid,
			})
			assert.NotNil(t, err, "%+v", err)
			assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)

		*/

		_, err = srv.DeleteSpace(usr2.Ctx(), &metav1.DeleteOptions{
			Uid: org.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
	}
}
