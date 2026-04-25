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
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	cfs "github.com/containerd/continuity/fs"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/cluster/common/wsclient"
	"github.com/octelium/cordium/cluster/supervisor/supervisor/oproxy"
	"github.com/octelium/cordium/cluster/supervisor/supervisor/sshagent"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	mu sync.Mutex

	ctxMain       context.Context
	ctxMainCancel context.CancelFunc

	wsC          ccordiumv1.WorkspaceServiceClient
	termC        ccordiumv1.TerminalServiceClient
	healthCheckC grpc_health_v1.HealthClient
	ccordiumv1.UnimplementedWorkspaceSupervisorServiceServer
	ccordiumv1.UnimplementedTerminalServiceServer
	grpc_health_v1.UnimplementedHealthServer

	status struct {
		mu     sync.RWMutex
		status cordiumv1.Workspace_Status_State
	}

	statusSubscribersMap struct {
		mu             sync.RWMutex
		subscribersMap map[string]*statusSubscription
	}

	initReq *ccordiumv1.InitializeRequest

	lis     net.Listener
	grpcSrv *grpc.Server

	runCmd *exec.Cmd

	isShuttingDown bool

	isInitializeRequested bool

	wsAgentExitCh     chan error
	apiShutdownCh     chan struct{}
	initializationCh  chan error
	storageExceededCh chan struct{}

	innerContainerCh chan struct{}
	healthCheckCh    chan error
	shutdownAckCh    chan struct{}

	octeliumUID int
	octeliumGID int

	buildDir        string
	buildContextDir string

	cpToContainer    []*cpInfo
	doNotOverrideCmd bool

	spec *cordiumv1.Workspace_Spec

	containerInitProcess bool

	octeliumProxy *oproxy.OcteliumProxy
	sshAgent      *sshagent.Agent

	// fromBuild  bool
	isFreshRun bool
	wsUID      string

	failureWrp struct {
		mu      sync.RWMutex
		failure *cordiumv1.Workspace_Status_Failure
	}

	eventPublisher *eventPublisher

	cgSuffix string

	myCgroup     string
	myPID        int
	isInner      bool
	wgPrivateKey *wgtypes.Key
	wgPublicKey  string

	shutdownReq *ccordiumv1.ShutdownRequest

	mountBinaries []string
}

func NewServer(ctx context.Context) (*Server, error) {

	ctx, cancel := context.WithCancel(ctx)

	ret := &Server{

		ctxMain:       ctx,
		ctxMainCancel: cancel,

		apiShutdownCh:     make(chan struct{}, 10),
		storageExceededCh: make(chan struct{}, 10),
		innerContainerCh:  make(chan struct{}, 10),
		shutdownAckCh:     make(chan struct{}, 10),
		wsAgentExitCh:     make(chan error, 10),
		initializationCh:  make(chan error, 10),
		healthCheckCh:     make(chan error, 10),
		buildDir:          "/octelium/build",
		// envFilePath:      "/home/octelium/octelium-env",
		buildContextDir: ".",
		eventPublisher: &eventPublisher{
			subMap: make(map[string]*eventSubscription),
		},
		cgSuffix: utilrand.GetRandomStringLowercase(6),
		myPID:    os.Getpid(),
		isInner:  os.Getenv("OCTELIUM_RUN_LAYER") == "INNER0",
	}

	if ret.isInner {
		zap.L().Debug("Initializing a new Server in inner mode")
	} else {
		zap.L().Debug("Initializing a new Server in outer mode")
	}

	zap.L().Debug("Env vars", zap.Strings("env", os.Environ()))

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	zap.L().Debug("Current user", zap.Any("user", usr))

	if ret.isInner {
		usr, err := user.Lookup("octelium")
		if err != nil {
			return nil, errors.Errorf("Could not find octelium user in inner mode: %+v", err)
		}

		uid, err := strconv.Atoi(usr.Uid)
		if err != nil {
			return nil, err
		}
		gid, err := strconv.Atoi(usr.Gid)
		if err != nil {
			return nil, err
		}

		ret.octeliumUID = uid
		ret.octeliumGID = gid
	} else {
		// choose a uid that's a multiple of 2^16 in order to prevent uid mapping conflicts on the same machine
		// uid := getUserIndex() * int(math.Pow(2, 16))

		uid := 543210

		ret.octeliumUID = uid
		ret.octeliumGID = uid
	}

	zap.L().Debug("octelium user uid", zap.Int("uid", ret.octeliumUID))

	ret.statusSubscribersMap.subscribersMap = make(map[string]*statusSubscription)

	ret.mountBinaries = []string{
		"/bin/cordium-workspace",
		"/bin/octelium",
		"/bin/cordium-git-cred-helper",
		"/bin/octeliumctl",
		"/bin/cordium",
	}

	return ret, nil
}

