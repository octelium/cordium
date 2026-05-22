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

package nocturne

import (
	"context"
	"time"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	wscontroller "github.com/octelium/cordium/cluster/nocturne/nocturne/controllers/workspaces"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type watcher struct {
	octeliumC octeliumc.ClientInterface
	regionRef *metav1.ObjectReference
	k8sC      kubernetes.Interface
}

func newWatcher(octeliumC octeliumc.ClientInterface, k8sC kubernetes.Interface, regionRef *metav1.ObjectReference) *watcher {
	return &watcher{
		octeliumC: octeliumC,
		regionRef: regionRef,
		k8sC:      k8sC,
	}
}

func (c *watcher) run(ctx context.Context) error {
	go c.startWSLoop(ctx)
	return nil
}

func (c *watcher) startWSLoop(ctx context.Context) {
	tickerCh := time.NewTicker(5 * time.Minute)
	defer tickerCh.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerCh.C:
			if err := c.handleWorkspaces(ctx); err != nil {
				zap.L().Error("Could not handle Workspace by watcher", zap.Error(err))
			}
		}
	}
}

func (c *watcher) handleWorkspaces(ctx context.Context) error {

	wsList, err := func() ([]*cordiumv1.Workspace, error) {
		var ret []*cordiumv1.Workspace
		var page uint32
		for {
			itmList, err := c.octeliumC.CordiumC().ListWorkspace(ctx, &rmetav1.ListOptions{
				Paginate:     true,
				ItemsPerPage: 500,
				Page:         page,
			})
			if err != nil {
				return nil, err
			}

			ret = append(ret, itmList.Items...)
			if itmList.ListResponseMeta == nil || !itmList.ListResponseMeta.HasMore {
				return ret, nil
			}

			page = page + 1
		}
	}()
	if err != nil {
		return err
	}

	cc, err := c.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	for _, ws := range wsList {
		if !c.isMyRegion(ws) {
			continue
		}
		if ws.Status.State == cordiumv1.Workspace_Status_STOPPED {
			continue
		}

		if err := c.doHandleStalledState(ctx, ws, cc); err != nil {
			zap.L().Warn("Could not doHandleStalledState", zap.Any("ws", ws), zap.Error(err))
		}
	}

	return nil
}

func (c *watcher) isMyRegion(ws *cordiumv1.Workspace) bool {
	if ws.Status.RegionRef == nil {
		return false
	}

	return c.regionRef.Uid == ws.Status.RegionRef.Uid
}

func (c *watcher) getTimeout(ws *cordiumv1.Workspace, cc *cordiumv1.ClusterConfig) time.Duration {

	var ret time.Duration

	if cc.Spec.Workspace != nil && cc.Spec.Workspace.Timeout != nil {

		switch ws.Status.SpaceType {
		case cordiumv1.Space_Status_ORGANIZATION:
			if cc.Spec.Workspace.Timeout.OrganizationSpaceDuration != nil {
				ret = umetav1.ToDuration(cc.Spec.Workspace.Timeout.OrganizationSpaceDuration).ToGo()
			}
		case cordiumv1.Space_Status_USER:
			if cc.Spec.Workspace.Timeout.UserSpaceDuration != nil {
				ret = umetav1.ToDuration(cc.Spec.Workspace.Timeout.UserSpaceDuration).ToGo()
			}
		}

		if ret == 0 && cc.Spec.Workspace.Timeout.DefaultDuration != nil {
			ret = umetav1.ToDuration(cc.Spec.Workspace.Timeout.DefaultDuration).ToGo()
		}

	}

	if ret == 0 {
		ret = 30 * time.Hour
	}

	return ret
}

