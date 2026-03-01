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

package broker

import (
	"context"
	"sync"
)

type BrokerConfig[T any] struct {
	SubBufferSize int

	OnDrop func(val T, count int)
}

type Broker[T any] struct {
	cfg       BrokerConfig[T]
	publishCh chan T
	subCh     chan chan T
	unsubCh   chan chan T
	done      chan struct{}
}

func NewBroker[T any](ctx context.Context, cfg BrokerConfig[T]) *Broker[T] {
	if cfg.SubBufferSize <= 0 {
		cfg.SubBufferSize = 16
	}

	b := &Broker[T]{
		cfg:       cfg,
		publishCh: make(chan T),
		subCh:     make(chan chan T),
		unsubCh:   make(chan chan T),
		done:      make(chan struct{}),
	}

	go b.run(ctx)
	return b
}

func (b *Broker[T]) run(ctx context.Context) {
	defer close(b.done)

	subs := make(map[chan T]struct{})

	for {
		select {
		case <-ctx.Done():
			for ch := range subs {
				close(ch)
			}
			return

		case ch := <-b.subCh:
			subs[ch] = struct{}{}

		case ch := <-b.unsubCh:
			if _, ok := subs[ch]; ok {
				delete(subs, ch)
				close(ch)
			}

		case val := <-b.publishCh:
			dropped := 0
			for ch := range subs {
				select {
				case ch <- val:
				default:
					dropped++
				}
			}
			if dropped > 0 && b.cfg.OnDrop != nil {
				b.cfg.OnDrop(val, dropped)
			}
		}
	}
}

func (b *Broker[T]) Publish(val T) bool {
	select {
	case b.publishCh <- val:
		return true
	case <-b.done:
		return false
	}
}

func (b *Broker[T]) Subscribe() (<-chan T, func()) {
	ch := make(chan T, b.cfg.SubBufferSize)

	select {
	case b.subCh <- ch:
	case <-b.done:
		close(ch)
		return ch, func() {}
	}

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			select {
			case b.unsubCh <- ch:
			case <-b.done:
			}
		})
	}

	return ch, unsub
}

func (b *Broker[T]) Wait() {
	<-b.done
}
