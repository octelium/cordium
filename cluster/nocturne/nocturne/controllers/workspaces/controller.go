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

package controller

import (
	"context"
	"sync"
	"time"

	snapshotset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/celengine"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	octeliumC octeliumc.ClientInterface
	k8sC      kubernetes.Interface
	domain    string
	jwkCtl    *jwkctl.Controller
	regionRef *metav1.ObjectReference

	wg      sync.WaitGroup
	ctxMain context.Context

	watcherMap struct {
		mp map[string]*statusWatcher
		mu sync.RWMutex
	}

	celEngine *celengine.CELEngine
	snapshotC snapshotset.Interface
}

func NewController(
	ctx context.Context,
	ctxMain context.Context,
	octeliumC octeliumc.ClientInterface,
	k8sC kubernetes.Interface,
	jwkCtl *jwkctl.Controller,
	regionRef *metav1.ObjectReference,
) (*Controller, error) {

	cc, err := octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	celEngine, err := celengine.New(ctx, &celengine.Opts{})
	if err != nil {
		return nil, err
	}

	ret := &Controller{
		ctxMain:   ctxMain,
		octeliumC: octeliumC,
		k8sC:      k8sC,
		domain:    cc.Status.Domain,
		jwkCtl:    jwkCtl,
		regionRef: regionRef,
		celEngine: celEngine,
		watcherMap: struct {
			mp map[string]*statusWatcher
			mu sync.RWMutex
		}{
			mp: make(map[string]*statusWatcher),
		},
	}

	if !ldflags.IsTest() {
		cfg, err := k8sutils.GetInClusterConfig()
		if err != nil {
			return nil, err
		}

		ret.snapshotC, err = snapshotset.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (c *Controller) isMyRegion(ws *cordiumv1.Workspace) bool {
	if ws.Status.RegionRef == nil {
		return false
	}
	return ws.Status.RegionRef.Uid == c.regionRef.Uid
}

func (c *Controller) OnAdd(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Metadata.CreatedAt.AsTime().Add(1 * time.Minute).Before(time.Now()) {
		zap.L().Debug("Most probably a stale Workspace. No need to start it")
		return nil
	}

	if err := c.setPersistentVolumeClaim(ctx, ws); err != nil {
		return errors.Errorf("Could not set persistentVolumeClaim: %+v", err)
	}

	if !c.isMyRegion(ws) {
		zap.L().Debug("Workspace is not in my region. No onAdd needed", zap.String("uid", ws.Metadata.Uid))
		return nil
	}

	if ws.Status.State != cordiumv1.Workspace_Status_INIT_REQUEST {
		zap.S().Debugf("Workspace: %s is not in init mode. Nothing to be done....", ws.Metadata.Name)
		return nil
	}

	return c.startWorkspace(ctx, ws)
}

func (c *Controller) startWorkspace(ctx context.Context, ws *cordiumv1.Workspace) error {

	{
		if err := c.setPersistentVolumeClaim(ctx, ws); err != nil {
			return errors.Errorf("Could not set persistentVolumeClaim: %+v", err)
		}
	}

	zap.L().Debug("Starting Workspace",
		zap.String("uid", ws.Metadata.Uid), zap.String("name", ws.Metadata.Name))

	if err := c.doOnAdd(ctx, ws); err != nil {
		return err
	}

	{
		c.watcherMap.mu.RLock()
		_, ok := c.watcherMap.mp[ws.Metadata.Uid]
		c.watcherMap.mu.RUnlock()
		if ok {
			return nil
		}
	}

	watcher, err := newStatusWatcher(c, ws)
	if err != nil {
		return err
	}

	c.watcherMap.mu.Lock()
	c.watcherMap.mp[ws.Metadata.Uid] = watcher
	c.watcherMap.mu.Unlock()

	return watcher.run(c.ctxMain)

}

func (c *Controller) OnUpdate(ctx context.Context, new, old *cordiumv1.Workspace) error {

	if !c.isMyRegion(new) {
		return nil
	}

	switch {
	case new.Status.State == cordiumv1.Workspace_Status_INIT_REQUEST &&
		old.Status.State == cordiumv1.Workspace_Status_STOPPED:
		{
			return c.startWorkspace(ctx, new)
		}
	case new.Status.State == cordiumv1.Workspace_Status_STOPPING_REQUEST &&
		old.Status.State != cordiumv1.Workspace_Status_STOPPING_REQUEST:
		{
			if err := c.stopWorkspace(ctx, new); err != nil {
				return err
			}

		}
	}

	return nil
}

func (c *Controller) OnDelete(ctx context.Context, ws *cordiumv1.Workspace) error {

	if c.isMyRegion(ws) {

		if err := c.stopWorkspace(ctx, ws); err != nil {
			zap.L().Warn("Could not stopWorkspace on delete", zap.Error(err))
		}
	}

	if err := c.removePersistentClaim(ctx, ws); err != nil {
		return err
	}

	return nil
}

func (c *Controller) stopWorkspace(ctx context.Context, ws *cordiumv1.Workspace) error {
	zap.L().Debug("Starting to send a shutdown call to Workspace",
		zap.String("uid", ws.Metadata.Uid), zap.String("name", ws.Metadata.Name))

	switch ws.Status.State {
	case cordiumv1.Workspace_Status_STOPPED, cordiumv1.Workspace_Status_UNKNOWN:
		zap.L().Debug("No need to close the Workspace",
			zap.String("wsUID", ws.Metadata.Uid), zap.String("state", ws.Status.State.String()))
		return nil
	}

	wssupC, err := suputils.GetWorkspaceSupClient(ws, nil)
	if err != nil {
		return err
	}
	defer wssupC.Close()

	zap.L().Debug("Sending a shutdown call",
		zap.String("uid", ws.Metadata.Uid), zap.String("name", ws.Metadata.Name))
	if _, err := wssupC.C().Shutdown(ctx, &ccordiumv1.ShutdownRequest{
		Workspace: ws,
	}); err != nil {

		zap.L().Warn("Could not shutdown Workspace. It could have been already closed from within",
			zap.String("uid", ws.Metadata.Uid), zap.Error(err))

		if err := c.doOnDeleteK8s(ctx, ws); err != nil {
			zap.L().Warn("Could not delete k8s resources after shutdown call fail", zap.Error(err))
		}

		ws.Status.LastState = ws.Status.State

		ws.Status.SessionRef = nil
		ws.Status.RegionRef = nil
		ws.Status.LastStoppedAt = pbutils.Now()
		ws.Status.State = cordiumv1.Workspace_Status_STOPPED
		ws.Status.Hostname = ""

		if run := ucordiumv1.ToWorkspace(ws).GetCurrentRun(); run != nil {
			run.StoppedAt = ws.Status.LastStoppedAt
		}

		ws, err = c.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
		if err != nil {
			if grpcerr.IsNotFound(err) {
				return nil
			}
			return err
		}

		c.watcherMap.mu.RLock()
		watcher, ok := c.watcherMap.mp[ws.Metadata.Uid]
		c.watcherMap.mu.RUnlock()
		if ok {
			go func() {
				if err := watcher.forceOnStop(context.Background()); err != nil {
					zap.L().Error("Could not force Stop", zap.Error(err))
				}
				watcher.close()
			}()
		}

	} else {
		zap.L().Debug("Workspace successfully shutdown",
			zap.String("uid", ws.Metadata.Uid), zap.String("name", ws.Metadata.Name))
	}

	if ws.Spec.IsEphemeral {
		if err := c.removePersistentClaim(ctx, ws); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) WaitUntilAllWatchersClosed() {
	zap.L().Info("Starting to wait for all Workspace watchers to close")
	c.wg.Wait()
	zap.L().Info("All Workspace watchers successfully closed")
}

func (c *Controller) removePersistentClaim(ctx context.Context, ws *cordiumv1.Workspace) error {

	zap.L().Debug("Removing PVC", zap.String("ws", ws.Metadata.Name))
	if err := c.k8sC.CoreV1().PersistentVolumeClaims(ns).Delete(ctx,
		c.getPVCName(ws), k8smetav1.DeleteOptions{}); err != nil {
		if !k8serr.IsNotFound(err) {
			zap.L().Warn("Could not delete pvc: %+v", zap.Error(err), zap.String("wsName", ws.Metadata.Name))
		}
	}

	return nil
}
