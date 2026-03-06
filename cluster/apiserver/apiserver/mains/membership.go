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

	"github.com/asaskevich/govalidator"
	"github.com/gosimple/slug"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
)

const maxMembersPerSpace = 2000

func (s *Server) CreateMembership(ctx context.Context, req *cordiumv1.CreateMembershipRequest) (*cordiumv1.Membership, error) {

	if req.SpaceRef == nil {
		return nil, grpcutils.InvalidArg("SpaceRef is not provided")
	}
	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Name: req.SpaceRef.Name,
		Uid:  req.SpaceRef.Uid,
	}, &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 1,
	}); err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, req.SpaceRef); err != nil {
		return nil, err
	}

	var usr *corev1.User
	var err error

	switch req.UserType.(type) {
	case *cordiumv1.CreateMembershipRequest_Email:
		email := req.GetEmail()
		if email == "" {
			return nil, grpcutils.InvalidArg("Email is empty")
		}

		if !govalidator.IsEmail(email) || !govalidator.IsASCII(email) || len(email) > 150 {
			return nil, grpcutils.InvalidArg("Invalid email")
		}

		usrs, err := s.octeliumC.CoreC().ListUser(ctx, &rmetav1.ListOptions{
			SpecLabels: map[string]string{
				"email": slug.Make(email),
			},
		})
		if err != nil {
			return nil, err
		}
		if len(usrs.Items) != 1 {
			return nil, grpcutils.InvalidArg("This User does not exist")
		}
		usr = usrs.Items[0]
		if usr.Spec.Email != email {
			return nil, grpcutils.InvalidArg("The User email does not match the provider info")
		}
	case *cordiumv1.CreateMembershipRequest_UserRef:
		if req.GetUserRef() == nil {
			return nil, grpcutils.InvalidArg("UserRef is not provided")
		}
		if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
			Name: req.GetUserRef().Name,
			Uid:  req.GetUserRef().Uid,
		}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
			return nil, err
		}

		usr, err = s.octeliumC.CoreC().GetUser(ctx, &rmetav1.GetOptions{
			Name: req.GetUserRef().Name,
			Uid:  req.GetUserRef().Uid,
		})
		if err != nil {
			return nil, serr.K8sNotFoundOrInternalWithErr(err)
		}
	default:
		return nil, grpcutils.InvalidArg("You must provide either a User email or a User Reference")
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: req.SpaceRef.Uid,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if i.User.Metadata.Uid == usr.Metadata.Uid {
		return nil, grpcutils.InvalidArg("You cannot add yourself as a Member")
	}

	if _, err := commonw.GetMembership(ctx, s.octeliumC,
		umetav1.GetObjectReference(usr),
		umetav1.GetObjectReference(org)); err == nil {
		return nil, grpcutils.InvalidArg("User is already a Member in this Space")
	} else if !grpcerr.IsNotFound(err) {
		return nil, grpcutils.InternalWithErr(err)
	}

	{
		itmList, err := s.octeliumC.CordiumC().ListMembership(ctx, ourscsrv.FilterBySpace(org))
		if err != nil {
			return nil, err
		}

		if len(itmList.Items) >= maxMembersPerSpace {
			return nil, serr.Unauthorized("Number of Members per Space has been exceeded")
		}
	}

	switch org.Status.Type {
	case cordiumv1.Space_Status_ORGANIZATION:
	case cordiumv1.Space_Status_USER:
		return nil, grpcutils.InvalidArg("Cannot add Members to User Spaces for now")
	}

	item := &cordiumv1.Membership{
		Metadata: &metav1.Metadata{
			Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(org), umetav1.GetObjectReference(usr)),
		},
		Spec: &cordiumv1.Membership_Spec{
			Role: cordiumv1.Membership_Spec_Role(req.Role),
		},
		Status: &cordiumv1.Membership_Status{
			UserRef:  umetav1.GetObjectReference(usr),
			SpaceRef: umetav1.GetObjectReference(org),
			UserInfo: workspacecommon.GetMembershipUserInfo(usr),
		},
	}

	if item.Spec.Role == cordiumv1.Membership_Spec_UNKNOWN {
		item.Spec.Role = cordiumv1.Membership_Spec_USER
	}

	if item.Spec.Role == cordiumv1.Membership_Spec_OWNER {
		if err := commonw.CheckIsMemberOwner(ctx, s.octeliumC, umetav1.GetObjectReference(org)); err != nil {
			return nil, err
		}

		canOwn, err := s.canOwnSpace(ctx, org)
		if err != nil {
			return nil, err
		}
		if !canOwn {
			return nil, grpcutils.Unauthorized("This User is not authorized to become an OWNER")
		}
	}

	item, err = s.octeliumC.CordiumC().CreateMembership(ctx, item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (s *Server) UpdateMembership(ctx context.Context, req *cordiumv1.Membership) (*cordiumv1.Membership, error) {

	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			ParentsMust: 2,
		},
		RequireData: true,
	}); err != nil {
		return nil, err
	}

	itm, err := s.octeliumC.CordiumC().GetMembership(ctx,
		&rmetav1.GetOptions{
			Uid:  req.Metadata.Uid,
			Name: req.Metadata.Name,
		})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	spc, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: itm.Status.SpaceRef.Uid,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, itm.Status.SpaceRef); err != nil {
		return nil, err
	}

	if itm.Spec == nil {
		return nil, serr.InvalidArg("Nil spec")
	}

	if itm.Spec.Role == cordiumv1.Membership_Spec_UNKNOWN {
		return nil, serr.InvalidArg("The Role cannot be unknown")
	}

	if req.Spec.Role != itm.Spec.Role {
		if spc.Status.UserRef.Uid == itm.Status.UserRef.Uid {
			return nil, grpcutils.InvalidArg("Space Creators cannot update their own Memberships")
		}
	}

	itm.Spec.Role = req.Spec.Role

	if itm.Spec.Role == cordiumv1.Membership_Spec_OWNER {
		if err := commonw.CheckIsMemberOwner(ctx, s.octeliumC, itm.Status.SpaceRef); err != nil {
			return nil, err
		}

		canOwn, err := s.canOwnSpace(ctx, spc)
		if err != nil {
			return nil, err
		}
		if !canOwn {
			return nil, grpcutils.Unauthorized("This User is not authorized to become an OWNER")
		}
	}

	ret, err := s.octeliumC.CordiumC().UpdateMembership(ctx, itm)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	ret.Status.GitProviderStateMap = nil

	return ret, nil
}