func (c *watcher) doHandleStalledState(ctx context.Context, ws *cordiumv1.Workspace, cc *cordiumv1.ClusterConfig) error {

	if !ws.Status.CurrentStateSetAt.IsValid() {
		return nil
	}

	currentStateAt := ws.Status.CurrentStateSetAt.AsTime()

	switch {
	case ws.Status.State == cordiumv1.Workspace_Status_INIT_REQUEST:
		if time.Now().After(currentStateAt.Add(5 * time.Minute)) {
			zap.L().Debug("Stale INITIALIZING_REQUEST state. Setting it to stopped",
				zap.String("name", ws.Metadata.Name),
				zap.String("uid", ws.Metadata.Uid))
			return c.doHandleStalledStateInitializingOrStopping(ctx, ws)
		}
	case ucordiumv1.ToWorkspace(ws).IsStopping():
		if time.Now().After(currentStateAt.Add(120 * time.Minute)) {
			zap.L().Debug("Stale STOPPING state. Setting it to stopped",
				zap.String("name", ws.Metadata.Name),
				zap.String("uid", ws.Metadata.Uid))
			return c.doHandleStalledStateInitializingOrStopping(ctx, ws)
		}
	case ws.Status.State == cordiumv1.Workspace_Status_RUNNING:
		if err := c.doHandleInactiveRunning(ctx, ws, cc, c.getTimeout(ws, cc)); err != nil {
			return err
		}
	case ucordiumv1.ToWorkspace(ws).IsPreRunning():
		if time.Now().After(currentStateAt.Add(180 * time.Minute)) {
			zap.L().Debug("Workspace pre-running state is stalled and timeout exceeded. Stopping Workspace...",
				zap.String("name", ws.Metadata.Name),
				zap.String("uid", ws.Metadata.Uid),
				zap.String("state", ws.Status.State.String()))
			if err := c.doSetStateToStopping(ctx, ws); err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}

func (c *watcher) doHandleStalledStateInitializingOrStopping(ctx context.Context, ws *cordiumv1.Workspace) error {
	if err := c.doSetState(ctx, ws, cordiumv1.Workspace_Status_STOPPED); err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if ws.Status.SessionRef != nil {
		if _, err := c.octeliumC.CoreC().DeleteSession(ctx, &rmetav1.DeleteOptions{
			Uid: ws.Status.SessionRef.Uid,
		}); err != nil {
			if !grpcerr.IsNotFound(err) {
				return err
			}
		}
	}
	if err := wscontroller.DoDeleteWorkspaceOwner(ctx, ws, c.k8sC); err != nil {
		return err
	}

	if ws.Status.IsBuild {
		if _, err := c.octeliumC.CordiumC().DeleteWorkspace(ctx, &rmetav1.DeleteOptions{
			Uid: ws.Metadata.Uid,
		}); err != nil {
			if !grpcerr.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (c *watcher) doHandleInactiveRunning(ctx context.Context,
	ws *cordiumv1.Workspace, cc *cordiumv1.ClusterConfig, timeout time.Duration) error {

	if ws.Status.State != cordiumv1.Workspace_Status_RUNNING {
		return nil
	}

	if ws.Status.IsBuild {
		return nil
	}

	if !ws.Status.LastActivityAt.IsValid() {
		return nil
	}

	if ws.Spec.Runtime != nil && ws.Spec.Runtime.Timeout != nil {
		switch ws.Spec.Runtime.Timeout.Mode {
		case cordiumv1.Workspace_Spec_Runtime_Timeout_DISABLED:
			if cc.Spec.Workspace != nil &&
				cc.Spec.Workspace.Timeout != nil &&
				cc.Spec.Workspace.Timeout.AllowNoTimeout {
				return nil
			}
		}
	}

	lastActivity := ws.Status.LastActivityAt.AsTime()

	if time.Now().After(lastActivity.Add(timeout)) {
		zap.L().Debug("Workspace timeout exceeded. Stopping Workspace...",
			zap.String("name", ws.Metadata.Name), zap.String("uid", ws.Metadata.Uid))
		if err := c.doSetStateToStopping(ctx, ws); err != nil {
			return err
		}
	}

	return nil
}

func (c *watcher) doSetStateToStopping(ctx context.Context, ws *cordiumv1.Workspace) error {
	return c.doSetState(ctx, ws, cordiumv1.Workspace_Status_STOPPING_REQUEST)
}

func (c *watcher) doSetState(ctx context.Context, ws *cordiumv1.Workspace, state cordiumv1.Workspace_Status_State) error {

	zap.L().Debug("Setting the state of the Workspace",
		zap.String("name", ws.Metadata.Name),
		zap.String("state", state.String()))
	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid: ws.Metadata.Uid,
	})
	if err != nil {
		return err
	}

	if ws.Status.State == state {
		return nil
	}

	ws.Status.LastState = ws.Status.State
	ws.Status.State = state
	ws.Status.LastStateSetAt = ws.Status.CurrentStateSetAt
	ws.Status.CurrentStateSetAt = pbutils.Now()

	if state == cordiumv1.Workspace_Status_STOPPING_REQUEST {
		ws.Status.StoppingReason = cordiumv1.Workspace_Status_STOPPING_REASON_CLUSTER
	}

	if _, err := c.octeliumC.CordiumC().UpdateWorkspace(ctx, ws); err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		if grpcerr.IsResourceChanged(err) {
			time.Sleep(200 * time.Millisecond)
			return c.doSetState(ctx, ws, state)
		}
		return err
	}

	return nil
}
