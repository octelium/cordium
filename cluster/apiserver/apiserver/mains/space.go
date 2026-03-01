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

	"github.com/asaskevich/govalidator"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"google.golang.org/protobuf/types/known/structpb"
)

const maxSpacesPerUser = 150

func (s *Server) CreateSpace(ctx context.Context, req *cordiumv1.Space) (*cordiumv1.Space, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	{
		spcList, err := s.octeliumC.CordiumC().ListSpace(ctx, urscsrv.FilterByUser(i.User))
		if err != nil {
			return nil, err
		}

		if len(spcList.Items) >= maxSpacesPerUser {
			return nil, serr.Unauthorized("Number of Spaces per User has been exceeded")
		}
	}

	if err := apivalidation.ValidateCommon(getFullNamResourceSpace(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			RequireName: true,
			ParentsMust: 1,
		},
	}); err != nil {
		return nil, grpcutils.InvalidArg(`Invalid Space name: %+v`, err)
	}

	nameArgs := strings.Split(req.Metadata.Name, ".")
	if len(nameArgs) != 2 {
		return nil, grpcutils.InvalidArg(`Invalid Space name`)
	}

	{
		_, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
			Name: req.Metadata.Name,
		})
		if err == nil {
			return nil, grpcutils.AlreadyExists("Space name already exists")
		}
		if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	spaceType := cordiumv1.Space_Status_ORGANIZATION
	switch nameArgs[1] {
	case i.User.Metadata.Name:
		spaceType = cordiumv1.Space_Status_USER
	case "cordium":
		spaceType = cordiumv1.Space_Status_ORGANIZATION
	default:
		return nil, grpcutils.InvalidArg(`Invalid Space name`)
	}

	item := &cordiumv1.Space{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec:     req.Spec,
		Status: &cordiumv1.Space_Status{
			UserRef: umetav1.GetObjectReference(i.User),
			Type:    spaceType,
		},
	}

	canOwnSpace, err := s.canOwnSpace(ctx, item)
	if err != nil {
		return nil, err
	}
	if !canOwnSpace {
		return nil, grpcutils.Unauthorized("You are not allowed to create this Space")
	}

	if item.Metadata.SystemLabels == nil {
		item.Metadata.SystemLabels = make(map[string]string)
	}

	item.Metadata.SystemLabels["type"] = strings.ToLower(spaceType.String())

	if err := s.validateSpace(ctx, item, false); err != nil {
		return nil, err
	}

	item, err = s.octeliumC.CordiumC().CreateSpace(ctx, item)
	if err != nil {
		return nil, err
	}

	usr := i.User

	mem := &cordiumv1.Membership{
		Metadata: &metav1.Metadata{
			Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(item), umetav1.GetObjectReference(usr)),
		},
		Spec: &cordiumv1.Membership_Spec{
			Role: cordiumv1.Membership_Spec_OWNER,
		},
		Status: &cordiumv1.Membership_Status{
			UserRef:  umetav1.GetObjectReference(usr),
			SpaceRef: umetav1.GetObjectReference(item),
			UserInfo: workspacecommon.GetMembershipUserInfo(usr),
		},
	}

	if _, err := s.octeliumC.CordiumC().CreateMembership(ctx, mem); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	if _, err := s.CreateTemplate(ctx, &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("default.%s", item.Metadata.Name),
		},
		Spec: &cordiumv1.Template_Spec{},
	}); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *Server) canOwnSpace(ctx context.Context, space *cordiumv1.Space) (bool, error) {

	usrCtx, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return false, err
	}
	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return false, serr.InternalWithErr(err)
	}

	if cc.Spec == nil || cc.Spec.Space == nil || cc.Spec.Space.Ownership == nil {
		return false, nil
	}

	return s.doCanOwnSpace(ctx, space, usrCtx, cc), nil
}

