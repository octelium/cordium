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
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/rscserver/rscserver"
	"github.com/octelium/octelium/apis/main/authv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/main/userv1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/user"
	"github.com/octelium/octelium/cluster/common/clusterconfig"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/postgresutils"
	"github.com/octelium/octelium/cluster/common/sessionc"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/apiutils/ucorev1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/mains"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/wrks"

	"github.com/octelium/octelium/cluster/authserver/authserver"
)

func Run(ctx context.Context) error {
	{
		zapCfg := zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Development:      true,
			Encoding:         "console",
			EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}

		logger, err := zapCfg.Build()
		if err != nil {
			return err
		}

		zap.ReplaceGlobals(logger)
	}

	{
		dbName := fmt.Sprintf("octelium%s", utilrand.GetRandomStringLowercase(8))

		os.Setenv("OCTELIUM_POSTGRES_NOSSL", "true")

		os.Setenv("OCTELIUM_POSTGRES_HOST", "localhost")
		os.Setenv("OCTELIUM_POSTGRES_USERNAME", "postgres")
		os.Setenv("OCTELIUM_POSTGRES_PASSWORD", "postgres")
		os.Setenv("OCTELIUM_TEST_RSCSERVER_PORT", fmt.Sprintf("%d", 10001))

		ldflags.PrivateRegistry = "false"
		ldflags.Mode = "production"
		ldflags.TestMode = "true"

		{
			db, err := postgresutils.NewDBWithNODB()
			if err != nil {
				return err
			}
			if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s;", dbName)); err != nil {
				return err
			}
			if err := db.Close(); err != nil {
				return err
			}
		}

		zap.S().Debugf("Starting new rsc server")

		os.Setenv("OCTELIUM_POSTGRES_DATABASE", dbName)

		rscSrv, err := rscserver.NewServer(ctx)
		if err != nil {
			return err
		}

		zap.S().Debugf("Running rsc server")
		err = rscSrv.Run(ctx)
		if err != nil {
			return err
		}

		time.Sleep(5 * time.Second)

		{
			clusterCfg := &corev1.ClusterConfig{
				ApiVersion: "cluster/v1",
				Kind:       "ClusterConfig",
				Metadata: &metav1.Metadata{
					Uid:             uuid.New().String(),
					ResourceVersion: uuid.New().String(),
					Name:            "default",
				},
				Spec: &corev1.ClusterConfig_Spec{},
				Status: &corev1.ClusterConfig_Status{
					Domain:  "example.com",
					Network: &corev1.ClusterConfig_Status_Network{},
				},
			}

			v6Prefix, err := utilrand.GetRandomBytes(4)
			if err != nil {
				return err
			}

			clusterCfg.Status.Network.V6RangePrefix = v6Prefix

			if err := clusterconfig.SetClusterSubnets(clusterCfg); err != nil {
				return err
			}

			_, err = rscSrv.CreateResource(ctx, clusterCfg, ucorev1.API, ucorev1.Version, ucorev1.KindClusterConfig)
			if err != nil {
				return err
			}
		}

		{
			clusterCfg := &cordiumv1.ClusterConfig{
				ApiVersion: "cordium/v1",
				Kind:       "ClusterConfig",
				Metadata: &metav1.Metadata{
					Uid:             uuid.New().String(),
					ResourceVersion: uuid.New().String(),
					Name:            "default",
				},
				Spec:   &cordiumv1.ClusterConfig_Spec{},
				Status: &cordiumv1.ClusterConfig_Status{},
			}

			_, err = rscSrv.CreateResource(ctx, clusterCfg, "cordium", "v1", "ClusterConfig")
			if err != nil {
				return err
			}
		}
	}

	zap.S().Debug("Starting octelium API server...")

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", "localhost:10000")
	if err != nil {
		return err
	}

	srv := admin.NewServer(&admin.Opts{
		OcteliumC: octeliumC,
	})
	usrSrv := user.NewServer(octeliumC)

	if err := genResources(ctx, octeliumC); err != nil {
		return err
	}

	authSrv, err := authserver.GetAuthGRPCServer(ctx, octeliumC)
	if err != nil {
		return err
	}

	zap.S().Debug("starting gRPC server...")

	mdlwr, err := newMiddleware(ctx, octeliumC)
	if err != nil {
		return err
	}

	s := grpc.NewServer(
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(mdlwr.StreamServerInterceptor())),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(mdlwr.UnaryServerInterceptor())),
	)

	corev1.RegisterMainServiceServer(s, srv)
	userv1.RegisterMainServiceServer(s, usrSrv)
	authv1.RegisterMainServiceServer(s, authSrv)

	{
		mainSrv, err := mains.NewServer(ctx, octeliumC)
		if err != nil {
			return err
		}
		cordiumv1.RegisterMainServiceServer(s, mainSrv)

		wrkSrv, err := wrks.NewServer(ctx, octeliumC)
		if err != nil {
			return err
		}
		cordiumv1.RegisterWorkspaceServiceServer(s, wrkSrv)

		if err := wrkSrv.Run(ctx); err != nil {
			return err
		}
	}

	go func() {
		zap.S().Debug("running gRPC server.")
		if err := s.Serve(lis); err != nil {
			zap.S().Infof("gRPC server closed: %+v", err)
		}
	}()

	go func() {

		zap.L().Debug("starting grpcWeb server")

		grpcWebSrv := &grpcWebSrv{
			srv: grpcweb.WrapServer(s),
		}

		srv := &http.Server{
			Handler: grpcWebSrv,
			Addr:    "127.0.0.1:10003",
		}

		if err := srv.ListenAndServe(); err != nil {
			zap.L().Fatal("Could not serve grpcWeb server", zap.Error(err))
		}
	}()

	/*
		go func() error {
			lis, err := net.Listen("tcp", "localhost:8090")
			if err != nil {
				return err
			}
			grpcSrv := grpc.NewServer()
			grpc_health_v1.RegisterHealthServer(grpcSrv, healthcheck.NewServer())
			if err := grpcSrv.Serve(lis); err != nil {
				zap.S().Infof("gRPC health check server closed: %+v", err)
			}
			return nil
		}()
	*/

	// time.Sleep(5 * time.Second)

	zap.L().Info("Mock API Server is now running")
	<-ctx.Done()
	zap.L().Debug("Shutting down gRPC server")
	s.Stop()

	return nil
}

