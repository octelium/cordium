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
