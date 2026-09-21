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
	"fmt"

	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
)

func (s *Server) validateAndSetWorkspace(ctx context.Context, req *cordiumv1.Workspace) error {
	if req == nil || req.Status == nil || req.Status.SpaceRef == nil {
		return grpcutils.InvalidArg("Invalid Workspace. It must have a SpaceRef")
	}

	validateReq := &wsutils.ValidateWorkspaceReq{
		Workspace: req,
	}
	var err error

	validateReq.Space, err = s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: req.Status.SpaceRef.Uid,
	})
	if err != nil {
		return err
	}
	validateReq.SecretList, err = s.octeliumC.CordiumC().ListSecret(ctx,
		&rmetav1.ListOptions{
			Filters: []*rmetav1.ListOptions_Filter{
				ourscsrv.FilterStatusSpaceUID(validateReq.Space.Metadata.Uid),
			},
		})
	if err != nil {
		return serr.InternalWithErr(err)
	}

	if req.Status.UserRef != nil {
		validateReq.UserSecretList, err = s.octeliumC.CordiumC().ListUserSecret(ctx,
			&rmetav1.ListOptions{
				Filters: []*rmetav1.ListOptions_Filter{
					urscsrv.FilterStatusUserUID(req.Status.UserRef.Uid),
				},
			})
		if err != nil {
			return serr.InternalWithErr(err)
		}
	}

	if err := wsutils.ValidateWorkspace(ctx, validateReq); err != nil {
		return err
	}

	return s.setWorkspaceVolumeMounts(ctx, req)
}

func (s *Server) setWorkspaceVolumeMounts(ctx context.Context, req *cordiumv1.Workspace) error {

	mounts := req.GetSpec().GetRuntime().GetVolumeMounts()
	if len(mounts) == 0 {
		return nil
	}

	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	if cc.Spec.Volume != nil && cc.Spec.Volume.Limit != nil &&
		cc.Spec.Volume.Limit.MaxMountsPerWorkspace != 0 &&
		uint32(len(mounts)) > cc.Spec.Volume.Limit.MaxMountsPerWorkspace {
		return grpcutils.InvalidArg("A Workspace can mount at most %d Volumes",
			cc.Spec.Volume.Limit.MaxMountsPerWorkspace)
	}

	for _, mount := range mounts {
		vol, err := s.octeliumC.CordiumC().GetVolume(ctx, &rmetav1.GetOptions{
			Uid:  mount.VolumeRef.Uid,
			Name: getFullVolumeName(mount.VolumeRef.Name, req.Status.SpaceRef),
		})
		if err != nil {
			if grpcerr.IsNotFound(err) {
				return grpcutils.InvalidArg("The Volume does not exist: %s",
					getVolumeRefName(mount.VolumeRef))
			}
			return serr.InternalWithErr(err)
		}

		if vol.Status.SpaceRef == nil || vol.Status.SpaceRef.Uid != req.Status.SpaceRef.Uid {
			return grpcutils.InvalidArg("The Volume does not exist: %s",
				getVolumeRefName(mount.VolumeRef))
		}

		mount.VolumeRef = umetav1.GetObjectReference(vol)
	}

	return nil
}

func getFullVolumeName(name string, spaceRef *metav1.ObjectReference) string {
	if name == "" || isNameFQDN(name, 2) {
		return name
	}

	if spaceRef == nil || spaceRef.Name == "" {
		return name
	}

	return fmt.Sprintf("%s.%s", name, spaceRef.Name)
}

func getVolumeRefName(ref *metav1.ObjectReference) string {
	if ref.Name != "" {
		return ref.Name
	}

	return ref.Uid
}
