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
	"strings"
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSpace(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	spc := h.CreateSpace(t, nil)

	t.Run("TheSpaceIsOwnedByTheUser", func(t *testing.T) {
		require.NotNil(t, spc.Status.UserRef)
		assert.Equal(t, h.UserName(t), spc.Status.UserRef.Name)
		assert.Equal(t, cordiumv1.Space_Status_USER, spc.Status.Type)
		assert.Equal(t, "user", spc.Metadata.SystemLabels["type"])
	})

	t.Run("ADefaultTemplateIsCreated", func(t *testing.T) {
		tmpl := h.GetTemplate(t, fmt.Sprintf("default.%s", spc.Metadata.Name))
		require.NotNil(t, tmpl.Status.SpaceRef)
		assert.Equal(t, spc.Metadata.Uid, tmpl.Status.SpaceRef.Uid)
	})

	t.Run("AnOwnerMembershipIsCreated", func(t *testing.T) {
		mem, err := h.CordiumC().GetSpaceMembership(ctx,
			&cordiumv1.GetSpaceMembershipRequest{
				SpaceRef: umetav1.GetObjectReference(spc),
			})
		require.Nil(t, err)
		assert.Equal(t, cordiumv1.Membership_Spec_OWNER, mem.Spec.Role)
		require.NotNil(t, mem.Status.UserRef)
		assert.Equal(t, h.UserName(t), mem.Status.UserRef.Name)
	})

	t.Run("TheSpaceIsListed", func(t *testing.T) {
		res, err := h.CordiumC().ListSpace(ctx, &cordiumv1.ListSpaceOptions{})
		require.Nil(t, err)

		var found bool
		for _, itm := range res.Items {
			if itm.Metadata.Uid == spc.Metadata.Uid {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("TheNameIsUnique", func(t *testing.T) {
		_, err := h.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{Name: spc.Metadata.Name},
			Spec:     &cordiumv1.Space_Spec{},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.AlreadyExists(err))
	})

	t.Run("AShortNameIsCompletedWithTheUser", func(t *testing.T) {
		name := h.Name()

		res, err := h.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{Name: name},
			Spec:     &cordiumv1.Space_Spec{},
		})
		require.Nil(t, err)

		t.Cleanup(func() {
			h.CordiumC().DeleteSpace(context.Background(),
				&metav1.DeleteOptions{Uid: res.Metadata.Uid})
		})

		assert.Equal(t, h.SpaceName(t, name), res.Metadata.Name)
	})

	t.Run("ASpaceOfAnotherUserIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", h.Name(), h.Name()),
			},
			Spec: &cordiumv1.Space_Spec{},
		})
		assert.NotNil(t, err)
	})

	t.Run("TheSpaceIsDeletable", func(t *testing.T) {
		doomed := h.CreateSpace(t, nil)

		_, err := h.CordiumC().DeleteSpace(ctx,
			&metav1.DeleteOptions{Uid: doomed.Metadata.Uid})
		require.Nil(t, err)

		_, err = h.CordiumC().GetSpace(ctx, &metav1.GetOptions{Uid: doomed.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})
}

func testSpaceMembership(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	spc := h.CreateOrganizationSpace(t, nil)
	usr := h.CreateWorkloadUser(t, nil)

	mem, err := h.CordiumC().CreateMembership(ctx, &cordiumv1.CreateMembershipRequest{
		SpaceRef: umetav1.GetObjectReference(spc),
		Role:     cordiumv1.CreateMembershipRequest_USER,
		UserType: &cordiumv1.CreateMembershipRequest_UserRef{
			UserRef: umetav1.GetObjectReference(usr),
		},
	})
	require.Nil(t, err)
	require.NotNil(t, mem.Status.UserRef)
	assert.Equal(t, usr.Metadata.Uid, mem.Status.UserRef.Uid)
	assert.Equal(t, cordiumv1.Membership_Spec_USER, mem.Spec.Role)

	t.Run("TheSameUserCannotBeAddedTwice", func(t *testing.T) {
		_, err := h.CordiumC().CreateMembership(ctx, &cordiumv1.CreateMembershipRequest{
			SpaceRef: umetav1.GetObjectReference(spc),
			Role:     cordiumv1.CreateMembershipRequest_USER,
			UserType: &cordiumv1.CreateMembershipRequest_UserRef{
				UserRef: umetav1.GetObjectReference(usr),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("TheOwnerCannotAddThemselves", func(t *testing.T) {
		self, err := h.CoreC().GetUser(ctx, &metav1.GetOptions{Name: h.UserName(t)})
		require.Nil(t, err)

		_, err = h.CordiumC().CreateMembership(ctx, &cordiumv1.CreateMembershipRequest{
			SpaceRef: umetav1.GetObjectReference(spc),
			Role:     cordiumv1.CreateMembershipRequest_USER,
			UserType: &cordiumv1.CreateMembershipRequest_UserRef{
				UserRef: umetav1.GetObjectReference(self),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("NocturneSyncsTheUserInfo", func(t *testing.T) {
		displayName := "e2e-" + h.Name()

		usr.Metadata.DisplayName = displayName
		h.UpdateUser(t, usr)

		h.Eventually(t, "the Membership UserInfo to be synchronized",
			charness.PropagationBudget, func(ctx context.Context) error {
				cur, err := h.CordiumC().GetMembership(ctx,
					&metav1.GetOptions{Uid: mem.Metadata.Uid})
				if err != nil {
					return err
				}
				if cur.Status.UserInfo == nil {
					return errors.Errorf("the Membership has no UserInfo")
				}
				if cur.Status.UserInfo.DisplayName != displayName {
					return errors.Errorf("the Membership UserInfo display name is %q, want %q",
						cur.Status.UserInfo.DisplayName, displayName)
				}

				return nil
			})
	})

	t.Run("TheMembershipIsRemovable", func(t *testing.T) {
		_, err := h.CordiumC().DeleteMembership(ctx,
			&metav1.DeleteOptions{Uid: mem.Metadata.Uid})
		require.Nil(t, err)

		_, err = h.CordiumC().GetMembership(ctx,
			&metav1.GetOptions{Uid: mem.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})

	t.Run("MembersCannotBeAddedToUserSpaces", func(t *testing.T) {
		userSpace := h.CreateSpace(t, nil)

		_, err := h.CordiumC().CreateMembership(ctx, &cordiumv1.CreateMembershipRequest{
			SpaceRef: umetav1.GetObjectReference(userSpace),
			Role:     cordiumv1.CreateMembershipRequest_USER,
			UserType: &cordiumv1.CreateMembershipRequest_UserRef{
				UserRef: umetav1.GetObjectReference(usr),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})
}

func testTemplate(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	spc := h.CreateSpace(t, nil)

	tmpl := h.CreateTemplate(t, spc, &cordiumv1.Template_Spec{
		Image: registryImage(alpineImage),
	})

	t.Run("TheTemplateBelongsToTheSpace", func(t *testing.T) {
		require.NotNil(t, tmpl.Status.SpaceRef)
		assert.Equal(t, spc.Metadata.Uid, tmpl.Status.SpaceRef.Uid)
		require.NotNil(t, tmpl.Status.UserRef)
		assert.Equal(t, h.UserName(t), tmpl.Status.UserRef.Name)
	})

	t.Run("TheTemplateIsUpdatable", func(t *testing.T) {
		cur := h.GetTemplate(t, tmpl.Metadata.Name)
		cur.Metadata.DisplayName = "e2e template"
		cur.Spec.Image = registryImage(ubuntuImage)

		res, err := h.CordiumC().UpdateTemplate(ctx, cur)
		require.Nil(t, err)
		assert.Equal(t, "e2e template", res.Metadata.DisplayName)
		require.NotNil(t, res.Spec.Image.GetRegistry())
		assert.Equal(t, ubuntuImage, res.Spec.Image.GetRegistry().Url)
	})

	t.Run("TheTemplateIsListed", func(t *testing.T) {
		res, err := h.CordiumC().ListTemplate(ctx, &cordiumv1.ListTemplateOptions{})
		require.Nil(t, err)

		var found bool
		for _, itm := range res.Items {
			if itm.Metadata.Uid == tmpl.Metadata.Uid {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("AShortNameIsCompletedWithTheSpace", func(t *testing.T) {
		name := h.Name()
		space, _, _ := strings.Cut(spc.Metadata.Name, ".")

		res, err := h.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", name, space),
			},
			Spec: &cordiumv1.Template_Spec{},
		})
		require.Nil(t, err)

		t.Cleanup(func() {
			h.CordiumC().DeleteTemplate(context.Background(),
				&metav1.DeleteOptions{Uid: res.Metadata.Uid})
		})

		assert.Equal(t, fmt.Sprintf("%s.%s", name, spc.Metadata.Name), res.Metadata.Name)
	})

	t.Run("ATemplateInANonExistentSpaceIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s.%s", h.Name(), h.Name(), h.UserName(t)),
			},
			Spec: &cordiumv1.Template_Spec{},
		})
		assert.NotNil(t, err)
	})

	t.Run("TheTemplateIsDeletable", func(t *testing.T) {
		doomed := h.CreateTemplate(t, spc, nil)

		_, err := h.CordiumC().DeleteTemplate(ctx,
			&metav1.DeleteOptions{Uid: doomed.Metadata.Uid})
		require.Nil(t, err)

		_, err = h.CordiumC().GetTemplate(ctx,
			&metav1.GetOptions{Uid: doomed.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})
}
