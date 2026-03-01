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

	"github.com/asaskevich/govalidator"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
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

	return wsutils.ValidateWorkspace(ctx, validateReq)
}

func isValidURL(arg string) bool {
	if !govalidator.IsURL(arg) {
		return false
	}
	if len(arg) > 256 {
		return false
	}
	return true
}

func isInList(lst []string, arg string) bool {
	for _, itm := range lst {
		if itm == arg {
			return true
		}
	}
	return false
}