func (s *Server) doCanOwnSpace(ctx context.Context, space *cordiumv1.Space, usrCtx *userctx.UserCtx,
	cc *cordiumv1.ClusterConfig) bool {

	if len(cc.Spec.Space.Ownership.Rules) == 0 {
		return false
	}

	reqCtxMap := map[string]any{
		"ctx": map[string]any{
			"user":  pbutils.MustConvertToMap(usrCtx.User),
			"space": pbutils.MustConvertToMap(space),
		},
	}

	var denyRules []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule
	var allowRules []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule
	for _, rule := range cc.Spec.Space.Ownership.Rules {
		switch rule.Effect {
		case cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_DENY:
			denyRules = append(denyRules, rule)
		case cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW:
			allowRules = append(allowRules, rule)
		}
	}

	for _, rule := range denyRules {

		cond, err := ovutils.ToCoreCondition(rule.Condition)
		if err != nil {
			continue
		}

		isMatched, err := s.celEngine.EvalCondition(ctx, cond, reqCtxMap)
		if err != nil {
			continue
		}
		if isMatched {
			return false
		}
	}

	for _, rule := range allowRules {
		cond, err := ovutils.ToCoreCondition(rule.Condition)
		if err != nil {
			continue
		}
		isMatched, err := s.celEngine.EvalCondition(ctx, cond, reqCtxMap)
		if err != nil {
			continue
		}
		if isMatched {
			return true
		}
	}

	return false

}

func (s *Server) UpdateSpace(ctx context.Context, req *cordiumv1.Space) (*cordiumv1.Space, error) {

	if err := apivalidation.ValidateCommon(getFullNamResourceSpace(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			RequireName: true,
			ParentsMust: 1,
		},
	}); err != nil {
		return nil, grpcutils.InvalidArg(`Invalid Space name: %+v`, err)
	}

	itm, err := s.octeliumC.CordiumC().GetSpace(ctx,
		&rmetav1.GetOptions{Name: req.Metadata.Name})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := s.checkIsResourceOwnerOrSpaceOwner(ctx, itm.Status.UserRef, umetav1.GetObjectReference(itm)); err != nil {
		return nil, err
	}
	common.MetadataUpdate(itm.Metadata, req.Metadata)
	itm.Spec = req.Spec

	if err := s.validateSpace(ctx, itm, true); err != nil {
		return nil, err
	}

	ret, err := s.octeliumC.CordiumC().UpdateSpace(ctx, itm)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return ret, nil
}

