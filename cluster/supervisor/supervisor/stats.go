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
	"time"

	"go.uber.org/zap"
)

func (s *Server) runStatsLoop() {
	zap.L().Debug("Starting running stats loop for Workspace container")
	tickerCh := time.NewTicker(60 * time.Second)
	defer tickerCh.Stop()
	defer zap.L().Debug("statsLoop exited...")

	ctx := s.ctxMain
	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerCh.C:
			resp, err := s.getContainerStats(ctx)
			if err != nil {
				zap.L().Debug("Could not get Workspace container stats", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			if len(resp) != 1 {
				continue
			}

			stats := resp[0]
			zap.L().Debug("Current container stats",
				zap.Any("stats", stats))
		}
	}
}
