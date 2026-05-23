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
