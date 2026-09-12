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

package controller

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	minRunLoopBackoff        = 250 * time.Millisecond
	maxRunLoopBackoff        = 10 * time.Second
	runLoopBackoffResetAfter = 30 * time.Second

	supInitTimeout        = 5 * time.Minute
	supInitTimeoutResumed = 15 * time.Minute

	maxUpdateStatusAttempts = 10
)

type statusWatcher struct {
	octeliumC octeliumc.ClientInterface

	uid      string
	name     string
	ctx      context.Context
	cancelFn context.CancelFunc
	isClosed bool
	mu       sync.RWMutex

	jwkCtl *jwkctl.Controller

	ctl *Controller

	wssupC  *suputils.WorkspaceSupClient
	svcAddr string

	ws *cordiumv1.Workspace

	isResumed bool

	runningStartedAt time.Time

	didOnStopping  bool
	didOnStopped   bool
	didOnInit      bool
	healthCheckErr bool
}

func newStatusWatcher(ctl *Controller, ws *cordiumv1.Workspace, isResumed bool) (*statusWatcher, error) {
	ctx, cancelFn := context.WithCancel(ctl.ctxMain)

	ret := &statusWatcher{
		ctl:       ctl,
		octeliumC: ctl.octeliumC,
		uid:       ws.Metadata.Uid,
		name:      ws.Metadata.Name,
		jwkCtl:    ctl.jwkCtl,
		ctx:       ctx,
		cancelFn:  cancelFn,
		isResumed: isResumed,

		ws: ws,
	}

	if ws.Status.State == cordiumv1.Workspace_Status_RUNNING && ws.Status.LastRunningAt.IsValid() {
		ret.runningStartedAt = ws.Status.LastRunningAt.AsTime()
	}

	ctl.wg.Add(1)

	zap.L().Debug("Workspace watcher created",
		zap.String("uid", ret.uid), zap.Bool("isResumed", isResumed))

	return ret, nil
}

func (c *statusWatcher) run() {
	zap.L().Debug("Workspace watcher is starting running", zap.String("name", c.name))
	go c.startSupervise()
}

func (c *statusWatcher) startSupervise() {
	defer func() {
		if c.ctl.ctxMain.Err() != nil {
			c.detach()
			return
		}
		c.close()
	}()

	if err := c.waitUntilWorkspaceSupInitialized(c.ctx); err != nil {
		if c.ctx.Err() != nil {
			return
		}
		zap.L().Warn("Could not wait until the Workspace supervisor is ready",
			zap.String("name", c.name), zap.Error(err))
		c.setHealthCheckErr()
		return
	}

	if c.ctx.Err() != nil {
		return
	}

	go c.startHealthCheckLoop(c.ctx)

	zap.L().Debug("Workspace watcher is now running", zap.String("name", c.name))

	c.startRunLoop(c.ctx)
}

func (c *statusWatcher) getSupInitTimeout() time.Duration {
	if c.isResumed {
		return supInitTimeoutResumed
	}
	return supInitTimeout
}

func (c *statusWatcher) setHealthCheckErr() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthCheckErr = true
}

