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

package apiserver

import (
	"context"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/mains"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/mans"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/wrks"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/commoninit"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/healthcheck"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const maxRecvMsgSize = 8 * 1024 * 1024

func Run(ctx context.Context) error {

	ctx, cancelFn := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	if ldflags.IsDev() {
		os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
		os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
	}

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

	recoveryOpts := []grpc_recovery.Option{
		grpc_recovery.WithRecoveryHandler(func(p any) error {
			zap.L().Error("Recovered from panic in gRPC handler",
				zap.Any("panic", p), zap.ByteString("stack", debug.Stack()))
			return grpcutils.Internal("Internal error")
		}),
	}

	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxRecvMsgSize),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 30 * time.Minute,
			Time:              2 * time.Minute,
			Timeout:           20 * time.Second,
		}),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				grpc_recovery.StreamServerInterceptor(recoveryOpts...),
				mdlwr.StreamServerInterceptor())),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				grpc_recovery.UnaryServerInterceptor(recoveryOpts...),
				mdlwr.UnaryServerInterceptor())),
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

	healthcheck.Run(vutils.HealthCheckPortManagedService)

	zap.L().Info("Cordium API Server is now running")
	<-ctx.Done()
	zap.L().Debug("Shutting down gRPC server")

	gracefulShutdownCh := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(gracefulShutdownCh)
	}()

	select {
	case <-gracefulShutdownCh:
		zap.L().Debug("gRPC server gracefully stopped")
	case <-time.After(15 * time.Second):
		zap.L().Warn("gRPC server graceful shutdown timeout exceeded")
		s.Stop()
	}

	return nil
}
