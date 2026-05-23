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

const maxSecretsPerSpace = 1000

func (s *Server) CreateSecret(ctx context.Context, req *cordiumv1.Secret) (*cordiumv1.Secret, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req), &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
			RequireName: true,
			ParentsMust: 2,
		},
		RequireData: true,
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
		itmList, err := s.octeliumC.CordiumC().ListSecret(ctx, ourscsrv.FilterBySpace(org))
		if err != nil {
			return nil, err
		}

		if len(itmList.Items) >= maxSecretsPerSpace {
			return nil, serr.Unauthorized("Number of Secrets per Space has been exceeded")
		}
	}

	{
		_, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
			Name: req.Metadata.Name,
		})
		if err == nil {
			return nil, grpcutils.InvalidArg("This Secret name already exists")
		} else if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	item := &cordiumv1.Secret{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec:     req.Spec,
		Status: &cordiumv1.Secret_Status{
			UserRef:  umetav1.GetObjectReference(i.User),
			SpaceRef: umetav1.GetObjectReference(org),
		},
		Data: req.Data,
	}

	if err := s.checkSecretData(ctx, item); err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	ret, err := s.octeliumC.CordiumC().CreateSecret(ctx, item)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	ret.Data = nil

	return ret, nil
}

func (s *Server) checkSecretData(ctx context.Context, req *cordiumv1.Secret) error {

	maxSize := 1500

	if req.Data == nil {
		return serr.InvalidArg("Data is not set")
	}

	switch req.Data.Type.(type) {
	case *cordiumv1.Secret_Data_Value:
		lenVal := len(req.Data.GetValue())
		if lenVal == 0 {
			return serr.InvalidArg("Empty value")
		}
		if lenVal > maxSize {
			return serr.InvalidArg("Secret data is too large")
		}

	case *cordiumv1.Secret_Data_ValueBytes:
		lenVal := len(req.Data.GetValueBytes())
		if lenVal == 0 {
			return serr.InvalidArg("Empty value")
		}
		if lenVal > maxSize {
			return serr.InvalidArg("Secret data is too large")
		}
	default:
		return serr.InvalidArg("Invalid Secret data type")
	}

	return nil

}

func (s *Server) ListSecret(ctx context.Context, req *cordiumv1.ListSecretOptions) (*cordiumv1.SecretList, error) {

	org, err := s.getMemberSpaceFromSpaceRef(ctx, getFullResourceRefSpace(ctx, req.SpaceRef))
	if err != nil {
		return nil, err
	}

	secretList, err := s.octeliumC.CordiumC().ListSecret(ctx,
		urscsrv.GetUserPublicListOptions(req, ourscsrv.FilterStatusSpaceUID(org.Metadata.Uid)))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	for _, sec := range secretList.Items {
		sec.Data = nil
	}

	return secretList, nil
}

func (s *Server) DeleteSecret(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	if err := apivalidation.CheckDeleteOptions(getFullDeleteOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	if _, err := s.octeliumC.CordiumC().DeleteSecret(ctx, &rmetav1.DeleteOptions{Uid: item.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) GetSecret(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Secret, error) {
	if err := apivalidation.CheckGetOptions(getFullGetOptionsSpaceChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 2,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	item.Data = nil

	return item, nil
}
