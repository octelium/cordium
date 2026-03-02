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

package wrks

import (
	"context"
	"errors"
	"io"
	"strings"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"go.uber.org/zap"
)

func (s *Server) CreateTerminal(ctx context.Context, req *cordiumv1.CreateTerminalRequest) (*cordiumv1.CreateTerminalResponse, error) {

	if err := apivalidation.CheckObjectRef(req.WorkspaceRef, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	wssupClient, err := s.getSupC(ctx, apivalidation.ObjectReferenceToGetOptions(req.WorkspaceRef))
	if err != nil {
		return nil, err
	}

	resp, err := wssupClient.TermC().CreateTerminal(ctx, &ccordiumv1.CreateTerminalRequest{
		Cols: req.Cols,
		Rows: req.Rows,
	})
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	s.activityCtl.Set(wssupClient.GetUID())

	return &cordiumv1.CreateTerminalResponse{
		Id: resp.Id,
	}, nil
}

func (s *Server) RemoveTerminal(ctx context.Context, req *cordiumv1.RemoveTerminalRequest) (*cordiumv1.RemoveTerminalResponse, error) {
	wsName, err := s.getWSNameFromTerminalID(req.Id)
	if err != nil {
		return nil, err
	}

	wssupClient, err := s.getSupC(ctx, &metav1.GetOptions{
		Name: wsName,
	})
	if err != nil {
		return nil, err
	}

	_, err = wssupClient.TermC().RemoveTerminal(ctx, &ccordiumv1.RemoveTerminalRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &cordiumv1.RemoveTerminalResponse{}, nil
}

func (s *Server) ListTerminal(ctx context.Context, req *cordiumv1.ListTerminalRequest) (*cordiumv1.ListTerminalResponse, error) {

	if err := apivalidation.CheckObjectRef(req.WorkspaceRef, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}
	zap.L().Debug("Starting listTerminal request", zap.Any("req", req))
	wssupClient, err := s.getSupC(ctx, apivalidation.ObjectReferenceToGetOptions(req.WorkspaceRef))
	if err != nil {
		return nil, err
	}

	resp, err := wssupClient.TermC().ListTerminal(ctx, &ccordiumv1.ListTerminalRequest{})
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	ret := &cordiumv1.ListTerminalResponse{}

	for _, itm := range resp.Items {
		ret.Items = append(ret.Items, &cordiumv1.Terminal{
			Id: itm.Id,
		})
	}

	zap.L().Debug("Listed terminals", zap.Any("termList", ret))

	return ret, nil
}

func (s *Server) WriteTerminalData(ctx context.Context, req *cordiumv1.WriteTerminalDataRequest) (*cordiumv1.WriteTerminalDataResponse, error) {
	wsName, err := s.getWSNameFromTerminalID(req.Id)
	if err != nil {
		return nil, err
	}

	if len(req.Data) > 10000 {
		return nil, grpcutils.InvalidArg("Data size is too big")
	}

	wssupClient, err := s.getSupC(ctx, &metav1.GetOptions{
		Name: wsName,
	})
	if err != nil {
		return nil, err
	}

	_, err = wssupClient.TermC().WriteDataTerminal(ctx, &ccordiumv1.WriteDataTerminalRequest{
		Id:   req.Id,
		Data: req.Data,
	})
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	s.activityCtl.Set(wssupClient.GetUID())

	return &cordiumv1.WriteTerminalDataResponse{}, nil
}

func (s *Server) SetTerminalWindowSize(ctx context.Context, req *cordiumv1.SetTerminalWindowSizeRequest) (*cordiumv1.SetTerminalWindowSizeResponse, error) {
	wsName, err := s.getWSNameFromTerminalID(req.Id)
	if err != nil {
		return nil, err
	}

	wssupClient, err := s.getSupC(ctx, &metav1.GetOptions{
		Name: wsName,
	})
	if err != nil {
		return nil, err
	}

	_, err = wssupClient.TermC().SetWindowSize(ctx, &ccordiumv1.SetWindowSizeRequest{
		Id:   req.Id,
		Cols: req.Cols,
		Rows: req.Rows,
	})
	if err != nil {
		return nil, err
	}

	s.activityCtl.Set(wssupClient.GetUID())

	return &cordiumv1.SetTerminalWindowSizeResponse{}, nil
}

func (s *Server) ListenTerminal(req *cordiumv1.ListenTerminalRequest, srv cordiumv1.WorkspaceService_ListenTerminalServer) error {
	ctx := srv.Context()

	zap.L().Debug("Starting ListenTerminal request", zap.Any("req", req))

	wsName, err := s.getWSNameFromTerminalID(req.Id)
	if err != nil {
		return err
	}

	wssupClient, err := s.getSupC(ctx, &metav1.GetOptions{
		Name: wsName,
	})
	if err != nil {
		return err
	}

	strm, err := wssupClient.TermC().ListenTerminal(ctx, &ccordiumv1.ListenTerminalRequest{
		Id: req.Id,
	})
	if err != nil {
		return err
	}

	zap.L().Debug("Starting ListenTerminal loop", zap.Any("req", req))

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("ctx done. Exiting ListenTerminal loop")
			return nil
		default:
			msg, err := strm.Recv()
			if err != nil {
				return err
			}

			switch msg.Type.(type) {
			case *ccordiumv1.ListenTerminalResponse_Stdout_:
				srv.Send(&cordiumv1.ListenTerminalResponse{
					Type: &cordiumv1.ListenTerminalResponse_Stdout_{
						Stdout: &cordiumv1.ListenTerminalResponse_Stdout{
							Data: msg.GetStdout().Data,
						},
					},
				})
			case *ccordiumv1.ListenTerminalResponse_WindowSize_:
				srv.Send(&cordiumv1.ListenTerminalResponse{
					Type: &cordiumv1.ListenTerminalResponse_WindowSize_{
						WindowSize: &cordiumv1.ListenTerminalResponse_WindowSize{
							Cols: msg.GetWindowSize().Cols,
							Rows: msg.GetWindowSize().Rows,
						},
					},
				})
			case *ccordiumv1.ListenTerminalResponse_Close_:
				srv.Send(&cordiumv1.ListenTerminalResponse{
					Type: &cordiumv1.ListenTerminalResponse_Close_{
						Close: &cordiumv1.ListenTerminalResponse_Close{},
					},
				})
			}

		}
	}
}