func (c *statusWatcher) doGetWorkspaceK8sSvcAddr(ctx context.Context) (string, error) {
	svcK8s, err := c.ctl.k8sC.CoreV1().Services(ns).Get(ctx, getK8sRscName(c.ws), k8smetav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if svcK8s.Spec.ClusterIP == "" {
		return "", errors.Errorf("Empty svc clusterIP")
	}

	addr, err := netip.ParseAddr(svcK8s.Spec.ClusterIP)
	if err != nil {
		return "", err
	}

	return addr.String(), nil
}

func (c *statusWatcher) setWorkspaceK8sSvcAddr(ctx context.Context) {
	if c.svcAddr != "" {
		return
	}

	addr, err := c.doGetWorkspaceK8sSvcAddr(ctx)
	if err != nil {
		zap.L().Debug("Could not get workspaceK8sServiceAddr",
			zap.String("name", c.name), zap.Error(err))
		return
	}

	zap.L().Debug("Found Workspace k8s service addr",
		zap.String("name", c.name), zap.String("addr", addr))
	c.svcAddr = addr
}

func (c *statusWatcher) waitUntilWorkspaceSupInitialized(ctx context.Context) error {
	if ldflags.IsTest() {
		wssupC, err := suputils.GetWorkspaceSupClient(c.ws, nil)
		if err != nil {
			return err
		}
		c.wssupC = wssupC
		return nil
	}
	zap.L().Debug("Starting waitUntilWorkspaceSupInitialized", zap.String("name", c.name))

	doFn := func(ctx context.Context) error {
		c.setWorkspaceK8sSvcAddr(ctx)

		ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		defer cancel()

		wssupC, err := suputils.GetWorkspaceSupClient(c.ws, &suputils.GetWorkspaceSupClientOpts{
			Host: c.svcAddr,
		})
		if err != nil {
			return err
		}

		resp, err := wssupC.HealthCheck().Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			wssupC.Close()
			return err
		}
		if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
			c.wssupC = wssupC
			return nil
		}

		wssupC.Close()
		return errors.Errorf("The Workspace supervisor server is not at SERVING mode")
	}

	tickerCh := time.NewTicker(2 * time.Second)
	defer tickerCh.Stop()

	timeoutCh := time.NewTimer(c.getSupInitTimeout())
	defer timeoutCh.Stop()

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("ctx done. Exiting waitUntilWorkspaceSupInitialized loop", zap.String("name", c.name))
			return errors.Errorf("ctx done while waiting for the Workspace supervisor")
		case <-timeoutCh.C:
			return errors.Errorf("Deadline for waitUntilWorkspaceSupInitialized exceeded")
		case <-tickerCh.C:
			err := doFn(ctx)
			if err == nil {
				zap.L().Debug("Workspace supervisor is now READY", zap.String("name", c.name))
				return nil
			}
			zap.L().Debug("Workspace supervisor is not ready yet...", zap.String("name", c.name), zap.Error(err))
		}
	}
}

func (c *statusWatcher) startRunLoop(ctx context.Context) {
	zap.L().Debug("Starting runLoop", zap.String("name", c.name))
	defer zap.L().Debug("runLoop ended", zap.String("name", c.name))

	backoff := minRunLoopBackoff

	for {
		if ctx.Err() != nil || c._isClosed() {
			return
		}

		startedAt := time.Now()
		err := c.doStartRunLoop(ctx)
		if err == nil {
			return
		}

		if ctx.Err() != nil || c._isClosed() {
			return
		}

		zap.L().Warn("Could not run the ListenState loop. Trying again...",
			zap.String("name", c.name), zap.Error(err))

		if time.Since(startedAt) >= runLoopBackoffResetAfter {
			backoff = minRunLoopBackoff
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = backoff * 2
		if backoff > maxRunLoopBackoff {
			backoff = maxRunLoopBackoff
		}
	}
}

func (c *statusWatcher) doStartRunLoop(ctx context.Context) error {
	defer zap.L().Debug("Exiting doStartRunLoop", zap.String("name", c.name))

	zap.L().Debug("Sending ListenState call", zap.String("name", c.name))

	strm, err := c.wssupC.C().ListenState(ctx, &ccordiumv1.ListenStateRequest{})
	if err != nil {
		return err
	}

	zap.L().Debug("Starting ListenState loop", zap.String("name", c.name))
	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("ctx done. Exiting status watcher run loop.", zap.String("name", c.name))
			return nil

		default:
			msg, err := strm.Recv()
			if err != nil {
				if grpcerr.IsCanceled(err) || ctx.Err() != nil || c._isClosed() {
					return nil
				}

				zap.L().Error("Could not recv msg by status watcher",
					zap.String("name", c.name), zap.Error(err))
				return err
			}

			if err := c.handleStatus(ctx, msg.State); err != nil {
				zap.L().Error("Could not handle Workspace status change", zap.String("name", c.name), zap.Error(err))
			}

		}
	}

}

func (c *statusWatcher) _isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isClosed
}

func (c *statusWatcher) close() error {
	return c.doClose(false)
}

func (c *statusWatcher) detach() error {
	return c.doClose(true)
}

