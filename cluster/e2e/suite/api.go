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
	"testing"

	"github.com/octelium/cordium/cluster/common/ovutils"
	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCordiumAPI(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	t.Run("TheRegionIsCordiumEnabled", func(t *testing.T) {
		rgn, err := h.CoreC().GetRegion(ctx, &metav1.GetOptions{Name: "default"})
		require.Nil(t, err)
		require.NotNil(t, rgn.Status.Ext)

		ext, ok := rgn.Status.Ext[ovutils.ExtInfoLabel]
		require.True(t, ok)

		extInfo := &cordiumv1.RegionExtInfo{}
		require.Nil(t, pbutils.StructToMessage(ext, extInfo))
		assert.True(t, extInfo.IsEnabled)

		assert.Equal(t, "true", rgn.Metadata.SpecLabels["has-workspace"])
	})

	t.Run("TheCordiumNamespaceIsInstalled", func(t *testing.T) {
		ns, err := h.CoreC().GetNamespace(ctx, &metav1.GetOptions{Name: "cordium"})
		require.Nil(t, err)
		assert.True(t, ns.Metadata.IsSystem)
	})

	t.Run("TheCordiumServicesAreInstalled", func(t *testing.T) {
		for _, name := range []string{
			"default-cordium.octelium-api",
			"default.cordium",
			"default-ssh.cordium",
		} {
			svc, err := h.CoreC().GetService(ctx, &metav1.GetOptions{Name: name})
			require.Nil(t, err, "the Service %s does not exist", name)
			require.NotNil(t, svc.Status.ManagedService)
			assert.NotEmpty(t, svc.Status.ManagedService.Image)
		}
	})

	t.Run("TheRegionIsListed", func(t *testing.T) {
		res, err := h.CordiumC().ListRegion(ctx, &cordiumv1.ListRegionOptions{})
		require.Nil(t, err)
		require.NotEmpty(t, res.Items)

		var found bool
		for _, itm := range res.Items {
			if itm.Metadata.Name == "default" {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("TheUserConfigIsCreatedOnDemand", func(t *testing.T) {
		cfg, err := h.CordiumC().GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
		require.Nil(t, err)
		require.NotNil(t, cfg.Status.UserRef)
		assert.NotEmpty(t, cfg.Status.UserRef.Name)

		again, err := h.CordiumC().GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
		require.Nil(t, err)
		assert.Equal(t, cfg.Metadata.Uid, again.Metadata.Uid)
	})

	t.Run("TheResourcesAreListable", func(t *testing.T) {
		_, err := h.CordiumC().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{})
		assert.Nil(t, err)

		_, err = h.CordiumC().ListSpace(ctx, &cordiumv1.ListSpaceOptions{})
		assert.Nil(t, err)

		_, err = h.CordiumC().ListTemplate(ctx, &cordiumv1.ListTemplateOptions{})
		assert.Nil(t, err)

		_, err = h.CordiumC().ListMembership(ctx, &cordiumv1.ListMembershipOptions{})
		assert.Nil(t, err)

		_, err = h.CordiumC().ListGitProvider(ctx, &cordiumv1.ListGitProviderOptions{})
		assert.Nil(t, err)

		_, err = h.CordiumC().ListUserSecret(ctx, &cordiumv1.ListUserSecretOptions{})
		assert.Nil(t, err)
	})
}

func testCordiumClusterConfig(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	cc := h.ClusterConfig(t)
	require.NotNil(t, cc.Spec)

	t.Run("SpaceOwnershipIsSetByDefault", func(t *testing.T) {
		require.NotNil(t, cc.Spec.Space)
		require.NotNil(t, cc.Spec.Space.Ownership)
		assert.NotEmpty(t, cc.Spec.Space.Ownership.Rules)
	})

	t.Run("TheWorkspaceLimitsCanBeUpdated", func(t *testing.T) {
		before := pbutils.Clone(cc).(*cordiumv1.ClusterConfig)

		t.Cleanup(func() {
			cur := h.ClusterConfig(t)
			cur.Spec = before.Spec
			if _, err := h.ManagementC().UpdateClusterConfig(context.Background(), cur); err != nil {
				t.Errorf("Could not restore the Cordium ClusterConfig: %+v", err)
			}
		})

		cur := h.ClusterConfig(t)
		if cur.Spec.Workspace == nil {
			cur.Spec.Workspace = &cordiumv1.ClusterConfig_Spec_Workspace{}
		}
		cur.Spec.Workspace.Limit = &cordiumv1.ClusterConfig_Spec_Workspace_Limit{
			MaxPerUser:       64,
			MaxActivePerUser: 32,
		}

		res, err := h.ManagementC().UpdateClusterConfig(ctx, cur)
		require.Nil(t, err)
		require.NotNil(t, res.Spec.Workspace)
		require.NotNil(t, res.Spec.Workspace.Limit)
		assert.Equal(t, uint32(64), res.Spec.Workspace.Limit.MaxPerUser)

		after := h.ClusterConfig(t)
		require.NotNil(t, after.Spec.Workspace)
		require.NotNil(t, after.Spec.Workspace.Limit)
		assert.Equal(t, uint32(32), after.Spec.Workspace.Limit.MaxActivePerUser)
	})

	t.Run("AnEmptySpecIsRefused", func(t *testing.T) {
		_, err := h.ManagementC().UpdateClusterConfig(ctx, &cordiumv1.ClusterConfig{})
		assert.NotNil(t, err)
	})
}
