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

package suite

import (
	"context"
	"fmt"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSecret(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	spc := h.CreateSpace(t, nil)
	sec := h.CreateSecret(t, spc, "e2e-secret-value")

	t.Run("TheSecretBelongsToTheSpace", func(t *testing.T) {
		require.NotNil(t, sec.Status.SpaceRef)
		assert.Equal(t, spc.Metadata.Uid, sec.Status.SpaceRef.Uid)
		require.NotNil(t, sec.Status.UserRef)
		assert.Equal(t, h.UserName(t), sec.Status.UserRef.Name)
	})

	t.Run("TheValueIsNotReadableBack", func(t *testing.T) {
		cur, err := h.CordiumC().GetSecret(ctx, &metav1.GetOptions{Uid: sec.Metadata.Uid})
		require.Nil(t, err)
		assert.Nil(t, cur.Data)

		res, err := h.CordiumC().ListSecret(ctx, &cordiumv1.ListSecretOptions{
			SpaceRef: umetav1.GetObjectReference(spc),
		})
		require.Nil(t, err)

		var found bool
		for _, itm := range res.Items {
			if itm.Metadata.Uid != sec.Metadata.Uid {
				continue
			}
			found = true
			assert.Nil(t, itm.Data)
		}
		assert.True(t, found)
	})

	t.Run("TheNameIsUnique", func(t *testing.T) {
		_, err := h.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
			Metadata: &metav1.Metadata{Name: sec.Metadata.Name},
			Spec:     &cordiumv1.Secret_Spec{},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{Value: "another-value"},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("AnEmptySecretIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", h.Name(), spc.Metadata.Name),
			},
			Spec: &cordiumv1.Secret_Spec{},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("ASecretInANonExistentSpaceIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s.%s", h.Name(), h.Name(), h.UserName(t)),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{Value: "value"},
			},
		})
		assert.NotNil(t, err)
	})

	t.Run("TheSecretIsDeletable", func(t *testing.T) {
		doomed := h.CreateSecret(t, spc, "doomed")

		_, err := h.CordiumC().DeleteSecret(ctx,
			&metav1.DeleteOptions{Uid: doomed.Metadata.Uid})
		require.Nil(t, err)

		_, err = h.CordiumC().GetSecret(ctx, &metav1.GetOptions{Uid: doomed.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})
}

func testUserSecret(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	sec := h.CreateUserSecret(t, "e2e-user-secret-value")

	t.Run("TheUserSecretBelongsToTheUser", func(t *testing.T) {
		require.NotNil(t, sec.Status.UserRef)
		assert.Equal(t, h.UserName(t), sec.Status.UserRef.Name)
	})

	t.Run("TheUserSecretIsListed", func(t *testing.T) {
		res, err := h.CordiumC().ListUserSecret(ctx, &cordiumv1.ListUserSecretOptions{})
		require.Nil(t, err)

		var found bool
		for _, itm := range res.Items {
			if itm.Metadata.Uid == sec.Metadata.Uid {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("TheValueIsNotReadableBack", func(t *testing.T) {
		cur, err := h.CordiumC().GetUserSecret(ctx, &metav1.GetOptions{Uid: sec.Metadata.Uid})
		require.Nil(t, err)
		assert.Equal(t, sec.Metadata.Name, cur.Metadata.Name)
		assert.Nil(t, cur.Data)
	})

	t.Run("AShortNameIsCompletedWithTheUser", func(t *testing.T) {
		name := h.Name()

		res, err := h.CordiumC().CreateUserSecret(ctx, &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{Name: name},
			Spec:     &cordiumv1.UserSecret_Spec{},
			Data: &cordiumv1.UserSecret_Data{
				Type: &cordiumv1.UserSecret_Data_Value{Value: "value"},
			},
		})
		require.Nil(t, err)

		t.Cleanup(func() {
			h.CordiumC().DeleteUserSecret(context.Background(),
				&metav1.DeleteOptions{Uid: res.Metadata.Uid})
		})

		assert.Equal(t, fmt.Sprintf("%s.%s", name, h.UserName(t)), res.Metadata.Name)
	})

	t.Run("TheUserSecretIsDeletable", func(t *testing.T) {
		doomed := h.CreateUserSecret(t, "doomed")

		_, err := h.CordiumC().DeleteUserSecret(ctx,
			&metav1.DeleteOptions{Uid: doomed.Metadata.Uid})
		require.Nil(t, err)

		_, err = h.CordiumC().GetUserSecret(ctx,
			&metav1.GetOptions{Uid: doomed.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})
}
