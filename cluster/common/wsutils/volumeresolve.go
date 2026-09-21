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

package wsutils

import (
	"context"
	"fmt"
	"sort"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
)

type VolumeMountError struct {
	Volume  string
	Message string
}

func (e *VolumeMountError) Error() string {
	return fmt.Sprintf("The Volume: %s cannot be mounted: %s", e.Volume, e.Message)
}

func newVolumeMountErr(volume string, format string, a ...any) *VolumeMountError {
	return &VolumeMountError{
		Volume:  volume,
		Message: fmt.Sprintf(format, a...),
	}
}

type ResolveVolumeMountsReq struct {
	Workspace *cordiumv1.Workspace
	Template  *cordiumv1.Template
	RegionRef *metav1.ObjectReference
}

type ResolveVolumeMountsResp struct {
	Mounts  []*ccordiumv1.ResolvedVolumeMount
	Volumes []*cordiumv1.Volume
}

func (r *ResolveVolumeMountsResp) GetRegionRef() (*metav1.ObjectReference, error) {
	var ret *metav1.ObjectReference

	for _, vol := range r.Volumes {
		if vol.Status.RegionRef == nil {
			return nil, newVolumeMountErr(vol.Metadata.Name, "It is not hosted in a Region yet")
		}

		if ret == nil {
			ret = vol.Status.RegionRef
			continue
		}

		if vol.Status.RegionRef.Uid != ret.Uid {
			return nil, newVolumeMountErr(vol.Metadata.Name,
				"The mounted Volumes are hosted in different Regions")
		}
	}

	return ret, nil
}

func ResolveVolumeMounts(ctx context.Context, octeliumC octeliumc.ClientInterface,
	req *ResolveVolumeMountsReq) (*ResolveVolumeMountsResp, error) {

	if req == nil || req.Workspace == nil || req.Workspace.Status == nil {
		return nil, newVolumeMountErr("", "Invalid Workspace")
	}

	ws := req.Workspace

	ret := &ResolveVolumeMountsResp{}

	if ws.Status.IsBuild {
		return ret, nil
	}

	spec, err := MergeSpec(&MergeSpecReq{
		Workspace: ws,
		Template:  req.Template,
	})
	if err != nil {
		return nil, err
	}

	mounts := spec.GetRuntime().GetVolumeMounts()
	if len(mounts) == 0 {
		return ret, nil
	}

	if err := ValidateVolumeMounts(mounts); err != nil {
		return nil, newVolumeMountErr("", "%s", err.Error())
	}

	if ws.Status.SpaceRef == nil {
		return nil, newVolumeMountErr("", "The Workspace does not belong to a Space")
	}

	for _, mount := range mounts {
		name := mount.VolumeRef.Name
		if name == "" {
			name = mount.VolumeRef.Uid
		}

		vol, err := octeliumC.CordiumC().GetVolume(ctx,
			&rmetav1.GetOptions{
				Uid:  mount.VolumeRef.Uid,
				Name: mount.VolumeRef.Name,
			})
		if err != nil {
			if grpcerr.IsNotFound(err) {
				return nil, newVolumeMountErr(name, "It does not exist")
			}
			return nil, err
		}

		if vol.Status.SpaceRef == nil || vol.Status.SpaceRef.Uid != ws.Status.SpaceRef.Uid {
			return nil, newVolumeMountErr(vol.Metadata.Name, "It belongs to another Space")
		}

		if req.RegionRef != nil {
			if vol.Status.RegionRef == nil || vol.Status.RegionRef.Uid != req.RegionRef.Uid {
				return nil, newVolumeMountErr(vol.Metadata.Name,
					"It is hosted in another Region than the Workspace")
			}
		}

		if ucordiumv1.ToVolume(vol).IsFailed() {
			return nil, newVolumeMountErr(vol.Metadata.Name, "Its storage could not be provisioned")
		}

		ret.Mounts = append(ret.Mounts, &ccordiumv1.ResolvedVolumeMount{
			VolumeRef: umetav1.GetObjectReference(vol),
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
		})
		ret.Volumes = append(ret.Volumes, vol)
	}

	sort.Slice(ret.Mounts, func(i, j int) bool {
		return ret.Mounts[i].MountPath < ret.Mounts[j].MountPath
	})

	return ret, nil
}
