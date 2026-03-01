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

	"github.com/octelium/cordium/cluster/common/octeliumc"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func tstAllowAllOwnSpace(t *testing.T, octeliumC octeliumc.ClientInterface) {

	ctx := context.Background()
	cc, err := octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err, "%+v", err)

	if cc.Spec.Space == nil {
		cc.Spec.Space = &cordiumv1.ClusterConfig_Spec_Space{}
	}

	cc.Spec.Space = &cordiumv1.ClusterConfig_Spec_Space{
		Ownership: &cordiumv1.ClusterConfig_Spec_Space_Ownership{
			Rules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
				{
					Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,
					Condition: &cordiumv1.Condition{
						Type: &cordiumv1.Condition_MatchAny{
							MatchAny: true,
						},
					},
				},
			},
			/*
				OrganizationRules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
					{
						Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,
						Conditions: []*metav1.Condition{
							{
								Type: &metav1.Condition_MatchAny{
									MatchAny: true,
								},
							},
						},
					},
				},
			*/
		},
	}

	_, err = octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err)
}

func TestList(t *testing.T) {

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

	tstAllowAllOwnSpace(t, fakeC.OcteliumC)

	{
		usr1, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		org1, err := srv.CreateSpace(usr1.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err, "%+v", err)

		org2, err := srv.CreateSpace(usr1.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		zap.L().Debug("Creating mem", zap.Any("org1", org1))

		{
			_, err := srv.ListMembership(usr1.Ctx(), &cordiumv1.ListMembershipOptions{})
			assert.NotNil(t, err, "%+v", err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		{
			memList, err := srv.ListMembership(usr1.Ctx(), &cordiumv1.ListMembershipOptions{
				SpaceRef: umetav1.GetObjectReference(org1),
			})
			assert.Nil(t, err, "%+v", err)
			assert.Equal(t, 1, len(memList.Items))
		}

		{
			_, err := srv.ListMembership(usr2.Ctx(), &cordiumv1.ListMembershipOptions{
				SpaceRef: umetav1.GetObjectReference(org1),
			})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsUnauthorized(err))
		}

		usr1Mem, err := srv.CreateMembership(usr1.Ctx(), &cordiumv1.CreateMembershipRequest{
			UserType: &cordiumv1.CreateMembershipRequest_UserRef{
				UserRef: umetav1.GetObjectReference(usr2.Usr),
			},
			SpaceRef: umetav1.GetObjectReference(org1),
		})
		assert.Nil(t, err)

		{
			memList, err := srv.ListMembership(usr2.Ctx(), &cordiumv1.ListMembershipOptions{
				SpaceRef: umetav1.GetObjectReference(org1),
			})
			assert.Nil(t, err, "%+v", err)
			assert.Equal(t, 2, len(memList.Items))
		}

		{
			memList, err := srv.ListMembership(usr1.Ctx(), &cordiumv1.ListMembershipOptions{
				SpaceRef: umetav1.GetObjectReference(org1),
			})
			assert.Nil(t, err, "%+v", err)
			assert.Equal(t, 2, len(memList.Items))
		}

		{
			orgList, err := srv.ListSpace(usr1.Ctx(), &cordiumv1.ListSpaceOptions{
				Common: &metav1.CommonListOptions{
					OrderBy: &metav1.CommonListOptions_OrderBy{
						Type: metav1.CommonListOptions_OrderBy_CREATED_AT,
						Mode: metav1.CommonListOptions_OrderBy_DESC,
					},
				},
			})
			assert.Nil(t, err, "%+v", err)
			zap.L().Debug("orgList", zap.Any("orgList", orgList))
			assert.Equal(t, 2, len(orgList.Items))
			assert.Equal(t, org2.Metadata.Uid, orgList.Items[0].Metadata.Uid)
		}

		{
			orgList, err := srv.ListSpace(usr2.Ctx(), &cordiumv1.ListSpaceOptions{})
			assert.Nil(t, err, "%+v", err)
			assert.Equal(t, 0, len(orgList.Items))
		}

		/*
			{
				orgList, err := srv.ListMembership(usr2.Ctx(), &cordiumv1.ListMembershipOptions{})
				assert.Nil(t, err, "%+v", err)
				assert.Equal(t, 1, len(orgList.Items))
				assert.Equal(t, org1.Metadata.Uid, orgList.Items[0].Metadata.Uid)
			}
		*/

		{
			_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		proj1, err := srv.CreateTemplate(usr1.Ctx(), &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org1.Metadata.Name),
			},
			Spec:   &cordiumv1.Template_Spec{},
			Status: &cordiumv1.Template_Status{},
		})
		assert.Nil(t, err)

		{
			_, err := srv.ListTemplate(usr1.Ctx(), &cordiumv1.ListTemplateOptions{})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		{
			pList, err := srv.ListTemplate(usr1.Ctx(), &cordiumv1.ListTemplateOptions{
				SpaceRef: umetav1.GetObjectReference(org1),
				Common: &metav1.CommonListOptions{
					OrderBy: &metav1.CommonListOptions_OrderBy{
						Type: metav1.CommonListOptions_OrderBy_CREATED_AT,
						Mode: metav1.CommonListOptions_OrderBy_DESC,
					},
				},
			})
			assert.Nil(t, err)
			assert.Equal(t, 2, len(pList.Items))
			assert.Equal(t, proj1.Metadata.Uid, pList.Items[0].Metadata.Uid)
		}

		{
			_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		{
			_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{
				SpaceRef: umetav1.GetObjectReference(org2),
			})
			assert.NotNil(t, err)

			assert.True(t, grpcerr.IsUnauthorized(err))
		}

		{
			_, err := srv.ListTemplate(usr1.Ctx(), &cordiumv1.ListTemplateOptions{})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		{
			_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{})
			assert.NotNil(t, err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		{

			_, err = srv.DeleteMembership(usr1.Ctx(), &metav1.DeleteOptions{
				Uid: usr1Mem.Metadata.Uid,
			})
			assert.Nil(t, err, "%+v", err)

			{
				memList, err := srv.ListMembership(usr1.Ctx(), &cordiumv1.ListMembershipOptions{
					SpaceRef: umetav1.GetObjectReference(org1),
				})
				assert.Nil(t, err, "%+v", err)
				assert.Equal(t, 1, len(memList.Items))
			}
			{
				_, err := srv.ListMembership(usr2.Ctx(), &cordiumv1.ListMembershipOptions{
					SpaceRef: umetav1.GetObjectReference(org1),
				})
				assert.NotNil(t, err)
				assert.True(t, grpcerr.IsUnauthorized(err))
			}

			orgList, err := srv.ListSpace(usr2.Ctx(), &cordiumv1.ListSpaceOptions{})
			assert.Nil(t, err)
			assert.Equal(t, 0, len(orgList.Items))

			{
				_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{})
				assert.NotNil(t, err)
				assert.True(t, grpcerr.IsInvalidArg(err))
			}

			{
				_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{})
				assert.NotNil(t, err)
				assert.True(t, grpcerr.IsInvalidArg(err))
			}

			{
				_, err := srv.ListTemplate(usr2.Ctx(), &cordiumv1.ListTemplateOptions{
					SpaceRef: umetav1.GetObjectReference(org1),
				})
				assert.NotNil(t, err)
				assert.True(t, grpcerr.IsUnauthorized(err))
			}

		}

	}
}
