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

package supervisor

import (
	"io"
	"sync"
	"time"

	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
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

func (s *Server) runListenEventLoop() {
	if err := s.startListenEvent(); err != nil {
		if errors.Is(err, io.EOF) || grpcerr.IsUnavailable(err) {
			return
		}
		zap.L().Error("Could not listen to events of the Workspace container", zap.Error(err))
	}
}

func (s *Server) startListenEvent() error {

	errN := 0
	flr, err := s.wsC.ListenEvent(s.ctxMain, &ccordiumv1.ListenEventRequest{})
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return errors.Errorf("Could not do listenEvent: %+v", err)
	}

	zap.L().Debug("Starting listenEvent loop")
	defer zap.L().Debug("Exiting listenEvent loop")
	for {
		select {
		case <-s.ctxMain.Done():
			return nil
		default:
			msg, err := flr.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					zap.L().Debug("Received EOF on listenEvent. Exiting the loop.")
					return nil
				}
				errN = errN + 1
				if errN > 100 {
					return err
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if err := s.handleEvent(msg); err != nil {
				zap.L().Error("Could not handle event", zap.Error(err), zap.Any("event", msg))
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
}

func (s *Server) handleEvent(event *ccordiumv1.ListenEventResponse) error {

	switch event.Type.(type) {
	case *ccordiumv1.ListenEventResponse_Failure:
		zap.L().Debug("Got failure from inside the Workspace container",
			zap.Any("failure", event.GetFailure()))
		s.setFailure(event.GetFailure())
	default:

	}

	s.eventPublisher.publish(event)

	return nil
}
