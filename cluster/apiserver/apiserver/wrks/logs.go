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

package wrks

import (
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
)

func (s *Server) ListenLog(req *cordiumv1.ListenLogRequest, srv cordiumv1.WorkspaceService_ListenLogServer) error {
	ctx := srv.Context()

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return err
	}

	if err := apivalidation.CheckObjectRef(req.WorkspaceRef, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return err
	}

	ws, supC, err := s.supClientMap.Get(
		apivalidation.ObjectReferenceToGetOptions(req.WorkspaceRef),
		i.Session.Status.UserRef)
	if err != nil {
		return err
	}

	zap.L().Debug("Starting a ListenLog request", zap.String("name", ws.Metadata.Name))

	if ucordiumv1.ToWorkspace(ws).IsInitializing() ||
		ucordiumv1.ToWorkspace(ws).IsStoppingOrStopped() {
		return grpcutils.InvalidArg("Workspace is not ready")
	}

	streamC, err := supC.C().ListenEvent(ctx, &ccordiumv1.ListenEventRequest{})
	if err != nil {
		return grpcutils.InternalWithErr(err)
	}

	defer zap.L().Debug("Exiting ListenLog loop", zap.String("name", ws.Metadata.Name))

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, err := streamC.Recv()
			if err != nil {
				if grpcerr.IsCanceled(err) {
					return nil
				}
				return grpcutils.InternalWithErr(err)
			}
			switch msg.Type.(type) {
			case *ccordiumv1.ListenEventResponse_ListenLogResponse:
				if err := srv.Send(msg.GetListenLogResponse()); err != nil {
					return err
				}
			}
		}
	}
}
