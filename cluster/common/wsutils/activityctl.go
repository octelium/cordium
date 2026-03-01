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
