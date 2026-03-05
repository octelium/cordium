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
	"strings"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
)

func (s *Server) getMemberSpaceFromSpaceRef(ctx context.Context, spaceRef *metav1.ObjectReference) (*cordiumv1.Space, error) {
	if spaceRef == nil {
		return nil, grpcutils.InvalidArg("SpaceRef is not provided")
	}

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Uid:  spaceRef.Uid,
		Name: spaceRef.Name,
	}, &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 1,
	}); err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMember(ctx, s.octeliumC, spaceRef); err != nil {
		return nil, err
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid:  spaceRef.Uid,
		Name: spaceRef.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	return org, nil
}

func (s *Server) checkIsResourceOwnerOrSpaceOwner(ctx context.Context, resourceUserRef *metav1.ObjectReference, spaceRef *metav1.ObjectReference) error {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return err
	}

	// get the membership since the resource owner could have been removed from the Space
	if _, err := commonw.GetMembershipBySpace(ctx, s.octeliumC, spaceRef); err != nil {
		if grpcerr.IsNotFound(err) {
			return serr.Unauthorized("The User is not a Member in this Space")
		}
		return err
	}

	if resourceUserRef != nil && resourceUserRef.Uid == i.User.Metadata.Uid {
		return nil
	}

	zap.L().Debug("checking if member is owner of Space")

	return commonw.CheckIsMemberOwner(ctx, s.octeliumC, spaceRef)
}

func (s *Server) checkResourceDefault(rsc umetav1.ResourceObjectI) error {
	if rsc == nil || rsc.GetMetadata() == nil || rsc.GetMetadata().Name == "" {
		return grpcutils.InvalidArg("Resource name is not set")
	}

	if strings.HasPrefix(rsc.GetMetadata().Name, "default.") {
		return grpcutils.InvalidArg("This resource is not a default resource")
	}

	return nil
}

func getFullGetOptionsSpaceChild(ctx context.Context, req *metav1.GetOptions) *metav1.GetOptions {
	if req == nil || req.Name == "" {
		return req
	}

	if isNameFQDN(req.Name, 2) {
		return req
	}
	if isNameFQDN(req.Name, 0) {
		req.Name = fmt.Sprintf("%s.default", req.Name)
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.Name = getFullGetOptionsUserCtx(i, req.Name, 2)
	return req
}

func getFullDeleteOptionsSpaceChild(ctx context.Context, req *metav1.DeleteOptions) *metav1.DeleteOptions {
	if req == nil || req.Name == "" {
		return req
	}

	if isNameFQDN(req.Name, 2) {
		return req
	}
	if isNameFQDN(req.Name, 0) {
		req.Name = fmt.Sprintf("%s.default", req.Name)
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.Name = getFullGetOptionsUserCtx(i, req.Name, 2)
	return req
}

func getFullNamResourceSpaceChild(ctx context.Context, req umetav1.ResourceObjectI) umetav1.ResourceObjectI {
	if req == nil || req.GetMetadata() == nil || req.GetMetadata().Name == "" {
		return req
	}

	if isNameFQDN(req.GetMetadata().Name, 2) {
		return req
	}
	if isNameFQDN(req.GetMetadata().Name, 0) {
		req.GetMetadata().Name = fmt.Sprintf("%s.default", req.GetMetadata().Name)
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.GetMetadata().Name = getFullGetOptionsUserCtx(i, req.GetMetadata().Name, 2)
	return req
}

func getFullGetOptionsUserCtx(i *userctx.UserCtx, name string, parents int) string {
	args := strings.Split(name, ".")

	switch {
	case len(args) == parents:
		return fmt.Sprintf("%s.%s", name, i.User.Metadata.Name)
	default:
		return name
	}
}

func isNameFQDN(name string, parents int) bool {
	args := strings.Split(name, ".")

	return len(args) == parents+1
}

type reqSpaceResource struct {
	name  string
	space string
}

func parseSpaceResource(arg string) (*reqSpaceResource, error) {
	args := strings.Split(arg, ".")
	if len(args) != 3 {
		return nil, serr.InvalidArg("Invalid requested name: %s", arg)
	}
	return &reqSpaceResource{
		name:  args[0],
		space: fmt.Sprintf("%s.%s", args[1], args[2]),
	}, nil
}

func getFullNameGetOptionsUserChild(ctx context.Context, req *metav1.GetOptions) *metav1.GetOptions {
	if req == nil || req.Name == "" {
		return req
	}

	if isNameFQDN(req.Name, 1) {
		return req
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.Name = getFullGetOptionsUserCtx(i, req.Name, 1)
	return req
}

func getFullNamResourceUserChild(ctx context.Context, req umetav1.ResourceObjectI) umetav1.ResourceObjectI {
	if req == nil || req.GetMetadata() == nil || req.GetMetadata().Name == "" {
		return req
	}

	if isNameFQDN(req.GetMetadata().Name, 1) {
		return req
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.GetMetadata().Name = getFullGetOptionsUserCtx(i, req.GetMetadata().Name, 1)
	return req
}

func getFullNamResourceUserChildWithUserCtx(i *userctx.UserCtx, req umetav1.ResourceObjectI) umetav1.ResourceObjectI {
	if req == nil || req.GetMetadata() == nil || req.GetMetadata().Name == "" {
		return req
	}

	if isNameFQDN(req.GetMetadata().Name, 1) {
		return req
	}

	req.GetMetadata().Name = getFullGetOptionsUserCtx(i, req.GetMetadata().Name, 1)
	return req
}

func getFullNamResourceSpace(ctx context.Context, req *cordiumv1.Space) *cordiumv1.Space {
	if req == nil || req.GetMetadata() == nil || req.GetMetadata().Name == "" {
		return req
	}

	if isNameFQDN(req.GetMetadata().Name, 1) {
		return req
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.GetMetadata().Name = getFullGetOptionsUserCtx(i, req.GetMetadata().Name, 1)
	return req
}

func getFullGetOptionsSpace(ctx context.Context, req *metav1.GetOptions) *metav1.GetOptions {
	if req == nil || req.Name == "" {
		return req
	}

	if isNameFQDN(req.Name, 1) {
		return req
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.Name = getFullGetOptionsUserCtx(i, req.Name, 1)
	return req
}

func getFullResourceRefSpace(ctx context.Context, req *metav1.ObjectReference) *metav1.ObjectReference {
	if req == nil || req.Name == "" {
		return req
	}

	if isNameFQDN(req.Name, 1) {
		return req
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return req
	}

	req.Name = getFullGetOptionsUserCtx(i, req.Name, 1)
	return req
}
