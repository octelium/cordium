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

	/*
		{
			usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
			assert.Nil(t, err)

			_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Space_Spec{},
			})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsUnauthorized(err))
		}
	*/

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
				Uid: org.Metadata.Uid,
			})

			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsUnauthorized(err))
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
}

/*
func TestPersonalSpace(t *testing.T) {

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

		org, err := srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.Nil(t, err)

		assert.Equal(t, fmt.Sprintf("personal.%s", usr.Usr.Metadata.Name), org.Metadata.Name)
		assert.Equal(t, cordiumv1.Space_Status_USER, org.Status.Type)

		memList, err := srv.ListMembership(usr.Ctx(), &cordiumv1.ListMembershipOptions{
			SpaceRef: umetav1.GetObjectReference(org),
		})
		assert.Nil(t, err)
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

		wsList, err := srv.ListWorkspace(usr.Ctx(), &cordiumv1.ListWorkspaceOptions{
			Filter: &cordiumv1.ListWorkspaceOptions_SpaceRef{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, 0, len(wsList.Items))

		orgN, err := srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.Nil(t, err)
		assert.Equal(t, org.Metadata.Uid, orgN.Metadata.Uid)

		_, err = srv.DeleteSpace(usr.Ctx(), &metav1.DeleteOptions{
			Uid: org.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: "personal",
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.NotNil(t, err)
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("personal-%s", utilrand.GetRandomStringLowercase(4)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.NotNil(t, err)
	}
}
*/

/*
func TestOwnSpace(t *testing.T) {

	ctx := context.Background()
	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	srv, err := NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	{
		_, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))
	}
	{
		_, err = srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))
	}

	cc, err := fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err)

	cc.Spec.Space = &cordiumv1.ClusterConfig_Spec_Space{
		Ownership: &cordiumv1.ClusterConfig_Spec_Space_Ownership{
			PersonalRules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
				{
					Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,
					Conditions: []*metav1.Condition{
						{
							Type: &metav1.Condition_Match{
								Match: fmt.Sprintf(`ctx.user.metadata.name == "%s"`, usr.Usr.Metadata.Name),
							},
						},
					},
				},
			},
		},
	}

	cc, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err)

	{
		_, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))
	}
	{
		spc, err := srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.Nil(t, err)

		_, err = srv.LeaveSpace(usr.Ctx(), &cordiumv1.LeaveSpaceRequest{
			Uid: spc.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))
	}

	cc.Spec.Space = &cordiumv1.ClusterConfig_Spec_Space{
		Ownership: &cordiumv1.ClusterConfig_Spec_Space_Ownership{
			OrganizationRules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
				{
					Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,
					Conditions: []*metav1.Condition{
						{
							Type: &metav1.Condition_Match{
								Match: fmt.Sprintf(`ctx.user.metadata.name == "%s"`, usr.Usr.Metadata.Name),
							},
						},
					},
				},
			},
		},
	}

	cc, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err)

	{
		_, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)
	}
	{
		_, err = srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.Nil(t, err)
	}
}
*/

/*
func TestSpaceWorkload(t *testing.T) {

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

	ctx := context.Background()

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_WORKLOAD,
			corev1.Session_Status_CLIENT)
		assert.Nil(t, err)

		_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		cc, err := srv.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
		assert.Nil(t, err)
		cc.Spec.Workload = &cordiumv1.ClusterConfig_Spec_Workload{
			AllowOrganizationSpace: true,
		}

		cc, err = srv.octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err)

		_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		_, err = srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		cc.Spec.Workload = &cordiumv1.ClusterConfig_Spec_Workload{
			AllowOrganizationSpace: true,
			AllowPersonalSpace:     true,
			AllowWorkspace:         true,
		}

		cc, err = srv.octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		assert.Nil(t, err)

		_, err = srv.GetPersonalSpace(usr.Ctx(), &cordiumv1.GetPersonalSpaceRequest{})
		assert.Nil(t, err, "%+v", err)
	}
}
*/