func (c *statusWatcher) doClose(isDetach bool) error {

	c.mu.Lock()
	if c.isClosed {
		c.mu.Unlock()
		return nil
	}
	c.isClosed = true
	didOnStopping := c.didOnStopping
	didOnStopped := c.didOnStopped
	healthCheckErr := c.healthCheckErr
	c.mu.Unlock()

	ctx := context.Background()

	zap.L().Debug("Closing status watcher",
		zap.String("name", c.name), zap.Bool("isDetach", isDetach))

	switch {
	case isDetach:
		zap.L().Debug("Detaching the status watcher. The Workspace is left running",
			zap.String("name", c.name))
	case healthCheckErr:
		zap.L().Debug("Health check error detected. Forcing stop")
		if err := c.forceOnStop(ctx); err != nil {
			zap.L().Error("Could not forceOnStop during close", zap.Error(err))
		}
	case !didOnStopping && didOnStopped:
		zap.L().Error("STOPPED has been handled while not handling STOPPING. This should never happen",
			zap.String("name", c.name))
	case !didOnStopping:
		zap.L().Debug("STOPPING has not been handled on close. Forcing stop", zap.String("name", c.name))
		if err := c.doStopWorkspace(ctx); err != nil {
			zap.L().Error("Could not stopWorkspace", zap.Error(err))
		}
	case !didOnStopped:
		zap.L().Debug("STOPPED has not been handled on close. Forcing stop", zap.String("name", c.name))
		if err := c.forceOnStop(ctx); err != nil {
			zap.L().Error("Could not forceOnStop", zap.Error(err))
		}
	default:
		zap.L().Debug("Initializing a normal close after a successful STOPPED Workspace", zap.String("name", c.name))
	}

	c.cancelFn()
	if c.wssupC != nil {
		c.wssupC.Close()
	}

	zap.L().Debug("Status watcher is now closed", zap.String("name", c.name))

	c.ctl.removeWatcher(c)

	c.ctl.wg.Done()

	return nil
}

func (c *statusWatcher) doUpdateStatus(ctx context.Context, st cordiumv1.Workspace_Status_State) error {
	for i := 0; i < maxUpdateStatusAttempts; i++ {
		err := c.doUpdateStatusOnce(ctx, st)
		if err == nil {
			return nil
		}

		if !grpcerr.IsResourceChanged(err) {
			return err
		}

		zap.L().Debug("Could not update Workspace status due to resource change. Trying again...",
			zap.String("name", c.name))
		time.Sleep(200 * time.Millisecond)
	}

	return errors.Errorf("Could not update the Workspace status after too many attempts")
}

