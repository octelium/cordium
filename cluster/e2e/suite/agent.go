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
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAgent(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	actor := h.NewActor(t)
	c := actor.CordiumC()

	spaceName := fmt.Sprintf("octelium.%s", actor.User.Metadata.Name)

	t.Cleanup(func() {
		spc, err := c.GetSpace(context.Background(), &metav1.GetOptions{Name: spaceName})
		if err != nil {
			return
		}

		c.DeleteSpace(context.Background(), &metav1.DeleteOptions{Uid: spc.Metadata.Uid})
	})

	var agentWS *cordiumv1.Workspace

	t.Run("InitializeAgentProvisionsTheAgent", func(t *testing.T) {
		res, err := c.InitializeAgent(ctx, &cordiumv1.InitializeAgentRequest{})
		require.Nil(t, err)
		require.NotNil(t, res.Status.AgentWorkspaceRef)

		agentWS, err = c.GetWorkspace(ctx, &metav1.GetOptions{Uid: res.Status.AgentWorkspaceRef.Uid})
		require.Nil(t, err)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED, agentWS.Status.State)
		assert.Equal(t, spaceName, agentWS.Status.SpaceRef.Name)
		assert.Equal(t, fmt.Sprintf("cordium-agent.%s", spaceName), agentWS.Status.TemplateRef.Name)
		require.Len(t, agentWS.Spec.Applications, 1)
		assert.True(t, agentWS.Spec.Applications[0].IsDefault)

		spc, err := c.GetSpace(ctx, &metav1.GetOptions{Name: spaceName})
		require.Nil(t, err)
		assert.True(t, spc.Metadata.IsSystem)
		assert.Equal(t, cordiumv1.Space_Status_USER, spc.Status.Type)

		tmpl, err := c.GetTemplate(ctx, &metav1.GetOptions{Uid: agentWS.Status.TemplateRef.Uid})
		require.Nil(t, err)
		require.NotNil(t, tmpl.Spec.Runtime)
		assert.Len(t, tmpl.Spec.Runtime.Tasks, 2)
	})

	require.NotNil(t, agentWS)

	t.Run("InitializeAgentIsIdempotent", func(t *testing.T) {
		res, err := c.InitializeAgent(ctx, &cordiumv1.InitializeAgentRequest{})
		require.Nil(t, err)
		assert.Equal(t, agentWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)

		usrCfg, err := c.GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
		require.Nil(t, err)
		assert.Equal(t, agentWS.Metadata.Uid, usrCfg.Status.AgentWorkspaceRef.Uid)
	})

	t.Run("TheAgentSpaceNameIsReserved", func(t *testing.T) {
		_, err := c.CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{Name: "octelium"},
			Spec:     &cordiumv1.Space_Spec{},
		})
		require.NotNil(t, err)
	})

	t.Run("DeletingTheAgentWorkspaceUnsetsIt", func(t *testing.T) {
		_, err := c.DeleteWorkspace(ctx, &metav1.DeleteOptions{Uid: agentWS.Metadata.Uid})
		require.Nil(t, err)

		usrCfg, err := c.GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
		require.Nil(t, err)
		assert.Nil(t, usrCfg.Status.AgentWorkspaceRef)

		res, err := c.InitializeAgent(ctx, &cordiumv1.InitializeAgentRequest{})
		require.Nil(t, err)
		require.NotNil(t, res.Status.AgentWorkspaceRef)
		assert.NotEqual(t, agentWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)

		agentWS, err = c.GetWorkspace(ctx, &metav1.GetOptions{Uid: res.Status.AgentWorkspaceRef.Uid})
		require.Nil(t, err)
	})

	t.Run("AnotherUserCannotUseTheAgentWorkspace", func(t *testing.T) {
		_, err := h.CordiumC().InitializeAgent(ctx, &cordiumv1.InitializeAgentRequest{
			WorkspaceRef: &metav1.ObjectReference{Uid: agentWS.Metadata.Uid},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)
	})
}
