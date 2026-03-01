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
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func (s *Server) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {

	// zap.S().Debugf("initializing health check")

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (s *Server) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	return nil
}

func (s *Server) runHealthCheckLoop() {
	zap.L().Debug("Starting runHealthCheckLoop")
	tickerCh := time.NewTicker(30 * time.Second)
	defer tickerCh.Stop()
	defer zap.L().Debug("runHealthCheckLoop exited...")

	errN := 0
	maxErrs := 20

	ctx := s.ctxMain

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerCh.C:
			err := s.doHealthCheck()
			if err == nil {
				errN = 0
			} else {
				zap.L().Warn("Workspace healthCheck error", zap.Error(err))
				errN = errN + 1
				if errN > maxErrs {
					zap.L().Warn("HealthCheck erroneous attempts exceeded",
						zap.Error(err))
					s.setFailure(&cordiumv1.Workspace_Status_Failure{
						Type: &cordiumv1.Workspace_Status_Failure_HealthCheck_{
							HealthCheck: &cordiumv1.Workspace_Status_Failure_HealthCheck{},
						},
					})
					s.healthCheckCh <- err
					return
				}
			}

		}
	}
}

func (s *Server) doHealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := s.healthCheckC.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return errors.Errorf("The Workspace server is not ready")
	}

	return nil
}