func (s *Server) DeleteMembership(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {

	if err := apivalidation.CheckDeleteOptions(getFullDeleteOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	itm, err := s.octeliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	spc, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: itm.Status.SpaceRef.Uid,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if spc.Status.UserRef.Uid == itm.Status.UserRef.Uid {
		return nil, grpcutils.Unauthorized("Space Creators cannot remove themselves")
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, itm.Status.SpaceRef); err != nil {
		return nil, err
	}

	if itm.Spec.Role == cordiumv1.Membership_Spec_OWNER {
		if err := commonw.CheckIsMemberOwner(ctx, s.octeliumC, itm.Status.SpaceRef); err != nil {
			return nil, err
		}
	}

	if _, err := s.octeliumC.CordiumC().DeleteMembership(ctx, &rmetav1.DeleteOptions{Uid: itm.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) ListMembership(ctx context.Context, req *cordiumv1.ListMembershipOptions) (*cordiumv1.MembershipList, error) {
	org, err := s.getMemberSpaceFromSpaceRef(ctx, getFullResourceRefSpace(ctx, req.SpaceRef))
	if err != nil {
		return nil, err
	}

	memList, err := s.octeliumC.CordiumC().ListMembership(ctx,
		urscsrv.GetUserPublicListOptions(req, ourscsrv.FilterStatusSpaceUID(org.Metadata.Uid)))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	for _, itm := range memList.Items {
		itm.Status.GitProviderStateMap = nil
	}

	return memList, nil
}

func (s *Server) GetSpaceMembership(ctx context.Context, req *cordiumv1.GetSpaceMembershipRequest) (*cordiumv1.Membership, error) {

	org, err := s.getMemberSpaceFromSpaceRef(ctx, req.SpaceRef)
	if err != nil {
		return nil, err
	}

	mem, err := commonw.GetMembershipBySpace(ctx, s.octeliumC, umetav1.GetObjectReference(org))
	if err != nil {
		return nil, grpcutils.K8sNotFoundOrInternalWithErr(err)
	}

	return mem, nil
}

func (s *Server) GetMembership(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Membership, error) {
	if err := apivalidation.CheckGetOptions(getFullGetOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMember(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	item.Status.GitProviderStateMap = nil

	return item, nil
}
