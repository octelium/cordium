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

	t.Run("a Workspace of another User is found by name", func(t *testing.T) {
		shared := &cordiumv1.Workspace{
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

		err := acache.SetWorkspace(shared)
		assert.Nil(t, err)

		res, err := acache.GetWorkspace(shared.Metadata.Name)
		assert.Nil(t, err)
		assert.Equal(t, shared.Metadata.Uid, res.Metadata.Uid)
	})

	t.Run("a Workspace without a UserRef does not panic", func(t *testing.T) {
		noUsr := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
				Uid:  uuid.New().String(),
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		}

		err := acache.SetWorkspace(noUsr)
		assert.Nil(t, err)

		res, err := acache.GetWorkspace(noUsr.Metadata.Name)
		assert.Nil(t, err)
		assert.Equal(t, noUsr.Metadata.Uid, res.Metadata.Uid)

		err = acache.DeleteWorkspace(noUsr)
		assert.Nil(t, err)
	})
}

func testCacheResource[T umetav1.ResourceObjectI](t *testing.T, obj T, set func(T) error, get func() (T, error), del func(T) error) {
	t.Helper()

	res, err := get()
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, res)
	assert.Nil(t, del(obj))

	expected := pbutils.Clone(obj)
	require.Nil(t, set(obj))
	obj.GetMetadata().Labels["test"] = "changed input"

	res, err = get()
	require.Nil(t, err)
	assert.True(t, pbutils.IsEqual(expected, res))
	assert.NotSame(t, obj, res)

	res.GetMetadata().ResourceVersion = "changed result"
	res.GetMetadata().Labels["test"] = "changed result"
	res, err = get()
	require.Nil(t, err)
	assert.True(t, pbutils.IsEqual(expected, res))

	obj.GetMetadata().ResourceVersion = "updated"
	require.Nil(t, set(obj))
	res, err = get()
	require.Nil(t, err)
	assert.True(t, pbutils.IsEqual(obj, res))

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
		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:   "workspace",
				Uid:    "workspace-uid",
				Labels: map[string]string{"test": "original"},
			},
			Status: &cordiumv1.Workspace_Status{},
		}

		testCacheResource(t, ws, c.SetWorkspace, func() (*cordiumv1.Workspace, error) {
			return c.GetWorkspace(ws.Metadata.Name)
		}, c.DeleteWorkspace)
	})

	t.Run("Space", func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		spc := &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name:   "space",
				Uid:    "space-uid",
				Labels: map[string]string{"test": "original"},
			},
			Status: &cordiumv1.Space_Status{},
		}

		testCacheResource(t, spc, c.SetSpace, func() (*cordiumv1.Space, error) {
			return c.GetSpace(spc.Metadata.Uid)
		}, c.DeleteSpace)
	})

	t.Run("Membership", func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		spcRef := &metav1.ObjectReference{Name: "space", Uid: "space-uid"}
		usrRef := &metav1.ObjectReference{Name: "user", Uid: "user-uid"}
		itm := &cordiumv1.Membership{
			Metadata: &metav1.Metadata{
				Name:   workspacecommon.GetMembershipName(spcRef, usrRef),
				Uid:    "membership-uid",
				Labels: map[string]string{"test": "original"},
			},
			Status: &cordiumv1.Membership_Status{SpaceRef: spcRef, UserRef: usrRef},
		}

		testCacheResource(t, itm, c.SetMembership, func() (*cordiumv1.Membership, error) {
			return c.GetMembership(spcRef, usrRef)
		}, c.DeleteMembership)
	})
}

func TestCacheResourceKinds(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		c, err := NewCache()
		require.Nil(t, err)
		spcRef := &metav1.ObjectReference{Name: "space", Uid: "space-uid"}
		usrRef := &metav1.ObjectReference{Name: "user", Uid: "user-uid"}
		key := workspacecommon.GetMembershipName(spcRef, usrRef)
		ws := &cordiumv1.Workspace{Metadata: &metav1.Metadata{Name: key, Uid: "workspace-uid"}}
		spc := &cordiumv1.Space{Metadata: &metav1.Metadata{Name: "space", Uid: key}}
		itm := &cordiumv1.Membership{Metadata: &metav1.Metadata{Name: key, Uid: "membership-uid"}}

		require.Nil(t, c.SetWorkspace(ws))
		require.Nil(t, c.SetSpace(spc))
		require.Nil(t, c.SetMembership(itm))

		time.Sleep(24 * time.Hour)

		resWS, err := c.GetWorkspace(key)
		require.Nil(t, err)
		assert.True(t, pbutils.IsEqual(ws, resWS))
		resSpace, err := c.GetSpace(key)
		require.Nil(t, err)
		assert.True(t, pbutils.IsEqual(spc, resSpace))
		resMembership, err := c.GetMembership(spcRef, usrRef)
		require.Nil(t, err)
		assert.True(t, pbutils.IsEqual(itm, resMembership))

		assert.Nil(t, c.DeleteWorkspace(ws))
		_, err = c.GetWorkspace(key)
		assert.ErrorIs(t, err, ErrNotFound)
		_, err = c.GetSpace(key)
		assert.Nil(t, err)
		_, err = c.GetMembership(spcRef, usrRef)
		assert.Nil(t, err)

		assert.Nil(t, c.DeleteSpace(spc))
		_, err = c.GetSpace(key)
		assert.ErrorIs(t, err, ErrNotFound)
		_, err = c.GetMembership(spcRef, usrRef)
		assert.Nil(t, err)
	})
}

func TestCacheInstances(t *testing.T) {
	t.Parallel()

	first, err := NewCache()
	require.Nil(t, err)
	ws := &cordiumv1.Workspace{Metadata: &metav1.Metadata{Name: "workspace", Uid: "first"}}
	require.Nil(t, first.SetWorkspace(ws))

	second, err := NewCache()
	require.Nil(t, err)
	res, err := second.GetWorkspace(ws.Metadata.Name)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, res)

	other := pbutils.Clone(ws).(*cordiumv1.Workspace)
	other.Metadata.Uid = "second"
	require.Nil(t, second.SetWorkspace(other))
	res, err = second.GetWorkspace(other.Metadata.Name)
	require.Nil(t, err)
	assert.True(t, pbutils.IsEqual(other, res))
	assert.Nil(t, second.DeleteWorkspace(other))

	res, err = first.GetWorkspace(ws.Metadata.Name)
	require.Nil(t, err)
	assert.True(t, pbutils.IsEqual(ws, res))
}

func TestCacheConcurrent(t *testing.T) {
	t.Parallel()

	c, err := NewCache()
	require.Nil(t, err)

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			ws := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("workspace-%d", i),
					Uid:  uuid.New().String(),
				},
			}
			for j := range 100 {
				ws.Metadata.ResourceVersion = fmt.Sprintf("%d", j)
				assert.Nil(t, c.SetWorkspace(ws))
				res, err := c.GetWorkspace(ws.Metadata.Name)
				assert.Nil(t, err)
				assert.True(t, pbutils.IsEqual(ws, res))
				assert.Nil(t, c.DeleteWorkspace(ws))
				_, err = c.GetWorkspace(ws.Metadata.Name)
				assert.ErrorIs(t, err, ErrNotFound)
			}
		})
	}
	wg.Wait()
}