func (s *Server) Run(ctx context.Context) error {

	if err := s.runPreInitCommands(ctx); err != nil {
		return errors.Errorf("Could not run init commands: %+v", err)
	}

	if s.isInner || ldflags.IsTest() {
		return s.doRunInner(ctx)
	}
	return s.doRunOuter(ctx)
}

func (s *Server) doRunOuter(ctx context.Context) error {

	zap.L().Debug("Starting running outer")
	if err := s.prepareCgroupsOuter(ctx); err != nil {
		return errors.Errorf("Could not prepare outer cgroup: %+v", err)
	}
	if err := s.runOuterPodman(ctx); err != nil {
		return errors.Errorf("Could not run outer podman: %+v", err)
	}

	zap.L().Debug("Successfully done running outer")

	return nil
}
func (s *Server) doRunInner(ctx context.Context) error {

	port := suputils.GetWorkspaceSupPort()
	zap.L().Debug("Starting running Workspace sup", zap.Int("listenPort", port))

	if err := func() error {
		// A stupid fix for now because in tests we get "the port is in use" error.
		var err error
		for i := 0; i < 100; i++ {
			s.lis, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				return nil
			}
			zap.L().Debug("Could not listen to WorkspaceSup port. Trying again...",
				zap.Error(err), zap.Int("listenPort", port))
			time.Sleep(1 * time.Second)
		}
		return errors.Errorf("Could not listen to WorkspaceSup port")
	}(); err != nil {
		return err
	}

	s.grpcSrv = grpc.NewServer(
		grpc.MaxConcurrentStreams(1000000),
	)

	ccordiumv1.RegisterWorkspaceSupervisorServiceServer(s.grpcSrv, s)
	ccordiumv1.RegisterTerminalServiceServer(s.grpcSrv, s)
	grpc_health_v1.RegisterHealthServer(s.grpcSrv, s)

	s.status.status = cordiumv1.Workspace_Status_INITIALIZING

	go func() {

		zap.L().Debug("running the grpc server")

		err := s.grpcSrv.Serve(s.lis)
		zap.L().Debug("gRPC server exited...", zap.Error(err))
	}()

	go s.waitAndDoShutdownByAPI()

	zap.L().Debug("Inner supervisor is now running")

	return nil
}

func (s *Server) waitAndDoShutdownByAPI() {
	zap.L().Debug("Started waiting for a Shutdown signal by API")
	<-s.apiShutdownCh
	zap.L().Debug("Received a Shutdown API call")
	if err := s.doShutdown(); err != nil {
		zap.L().Error("Could not do shutdown received by API", zap.Error(err))
	}
}

func (s *Server) Close() error {
	zap.L().Debug("Closing Workspace sup")
	s.grpcSrv.Stop()
	s.lis.Close()

	if s.octeliumProxy != nil {
		s.octeliumProxy.Close()
	}

	zap.L().Debug("Workspace sup closed")

	return nil
}

func (s *Server) WaitForTerm(ctx context.Context) error {
	if s.isInner || ldflags.IsTest() {
		return s.waitForTermInner(ctx)
	}
	return s.waitForTermOuter(ctx)
}

func (s *Server) waitForTermInner(ctx context.Context) error {
	select {
	case <-ctx.Done():
		zap.L().Debug("Workspace supervisor received TERM signal", zap.Error(ctx.Err()))
	case <-s.storageExceededCh:
		zap.L().Debug("Maximum storage exceeded. Shutting down...")
	case err := <-s.healthCheckCh:
		zap.L().Debug("Health check error. Shutting down...", zap.Error(err))
	case err := <-s.wsAgentExitCh:
		zap.L().Debug("Workspace agent exited...", zap.Error(err))
	case err := <-s.initializationCh:
		if err != nil {
			zap.L().Debug("Workspace initialization err", zap.Error(err))
		}
	}

	if err := s.doShutdown(); err != nil {
		zap.L().Error("Could not doShutdown:", zap.Error(err))
	}

	zap.L().Debug("Waiting for shutdown ack signal")
	<-s.shutdownAckCh

	gracefulShutdownCh := make(chan struct{}, 10)

	go func() {
		s.grpcSrv.GracefulStop()
		gracefulShutdownCh <- struct{}{}
	}()

	select {
	case <-gracefulShutdownCh:
		zap.L().Debug("Supervisor gRPC server gracefully stopped")
	case <-time.After(2000 * time.Millisecond):
		zap.L().Debug("Supervisor gRPC server graceful shutdown timeout exceeded.")
	}
	zap.L().Debug("Exiting inner supervisor...")

	return nil
}

