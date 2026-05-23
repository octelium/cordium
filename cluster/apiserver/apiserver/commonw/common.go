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

package commonw

import (
	"context"
	"fmt"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
)

func GetWorkspaceHostname(wsName string, region *corev1.Region, cc *corev1.ClusterConfig) string {
	if region.Metadata.Name == "default" {
		return fmt.Sprintf("%s.cordium.%s", wsName, cc.Status.Domain)
	}
	return fmt.Sprintf("%s.%s.cordium.%s", wsName, region.Metadata.Name, cc.Status.Domain)
}

func GetUserCtx(ctx context.Context) (*userctx.UserCtx, error) {
	return userctx.GetUserCtx(ctx)
}

func GetMembership(ctx context.Context, octeliumC octeliumc.ClientInterface, userRef *metav1.ObjectReference, spaceRef *metav1.ObjectReference) (*cordiumv1.Membership, error) {

	mem, err := octeliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
		Name: workspacecommon.GetMembershipName(spaceRef, userRef),
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	return mem, nil
}

func GetMembershipBySpace(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) (*cordiumv1.Membership, error) {
	i, err := GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	return GetMembership(ctx, octeliumC, umetav1.GetObjectReference(i.User), spaceRef)
}

func IsMemberAdmin(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) (bool, error) {
	mem, err := GetMembershipBySpace(ctx, octeliumC, spaceRef)
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	switch mem.Spec.Role {
	case cordiumv1.Membership_Spec_ADMIN, cordiumv1.Membership_Spec_OWNER:
		return true, nil
	default:
		return false, nil
	}
}

func IsMemberOwner(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) (bool, error) {
	mem, err := GetMembershipBySpace(ctx, octeliumC, spaceRef)
	if err != nil {
		if grpcerr.IsNotFound(err) {
			zap.L().Debug("owner mem not found")
			return false, nil
		}
		return false, err
	}

	switch mem.Spec.Role {
	case cordiumv1.Membership_Spec_OWNER:
		zap.L().Debug("member is owner", zap.Any("mem", mem))
		return true, nil
	default:
		return false, nil
	}
}

func IsMember(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) (bool, error) {
	_, err := GetMembershipBySpace(ctx, octeliumC, spaceRef)
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func CheckIsMemberAdmin(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) error {
	ok, err := IsMemberAdmin(ctx, octeliumC, spaceRef)
	if err != nil {
		return err
	}
	if !ok {
		return serr.Unauthorized("The User is not authorized to do this operation")
	}

	return nil
}

func CheckIsMemberOwner(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) error {
	ok, err := IsMemberOwner(ctx, octeliumC, spaceRef)
	if err != nil {
		return err
	}
	if !ok {
		return serr.Unauthorized("The User is not authorized to do this operation")
	}
	zap.L().Debug("mem is owner")

	return nil
}

func CheckIsMember(ctx context.Context, octeliumC octeliumc.ClientInterface, spaceRef *metav1.ObjectReference) error {
	ok, err := IsMember(ctx, octeliumC, spaceRef)
	if err != nil {
		return err
	}
	if !ok {
		return serr.Unauthorized("The User is not authorized to operation")
	}

	return nil
}
