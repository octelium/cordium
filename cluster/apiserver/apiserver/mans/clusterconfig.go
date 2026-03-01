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

package mans

import (
	"context"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	apisrvcommon "github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/grpcutils"
)

func (s *Server) GetClusterConfig(ctx context.Context, req *cordiumv1.GetClusterConfigRequest) (*cordiumv1.ClusterConfig, error) {
	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return cc, nil
}

func (s *Server) UpdateClusterConfig(ctx context.Context, req *cordiumv1.ClusterConfig) (*cordiumv1.ClusterConfig, error) {

	if err := s.validateClusterConfig(ctx, req); err != nil {
		return nil, err
	}

	cfg, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	apisrvcommon.MetadataUpdate(cfg.Metadata, req.Metadata)
	cfg.Spec = req.Spec

	ccOut, err := s.octeliumC.CordiumC().UpdateClusterConfig(ctx, cfg)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return ccOut, nil
}

func (s *Server) validateClusterConfig(ctx context.Context, req *cordiumv1.ClusterConfig) error {

	if req.Spec == nil {
		return grpcutils.InvalidArg("Nil spec")
	}

	return nil
}
