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

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"go.uber.org/zap"
)

func (s *Server) OnWorkspaceCreate(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Status.UserRef == nil {
		return nil
	}

	if err := s.supClientMap.Set(ws); err != nil {
		return err
	}

	return nil
}

func (s *Server) OnWorkspaceUpdate(ctx context.Context, new, old *cordiumv1.Workspace) error {
	if new.Status.UserRef == nil {
		return nil
	}

	if err := s.supClientMap.Set(new); err != nil {
		return err
	}

	if err := s.onWorkspaceStop(ctx, new, old); err != nil {
		zap.L().Warn("Could not do onWorkspaceStop", zap.Error(err),
			zap.String("wsUID", new.Metadata.Uid))
	}

	/*
		if new.Status.State == old.Status.State {
			zap.L().Debug("Workspace state has not changed. Nothing to be published", zap.String("uid", new.Metadata.Uid))
			return nil
		}
	*/

	s.dctxMap.mu.RLock()
	defer s.dctxMap.mu.RUnlock()

	for _, dctx := range s.dctxMap.dctxMap {
		if err := s.doSendWorkspaceUpdateMessage(ctx, dctx, new); err != nil {
			zap.L().Error("Could not send Workspace update msg", zap.Error(err))
		}
	}

	return nil
}

func (s *Server) onWorkspaceStop(ctx context.Context, new, old *cordiumv1.Workspace) error {
	if ucordiumv1.ToWorkspace(new).IsStoppingOrStopped() &&
		!ucordiumv1.ToWorkspace(old).IsStoppingOrStopped() {
		if err := s.tunnelSrv.remove(new); err != nil {
			return err
		}

		if err := s.removeTerminalListeners(new); err != nil {
			return err
		}

	}

	return nil
}

func (s *Server) OnWorkspaceDelete(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Status.UserRef == nil {
		return nil
	}

	err := s.tunnelSrv.remove(ws)
	if err != nil {
		zap.L().Warn("Could not remove tun ctx", zap.Error(err))
	}

	if err := s.supClientMap.Remove(ws); err != nil {
		return err
	}

	if err := s.removeTerminalListeners(ws); err != nil {
		return err
	}

	return err
}

func (s *Server) removeTerminalListeners(ws *cordiumv1.Workspace) error {
	if ws.Status.UserRef == nil {
		return nil
	}

	s.dctxMap.mu.Lock()
	defer s.dctxMap.mu.Unlock()

	for _, dctx := range s.dctxMap.dctxMap {
		if dctx.usrRef.Uid == ws.Status.UserRef.Uid {

			if err := dctx.removeTerminalsByWorkspaceUID(ws.Metadata.Uid); err != nil {
				zap.L().Warn("Could not removeTerminalsByWorkspaceUID", zap.Error(err))
			}

		}
	}

	return nil
}
