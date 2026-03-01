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
	"errors"
	"io"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"go.uber.org/zap"
)

func (s *Server) Shutdown(ctx context.Context, req *ccordiumv1.ShutdownRequest) (*ccordiumv1.ShutdownResponse, error) {

	zap.L().Debug("Shutdown requested by API")

	s.mu.Lock()
	isShuttingDown := s.isShuttingDown
	s.mu.Unlock()
	if isShuttingDown {
		zap.L().Debug("Already shutting down. No need to signal apiShutdownCh")
		return &ccordiumv1.ShutdownResponse{}, nil
	}

	s.shutdownReq = req

	s.apiShutdownCh <- struct{}{}

	return &ccordiumv1.ShutdownResponse{}, nil
}

func (s *Server) ShutdownAck(ctx context.Context, req *ccordiumv1.ShutdownAckRequest) (*ccordiumv1.ShutdownAckResponse, error) {

	zap.L().Debug("Shutdown ack requested by API")

	s.shutdownAckCh <- struct{}{}

	return &ccordiumv1.ShutdownAckResponse{}, nil
}

func (s *Server) Initialize(ctx context.Context, req *ccordiumv1.InitializeRequest) (*ccordiumv1.InitializeResponse, error) {
	zap.L().Debug("Initialize requested", zap.Any("req", req))

	var isInitializeRequested bool
	s.mu.Lock()
	s.initReq = req
	isInitializeRequested = s.isInitializeRequested
	s.mu.Unlock()

	if isInitializeRequested {
		zap.L().Debug("Initialize already requested. Nothing to be done...")
		return &ccordiumv1.InitializeResponse{}, nil
	}

	s.mu.Lock()
	s.isInitializeRequested = true
	s.mu.Unlock()

	go s.initialize()

	return &ccordiumv1.InitializeResponse{}, nil
}

func (s *Server) initialize() {
	err := s.doInitialize()
	if err == nil {
		zap.L().Debug("successfully done doInitialize")
		return
	}

	zap.L().Error("Could not doInitialize", zap.Error(err))

	if failure := s.getFailure(); failure != nil {
		zap.L().Debug("Failure was already set", zap.Any("failure", failure))
	} else {
		zap.L().Debug("Failure is not set. Guessing one...")
		if errors.Is(err, context.DeadlineExceeded) {
			s.setFailure(&cordiumv1.Workspace_Status_Failure{
				Type: &cordiumv1.Workspace_Status_Failure_StartupTimeoutExceeded_{
					StartupTimeoutExceeded: &cordiumv1.Workspace_Status_Failure_StartupTimeoutExceeded{},
				},
			})
		} else {
			s.setFailure(&cordiumv1.Workspace_Status_Failure{
				Type: &cordiumv1.Workspace_Status_Failure_StartupUnknown_{
					StartupUnknown: &cordiumv1.Workspace_Status_Failure_StartupUnknown{},
				},
			})
		}
	}

	s.initializationCh <- err
}

func (s *Server) GetTunnelPublicKey(ctx context.Context, req *ccordiumv1.GetTunnelPublicKeyRequest) (*ccordiumv1.GetTunnelPublicKeyResponse, error) {

	return &ccordiumv1.GetTunnelPublicKeyResponse{
		PublicKey: s.wgPublicKey,
	}, nil

}

func (s *Server) GetState(ctx context.Context, req *ccordiumv1.GetStateRequest) (*ccordiumv1.GetStateResponse, error) {
	if ldflags.IsTest() {
		return &ccordiumv1.GetStateResponse{
			State: cordiumv1.Workspace_Status_RUNNING,
		}, nil
	}

	status := s.getStatus()

	zap.L().Debug("Status requested", zap.String("status", status.String()))

	return &ccordiumv1.GetStateResponse{
		State: status,
	}, nil
}

func (s *Server) CreateTerminal(ctx context.Context, req *ccordiumv1.CreateTerminalRequest) (*ccordiumv1.CreateTerminalResponse, error) {
	return s.termC.CreateTerminal(ctx, req)
}

func (s *Server) RemoveTerminal(ctx context.Context, req *ccordiumv1.RemoveTerminalRequest) (*ccordiumv1.RemoveTerminalResponse, error) {
	return s.termC.RemoveTerminal(ctx, req)
}

func (s *Server) ListTerminal(ctx context.Context, req *ccordiumv1.ListTerminalRequest) (*ccordiumv1.ListTerminalResponse, error) {
	return s.termC.ListTerminal(ctx, req)
}

func (s *Server) ListenTerminal(req *ccordiumv1.ListenTerminalRequest, srv ccordiumv1.TerminalService_ListenTerminalServer) error {

	ctx := srv.Context()
	strm, err := s.termC.ListenTerminal(ctx, req)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("ctx done. Exiting ListenTerminal loop")
			return nil
		default:
			msg, err := strm.Recv()
			if err != nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			srv.Send(msg)

		}
	}

}

