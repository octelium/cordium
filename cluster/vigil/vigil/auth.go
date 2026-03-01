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

package vigil

import (
	"context"
	"errors"

	"github.com/octelium/cordium/cluster/vigil/vigil/acache"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/cluster/vigil/vigil/modes"
	"go.uber.org/zap"
)

func (s *srv) doPostAuthorize(ctx context.Context, req *modes.PostAuthorizeRequest) (*modes.PostAuthorizeResponse, error) {

	zap.L().Debug("New doPostAuthorize", zap.Any("req", req))
	if req == nil || req.Request == nil || req.Request.Request == nil ||
		req.Request.Request.GetSsh() == nil || req.Request.Request.GetSsh().GetConnect() == nil ||
		req.Request.Request.GetSsh().GetConnect().User == "" {
		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}

	ws, err := s.aCache.GetWorkspace(req.Request.Request.GetSsh().GetConnect().User)
	if err != nil {
		if errors.Is(err, acache.ErrNotFound) {
			return &modes.PostAuthorizeResponse{
				IsAuthorized: false,
			}, nil
		}
		return nil, err
	}
	i := req.Resp.RequestContext

	zap.L().Debug("Found Workspace from ssh user",
		zap.String("wsName", ws.Metadata.Name),
		zap.String("sessName", i.Session.Metadata.Name),
	)

	if ws.Status.UserRef == nil {

		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}
	if ws.Status.SessionRef == nil {
		zap.L().Debug("Workspace has no Session", zap.String("wsUID", ws.Metadata.Uid))

		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}
	if ws.Status.RegionRef == nil {
		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}
	if ws.Status.RegionRef.Uid != s.regionRef.Uid {
		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}

	if ws.Status.UserRef.Uid != i.User.Metadata.Uid {
		zap.L().Debug("Workspace is not owned by the User",
			zap.String("wsUserUID", ws.Status.UserRef.Uid),
			zap.String("userUID", i.User.Metadata.Uid))

		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}

	if !ucordiumv1.ToWorkspace(ws).IsPreparingOrRunning() {
		zap.L().Debug("Workspace is not preparing or starting",
			zap.String("wsUID", ws.Metadata.Uid))

		return &modes.PostAuthorizeResponse{
			IsAuthorized: false,
		}, nil
	}

	if ws.Status.SpaceRef != nil && ws.Status.SpaceRef.Uid != "" {
		space, err := s.aCache.GetSpace(ws.Status.SpaceRef.Uid)
		if err != nil {
			if errors.Is(err, acache.ErrNotFound) {
				return &modes.PostAuthorizeResponse{
					IsAuthorized: false,
				}, nil
			}
			return nil, err
		}

		if space.Spec.Authorization != nil && space.Spec.Authorization.DisableSSH {
			return &modes.PostAuthorizeResponse{
				IsAuthorized: false,
			}, nil
		}
	}

	zap.L().Debug("Workspace access is authorized",
		zap.String("wsName", ws.Metadata.Name),
		zap.String("sessName", i.Session.Metadata.Name),
	)

	return &modes.PostAuthorizeResponse{
		IsAuthorized: true,
	}, nil
}
