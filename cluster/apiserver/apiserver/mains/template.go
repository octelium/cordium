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
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
)

const maxTemplatesPerEnvironment = 1000

func (s *Server) CreateTemplate(ctx context.Context, req *cordiumv1.Template) (*cordiumv1.Template, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			RequireName: true,
			ParentsMust: 2,
		},
	}); err != nil {
		return nil, err
	}

	nameArgs := strings.Split(req.Metadata.Name, ".")
	if len(nameArgs) != 3 {
		return nil, serr.InvalidArg("Invalid Template name: %s", req.Metadata.Name)
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Name: fmt.Sprintf("%s.%s", nameArgs[1], nameArgs[2]),
	})
	if err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, umetav1.GetObjectReference(org)); err != nil {
		return nil, err
	}

	{
		itmList, err := s.octeliumC.CordiumC().ListTemplate(ctx, ourscsrv.FilterBySpace(org))
		if err != nil {
			return nil, err
		}

		if len(itmList.Items) >= maxTemplatesPerEnvironment {
			return nil, serr.Unauthorized("Number of Templates per Environment has been exceeded")
		}
	}

	_, err = s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Name: req.Metadata.Name,
	})
	if err == nil {
		return nil, serr.InvalidArg("Template name already exists for this Environment")
	} else if !grpcerr.IsNotFound(err) {
		return nil, serr.InternalWithErr(err)
	}

	template := &cordiumv1.Template{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec:     req.Spec,
		Status: &cordiumv1.Template_Status{
			SpaceRef: umetav1.GetObjectReference(org),
			UserRef:  umetav1.GetObjectReference(i.User),

			BuildInfo: &cordiumv1.Template_Status_BuildInfo{},
		},
	}
	if err := s.validateAndSetTemplate(ctx, template); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().CreateTemplate(ctx, template)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (s *Server) UpdateTemplate(ctx context.Context, req *cordiumv1.Template) (*cordiumv1.Template, error) {

	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			ParentsMust: 2,
		},
		RequireStatus: true,
	}); err != nil {
		return nil, err
	}

	template, err := s.octeliumC.CordiumC().GetTemplate(ctx,
		&rmetav1.GetOptions{
			Name: req.Metadata.Name,
			Uid:  req.Metadata.Uid,
		})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	common.MetadataUpdate(template.Metadata, req.Metadata)
	template.Spec = req.Spec

	if err := s.validateAndSetTemplate(ctx, template); err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, template.Status.SpaceRef); err != nil {
		return nil, err
	}

	template, err = s.octeliumC.CordiumC().UpdateTemplate(ctx, template)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return template, nil
}

func (s *Server) DeleteTemplate(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {

	if err := apivalidation.CheckDeleteOptions(getFullDeleteOptionsSpaceChild(ctx, req),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
		return nil, err
	}
	tmpl, err := s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := s.checkResourceDefault(tmpl); err != nil {
		return nil, grpcutils.InvalidArg("You cannot delete the default Template: %s", tmpl.Metadata.Name)
	}

	if err := s.checkIsResourceOwnerOrSpaceOwner(ctx, tmpl.Status.UserRef, tmpl.Status.SpaceRef); err != nil {
		return nil, err
	}

	if _, err := s.octeliumC.CordiumC().DeleteTemplate(ctx, &rmetav1.DeleteOptions{Uid: tmpl.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) chooseRegion(ctx context.Context, ws *cordiumv1.Workspace, req *metav1.ObjectReference) (*corev1.Region, error) {

	if usrCfg, err := s.GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{}); err == nil {
		if usrCfg.Status.PreferredRegionRef != nil {
			if region, err := s.octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
				Uid: usrCfg.Status.PreferredRegionRef.Uid,
			}); err == nil {
				if regionHasCordium(region) {
					return region, nil
				}
			}
		}
	}

	if req == nil {
		regionList, err := s.octeliumC.CoreC().ListRegion(ctx, &rmetav1.ListOptions{
			SpecLabels: map[string]string{
				"has-workspace": "true",
			},
		})
		if err != nil {
			return nil, err
		}
		if len(regionList.Items) < 1 {
			return nil, grpcutils.InvalidArg("No Region accepts Workspaces")
		}
		if len(regionList.Items) == 1 {
			return regionList.Items[0], nil
		}

		return regionList.Items[utilrand.GetRandomRangeMath(0, len(regionList.Items)-1)], nil
	}

	if !govalidator.IsUUIDv4(req.Uid) {
		return nil, grpcutils.InvalidArg("Invalid UID")
	}

	region, err := s.octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
		Uid: req.Uid,
	})
	if err != nil {
		return nil, err
	}
	if !regionHasCordium(region) {
		return nil, grpcutils.InvalidArg("This Region does not accept Workspaces")
	}

	return region, nil
}

func (s *Server) validateAndSetTemplate(ctx context.Context, req *cordiumv1.Template) error {

	spec := req.Spec

	ws := &cordiumv1.Workspace{
		Metadata: req.Metadata,
		Spec: &cordiumv1.Workspace_Spec{
			Image:                  spec.Image,
			Runtime:                spec.Runtime,
			Repository:             spec.Repository,
			AdditionalRepositories: spec.AdditionalRepositories,
			Limit:                  spec.Limit,
		},
		Status: &cordiumv1.Workspace_Status{
			SpaceRef: req.Status.SpaceRef,
		},
	}

	if err := s.validateAndSetWorkspace(ctx, ws); err != nil {
		return err
	}

	req.Metadata.SpecLabels = ws.Metadata.SpecLabels

	if req.Spec.GitProvider != "" {

		gitProvider, err := s.octeliumC.CordiumC().GetGitProvider(ctx, &rmetav1.GetOptions{
			Name: req.Spec.GitProvider,
		})
		if err != nil {
			return err
		}

		if gitProvider.Status.SpaceRef.Uid != req.Status.SpaceRef.Uid {
			return grpcutils.InvalidArg("GitProvider does not exist: %s", req.Spec.GitProvider)
		}
		req.Status.GitProviderRef = umetav1.GetObjectReference(gitProvider)
	} else {
		req.Status.GitProviderRef = nil
	}

	return nil
}

func (s *Server) ListTemplate(ctx context.Context, req *cordiumv1.ListTemplateOptions) (*cordiumv1.TemplateList, error) {

	org, err := s.getMemberSpaceFromSpaceRef(ctx, getFullResourceRefSpace(ctx, req.SpaceRef))
	if err != nil {
		return nil, err
	}

	ret, err := s.octeliumC.CordiumC().ListTemplate(ctx,
		urscsrv.GetUserPublicListOptions(req, ourscsrv.FilterStatusSpaceUID(org.Metadata.Uid)))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *Server) GetTemplate(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Template, error) {

	if err := apivalidation.CheckGetOptions(getFullGetOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMember(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	return item, nil
}
