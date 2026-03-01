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

	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/pkg/errors"
)

func (s *Server) ListRegion(ctx context.Context, req *cordiumv1.ListRegionOptions) (*cordiumv1.RegionList, error) {

	itmList, err := s.octeliumC.CoreC().ListRegion(ctx,
		urscsrv.GetUserPublicListOptions(req, urscsrv.FilterFieldBooleanTrue("status.ext.cordium.isEnabled")))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	ret := &cordiumv1.RegionList{
		ApiVersion:       "cordium/v1",
		Kind:             "RegionList",
		ListResponseMeta: itmList.ListResponseMeta,
	}

	for _, itm := range itmList.Items {

		rgn := &cordiumv1.Region{
			Metadata: &metav1.Metadata{
				Uid:  itm.Metadata.Uid,
				Name: itm.Metadata.Name,
			},

			Spec: &cordiumv1.Region_Spec{},

			Status: &cordiumv1.Region_Status{},
		}
		ret.Items = append(ret.Items, rgn)
	}

	return ret, nil

}

func getRegionExtInfo(r *corev1.Region) (*cordiumv1.RegionExtInfo, error) {
	if r.Status.Ext == nil || r.Status.Ext[ovutils.ExtInfoLabel] == nil {
		return nil, errors.Errorf("Region ext info does not exist")
	}

	ret := &cordiumv1.RegionExtInfo{}
	if err := pbutils.StructToMessage(r.Status.Ext[ovutils.ExtInfoLabel], ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func regionHasCordium(r *corev1.Region) bool {
	ext, err := getRegionExtInfo(r)
	if err != nil {
		return false
	}

	return ext.IsEnabled
}
