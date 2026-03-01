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

package portal

import (
	"sync"

	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"go.uber.org/zap"
)

type workspaceMap struct {
	wsMap map[string]*workspaceCtx
	mu    sync.RWMutex
}

func (w *workspaceMap) removeByName(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if wsCtx, ok := w.wsMap[name]; ok {
		wsCtx.close()
		delete(w.wsMap, name)
	}

	return nil
}

func (w *workspaceMap) remove(ws *cordiumv1.Workspace) error {
	return w.removeByName(ws.Metadata.Name)
}

type workspaceCtx struct {
	// uid string

	c  *suputils.WorkspaceSupClient
	mu sync.Mutex

	// hasListenEvent bool
	ws       *cordiumv1.Workspace
	isClosed bool
}

func (c *workspaceCtx) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		return nil
	}
	c.isClosed = true

	zap.L().Debug("Closing wsCtx", zap.String("name", c.ws.Metadata.Name))

	if c.c != nil {
		c.c.Close()
	}

	return nil
}
