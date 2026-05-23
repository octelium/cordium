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

	"github.com/asaskevich/govalidator"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
)

const maxGitProviderPerSpace = 100

func (s *Server) CreateGitProvider(ctx context.Context, req *cordiumv1.GitProvider) (*cordiumv1.GitProvider, error) {

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

	nameReq, err := parseSpaceResource(req.Metadata.Name)
	if err != nil {
		return nil, err
	}
	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Name: nameReq.space,
	})
	if err != nil {
		return nil, err
	}

	{
		itmList, err := s.octeliumC.CordiumC().ListGitProvider(ctx, ourscsrv.FilterBySpace(org))
		if err != nil {
			return nil, err
		}

		if len(itmList.Items) >= maxGitProviderPerSpace {
			return nil, serr.Unauthorized("Number of Git Provider per Space has been exceeded")
		}
	}

	{
		_, err := s.octeliumC.CordiumC().GetGitProvider(ctx, &rmetav1.GetOptions{
			Name: req.Metadata.Name,
		})
		if err == nil {
			return nil, grpcutils.InvalidArg("This GitProvider name already exists")
		} else if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	item := &cordiumv1.GitProvider{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec:     req.Spec,
		Status: &cordiumv1.GitProvider_Status{
			UserRef:  umetav1.GetObjectReference(i.User),
			SpaceRef: umetav1.GetObjectReference(org),
		},
	}

	if err := s.validateGitProvider(ctx, item); err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	item, err = s.octeliumC.CordiumC().CreateGitProvider(ctx, item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (s *Server) UpdateGitProvider(ctx context.Context, req *cordiumv1.GitProvider) (*cordiumv1.GitProvider, error) {
	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			ParentsMust: 2,
		},
		RequireStatus: true,
	}); err != nil {
		return nil, err
	}

	itm, err := s.octeliumC.CordiumC().GetGitProvider(ctx,
		&rmetav1.GetOptions{
			Name: req.Metadata.Name,
			Uid:  req.Metadata.Uid,
		})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: itm.Status.SpaceRef.Uid,
	})
	if err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, umetav1.GetObjectReference(org)); err != nil {
		return nil, err
	}

	if err := s.checkIsResourceOwnerOrSpaceOwner(ctx, itm.Status.UserRef, itm.Status.SpaceRef); err != nil {
		return nil, err
	}

	common.MetadataUpdate(itm.Metadata, req.Metadata)
	itm.Spec = req.Spec

	if err := s.validateGitProvider(ctx, itm); err != nil {
		return nil, err
	}

	ret, err := s.octeliumC.CordiumC().UpdateGitProvider(ctx, itm)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return ret, nil
}