func (c *statusWatcher) doUpdateStatusOnce(ctx context.Context, st cordiumv1.Workspace_Status_State) error {

	state := st
	now := time.Now()
	if state == cordiumv1.Workspace_Status_UNKNOWN {
		zap.L().Debug("No need to update Workspace state for the status",
			zap.String("name", c.name),
			zap.String("status", st.String()))
		return nil
	}
	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: c.uid})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if ws.Status.State == state {
		zap.L().Debug("Workspace is already at the state. Nothing to be done",
			zap.String("name", c.name), zap.String("state", state.String()))
		return nil
	}

	zap.L().Debug("status watcher setting state", zap.String("name", c.name),
		zap.String("state", st.String()))

	ws.Status.LastState = ws.Status.State
	ws.Status.State = state

	ws.Status.LastStateSetAt = ws.Status.CurrentStateSetAt
	ws.Status.CurrentStateSetAt = pbutils.Now()

	switch state {
	case cordiumv1.Workspace_Status_RUNNING:
		c.runningStartedAt = now
		ws.Status.LastRunningAt = ws.Status.CurrentStateSetAt
		ws.Status.LastActivityAt = ws.Status.CurrentStateSetAt
		ws.Status.SuccessfulRuns = ws.Status.SuccessfulRuns + 1
	case cordiumv1.Workspace_Status_STOPPING:
		if !c.runningStartedAt.IsZero() {
			var curSeconds uint32
			if ws.Status.TotalLastRunsDuration != nil {
				curSeconds = curSeconds + ws.Status.TotalLastRunsDuration.GetSeconds()
			}
			curSeconds = curSeconds + uint32(time.Since(c.runningStartedAt).Seconds())
			ws.Status.TotalLastRunsDuration = &metav1.Duration{
				Type: &metav1.Duration_Seconds{
					Seconds: curSeconds,
				},
			}
		}
	case cordiumv1.Workspace_Status_STOPPED:
		ws.Status.SessionRef = nil
		ws.Status.RegionRef = nil
		ws.Status.LastStoppedAt = ws.Status.CurrentStateSetAt
		ws.Status.State = cordiumv1.Workspace_Status_STOPPED
		ws.Status.Hostname = ""
		if run := ucordiumv1.ToWorkspace(ws).GetCurrentRun(); run != nil {
			run.StoppedAt = ws.Status.LastStoppedAt
		}
	}

	_, err = c.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
	if err != nil {
		// If deleted when Stopping
		if grpcerr.IsNotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

func (c *statusWatcher) handleStatus(ctx context.Context, st cordiumv1.Workspace_Status_State) error {

	if c._isClosed() {
		zap.L().Debug("watcher is already closed. Won't handle status...", zap.String("name", c.name))
		return nil
	}

	zap.L().Debug("Starting handling status change",
		zap.String("name", c.name), zap.String("status", st.String()))

	switch st {
	case cordiumv1.Workspace_Status_INIT_REQUEST:
		return nil
	}

	switch st {

	case cordiumv1.Workspace_Status_INITIALIZING:
		if err := c.onReadyInit(ctx); err != nil {
			return err
		}

	case cordiumv1.Workspace_Status_STOPPING:
		c.mu.Lock()
		didOnStopping := c.didOnStopping
		c.didOnStopping = true
		c.mu.Unlock()
		if !didOnStopping {
			go c.cleanupStaleStoppingState(ctx)
		}
	case cordiumv1.Workspace_Status_STOPPED:

		defer c.close()

		if err := c.onStopped(ctx); err != nil {
			return err
		}

	}

	if err := c.doUpdateStatus(ctx, st); err != nil {
		zap.L().Error("Could not update state", zap.String("name", c.name), zap.Error(err))
	}

	return nil
}

func (c *statusWatcher) cleanupStaleStoppingState(ctx context.Context) {

	select {
	case <-ctx.Done():
		zap.L().Debug("No need to cleanup stopping state. ctx done.", zap.String("name", c.name))
		return
	case <-time.After(45 * time.Minute):
	}

	zap.L().Debug("Checking if onStopped is needed", zap.String("wsName", c.ws.Metadata.Name))
	if c._isClosed() {
		zap.L().Debug("watcher is already closed. No need to do onStopped",
			zap.String("wsName", c.ws.Metadata.Name))
		return
	}
	defer c.close()

	ctx = context.Background()

	if err := c.forceOnStop(ctx); err != nil {
		zap.L().Error("Could not force stop after state stopping state", zap.Error(err))
	}
}

func (c *statusWatcher) forceOnStop(ctx context.Context) error {
	zap.L().Debug("Setting the status to STOPPED by force", zap.String("wsName", c.ws.Metadata.Name))
	if err := c.doUpdateStatus(ctx, cordiumv1.Workspace_Status_STOPPED); err != nil {
		zap.L().Error("Could not set status to STOPPED by force", zap.Error(err))
	}

	if err := c.onStopped(ctx); err != nil {
		zap.L().Error("Could not do forced onStopped", zap.String("wsName", c.ws.Metadata.Name), zap.Error(err))
		return err
	}

	return nil
}

func (c *statusWatcher) onReadyInit(ctx context.Context) error {

	c.mu.RLock()
	didOnInit := c.didOnInit
	c.mu.RUnlock()
	if didOnInit {
		return nil
	}

	zap.L().Debug("Starting onReadyInit", zap.String("name", c.name))
	c.mu.Lock()
	c.didOnInit = true
	c.mu.Unlock()

	ccc, err := c.octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: c.uid})
	if err != nil {
		return err
	}

	if ws.Status.SpaceRef == nil {
		return errors.Errorf("A Workspace must belong to a Space")
	}

	if ws.Status.TemplateRef == nil {
		return errors.Errorf("A Workspace must belong to a Template")
	}

	space, err := c.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.SpaceRef.Uid,
	})
	if err != nil {
		return err
	}

	template, err := c.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.TemplateRef.Uid,
	})
	if err != nil {
		return err
	}

	zap.L().Debug("Sending initialize api call", zap.String("uid", ws.Metadata.Uid))

	var accessToken string
	var refreshToken string
	var tunnelPeerPublicKey string
	var orgSecretList *cordiumv1.SecretList
	var expiresIn int64

	var gitProviderInfo *ccordiumv1.GitProviderInfo
	var userSecretList *cordiumv1.UserSecretList
	var userConfig *cordiumv1.UserConfig

	orgSecretList, err = c.octeliumC.CordiumC().ListSecret(ctx, ourscsrv.FilterBySpaceRef(ws.Status.SpaceRef))
	if err != nil {
		return err
	}

	var templateHasSnapshot bool
	if !ws.Status.IsBuild && ws.Status.TemplateRef != nil {
		project, err := c.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
			Uid: ws.Status.TemplateRef.Uid,
		})
		if err != nil {
			return err
		}

		if _, err := c.ctl.snapshotC.SnapshotV1().
			VolumeSnapshots(ns).
			Get(ctx, c.ctl.getTemplateBuildName(project), k8smetav1.GetOptions{}); err == nil {
			templateHasSnapshot = true
		}

		if project.Spec.GitProvider != "" && ws.Status.UserRef != nil {

			sec, err := c.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
				Name: fmt.Sprintf("git-provider-tokens.%s", ws.Status.UserRef.Name),
			})
			if err == nil {
				zap.L().Debug("Found git-provider-tokens UserSecret")
				if sec.Data != nil && sec.Data.GetAttrs() != nil {
					attrMap := sec.Data.GetAttrs().AsMap()
					gitProviderMapAny, ok := attrMap[ws.Metadata.Uid]
					if ok {
						zap.L().Debug("Found gitProviderInfo in git-provider-tokens UserSecret")
						info := &ccordiumv1.GitProviderInfo{}
						if err := pbutils.UnmarshalFromMap(gitProviderMapAny.(map[string]any), info); err != nil {
							return err
						}

						zap.L().Debug("Found gitProviderInfo in git-provider-tokens UserSecret",
							zap.String("username", info.Username),
							zap.String("email", info.Email),
							zap.Time("createdAt", info.CreatedAt.AsTime()),
							zap.Time("expiresAt", info.ExpiresAt.AsTime()))

						if info.ExpiresAt.IsValid() && info.ExpiresAt.AsTime().Before(time.Now()) {
							zap.L().Debug("gitProviderInfo is expired. Not going using it...")
						} else {

							zap.L().Debug("gitProviderInfo is valid")
							gitProviderInfo = info
						}

					}

				}
			} else if !grpcerr.IsNotFound(err) {
				return err
			}

		}
	}

	if !ws.Status.IsBuild && ws.Status.SessionRef != nil {
		peerPrivKeySecret, err := c.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
			Name: "sys:ws-tunnel-wgkey",
		})
		if err != nil {
			return err
		}

		peerPrivKey, err := wgtypes.ParseKey(ucordiumv1.ToSecret(peerPrivKeySecret).GetValueStr())
		if err != nil {
			return err
		}
		tunnelPeerPublicKey = peerPrivKey.PublicKey().String()

		sess, err := c.octeliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{Uid: ws.Status.SessionRef.Uid})
		if err != nil {
			return err
		}

		accessToken, err = c.jwkCtl.CreateAccessToken(sess)
		if err != nil {
			return err
		}

		refreshToken, err = c.jwkCtl.CreateRefreshToken(sess)
		if err != nil {
			return err
		}
		expiresIn = int64(sess.Status.Authentication.AccessTokenDuration.GetSeconds())

		userSecretList, err = c.getUserSecretList(ctx, ws)
		if err != nil {
			return err
		}

		userConfig, err = c.octeliumC.CordiumC().GetUserConfig(ctx, &rmetav1.GetOptions{
			Name: ovutils.GetUserConfigName(ws.Status.UserRef),
		})
		if err != nil && !grpcerr.IsNotFound(err) {
			return err
		}
	}

	cco, err := c.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	initializeReq := &ccordiumv1.InitializeRequest{
		Workspace: ws,
		Space:     space,
		Template:  template,
		ClientInfo: &ccordiumv1.InitializeRequest_ClientInfo{
			Domain:       ccc.Status.Domain,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresIn,
		},

		TunnelPeerPublicKey: tunnelPeerPublicKey,
		SecretList:          orgSecretList,

		GitProviderInfo:     gitProviderInfo,
		UserSecretList:      userSecretList,
		UserConfig:          userConfig,
		TemplateHasSnapshot: templateHasSnapshot,
		ClusterConfig:       cco,
	}

	zap.L().Debug("Sending an Initialize call", zap.String("name", c.name))
	if err := func() error {
		for i := 0; i < 3000; i++ {
			err := c.doInitialize(ctx, initializeReq)
			if err == nil {
				return nil
			}

			zap.L().Warn("Could not do initialize api call. Trying again....", zap.Error(err))
			time.Sleep(300 * time.Millisecond)
		}
		return errors.Errorf("Could not do initialize api call: %+v", err)
	}(); err != nil {
		return err
	}

	zap.L().Debug("Workspace successfully initialized", zap.String("name", c.name))
	return nil
}

