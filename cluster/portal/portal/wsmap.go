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
