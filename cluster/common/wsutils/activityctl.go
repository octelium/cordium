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

package wsutils

import (
	"context"
	"sync"
	"time"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
)

type ActivityCtl struct {
	mu          sync.Mutex
	activityMap map[string]time.Time
	wsCh        chan string
	duration    time.Duration
	octeliumC   octeliumc.ClientInterface
}

func NewActivityCtl(octeliumC octeliumc.ClientInterface) (*ActivityCtl, error) {
	ret := &ActivityCtl{
		activityMap: make(map[string]time.Time),
		wsCh:        make(chan string, 2000),
		duration:    5 * time.Minute,
		octeliumC:   octeliumC,
	}

	return ret, nil
}

func (c *ActivityCtl) Set(workspaceUID string) {
	c.wsCh <- workspaceUID
}

func (c *ActivityCtl) Run(ctx context.Context) error {
	go c.startCheckLoop(ctx)
	go c.startWsLoop(ctx)
	return nil
}

func (c *ActivityCtl) startWsLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case wsUID, ok := <-c.wsCh:
			if !ok {
				return
			}

			c.mu.Lock()
			c.activityMap[wsUID] = time.Now()
			c.mu.Unlock()
		}
	}
}

func (c *ActivityCtl) startCheckLoop(ctx context.Context) {
	tickerCh := time.NewTicker(c.duration)
	defer tickerCh.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerCh.C:
			if err := c.doCheck(ctx); err != nil {
				zap.L().Debug("Could not doCheck", zap.Error(err))
			}
		}
	}
}

func (c *ActivityCtl) doCheck(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for wsUID, lastActivity := range c.activityMap {

		zap.L().Debug("Updating last activity of Workspace",
			zap.String("uid", wsUID), zap.Time("lastActivity", lastActivity))

		ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
			Uid: wsUID,
		})
		if err != nil {
			if grpcerr.IsNotFound(err) {
				delete(c.activityMap, wsUID)
			} else {
				zap.L().Warn("ActivityCtl: Could not get Workspace",
					zap.String("uid", wsUID), zap.Error(err))
			}
			continue
		}

		ws.Status.LastActivityAt = pbutils.Timestamp(lastActivity)
		_, err = c.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
		if err != nil && !grpcerr.IsNotFound(err) {
			zap.L().Warn("ActivityCtl: Could not update Workspace",
				zap.String("uid", wsUID), zap.Error(err))
		} else {
			delete(c.activityMap, wsUID)
		}

	}

	return nil
}
