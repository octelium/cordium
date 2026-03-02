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

package mains

import (
	"context"
	"sync"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
)

func (s *Server) WatchWorkspace(req *cordiumv1.WatchWorkspaceRequest, stream cordiumv1.MainService_WatchWorkspaceServer) error {
	ctx := stream.Context()

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return err
	}

	wsList, err := s.octeliumC.CordiumC().ListWorkspace(ctx, urscsrv.FilterByUser(i.User))
	if err != nil {
		return err
	}

	if len(wsList.Items) < 1 {
		return grpcutils.InvalidArg("No Workspaces found for this User")
	}

	sub := s.wsWatchMan.newSub(i, stream, req)
	defer s.wsWatchMan.removeSub(sub)

	<-ctx.Done()

	return nil
}

type watchWorkspaceSubscriptionManager struct {
	mu sync.RWMutex
	mp map[string]*watchWorkspaceSubscription
}

func (w *watchWorkspaceSubscriptionManager) newSub(i *userctx.UserCtx,
	stream cordiumv1.MainService_WatchWorkspaceServer, req *cordiumv1.WatchWorkspaceRequest) *watchWorkspaceSubscription {
	ret := &watchWorkspaceSubscription{
		id:      utilrand.GetRandomString(16),
		userUID: i.User.Metadata.Uid,
		stream:  stream,
		req:     req,
	}

	w.mu.Lock()
	w.mp[ret.id] = ret
	w.mu.Unlock()

	return ret
}

func (w *watchWorkspaceSubscriptionManager) removeSub(s *watchWorkspaceSubscription) {
	w.mu.Lock()
	delete(w.mp, s.id)
	w.mu.Unlock()
}

func (w *watchWorkspaceSubscriptionManager) onCreate(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Status.UserRef == nil {
		return nil
	}
	msg := &cordiumv1.WatchWorkspaceResponse{
		Type: &cordiumv1.WatchWorkspaceResponse_Create_{
			Create: &cordiumv1.WatchWorkspaceResponse_Create{
				Item: ws,
			},
		},
	}

	return w.publishMsg(msg, ws.Status.UserRef, umetav1.GetObjectReference(ws))
}

func (w *watchWorkspaceSubscriptionManager) onUpdate(ctx context.Context, new, old *cordiumv1.Workspace) error {
	if new.Status.UserRef == nil || old.Status.UserRef == nil {
		return nil
	}
	msg := &cordiumv1.WatchWorkspaceResponse{
		Type: &cordiumv1.WatchWorkspaceResponse_Update_{
			Update: &cordiumv1.WatchWorkspaceResponse_Update{
				NewItem: new,
				OldItem: old,
			},
		},
	}

	return w.publishMsg(msg, new.Status.UserRef, umetav1.GetObjectReference(new))
}

func (w *watchWorkspaceSubscriptionManager) onDelete(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Status.UserRef == nil {
		return nil
	}
	msg := &cordiumv1.WatchWorkspaceResponse{
		Type: &cordiumv1.WatchWorkspaceResponse_Delete_{
			Delete: &cordiumv1.WatchWorkspaceResponse_Delete{
				Item: ws,
			},
		},
	}

	return w.publishMsg(msg, ws.Status.UserRef, umetav1.GetObjectReference(ws))
}

func (w *watchWorkspaceSubscriptionManager) publishMsg(msg *cordiumv1.WatchWorkspaceResponse,
	usrRef *metav1.ObjectReference, wsRef *metav1.ObjectReference) error {

	usrUID := usrRef.Uid
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, sub := range w.mp {
		if sub.userUID == usrUID &&
			(sub.req.WorkspaceRef == nil || sub.req.WorkspaceRef.Uid == wsRef.Uid) {
			sub.stream.Send(msg)
		}
	}

	return nil
}

type watchWorkspaceSubscription struct {
	id      string
	userUID string
	stream  cordiumv1.MainService_WatchWorkspaceServer
	req     *cordiumv1.WatchWorkspaceRequest
}
