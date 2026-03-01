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

package portal

import (
	"context"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/pkg/errors"
)

func (s *Server) doSendWorkspaceUpdateMessage(ctx context.Context, dctx *dctx, ws *cordiumv1.Workspace) error {
	if ws.Status.IsBuild || ws.Metadata.IsUserHidden || ws.Metadata.IsSystemHidden || ws.Status.UserRef == nil {
		return nil
	}

	if dctx.usrRef.Uid != ws.Status.UserRef.Uid {
		return nil
	}

	/*
		zap.L().Debug("Sending Workspace state",
			zap.String("state", ws.Status.State.String()), zap.String("dctxID", dctx.id))
	*/
	if err := dctx.sendMessageServer(&cordiumv1.ServerMessage{
		Type: &cordiumv1.ServerMessage_WorkspaceUpdate_{
			WorkspaceUpdate: &cordiumv1.ServerMessage_WorkspaceUpdate{
				Workspace: ws,
			},
		},
	}); err != nil {
		return errors.Errorf("Could not send Workspace state: %+v", err)
	}

	return nil

}