func (c *statusWatcher) doInitialize(ctx context.Context, req *ccordiumv1.InitializeRequest) error {
	ctx, cancelFn := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancelFn()

	_, err := c.wssupC.C().Initialize(ctx, req, grpc.WaitForReady(true))
	return err
}

func (c *statusWatcher) onStopped(ctx context.Context) error {
	c.mu.RLock()
	didOnStopped := c.didOnStopped
	healthCheckErr := c.healthCheckErr
	c.mu.RUnlock()
	if didOnStopped {
		zap.L().Debug("Already did onStopped. Nothing to be done", zap.String("name", c.name))
		return nil
	}
	c.mu.Lock()
	c.didOnStopped = true
	c.mu.Unlock()
	zap.L().Debug("Starting onStopped", zap.String("name", c.name))
	var failure *cordiumv1.Workspace_Status_Failure

	if healthCheckErr {
		failure = &cordiumv1.Workspace_Status_Failure{
			Type: &cordiumv1.Workspace_Status_Failure_HealthCheck_{
				HealthCheck: &cordiumv1.Workspace_Status_Failure_HealthCheck{},
			},
		}
	} else {
		failureResp, err := c.wssupC.C().GetFailure(ctx, &ccordiumv1.GetFailureRequest{})
		if err == nil {
			failure = failureResp.Failure

		} else {
			zap.L().Warn("Could not getFailure. Setting failure to unknown", zap.Error(err), zap.String("name", c.name))
			failure = &cordiumv1.Workspace_Status_Failure{
				Type: &cordiumv1.Workspace_Status_Failure_Unknown_{
					Unknown: &cordiumv1.Workspace_Status_Failure_Unknown{},
				},
			}
		}

		if _, err := c.wssupC.C().ShutdownAck(ctx, &ccordiumv1.ShutdownAckRequest{}); err != nil {
			zap.L().Debug("Could not do shutdown ack", zap.Error(err))
		}
	}

	if failure != nil {
		zap.L().Debug("Found failure", zap.Any("failure", failure), zap.String("name", c.name))
	}

	if sessRef := c.getSessionRef(ctx); sessRef != nil {
		zap.L().Debug("Deleting the Workspace Session",
			zap.String("name", c.name), zap.String("sessUID", sessRef.Uid))
		_, err := c.octeliumC.CoreC().DeleteSession(ctx, &rmetav1.DeleteOptions{Uid: sessRef.Uid})
		if err != nil && !grpcerr.IsNotFound(err) {
			return err
		}
	}

	if err := c.ctl.doOnDeleteK8s(ctx, c.ws); err != nil {
		return err
	}

	if c.ws.Status.IsBuild {
		zap.L().Debug("Workspace is a prebuild. Deleting it", zap.String("name", c.name))

		zap.L().Debug("Getting the prebuild to set it as the current Template prebuild",
			zap.String("uid", c.ws.Status.TemplateRef.Uid))

		tmpl, err := c.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
			Uid: c.ws.Status.TemplateRef.Uid,
		})
		if err != nil {
			if grpcerr.IsNotFound(err) {
				return nil
			}
			return err
		}

		if tmpl.Status.BuildInfo != nil {
			runIdx := slices.IndexFunc(tmpl.Status.BuildInfo.Builds,
				func(b *cordiumv1.Template_Status_BuildInfo_Build) bool {
					return c.ws.Metadata.SystemLabels != nil && b.Id == c.ws.Metadata.SystemLabels["build-id"]
				})
			if runIdx >= 0 {
				run := tmpl.Status.BuildInfo.Builds[runIdx]

				if run.IsCanceled {
					run.State = cordiumv1.Template_Status_BuildInfo_Build_STATE_FAILED
				} else {
					run.DoneAt = pbutils.Now()
					if failure != nil {
						run.State = cordiumv1.Template_Status_BuildInfo_Build_STATE_FAILED
						run.Failure = failure
					} else {
						run.State = cordiumv1.Template_Status_BuildInfo_Build_STATE_READY
						tmpl.Status.BuildInfo.CurrentReadyBuildID = run.Id
					}
				}

				if tmpl.Status.BuildInfo.CurrentRunningBuildID == run.Id {
					tmpl.Status.BuildInfo.CurrentRunningBuildID = ""
				}

				tmpl, err = c.octeliumC.CordiumC().UpdateTemplate(ctx, tmpl)
				if err != nil {
					return err
				}

				if run.State == cordiumv1.Template_Status_BuildInfo_Build_STATE_READY {
					if err := c.ctl.createTemplateSnapshot(ctx, c.ws, tmpl); err != nil {
						zap.L().Warn("Could not createTemplateSnapshot", zap.Error(err))
					}
				}

			}

		}

		return nil
	}

	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid: c.uid,
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	doUpdateWS := false

	if failure != nil {
		ws.Status.Failure = failure

		if run := ucordiumv1.ToWorkspace(ws).GetCurrentRun(); run != nil {
			run.Failure = failure
		}

		if failure.GetHealthCheck() != nil {
			ws.Status.StoppingReason = cordiumv1.Workspace_Status_STOPPING_REASON_ERROR
		}

		doUpdateWS = true
	}

	if doUpdateWS {
		_, err = c.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
		if err != nil {
			return err
		}
	}

	/*
		if ws.Status.IsBuild {
			zap.L().Debug("Deleting build Workspace", zap.String("ws", ws.Metadata.Name))
			_, err := c.octeliumC.CordiumC().DeleteWorkspace(ctx, &rmetav1.DeleteOptions{
				Uid: c.ws.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return err
				}
			}
		}
	*/

	return nil
}

