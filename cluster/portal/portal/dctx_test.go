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
	"context"
	"testing"
	"time"

	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestHandleClientMsgListenTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	regionRef := &metav1.ObjectReference{
		Name: utilrand.GetRandomStringCanonical(8),
		Uid:  vutils.UUIDv4(),
	}

	c := &dctx{
		id: utilrand.GetRandomStringCanonical(8),
		usrRef: &metav1.ObjectReference{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		},
		supClientMap: suputils.NewSupervisorCtxMap(regionRef),
		msgSrvCh:     make(chan *cordiumv1.ServerMessage, 16),
		terminalMap: terminalMap{
			terminalMap: make(map[string]*terminal),
		},
	}

	msg := &cordiumv1.ClientMessage{
		Type: &cordiumv1.ClientMessage_ListenTerminalRequest{
			ListenTerminalRequest: &cordiumv1.ListenTerminalRequest{
				Id: "unknown-a1b2",
			},
		},
	}

	err := c.handleClientMsg(ctx, msg)
	assert.NotNil(t, err)

	doneCh := make(chan struct{})
	go func() {
		c.terminalMap.mu.Lock()
		c.terminalMap.mu.Unlock()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("The terminalMap mutex is still held after a failed ListenTerminal request")
	}

	assert.Equal(t, 0, len(c.terminalMap.terminalMap))
}
