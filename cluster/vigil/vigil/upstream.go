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
