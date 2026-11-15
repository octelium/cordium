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

package acache

import (
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	t.Parallel()

	acache, err := NewCache()
	assert.Nil(t, err)

	{
		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
				Uid:  uuid.New().String(),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				UserRef: &metav1.ObjectReference{
					Name: utilrand.GetRandomStringCanonical(8),
					Uid:  uuid.New().String(),
				},
			},
		}

		err = acache.SetWorkspace(ws)
		assert.Nil(t, err)

		res, err := acache.GetWorkspace(ws.Metadata.Name)
		assert.Nil(t, err)
		assert.Equal(t, res.Metadata.Uid, ws.Metadata.Uid)
	}

	{
		sess := &corev1.Session{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
				Uid:  uuid.New().String(),
			},
			Spec: &corev1.Session_Spec{},
			Status: &corev1.Session_Status{
				UserRef: &metav1.ObjectReference{
					Name: utilrand.GetRandomStringCanonical(8),
					Uid:  uuid.New().String(),
				},
			},
		}

		err = acache.SetSession(sess)
		assert.Nil(t, err)

		res, err := acache.GetSession(sess.Metadata.Uid)
		assert.Nil(t, err)
		assert.Equal(t, res.Metadata.Uid, sess.Metadata.Uid)
	}
}

func testCacheResource[T umetav1.ResourceObjectI](t *testing.T, obj T, set func(T) error, get func() (T, error), del func(T) error) {
	t.Helper()

	res, err := get()
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, res)
	assert.Nil(t, del(obj))

	require.Nil(t, set(obj))
	res, err = get()
	require.Nil(t, err)
	assert.Same(t, obj, res)

	updated := pbutils.Clone(obj).(T)
	updated.GetMetadata().ResourceVersion = "updated"
	require.Nil(t, set(updated))
	res, err = get()
	require.Nil(t, err)
	assert.Same(t, updated, res)

	assert.Nil(t, del(obj))
	res, err = get()
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, res)
	assert.Nil(t, del(obj))
}

func TestCacheResources(t *testing.T) {
	t.Parallel()

	t.Run("Workspace", func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		ws := &cordiumv1.Workspace{Metadata: &metav1.Metadata{Name: "workspace", Uid: "workspace-uid"}}

		testCacheResource(t, ws, c.SetWorkspace, func() (*cordiumv1.Workspace, error) {
			return c.GetWorkspace(ws.Metadata.Name)
		}, c.DeleteWorkspace)
	})

	t.Run("Space", func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		spc := &cordiumv1.Space{Metadata: &metav1.Metadata{Name: "space", Uid: "space-uid"}}

		testCacheResource(t, spc, c.SetSpace, func() (*cordiumv1.Space, error) {
			return c.GetSpace(spc.Metadata.Uid)
		}, c.DeleteSpace)
	})

	t.Run("Session", func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		sess := &corev1.Session{Metadata: &metav1.Metadata{Name: "session", Uid: "session-uid"}}

		testCacheResource(t, sess, c.SetSession, func() (*corev1.Session, error) {
			return c.GetSession(sess.Metadata.Uid)
		}, c.DeleteSession)
	})

	t.Run("Membership", func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		spcRef := &metav1.ObjectReference{Name: "space", Uid: "space-uid"}
		usrRef := &metav1.ObjectReference{Name: "user", Uid: "user-uid"}
		itm := &cordiumv1.Membership{Metadata: &metav1.Metadata{
			Name: workspacecommon.GetMembershipName(spcRef, usrRef),
			Uid:  "membership-uid",
		}}

		testCacheResource(t, itm, c.SetMembership, func() (*cordiumv1.Membership, error) {
			return c.GetMembership(spcRef, usrRef)
		}, c.DeleteMembership)
	})
}

func TestCacheResourceKindsAndExpiration(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		spcRef := &metav1.ObjectReference{Name: "space", Uid: "space-uid"}
		usrRef := &metav1.ObjectReference{Name: "user", Uid: "user-uid"}
		key := workspacecommon.GetMembershipName(spcRef, usrRef)
		ws := &cordiumv1.Workspace{Metadata: &metav1.Metadata{Name: key, Uid: "workspace-uid"}}
		spc := &cordiumv1.Space{Metadata: &metav1.Metadata{Name: "space", Uid: key}}
		sess := &corev1.Session{Metadata: &metav1.Metadata{Name: "session", Uid: key}}
		itm := &cordiumv1.Membership{Metadata: &metav1.Metadata{Name: key, Uid: "membership-uid"}}

		require.Nil(t, c.SetWorkspace(ws))
		require.Nil(t, c.SetSpace(spc))
		require.Nil(t, c.SetSession(sess))
		require.Nil(t, c.SetMembership(itm))

		time.Sleep(24 * time.Hour)

		resWS, err := c.GetWorkspace(key)
		require.Nil(t, err)
		assert.Same(t, ws, resWS)
		resSpace, err := c.GetSpace(key)
		require.Nil(t, err)
		assert.Same(t, spc, resSpace)
		resSession, err := c.GetSession(key)
		require.Nil(t, err)
		assert.Same(t, sess, resSession)
		resMembership, err := c.GetMembership(spcRef, usrRef)
		require.Nil(t, err)
		assert.Same(t, itm, resMembership)

		assert.Nil(t, c.DeleteWorkspace(ws))
		_, err = c.GetWorkspace(key)
		assert.ErrorIs(t, err, ErrNotFound)
		_, err = c.GetSpace(key)
		assert.Nil(t, err)
		_, err = c.GetSession(key)
		assert.Nil(t, err)
		_, err = c.GetMembership(spcRef, usrRef)
		assert.Nil(t, err)

		assert.Nil(t, c.DeleteSpace(spc))
		_, err = c.GetSpace(key)
		assert.ErrorIs(t, err, ErrNotFound)
		_, err = c.GetSession(key)
		assert.Nil(t, err)
		_, err = c.GetMembership(spcRef, usrRef)
		assert.Nil(t, err)

		assert.Nil(t, c.DeleteSession(sess))
		_, err = c.GetSession(key)
		assert.ErrorIs(t, err, ErrNotFound)
		_, err = c.GetMembership(spcRef, usrRef)
		assert.Nil(t, err)
	})
}

func TestCacheConcurrent(t *testing.T) {
	t.Parallel()

	c, err := NewCache()
	require.Nil(t, err)

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			ws := &cordiumv1.Workspace{Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("workspace-%d", i),
				Uid:  uuid.New().String(),
			}}
			for range 100 {
				assert.Nil(t, c.SetWorkspace(ws))
				res, err := c.GetWorkspace(ws.Metadata.Name)
				assert.Nil(t, err)
				assert.Same(t, ws, res)
				assert.Nil(t, c.DeleteWorkspace(ws))
				_, err = c.GetWorkspace(ws.Metadata.Name)
				assert.ErrorIs(t, err, ErrNotFound)
			}
		})
	}
	wg.Wait()
}
