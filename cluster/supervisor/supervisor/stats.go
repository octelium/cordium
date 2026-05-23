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
