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

package workspace

import (
	"sync"

	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"go.uber.org/zap"
)

type eventPublisher struct {
	mu     sync.RWMutex
	subMap map[string]*eventSubscription
}

func (p *eventPublisher) subscribe() *eventSubscription {
	ret := &eventSubscription{
		id:   vutils.UUIDv4(),
		resp: make(chan *ccordiumv1.ListenEventResponse, 1000),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subMap[ret.id] = ret
	return ret
}

func (p *eventPublisher) unsubscribe(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.subMap, id)
}

func (p *eventPublisher) publish(event *ccordiumv1.ListenEventResponse) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, itm := range p.subMap {
		itm.resp <- event
	}
}

type eventSubscription struct {
	id   string
	resp chan *ccordiumv1.ListenEventResponse
}

func (s *Server) ListenEvent(req *ccordiumv1.ListenEventRequest, srv ccordiumv1.WorkspaceService_ListenEventServer) error {

	ctx := srv.Context()

	zap.L().Debug("Starting ListenEvent loop")

	sub := s.eventPublisher.subscribe()
	defer s.eventPublisher.unsubscribe(sub.id)

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting ListenEvent. ctx done")
			return nil
		case msg, ok := <-sub.resp:
			if !ok {
				zap.L().Debug("Exiting ListenTerminal. Subscription ended")
				return nil
			}

			if err := srv.Send(msg); err != nil {
				zap.L().Error("Could not send ListenEventResp",
					zap.Error(err))
			}
		}
	}

}
