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

package mains

import (
	"context"
	"sync"
	"testing"
	"time"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type tstWatchWorkspaceStream struct {
	grpc.ServerStream
	ctx     context.Context
	blockCh chan struct{}

	mu   sync.Mutex
	msgs []*cordiumv1.WatchWorkspaceResponse
}

func (s *tstWatchWorkspaceStream) Context() context.Context {
	return s.ctx
}

func (s *tstWatchWorkspaceStream) Send(msg *cordiumv1.WatchWorkspaceResponse) error {
	if s.blockCh != nil {
		<-s.blockCh
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, msg)

	return nil
}

func (s *tstWatchWorkspaceStream) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs)
}

func TestWatchWorkspace(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	newWorkspaceMsg := func() *cordiumv1.WatchWorkspaceResponse {
		return &cordiumv1.WatchWorkspaceResponse{
			Type: &cordiumv1.WatchWorkspaceResponse_Create_{
				Create: &cordiumv1.WatchWorkspaceResponse_Create{
					Item: &cordiumv1.Workspace{
						Metadata: &metav1.Metadata{
							Name: utilrand.GetRandomStringCanonical(8),
						},
					},
				},
			},
		}
	}

	usrRef := umetav1.GetObjectReference(usr.Usr)
	wsRef := &metav1.ObjectReference{
		Uid: utilrand.GetRandomString(16),
	}

	waitForSubs := func(n int) {
		for range 50 {
			if srv.wsWatchMan.len() == n {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	t.Run("a User with no Workspaces can watch", func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(usr.Ctx())
		defer cancelFn()

		strm := &tstWatchWorkspaceStream{ctx: ctx}

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.WatchWorkspace(&cordiumv1.WatchWorkspaceRequest{}, strm)
		}()

		waitForSubs(1)
		assert.Equal(t, 1, srv.wsWatchMan.len())

		err := srv.wsWatchMan.publishMsg(newWorkspaceMsg(), usrRef, wsRef)
		assert.Nil(t, err)

		for range 50 {
			if strm.len() > 0 {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.Equal(t, 1, strm.len())

		cancelFn()
		assert.Nil(t, <-errCh)

		waitForSubs(0)
		assert.Equal(t, 0, srv.wsWatchMan.len())
	})

	t.Run("a blocked subscriber is dropped and does not block the publisher", func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(usr.Ctx())
		defer cancelFn()

		blockCh := make(chan struct{})
		defer close(blockCh)

		strm := &tstWatchWorkspaceStream{ctx: ctx, blockCh: blockCh}

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.WatchWorkspace(&cordiumv1.WatchWorkspaceRequest{}, strm)
		}()

		waitForSubs(1)
		assert.Equal(t, 1, srv.wsWatchMan.len())

		startedAt := time.Now()
		for range watchWorkspaceBufferSize + 10 {
			err := srv.wsWatchMan.publishMsg(newWorkspaceMsg(), usrRef, wsRef)
			assert.Nil(t, err)
		}
		assert.Less(t, time.Since(startedAt), 10*time.Second)

		assert.Nil(t, <-errCh)
		assert.Equal(t, 0, srv.wsWatchMan.len())
	})
}
