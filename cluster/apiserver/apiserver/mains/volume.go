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

const maxVolumesPerSpace = 64
const defaultVolumeMegabytes = 10 * 1000
const maxVolumeMegabytes = 10_000_000

func (s *Server) CreateVolume(ctx context.Context, req *cordiumv1.Volume) (*cordiumv1.Volume, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req),
		&apivalidation.ValidateCommonOpts{
			ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
				RequireName: true,
				ParentsMust: 2,
			},
		}); err != nil {
		return nil, err
	}

	if req.Spec == nil {
		return nil, serr.InvalidArg("Spec is not set")
	}

	nameReq, err := parseSpaceResource(req.Metadata.Name)
	if err != nil {
		return nil, err
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Name: nameReq.space,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, umetav1.GetObjectReference(org)); err != nil {
		return nil, err
	}

	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	{
		itmList, err := s.octeliumC.CordiumC().ListVolume(ctx,
			ourscsrv.SetCountOnly(ourscsrv.FilterBySpace(org)))
		if err != nil {
			return nil, serr.InternalWithErr(err)
		}

		if itmList.GetListResponseMeta().GetTotalCount() >= uint32(s.getMaxVolumesPerSpace(cc)) {
			return nil, serr.Unauthorized("Number of Volumes per Space has been exceeded")
		}
	}

	{
		_, err := s.octeliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
			Name: req.Metadata.Name,
		})
		if err == nil {
			return nil, grpcutils.InvalidArg("This Volume name already exists")
		} else if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	item := &cordiumv1.Volume{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec: &cordiumv1.Volume_Spec{
			Size: &cordiumv1.Volume_Spec_Size{
				Megabytes: s.getVolumeMegabytes(req, cc),
			},
			AccessMode: func() cordiumv1.Volume_AccessMode {
				if req.Spec.AccessMode == cordiumv1.Volume_ACCESS_MODE_UNSET {
					return cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE
				}
				return req.Spec.AccessMode
			}(),
		},
		Status: &cordiumv1.Volume_Status{
			State:    cordiumv1.Volume_Status_STATE_PENDING,
			SpaceRef: umetav1.GetObjectReference(org),
			UserRef:  umetav1.GetObjectReference(i.User),
		},
	}

	if err := s.checkVolumeSize(item, cc); err != nil {
		return nil, err
	}

	region, err := s.chooseRegion(ctx, nil, func() *metav1.ObjectReference {
		if req.Status == nil {
			return nil
		}
		return req.Status.RegionRef
	}())
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	item.Status.RegionRef = umetav1.GetObjectReference(region)

	ret, err := s.octeliumC.CordiumC().CreateVolume(ctx, item)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return ret, nil
}

func (s *Server) UpdateVolume(ctx context.Context, req *cordiumv1.Volume) (*cordiumv1.Volume, error) {

	if err := apivalidation.ValidateCommon(getFullNamResourceSpaceChild(ctx, req),
		&apivalidation.ValidateCommonOpts{
			ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
				ParentsMust: 2,
			},
		}); err != nil {
		return nil, err
	}

	if req.Spec == nil {
		return nil, serr.InvalidArg("Spec is not set")
	}

	item, err := s.octeliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
		Uid:  req.Metadata.Uid,
		Name: req.Metadata.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	if req.Spec.AccessMode != cordiumv1.Volume_ACCESS_MODE_UNSET &&
		req.Spec.AccessMode != item.Spec.AccessMode {
		return nil, grpcutils.InvalidArg("The accessMode of a Volume is immutable")
	}

	if megabytes := req.Spec.GetSize().GetMegabytes(); megabytes != 0 &&
		megabytes != item.Spec.Size.Megabytes {
		if megabytes < item.Spec.Size.Megabytes {
			return nil, grpcutils.InvalidArg(
				"A Volume can only be grown. It is currently %d MB while %d MB is requested",
				item.Spec.Size.Megabytes, megabytes)
		}

		item.Spec.Size.Megabytes = megabytes

		if err := s.checkVolumeSize(item, cc); err != nil {
			return nil, err
		}
	}

	common.MetadataUpdate(item.Metadata, req.Metadata)

	ret, err := s.octeliumC.CordiumC().UpdateVolume(ctx, item)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return ret, nil
}