func (c *statusWatcher) getSessionRef(ctx context.Context) *metav1.ObjectReference {
	if c.ws.Status.SessionRef != nil {
		return c.ws.Status.SessionRef
	}

	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: c.uid})
	if err != nil {
		return nil
	}

	return ws.Status.SessionRef
}

func (c *statusWatcher) doStopWorkspace(ctx context.Context) error {
	zap.L().Debug("Stopping the Workspace by the watcher", zap.String("name", c.name))
	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid: c.uid,
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	if ucordiumv1.ToWorkspace(ws).IsStopping() {

		zap.L().Debug("Workspace is already stopped/stopping. No need to stop by the watcher",
			zap.String("name", c.name))
		return nil
	}

	return c.ctl.stopWorkspace(ctx, ws)
}

func (c *statusWatcher) getUserSecretList(ctx context.Context, ws *cordiumv1.Workspace) (*cordiumv1.UserSecretList, error) {
	zap.L().Debug("Starting getting SSH key UserSecrets")
	if ws.Status.UserRef == nil || ws.Status.IsBuild {
		return nil, nil
	}

	secList, err := c.octeliumC.CordiumC().ListUserSecret(ctx, urscsrv.FilterByUserRef(ws.Status.UserRef))
	if err != nil {
		return nil, err
	}

	return secList, nil
}

