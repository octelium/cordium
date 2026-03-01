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

package wrks

import (
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
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

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Name: req.Name,
		Uid:  req.Uid,
	}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return err
	}

	ws, supC, err := s.supClientMap.Get(&metav1.GetOptions{
		Name: req.Name,
		Uid:  req.Uid,
	}, i.Session.Status.UserRef)
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
