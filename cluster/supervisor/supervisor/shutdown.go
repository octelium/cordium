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

package supervisor

import (
	"context"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
)

func (s *Server) doShutdown() error {

	s.mu.Lock()
	if s.isShuttingDown {
		s.mu.Unlock()
		return nil
	}

	s.isShuttingDown = true
	s.mu.Unlock()
	s.ctxMainCancel()

	if s.isInner || ldflags.IsTest() {
		return s.doShutdownInner()
	}
	return s.doShutdownOuter()
}

func (s *Server) doShutdownOuter() error {
	if ldflags.IsTest() {
		return nil
	}
	zap.L().Debug("Starting outer shutdown")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	if err := s.podmanStopOuter(ctx); err != nil {
		zap.L().Warn("Outer podman exited with an error", zap.Error(err))
	}

	/*

		if err := s.unsetIPTablesRules(ctx); err != nil {
			zap.L().Error("Could not unset iptables rules", zap.Error(err))
		}
	*/

	if err := s.removeCgroupOuter(); err != nil {
		zap.L().Error("Could not remove outer cgroup", zap.Error(err))
	} else {
		zap.L().Debug("Successfully removed outer cgroup")
	}

	zap.L().Debug("Outer shutdown completed")

	return nil
}

func (s *Server) waitUntilRunningForShutdown(ctx context.Context) {
	// This fn is to avoid shutting down while the status is still in STARTING_RUNTIME
	// which can happen frequently for Build Workspaces that stops once initialization is complete
	zap.L().Debug("Starting waitUntilRunningForShutdown")

	switch s.getStatus() {
	case cordiumv1.Workspace_Status_RUNNING,
		cordiumv1.Workspace_Status_STOPPING_REQUEST,
		cordiumv1.Workspace_Status_STOPPING:
		return
	}

	tickerCh := time.NewTicker(300 * time.Millisecond)
	defer tickerCh.Stop()

	timeoutCh := time.NewTimer(20 * time.Second)
	defer timeoutCh.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeoutCh.C:
			zap.L().Debug("Timeout exceeded. Exiting waitUntilRunningForShutdown")
			return
		case <-tickerCh.C:
			st := s.getStatus()
			if st == cordiumv1.Workspace_Status_RUNNING {
				zap.L().Debug("Status is now RUNNING. Exiting wait loop")
				return
			}
			zap.L().Debug("Status is still not RUNNING. ")
		}
	}
}

func (s *Server) doShutdownInner() error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	s.waitUntilRunningForShutdown(ctx)
	lastStatus := s.getStatus()
	zap.L().Debug("Starting inner shutdown")
	s.setStatus(cordiumv1.Workspace_Status_STOPPING)

	cmds := []string{
		"podman stop workspace -t 60",
		// "rm -rf /octelium/podman/tmp",
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running inner shutdown cmd", zap.String("cmd", cmdStr))
		cmd := s.getCommandAsOctelium(ctx, cmdStr)

		if err := cmd.Run(); err != nil {
			zap.S().Errorf("Could not run shutdown cmd: %s: %+v", cmdStr, err)
		}
	}

	if s.octeliumProxy != nil {
		s.octeliumProxy.Close()
	}

	switch lastStatus {
	case cordiumv1.Workspace_Status_RUNNING,
		cordiumv1.Workspace_Status_STARTING_RUNTIME,
		cordiumv1.Workspace_Status_STOPPING_REQUEST,
		cordiumv1.Workspace_Status_STOPPING:
	default:
		zap.L().Debug("Last status is not RUNNING or Stopping. No need to save Workspace storage...",
			zap.String("lastStatus", lastStatus.String()))

		s.setStatus(cordiumv1.Workspace_Status_STOPPED)
		return nil
	}

	{
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmdStr := "podman inspect --type container workspace"
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		err := cmd.Run()
		if err != nil {
			zap.L().Warn("Could not inspect at shutdown", zap.Error(err))
		}
	}

	{
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmdStr := "podman container cleanup workspace"
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		err := cmd.Run()
		if err != nil {
			zap.L().Warn("Could not cleanup Workspace", zap.Error(err))
		}
	}

	{
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmdStr := "podman inspect --type container workspace"
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		err := cmd.Run()
		if err != nil {
			zap.L().Warn("Could not inspect at shutdown", zap.Error(err))
		}
	}

	s.setStatus(cordiumv1.Workspace_Status_STOPPED)

	zap.L().Debug("Inner shutdown completed")

	return nil
}