func (s *Server) DeleteSpace(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {

	if err := apivalidation.CheckDeleteOptions(req, &apivalidation.CheckGetOptionsOpts{
		HasUID: true,
	}); err != nil {
		return nil, err
	}

	itm, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: req.Uid,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := s.checkIsResourceOwnerOrSpaceOwner(ctx, itm.Status.UserRef, umetav1.GetObjectReference(itm)); err != nil {
		return nil, err
	}

	{
		memList, err := s.octeliumC.CordiumC().ListMembership(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return nil, err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteMembership(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return nil, grpcutils.InternalWithErr(err)
				}
			}
		}
	}

	{
		memList, err := s.octeliumC.CordiumC().ListTemplate(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return nil, err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteTemplate(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return nil, grpcutils.InternalWithErr(err)
				}
			}
		}
	}

	{
		memList, err := s.octeliumC.CordiumC().ListSecret(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return nil, err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteSecret(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return nil, grpcutils.InternalWithErr(err)
				}
			}
		}
	}

	{
		memList, err := s.octeliumC.CordiumC().ListGitProvider(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return nil, err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteGitProvider(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return nil, grpcutils.InternalWithErr(err)
				}
			}
		}
	}

	if _, err := s.octeliumC.CordiumC().DeleteSpace(ctx, &rmetav1.DeleteOptions{Uid: itm.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) GetSpace(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Space, error) {

	if err := apivalidation.CheckGetOptions(getFullGetOptionsSpace(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 1,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMember(ctx, s.octeliumC, umetav1.GetObjectReference(item)); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *Server) ListSpace(ctx context.Context, req *cordiumv1.ListSpaceOptions) (*cordiumv1.SpaceList, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	switch req.Mode {
	case cordiumv1.ListSpaceOptions_MODE_CREATED_BY, cordiumv1.ListSpaceOptions_MODE_UNSET:
		filters := []*rmetav1.ListOptions_Filter{
			urscsrv.FilterStatusUserUID(i.User.Metadata.Uid),
		}
		switch req.Type {
		case cordiumv1.Space_Status_USER:
			filters = append(filters, &rmetav1.ListOptions_Filter{
				Field: "status.type",
				Op:    rmetav1.ListOptions_Filter_OP_EQ,
				Value: &structpb.Value{
					Kind: &structpb.Value_StringValue{
						StringValue: "PERSONAL",
					},
				},
			})

		case cordiumv1.Space_Status_ORGANIZATION:
			filters = append(filters, &rmetav1.ListOptions_Filter{
				Field: "status.type",
				Op:    rmetav1.ListOptions_Filter_OP_EQ,
				Value: &structpb.Value{
					Kind: &structpb.Value_StringValue{
						StringValue: "ORGANIZATION",
					},
				},
			})
		}
		ret, err := s.octeliumC.CordiumC().ListSpace(ctx, urscsrv.GetUserPublicListOptions(req, filters...))
		if err != nil {
			return nil, err
		}
		return ret, nil
	case cordiumv1.ListSpaceOptions_MODE_MEMBER:
		filters := []*rmetav1.ListOptions_Filter{
			urscsrv.FilterStatusUserUID(i.User.Metadata.Uid),
		}

		switch req.Type {
		case cordiumv1.Space_Status_USER:
			filters = append(filters, &rmetav1.ListOptions_Filter{
				Field: "status.type",
				Op:    rmetav1.ListOptions_Filter_OP_EQ,
				Value: &structpb.Value{
					Kind: &structpb.Value_StringValue{
						StringValue: "PERSONAL",
					},
				},
			})

		case cordiumv1.Space_Status_ORGANIZATION:
			filters = append(filters, &rmetav1.ListOptions_Filter{
				Field: "status.type",
				Op:    rmetav1.ListOptions_Filter_OP_EQ,
				Value: &structpb.Value{
					Kind: &structpb.Value_StringValue{
						StringValue: "ORGANIZATION",
					},
				},
			})
		}
		memList, err := s.octeliumC.CordiumC().ListMembership(ctx,
			urscsrv.GetUserPublicListOptions(req, filters...))
		if err != nil {
			return nil, err
		}
		ret := &cordiumv1.SpaceList{
			ApiVersion:       ucordiumv1.APIVersion,
			Kind:             "SpaceList",
			ListResponseMeta: memList.ListResponseMeta,
		}

		for _, itm := range memList.Items {
			org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
				Uid: itm.Status.SpaceRef.Uid,
			})
			if err != nil {
				return nil, err
			}
			ret.Items = append(ret.Items, org)
		}

		return ret, nil
	default:
		return nil, grpcutils.InvalidArg("Invalid Mode")
	}
}

func (s *Server) LeaveSpace(ctx context.Context, req *cordiumv1.LeaveSpaceRequest) (*cordiumv1.LeaveSpaceResponse, error) {

	usrCtx, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	spc, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	mem, err := commonw.GetMembership(ctx, s.octeliumC, umetav1.GetObjectReference(usrCtx.User), umetav1.GetObjectReference(spc))
	if err != nil {
		return nil, err
	}

	if spc.Status.UserRef.Uid == mem.Status.UserRef.Uid {
		return nil, grpcutils.Unauthorized("Space Creators cannot leave the Space. You can only delete the Space")
	}

	if _, err := s.octeliumC.CordiumC().DeleteMembership(ctx, &rmetav1.DeleteOptions{Uid: mem.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &cordiumv1.LeaveSpaceResponse{}, nil
}

func (s *Server) validateSpace(ctx context.Context, req *cordiumv1.Space, isUpdate bool) error {

	spec := req.Spec

	if spec.Limit != nil {
		if req.Status.Type == cordiumv1.Space_Status_USER {
			return grpcutils.Unauthorized("Cannot set Limits for PERSONAL Spaces")
		}
	}

	if spec.Runtime != nil {
		specContainer := spec.Runtime

		if specContainer.EnvVars != nil {
			if len(specContainer.EnvVars) > 128 {
				return serr.InvalidArg("Too many container env vars")
			}

			for _, envVar := range specContainer.EnvVars {
				if envVar.Key == "" {
					return serr.InvalidArg("Env variable cannot have an empty key")
				}
				if !govalidator.IsASCII(envVar.Key) {
					return serr.InvalidArg("Invalid env var key")
				}

				if len(envVar.Key) > 64 {
					return serr.InvalidArg("Too long env var key")
				}

				switch envVar.Type.(type) {
				case *cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret:
					if req.Status.Type != cordiumv1.Space_Status_ORGANIZATION {
						return grpcutils.Unauthorized("Secrets can only be used for ORGANIZATION Spaces")
					}
					if isUpdate {
						if envVar.GetFromSecret() == "" {
							return serr.InvalidArg("Empty Secret name for the env variable with key: %s", envVar.Key)
						}

						sec, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
							Name: envVar.GetFromSecret(),
						})
						if err != nil {
							return serr.K8sNotFoundOrInternalWithErr(err)
						}
						if sec.Status.SpaceRef.Uid != req.Metadata.Uid {
							return grpcutils.InvalidArg("The Secret does not exist: %s", envVar.GetFromSecret())
						}
					}

				case *cordiumv1.Workspace_Spec_Runtime_EnvVar_Value:
					if len(envVar.GetValue()) == 0 {
						return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
					}

					if len(envVar.GetValue()) > 1024 {
						return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
					}
				default:
					return serr.InvalidArg("No env variable value for the key: %s", envVar.Key)
				}
			}
		}

		if len(specContainer.Tasks) > 128 {
			return serr.InvalidArg("Too many tasks")
		}

		for _, cmd := range specContainer.Tasks {
			if len(cmd.EnvVars) > 256 {
				serr.InvalidArg("Too large container env var list")
			}

			if cmd.Run == "" {
				return serr.InvalidArg("Empty container command")
			}

			if len(cmd.Run) > 5000 {
				return serr.InvalidArg("Command is too large")
			}

			if cmd.Type == cordiumv1.Workspace_Spec_Runtime_Task_UNKNOWN {
				return serr.InvalidArg("The type of the container command: %s must be set", cmd.Run)
			}

			if cmd.WorkingDir != "" {
				if !govalidator.IsUnixFilePath(cmd.WorkingDir) {
					return serr.InvalidArg("Invalid working dir path: %s", cmd.WorkingDir)
				}
				if len(cmd.WorkingDir) > 256 {
					return serr.InvalidArg("Too long working dir path: %s", cmd.WorkingDir)
				}
			}

			if len(cmd.EnvVars) > 128 {
				return serr.InvalidArg("Too many env vars")
			}

			for _, envVar := range cmd.EnvVars {
				if envVar.Key == "" {
					return serr.InvalidArg("Env variable cannot have an empty key")
				}
				if !govalidator.IsASCII(envVar.Key) {
					return serr.InvalidArg("Invalid env var key")
				}

				if len(envVar.Key) > 64 {
					return serr.InvalidArg("Too long env var key")
				}

				if len(envVar.Value) == 0 {
					return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
				}

				if len(envVar.Value) > 1024 {
					return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
				}

			}
		}
	}

	return nil
}
