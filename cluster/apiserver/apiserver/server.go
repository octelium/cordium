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

package apiserver

import (
	"context"
	"net"
	"os"
	"os/signal"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/mains"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/mans"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/wrks"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/commoninit"
	"github.com/octelium/octelium/cluster/common/healthcheck"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func Run(ctx context.Context) error {

	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancelFn()

	if ldflags.IsDev() {
		os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
		os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
	}

	healthcheck.Run(vutils.HealthCheckPortManagedService)

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}

	if err := commoninit.Run(ctx, nil); err != nil {
		return err
	}

	lis, err := net.Listen("tcp", vutils.ManagedServiceAddr)
	if err != nil {
		return err
	}

	mainSrv, err := mains.NewServer(ctx, octeliumC)
	if err != nil {
		return err
	}

	manSrv, err := mans.NewServer(ctx, octeliumC)
	if err != nil {
		return err
	}

	workspaceSrv, err := wrks.NewServer(ctx, octeliumC)
	if err != nil {
		return err
	}

	if err := mainSrv.Run(ctx); err != nil {
		return err
	}

	if err := workspaceSrv.Run(ctx); err != nil {
		return err
	}

	zap.S().Debug("starting gRPC server...")

	mdlwr, err := userctx.New(ctx, octeliumC)
	if err != nil {
		return err
	}

	s := grpc.NewServer(
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(mdlwr.StreamServerInterceptor())),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(mdlwr.UnaryServerInterceptor())),
	)

	cordiumv1.RegisterMainServiceServer(s, mainSrv)
	cordiumv1.RegisterWorkspaceServiceServer(s, workspaceSrv)
	cordiumv1.RegisterManagementServiceServer(s, manSrv)

	go func() {
		zap.S().Debug("running gRPC server.")
		if err := s.Serve(lis); err != nil {
			zap.S().Infof("gRPC server closed: %+v", err)
		}
	}()

	zap.L().Info("Cordium API Server is now running")
	<-ctx.Done()
	zap.L().Debug("Shutting down gRPC server")
	s.Stop()

	return nil
}