func (c *statusWatcher) startHealthCheckLoop(ctx context.Context) {
	tickerCh := time.NewTicker(30 * time.Second)
	maxErrs := 60
	defer tickerCh.Stop()
	errN := 0

	zap.L().Debug("Starting startHealthCheckLoop", zap.String("name", c.name))
	defer zap.L().Debug("Exiting startHealthCheckLoop", zap.String("name", c.name))
	for {
		select {
		case <-ctx.Done():

			return
		case <-tickerCh.C:
			err := c.doHealthCheck(ctx)
			if err == nil {
				errN = 0
			} else {
				c.mu.RLock()
				shouldExitLoop := c.didOnStopping || c.didOnStopped
				c.mu.RUnlock()
				if shouldExitLoop {
					return
				}
				zap.L().Warn("Health check error",
					zap.String("name", c.name), zap.Error(err), zap.Int("attempt", errN))
				errN = errN + 1
				if errN >= maxErrs {
					zap.L().Warn("HealthCheck max attempts exceeded. Closing the watcher", zap.Int("max", maxErrs))
					c.setHealthCheckErr()
					c.close()
					return
				}
			}
		}
	}
}

func (c *statusWatcher) doHealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.wssupC.HealthCheck().Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}

	if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
		return nil
	}
	return errors.Errorf("The Workspace supervisor server is not at SERVING mode")
}