type grpcWebSrv struct {
	srv *grpcweb.WrappedGrpcServer
}

func (s *grpcWebSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	/*
		zap.L().Debug("New req====",
			zap.String("proto", r.Proto),
			zap.String("path", r.URL.Path),
			zap.String("scheme", r.URL.Scheme),
			zap.String("host", r.Host),
			zap.Any("hdrs", r.Header),
			zap.Bool("isGRPC", s.srv.IsGrpcWebRequest(r)),
		)
	*/
	s.srv.ServeHTTP(w, r)
}

type Middleware struct {
	usr  *corev1.User
	sess *corev1.Session
	dev  *corev1.Device

	accessToken  string
	refreshToken string
}

func newMiddleware(ctx context.Context, octeliumC octeliumc.ClientInterface) (*Middleware, error) {

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC: octeliumC,
	})

	usr, err := adminSrv.CreateUser(ctx, &corev1.User{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &corev1.User_Spec{
			Type:  corev1.User_Spec_HUMAN,
			Email: "george@octelium.com",
		},
	})
	if err != nil {
		return nil, err
	}

	dev, err := octeliumC.CoreC().CreateDevice(ctx, &corev1.Device{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(6),
		},
		Spec: &corev1.Device_Spec{
			State: corev1.Device_Spec_ACTIVE,
		},
		Status: &corev1.Device_Status{
			UserRef: umetav1.GetObjectReference(usr),
			OsType:  corev1.Device_Status_LINUX,
		},
	})
	if err != nil {
		return nil, err
	}

	sess, err := sessionc.CreateSession(ctx, &sessionc.CreateSessionOpts{
		Usr:       usr,
		Device:    dev,
		OcteliumC: octeliumC,
		SessType:  corev1.Session_Status_CLIENTLESS,
		IsBrowser: true,
	})
	if err != nil {
		return nil, err
	}

	jwkCtl, err := jwkctl.NewJWKController(ctx, octeliumC)
	if err != nil {
		return nil, err
	}

	accessToken, err := jwkCtl.CreateAccessToken(sess)
	if err != nil {
		return nil, err
	}

	refreshToken, err := jwkCtl.CreateRefreshToken(sess)
	if err != nil {
		return nil, err
	}

	return &Middleware{
		usr:  usr,
		dev:  dev,
		sess: sess,

		accessToken:  accessToken,
		refreshToken: refreshToken,
	}, nil

}

func (m *Middleware) getDownstream(ctx context.Context) (*userctx.UserCtx, error) {

	return &userctx.UserCtx{
		User:    m.usr,
		Session: m.sess,
		Groups:  nil,
		Device:  m.dev,
	}, nil
}

func (m *Middleware) UnaryServerInterceptor() grpc.UnaryServerInterceptor {

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		zap.L().Debug("New req", zap.String("fullMethod", info.FullMethod))
		i, err := m.getDownstream(ctx)
		if err != nil {
			zap.S().Debugf("Could not authenticate User: %+v", err)
			return nil, status.Errorf(codes.Unauthenticated, "Could not authenticate User")
		}

		newCtx := context.WithValue(ctx, "octelium-user-ctx", i)
		newCtx = context.WithValue(newCtx, "x-octelium-auth", m.accessToken)
		newCtx = context.WithValue(newCtx, "x-octelium-refresh-token", m.refreshToken)

		md, _ := metadata.FromIncomingContext(newCtx)

		md["x-octelium-refresh-token"] = []string{
			m.refreshToken,
		}
		md["x-octelium-auth"] = []string{
			m.accessToken,
		}

		newCtx = metadata.NewIncomingContext(newCtx, md)

		// zap.L().Debug("MD", zap.Any("md", md))

		return handler(newCtx, req)
	}
}

func (m *Middleware) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

		ctx := stream.Context()

		i, err := m.getDownstream(ctx)
		if err != nil {
			zap.S().Debugf("Could not authenticate User: %+v", err)
			return status.Errorf(codes.Unauthenticated, "Could not authenticate User")
		}

		newCtx := context.WithValue(ctx, "octelium-user-ctx", i)
		newCtx = context.WithValue(newCtx, "x-octelium-auth", m.accessToken)
		newCtx = context.WithValue(newCtx, "x-octelium-refresh-token", m.refreshToken)

		// md, _ := metadata.FromIncomingContext(newCtx)

		// zap.L().Debug("MD", zap.Any("md", md))

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx

		return handler(srv, wrapped)
	}
}