func (s *Server) WriteDataTerminal(ctx context.Context, req *ccordiumv1.WriteDataTerminalRequest) (*ccordiumv1.WriteDataTerminalResponse, error) {
	return s.termC.WriteDataTerminal(ctx, req)
}

func (s *Server) SetWindowSize(ctx context.Context, req *ccordiumv1.SetWindowSizeRequest) (*ccordiumv1.SetWindowSizeResponse, error) {
	return s.termC.SetWindowSize(ctx, req)
}

func (s *Server) ListenState(req *ccordiumv1.ListenStateRequest, srv ccordiumv1.WorkspaceSupervisorService_ListenStateServer) error {

	ctx := srv.Context()

	zap.L().Debug("ListenState request")

	initStatus := s.getStatus()

	zap.L().Debug("Sending init status in ListenState", zap.String("status", initStatus.String()))
	if err := srv.Send(&ccordiumv1.ListenStateResponse{
		State: initStatus,
	}); err != nil {
		return err
	}

	sub := s.subscribeStatusListener()

	defer s.unsubscribeStatusListener(sub)

	zap.L().Debug("Starting ListenState loop")

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting ListenState. ctx done")
			return nil
		case status, ok := <-sub.statusCh:
			if !ok {
				return nil
			}

			zap.L().Debug("Sending status on change", zap.String("state", status.String()))
			if err := srv.Send(&ccordiumv1.ListenStateResponse{
				State: status,
			}); err != nil {
				return err
			}
		}
	}

}

type statusSubscription struct {
	id       string
	statusCh chan cordiumv1.Workspace_Status_State
}

func (s *Server) subscribeStatusListener() *statusSubscription {
	statusSubscription := &statusSubscription{
		id:       utilrand.GetRandomString(12),
		statusCh: make(chan cordiumv1.Workspace_Status_State, 1000),
	}
	s.statusSubscribersMap.mu.Lock()
	defer s.statusSubscribersMap.mu.Unlock()
	s.statusSubscribersMap.subscribersMap[statusSubscription.id] = statusSubscription

	return statusSubscription
}

func (s *Server) unsubscribeStatusListener(sub *statusSubscription) {

	s.statusSubscribersMap.mu.Lock()
	defer s.statusSubscribersMap.mu.Unlock()
	delete(s.statusSubscribersMap.subscribersMap, sub.id)
}

func (s *Server) GetFailure(ctx context.Context, req *ccordiumv1.GetFailureRequest) (*ccordiumv1.GetFailureResponse, error) {
	s.failureWrp.mu.RLock()
	defer s.failureWrp.mu.RUnlock()
	return &ccordiumv1.GetFailureResponse{
		Failure: s.failureWrp.failure,
	}, nil
}

func (s *Server) ListenEvent(req *ccordiumv1.ListenEventRequest, srv ccordiumv1.WorkspaceSupervisorService_ListenEventServer) error {

	ctx := srv.Context()

	zap.L().Debug("Starting ListenEvent loop on WorkspaceSup")

	sub := s.eventPublisher.subscribe()
	defer s.eventPublisher.unsubscribe(sub.id)

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting ListenEvent. ctx done")
			return nil
		case msg, ok := <-sub.resp:
			if !ok {
				zap.L().Debug("Exiting ListenTerminal. Subscription ended")
				return nil
			}

			if err := srv.Send(msg); err != nil {
				zap.L().Error("Could not send ListenEventResp",
					zap.Error(err))
			}
		}
	}

}

func (s *Server) Exec(srv cordiumv1.WorkspaceService_ExecServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	zap.L().Debug("Starting exec request")

	upstream, err := s.termC.Exec(ctx, grpc_retry.Disable())
	if err != nil {
		return err
	}

	errCh := make(chan error, 2)

	go func() {
		defer upstream.CloseSend()
		zap.L().Debug("Starting downstream-upstream loop")
		defer zap.L().Debug("Exiting downstream-upstream loop")
		for {
			msg, err := srv.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errCh <- err
				} else {
					errCh <- nil
				}
				return
			}
			if err := upstream.Send(msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		zap.L().Debug("Starting upstream-downstream loop")
		defer zap.L().Debug("Exiting upstream-downstream loop")
		for {
			msg, err := upstream.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errCh <- err
				} else {
					errCh <- nil
				}
				return
			}
			if err := srv.Send(msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	zap.L().Debug("Waiting for first terminal event")
	err = <-errCh
	zap.L().Debug("Exec done, shutting down", zap.Error(err))
	return err
}