func (s *Server) GetVolume(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Volume, error) {

	if err := apivalidation.CheckGetOptions(getFullGetOptionsSpaceChild(ctx, req),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
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

func (s *Server) ListVolume(ctx context.Context,
	req *cordiumv1.ListVolumeOptions) (*cordiumv1.VolumeList, error) {

	org, err := s.getMemberSpaceFromSpaceRef(ctx, getFullResourceRefSpace(ctx, req.SpaceRef))
	if err != nil {
		return nil, err
	}

	itmList, err := s.octeliumC.CordiumC().ListVolume(ctx,
		urscsrv.GetUserPublicListOptions(req, ourscsrv.FilterStatusSpaceUID(org.Metadata.Uid)))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return itmList, nil
}

func (s *Server) DeleteVolume(ctx context.Context,
	req *metav1.DeleteOptions) (*metav1.OperationResult, error) {

	if err := apivalidation.CheckDeleteOptions(getFullDeleteOptionsSpaceChild(ctx, req),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, item.Status.SpaceRef); err != nil {
		return nil, err
	}

	if err := s.checkVolumeIsNotMounted(ctx, item); err != nil {
		return nil, err
	}

	if _, err := s.octeliumC.CordiumC().DeleteVolume(ctx, &rmetav1.DeleteOptions{
		Uid: item.Metadata.Uid,
	}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) checkVolumeIsNotMounted(ctx context.Context, vol *cordiumv1.Volume) error {

	tmplList, err := s.octeliumC.CordiumC().ListTemplate(ctx, ourscsrv.FilterBySpaceRef(vol.Status.SpaceRef))
	if err != nil {
		return serr.InternalWithErr(err)
	}

	for _, tmpl := range tmplList.Items {
		if hasVolumeMount(tmpl.GetSpec().GetRuntime().GetVolumeMounts(), vol) {
			return grpcutils.InvalidArg(
				"The Volume cannot be deleted while it is mounted by the Template: %s",
				tmpl.Metadata.Name)
		}
	}

	wsList, err := s.octeliumC.CordiumC().ListWorkspace(ctx, ourscsrv.FilterBySpaceRef(vol.Status.SpaceRef))
	if err != nil {
		return serr.InternalWithErr(err)
	}

	for _, ws := range wsList.Items {
		if hasVolumeMount(ws.GetSpec().GetRuntime().GetVolumeMounts(), vol) {
			return grpcutils.InvalidArg(
				"The Volume cannot be deleted while it is mounted by the Workspace: %s",
				ws.Metadata.Name)
		}
	}

	return nil
}

func hasVolumeMount(mounts []*cordiumv1.Workspace_Spec_Runtime_VolumeMount, vol *cordiumv1.Volume) bool {
	for _, mount := range mounts {
		if mount.VolumeRef == nil {
			continue
		}
		if mount.VolumeRef.Uid == vol.Metadata.Uid {
			return true
		}
	}

	return false
}

func (s *Server) getMaxVolumesPerSpace(cc *cordiumv1.ClusterConfig) int {
	if cc.Spec.Volume != nil && cc.Spec.Volume.Limit != nil &&
		cc.Spec.Volume.Limit.MaxPerSpace != 0 &&
		cc.Spec.Volume.Limit.MaxPerSpace < 1000000 {
		return int(cc.Spec.Volume.Limit.MaxPerSpace)
	}

	return maxVolumesPerSpace
}

func (s *Server) getVolumeMegabytes(req *cordiumv1.Volume, cc *cordiumv1.ClusterConfig) uint32 {
	if megabytes := req.GetSpec().GetSize().GetMegabytes(); megabytes != 0 {
		return megabytes
	}

	if cc.Spec.Volume != nil && cc.Spec.Volume.Limit != nil &&
		cc.Spec.Volume.Limit.DefaultSize.GetMegabytes() != 0 {
		return cc.Spec.Volume.Limit.DefaultSize.Megabytes
	}

	return defaultVolumeMegabytes
}

func (s *Server) checkVolumeSize(vol *cordiumv1.Volume, cc *cordiumv1.ClusterConfig) error {
	megabytes := vol.Spec.Size.Megabytes

	if megabytes < 1000 {
		return grpcutils.InvalidArg("A Volume must be at least 1000 MB")
	}

	if megabytes > maxVolumeMegabytes {
		return grpcutils.InvalidArg("The Volume size is too large: %d MB", megabytes)
	}

	if cc.Spec.Volume != nil && cc.Spec.Volume.Limit != nil &&
		cc.Spec.Volume.Limit.MaxSize.GetMegabytes() != 0 &&
		megabytes > cc.Spec.Volume.Limit.MaxSize.Megabytes {
		return grpcutils.InvalidArg(
			"The Volume size exceeds the Cluster maximum of %d MB",
			cc.Spec.Volume.Limit.MaxSize.Megabytes)
	}

	return nil
}