func (s *Server) getSupC(ctx context.Context, req *metav1.GetOptions) (*suputils.WorkspaceSupClient, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(req, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	ws, supC, err := s.supClientMap.Get(req, i.Session.Status.UserRef)
	if err != nil {
		return nil, err
	}

	if !ucordiumv1.ToWorkspace(ws).IsPreparingOrRunning() {
		return nil, grpcutils.InvalidArg("The Workspace is not ready: %s", ws.Metadata.Name)
	}

	return supC, nil
}

func (s *Server) getWSNameFromTerminalID(arg string) (string, error) {
	if err := wsutils.CheckTerminalID(arg); err != nil {
		return "", err
	}

	args := strings.Split(arg, "-")
	if len(args) != 2 {
		return "", grpcutils.InvalidArg("Invalid Terminal ID")
	}

	return args[0], nil
}

func (s *Server) Exec(srv cordiumv1.WorkspaceService_ExecServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	zap.L().Debug("New doExec req")

	msg, err := srv.Recv()
	if err != nil {
		return err
	}

	if msg.GetRequest() == nil {
		return grpcutils.InvalidArg("Init message must be request")
	}

	zap.L().Debug("Got init msg", zap.Any("req", msg.GetRequest()))

	wssupClient, err := s.getSupC(ctx, apivalidation.ObjectReferenceToGetOptions(msg.GetRequest().WorkspaceRef))
	if err != nil {
		return err
	}

	zap.L().Debug("Doing doExec to upstream")

	upstream, err := wssupClient.TermC().Exec(ctx, grpc_retry.Disable())
	if err != nil {
		return err
	}

	zap.L().Debug("doExec to upstream started")

	if err := upstream.Send(msg); err != nil {
		return err
	}

	errCh := make(chan error, 2)

	go func() {

		zap.L().Debug("Starting downstream loop")
		defer zap.L().Debug("Exiting downstream loop")
		defer upstream.CloseSend()
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
		zap.L().Debug("Starting upstream loop")
		defer zap.L().Debug("Exiting upstream loop")

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

	if err := <-errCh; err != nil {
		return err
	}

	return nil
}
