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