func (s *Server) waitForTermOuter(ctx context.Context) error {

	select {
	case <-ctx.Done():
		zap.L().Debug("Workspace supervisor received TERM signal in outer", zap.Error(ctx.Err()))
		if err := s.doShutdown(); err != nil {
			zap.L().Error("Could not doShutdown for outer supervisor", zap.Error(err))
		}
	case <-s.innerContainerCh:
		zap.L().Debug("Inner container exited...")
		if err := s.doShutdown(); err != nil {
			zap.L().Error("Could not doShutdown for outer supervisor", zap.Error(err))
		}

		ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancelFn()

		zap.L().Debug("Outer supervisor waiting for exit...")

		select {
		case <-ctx.Done():
			zap.L().Debug("Received term signal")
		case <-time.After(20 * time.Second):
			zap.L().Debug("Timeout exceeded. Exiting...")
		}
	}

	zap.L().Debug("Exiting outer supervisor...")

	return nil
}

func Run(_ context.Context) error {
	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	srv, err := NewServer(ctx)
	if err != nil {
		return errors.Errorf("Could not initialize Workspace supervisor: %+v", err)
	}

	if err := srv.Run(ctx); err != nil {
		return err
	}

	if err := srv.WaitForTerm(ctx); err != nil {
		return err
	}

	if ldflags.IsDev() {
		zap.L().Sync()
	}

	return nil
}

func getUserIndex() int {
	// since every pod gets a unique addr. We sue the default iface to get the index octet as
	// a unique id per node in order to avoid uidmap overlappings.
	mip, err := getDefaultIfaceAddr()
	if err != nil {
		return utilrand.GetRandomRangeMath(int(math.Pow(2, 12)), int(math.Pow(2, 13)-1))
	}
	lastOctet := int(mip.To4()[3])
	if lastOctet > 0 {
		zap.L().Debug("Got userIndex for uid from default interface index", zap.Int("index", lastOctet))
		return lastOctet
	}
	return utilrand.GetRandomRangeMath(int(math.Pow(2, 12)), int(math.Pow(2, 13)-1))
}

func (s *Server) getOcteliumDirSize(ctx context.Context) (int64, error) {

	usg, err := cfs.DiskUsage(ctx, "/octelium")
	if err != nil {
		return 0, err
	}

	return usg.Size, nil

}

func (s *Server) waitUntilWorkspaceAgentReady() error {

	zap.L().Debug("Initializing waitUntilWorkspaceAgentReady")

	tickerCh := time.NewTicker(500 * time.Millisecond)
	defer tickerCh.Stop()

	doCheck := func() error {
		grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{})
		if err != nil {
			return errors.Errorf("could not grpc dial: %+v", err)
		}

		s.wsC = ccordiumv1.NewWorkspaceServiceClient(grpcConn)
		s.termC = ccordiumv1.NewTerminalServiceClient(grpcConn)
		s.healthCheckC = grpc_health_v1.NewHealthClient(grpcConn)

		ctx, cancel := context.WithTimeout(s.ctxMain, 300*time.Millisecond)
		defer cancel()
		if resp, err := s.healthCheckC.Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err == nil &&
			resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
			zap.L().Debug("Workspace healthCheck passed and it is now serving...")
			return nil
		}

		grpcConn.Close()
		return err
	}

	if ldflags.IsTest() {
		return doCheck()
	}

	timeoutCh := time.NewTimer(5 * time.Minute)
	defer timeoutCh.Stop()

	for {
		select {
		case <-s.ctxMain.Done():
			return nil
		case <-tickerCh.C:
			if err := doCheck(); err == nil {
				zap.L().Debug("Workspace agent is now READY")
				return nil
			}
			zap.L().Debug("Workspace agent is still not ready...")
		case <-timeoutCh.C:
			return errors.Errorf("Timeout exceeded waiting for Workspace agent to be ready")
		}
	}

}