func (s *Server) DeleteGitProvider(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	if err := apivalidation.CheckDeleteOptions(getFullDeleteOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	itm, err := s.octeliumC.CordiumC().GetGitProvider(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := s.checkIsResourceOwnerOrSpaceOwner(ctx, itm.Status.UserRef, itm.Status.SpaceRef); err != nil {
		return nil, err
	}

	if _, err := s.octeliumC.CordiumC().DeleteGitProvider(ctx, &rmetav1.DeleteOptions{Uid: itm.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) ListGitProvider(ctx context.Context, req *cordiumv1.ListGitProviderOptions) (*cordiumv1.GitProviderList, error) {

	org, err := s.getMemberSpaceFromSpaceRef(ctx, req.SpaceRef)
	if err != nil {
		return nil, err
	}

	itmList, err := s.octeliumC.CordiumC().ListGitProvider(ctx,
		urscsrv.GetUserPublicListOptions(req, ourscsrv.FilterStatusSpaceUID(org.Metadata.Uid)))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return itmList, nil
}

func (s *Server) validateGitProvider(ctx context.Context, req *cordiumv1.GitProvider) error {
	spec := req.Spec
	switch spec.Type.(type) {
	case *cordiumv1.GitProvider_Spec_Github_:
		github := spec.GetGithub()
		if err := apivalidation.ValidateGenASCII(github.ClientID); err != nil {
			return grpcutils.InvalidArg("Invalid clientID. %s", err.Error())
		}
		if err := s.validateSecretOwner(ctx, github.ClientSecret, req.Status.SpaceRef); err != nil {
			return err
		}

		if len(github.Scopes) > 10 {
			return grpcutils.InvalidArg("Too many scopes")
		}

		for _, scope := range github.Scopes {
			if err := apivalidation.ValidateGenASCII(scope); err != nil {
				return grpcutils.InvalidArg("Invalid scope %s: %s", scope, err.Error())
			}
		}

	case *cordiumv1.GitProvider_Spec_Gitlab_:
		gitlab := spec.GetGitlab()
		if err := apivalidation.ValidateGenASCII(gitlab.ClientID); err != nil {
			return grpcutils.InvalidArg("Invalid clientID. %s", err.Error())
		}
		if err := s.validateSecretOwner(ctx, gitlab.ClientSecret, req.Status.SpaceRef); err != nil {
			return err
		}

		if len(gitlab.Scopes) > 10 {
			return grpcutils.InvalidArg("Too many scopes")
		}

		for _, scope := range gitlab.Scopes {
			if err := apivalidation.ValidateGenASCII(scope); err != nil {
				return grpcutils.InvalidArg("Invalid scope %s: %s", scope, err.Error())
			}
		}

	case *cordiumv1.GitProvider_Spec_Oauth2:
		oauth2C := spec.GetOauth2()

		if err := apivalidation.ValidateGenASCII(oauth2C.ClientID); err != nil {
			return grpcutils.InvalidArg("Invalid clientID. %s", err.Error())
		}
		if !govalidator.IsURL(oauth2C.AuthURL) {
			return grpcutils.InvalidArg("Invalid auth URL")
		}
		if !govalidator.IsURL(oauth2C.TokenURL) {
			return grpcutils.InvalidArg("Invalid token URL")
		}

		if err := s.validateSecretOwner(ctx, oauth2C.ClientSecret, req.Status.SpaceRef); err != nil {
			return err
		}

		if len(oauth2C.Scopes) == 0 {
			return grpcutils.InvalidArg("You must provide at least one scope")
		}

		if len(oauth2C.Scopes) > 10 {
			return grpcutils.InvalidArg("Too many scopes")
		}

		for _, scope := range oauth2C.Scopes {
			if err := apivalidation.ValidateGenASCII(scope); err != nil {
				return grpcutils.InvalidArg("Invalid scope %s: %s", scope, err.Error())
			}
		}

	default:
		return grpcutils.InvalidArg("You must provide a GitProvider type")
	}

	return nil
}

type secretOwner interface {
	GetFromSecret() string
}

func (s *Server) validateSecretOwner(ctx context.Context, secOwner secretOwner, spaceRef *metav1.ObjectReference) error {
	if secOwner == nil {
		return grpcutils.InvalidArg("You must set fromSecret")
	}
	if secOwner.GetFromSecret() == "" {
		return grpcutils.InvalidArg("Empty Secret name")
	}

	sec, err := s.octeliumC.CordiumC().GetSecret(ctx,
		&rmetav1.GetOptions{
			Name: secOwner.GetFromSecret(),
		})
	if err == nil {
		if sec.Status.SpaceRef.Uid != spaceRef.Uid {
			return grpcutils.InvalidArg("Secret does not exist: %s", secOwner.GetFromSecret())
		}
		return nil
	}
	if !grpcerr.IsNotFound(err) {
		return grpcutils.InternalWithErr(err)
	}
	return grpcutils.InvalidArg("The Secret %s is not found", secOwner.GetFromSecret())
}

func (s *Server) GetGitProvider(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.GitProvider, error) {
	if err := apivalidation.CheckGetOptions(getFullGetOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetGitProvider(ctx, &rmetav1.GetOptions{
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
