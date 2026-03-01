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

package suputils

import (
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func GetWorkspaceSupHost(ws *cordiumv1.Workspace) string {
	if ldflags.IsTest() {
		return "127.0.0.1"
	}
	/*
		if ws.Status.InternalIPAddress != "" {
			return ws.Status.InternalIPAddress
		}
	*/
	return fmt.Sprintf("ws-%s.%s.svc", ws.Metadata.Name, workspacecommon.K8sNS)
}

func GetWorkspaceSupPort() int {
	if ldflags.IsTest() {
		return 28904
	}
	return 8080
}

func getWorkspaceSupHostAddr(ws *cordiumv1.Workspace) string {
	return net.JoinHostPort(GetWorkspaceSupHost(ws), fmt.Sprintf("%d", GetWorkspaceSupPort()))
}

type WorkspaceSupClient struct {
	c ccordiumv1.WorkspaceSupervisorServiceClient

	termC        ccordiumv1.TerminalServiceClient
	healthCheckC grpc_health_v1.HealthClient

	conn *grpc.ClientConn

	uid  string
	name string
}

func (w *WorkspaceSupClient) C() ccordiumv1.WorkspaceSupervisorServiceClient {
	return w.c
}

func (w *WorkspaceSupClient) TermC() ccordiumv1.TerminalServiceClient {
	return w.termC
}

func (w *WorkspaceSupClient) HealthCheck() grpc_health_v1.HealthClient {
	return w.healthCheckC
}

func (w *WorkspaceSupClient) GetUID() string {
	return w.uid
}

func (w *WorkspaceSupClient) Close() error {

	if w.conn != nil {
		return w.conn.Close()
	}

	return nil
}

type GetWorkspaceSupClientOpts struct {
	Host string
}

func GetWorkspaceSupClient(ws *cordiumv1.Workspace, o *GetWorkspaceSupClientOpts) (*WorkspaceSupClient, error) {
	ret := &WorkspaceSupClient{
		name: ws.Metadata.Name,
		uid:  ws.Metadata.Uid,
	}

	retryCodes := []codes.Code{
		codes.Unavailable,
		codes.ResourceExhausted,
		codes.Unknown,
		codes.Aborted,
		codes.DataLoss,
		codes.Internal,
		codes.DeadlineExceeded,
	}

	unaryMiddlewares := []grpc.UnaryClientInterceptor{
		grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(64),
			grpc_retry.WithPerRetryTimeout(3*time.Second),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(1000*time.Millisecond)),
			grpc_retry.WithCodes(retryCodes...)),
	}

	streamMiddlewares := []grpc.StreamClientInterceptor{
		grpc_retry.StreamClientInterceptor(
			grpc_retry.WithMax(math.MaxUint32),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(1000*time.Millisecond)),
			grpc_retry.WithCodes(retryCodes...)),
	}

	grpcConn, err := grpc.Dial(
		func() string {
			if o != nil && o.Host != "" {
				addr := net.JoinHostPort(o.Host, fmt.Sprintf("%d", GetWorkspaceSupPort()))
				zap.L().Debug("Connecting to supervisor via address",
					zap.String("addr", addr),
					zap.String("wsName", ws.Metadata.Name))
				return addr
			}
			return getWorkspaceSupHostAddr(ws)
		}(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unaryMiddlewares...)),
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streamMiddlewares...)),
	)
	if err != nil {
		return nil, err
	}

	ret.conn = grpcConn
	ret.c = ccordiumv1.NewWorkspaceSupervisorServiceClient(grpcConn)
	ret.termC = ccordiumv1.NewTerminalServiceClient(grpcConn)
	ret.healthCheckC = grpc_health_v1.NewHealthClient(grpcConn)

	return ret, nil
}

type SupervisorCMap struct {
	regionRef *metav1.ObjectReference
	mu        sync.RWMutex
	mp        map[string]*wsCtx
}

type wsCtx struct {
	c  *WorkspaceSupClient
	ws *cordiumv1.Workspace
}

func NewSupervisorCtxMap(regionRef *metav1.ObjectReference) *SupervisorCMap {
	return &SupervisorCMap{
		regionRef: regionRef,
		mp:        make(map[string]*wsCtx),
	}
}

func (s *SupervisorCMap) Set(ws *cordiumv1.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ws.Status.RegionRef == nil {
		return s.doRemove(ws)
	}
	if ws.Status.RegionRef.Uid != s.regionRef.Uid {
		return nil
	}

	if ws.Status.UserRef == nil {
		return nil
	}

	if ucordiumv1.ToWorkspace(ws).IsStoppingOrStopped() {
		return s.doRemove(ws)
	}

	if wsCtx, ok := s.mp[ws.Metadata.Name]; ok {
		// zap.L().Debug("Updating Workspace in supClientMap", zap.String("name", ws.Metadata.Name))
		wsCtx.ws = ws
		return nil
	}

	// zap.L().Debug("Adding Workspace in supClientMap", zap.String("name", ws.Metadata.Name))

	supC, err := GetWorkspaceSupClient(ws, nil)
	if err != nil {
		return err
	}
	wsCtx := &wsCtx{
		c:  supC,
		ws: ws,
	}

	s.mp[ws.Metadata.Name] = wsCtx
	s.mp[ws.Metadata.Uid] = wsCtx

	return nil
}

func (s *SupervisorCMap) Remove(ws *cordiumv1.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doRemove(ws)
}

func (s *SupervisorCMap) doRemove(ws *cordiumv1.Workspace) error {
	supC, ok := s.mp[ws.Metadata.Name]
	if !ok {
		return nil
	}

	// zap.L().Debug("Removing Workspace from supClientMap", zap.String("name", ws.Metadata.Name))

	err := supC.c.Close()
	delete(s.mp, ws.Metadata.Name)
	delete(s.mp, ws.Metadata.Uid)

	return err
}

func (s *SupervisorCMap) Get(req *metav1.GetOptions, usrRef *metav1.ObjectReference) (*cordiumv1.Workspace, *WorkspaceSupClient, error) {

	if req == nil {
		return nil, nil, grpcutils.InvalidArg("Nil Workspace req")
	}

	if req.Name != "" {
		return s.GetByNameOrUID(req.Name, usrRef)
	}
	return s.GetByNameOrUID(req.Uid, usrRef)
}

func (s *SupervisorCMap) GetByNameOrUID(key string, usrRef *metav1.ObjectReference) (*cordiumv1.Workspace, *WorkspaceSupClient, error) {
	if usrRef == nil {
		return nil, nil, grpcutils.InvalidArg("No UserRef specified")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	ret, ok := s.mp[key]
	if !ok {
		return nil, nil, grpcutils.NotFound("Workspace does not exist: %s", key)
	}
	ws := ret.ws
	if ws.Status.UserRef == nil {
		return nil, nil, grpcutils.InvalidArg("Invalid Workspace: %s", key)
	}
	if ws.Status.UserRef.Uid != usrRef.Uid {
		return nil, nil, grpcutils.InvalidArg("Invalid Workspace: %s", key)
	}

	return ws, ret.c, nil
}
