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

func TestSpace(t *testing.T) {

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
		assert.Equal(t, org.Status.Type, cordiumv1.Space_Status_ORGANIZATION)

		{
			org.Spec.Runtime = &cordiumv1.Space_Spec_Runtime{
				EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
					{
						Key: "key1",
						Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{
							Value: "val1",
						},
					},
				},
			}

			org2, err := srv.UpdateSpace(usr.Ctx(), org)
			assert.Nil(t, err, "%+v", err)
			assert.True(t, pbutils.IsEqual(org.Spec, org2.Spec))
		}
		{
			_, err := srv.LeaveSpace(usr.Ctx(), &cordiumv1.LeaveSpaceRequest{
				SpaceRef: umetav1.GetObjectReference(org),
			})

			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)
		}

		{
			_, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
				Metadata: &metav1.Metadata{
					Name: (org.Metadata.Name),
				},
				Spec: &cordiumv1.Space_Spec{},
				Status: &cordiumv1.Space_Status{
					Type: cordiumv1.Space_Status_ORGANIZATION,
				},
			})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.AlreadyExists(err))
		}

		memList, err := srv.ListMembership(usr.Ctx(), &cordiumv1.ListMembershipOptions{
			SpaceRef: umetav1.GetObjectReference(org),
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, 1, len(memList.Items))
		assert.Equal(t, org.Metadata.Uid, memList.Items[0].Status.SpaceRef.Uid)
		assert.Equal(t, usr.Usr.Metadata.Uid, memList.Items[0].Status.UserRef.Uid)
		assert.Equal(t, cordiumv1.Membership_Spec_OWNER, memList.Items[0].Spec.Role)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.GetSpace(usr2.Ctx(), &metav1.GetOptions{
			Uid: org.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		_, err = srv.DeleteSpace(usr2.Ctx(), &metav1.DeleteOptions{
			Uid: org.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		_, err = srv.GetSpace(usr.Ctx(), &metav1.GetOptions{
			Uid: org.Metadata.Uid,
		})
		assert.Nil(t, err)

		_, err = srv.DeleteSpace(usr.Ctx(), &metav1.DeleteOptions{
			Uid: org.Metadata.Uid,
		})
		assert.Nil(t, err, "%+v", err)
	}

	t.Run("short name", func(t *testing.T) {
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		name := utilrand.GetRandomStringCanonical(8)
		org, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: name,
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_USER,
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, org.Status.Type, cordiumv1.Space_Status_USER)

		assert.Equal(t, fmt.Sprintf("%s.%s", name, usr.Usr.Metadata.Name), org.Metadata.Name)

		_, err = srv.GetTemplate(usr.Ctx(), &metav1.GetOptions{
			Name: fmt.Sprintf("default.%s", org.Metadata.Name),
		})
		assert.Nil(t, err)
	})
}
