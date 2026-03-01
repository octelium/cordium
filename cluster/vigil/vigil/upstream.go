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
	"net"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/cluster/vigil/vigil/loadbalancer"
	"github.com/octelium/octelium/cluster/vigil/vigil/modes"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/pkg/errors"
)

func (s *srv) doGetUpstream(ctx context.Context, opts *modes.Opts, req *corev1.RequestContext) (*loadbalancer.Upstream, error) {

	// zap.L().Debug("New doGetUpstream", zap.Any("req", req))
	if req == nil || req.Request == nil ||
		req.Request.GetSsh() == nil || req.Request.GetSsh().GetConnect() == nil ||
		req.Request.GetSsh().GetConnect().User == "" {

		return nil, errors.Errorf("Invalid doGetUpstream req")
	}

	ws, err := s.aCache.GetWorkspace(req.Request.GetSsh().GetConnect().User)
	if err != nil {
		return nil, err
	}
	if ws.Status.SessionRef == nil {
		return nil, errors.Errorf("Workspace Session is not set")
	}

	sess, err := s.aCache.GetSession(ws.Status.SessionRef.Uid)
	if err != nil {
		return nil, err
	}
	if sess.Status.Connection == nil ||
		!sess.Status.Connection.ESSHEnable {
		return nil, errors.Errorf("The Session is not connected or does not support eSSH")
	}

	ret := &loadbalancer.Upstream{
		HostPort:         net.JoinHostPort(suputils.GetWorkspaceSupHost(ws), "2022"),
		IsUser:           true,
		IsESSH:           true,
		Ed25519PublicKey: sess.Status.Connection.Ed25519PublicKey,
	}

	// zap.L().Debug("Found upstream", zap.Any("upstream", ret))

	return ret, nil
}
