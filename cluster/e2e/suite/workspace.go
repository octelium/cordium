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

func testWorkspaceAPI(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
				envVar("E2E", "1"),
			},
		},
	})

	t.Run("TheDefaultSpaceAndTemplateAreUsed", func(t *testing.T) {
		assert.NotEmpty(t, ws.Metadata.Name)
		require.NotNil(t, ws.Status.UserRef)
		assert.Equal(t, h.UserName(t), ws.Status.UserRef.Name)

		require.NotNil(t, ws.Status.TemplateRef)
		assert.Equal(t, fmt.Sprintf("default.default.%s", h.UserName(t)),
			ws.Status.TemplateRef.Name)

		require.NotNil(t, ws.Status.SpaceRef)
		assert.Equal(t, fmt.Sprintf("default.%s", h.UserName(t)),
			ws.Status.SpaceRef.Name)

		assert.Equal(t, cordiumv1.Space_Status_USER, ws.Status.SpaceType)
	})

	t.Run("TheWorkspaceStartsOutStopped", func(t *testing.T) {
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED, ws.Status.State)
		assert.Nil(t, ws.Status.RegionRef)
		assert.Nil(t, ws.Status.SessionRef)
		assert.Empty(t, ws.Status.Hostname)
		assert.Zero(t, ws.Status.SuccessfulRuns)
	})

	t.Run("TheLimitsAreSetFromTheCluster", func(t *testing.T) {
		require.NotNil(t, ws.Status.Limit)
		assert.NotNil(t, ws.Status.Limit.Cpu)
		assert.NotNil(t, ws.Status.Limit.Memory)
		assert.NotNil(t, ws.Status.Limit.Storage)
	})

	t.Run("TheWorkspaceIsRetrievableByNameAndUID", func(t *testing.T) {
		byName, err := h.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Name: ws.Metadata.Name})
		require.Nil(t, err)
		assert.Equal(t, ws.Metadata.Uid, byName.Metadata.Uid)

		byUID, err := h.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Uid: ws.Metadata.Uid})
		require.Nil(t, err)
		assert.Equal(t, ws.Metadata.Name, byUID.Metadata.Name)
	})

	t.Run("TheWorkspaceIsListed", func(t *testing.T) {
		res, err := h.CordiumC().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{})
		require.Nil(t, err)

		var found bool
		for _, itm := range res.Items {
			if itm.Metadata.Uid == ws.Metadata.Uid {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("TheSpecIsUpdatable", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		cur.Metadata.DisplayName = "e2e workspace"
		cur.Spec.Runtime.EnvVars = append(cur.Spec.Runtime.EnvVars, envVar("E2E_EXTRA", "2"))

		res, err := h.CordiumC().UpdateWorkspace(ctx, cur)
		require.Nil(t, err)
		assert.Equal(t, "e2e workspace", res.Metadata.DisplayName)
		require.Len(t, res.Spec.Runtime.EnvVars, 2)
		assert.Equal(t, "E2E_EXTRA", res.Spec.Runtime.EnvVars[1].Key)
	})

	t.Run("AnExplicitTemplateIsUsed", func(t *testing.T) {
		spc := h.CreateSpace(t, nil)
		tmpl := h.CreateTemplate(t, spc, &cordiumv1.Template_Spec{
			Image: registryImage(alpineImage),
		})

		other := h.CreateWorkspace(t, &cordiumv1.Workspace{
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			},
		})

		require.NotNil(t, other.Status.TemplateRef)
		assert.Equal(t, tmpl.Metadata.Uid, other.Status.TemplateRef.Uid)
		require.NotNil(t, other.Status.SpaceRef)
		assert.Equal(t, spc.Metadata.Uid, other.Status.SpaceRef.Uid)
	})

	t.Run("ANonExistentTemplateIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("%s.%s.%s", h.Name(), h.Name(), h.UserName(t)),
				},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})

	t.Run("TheListIsFilterableBySpaceAndTemplate", func(t *testing.T) {
		spc := h.CreateSpace(t, nil)
		tmpl := h.CreateTemplate(t, spc, nil)

		scoped := h.CreateWorkspace(t, &cordiumv1.Workspace{
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			},
		})

		bySpace, err := h.CordiumC().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{
			Filter: &cordiumv1.ListWorkspaceOptions_SpaceRef{
				SpaceRef: umetav1.GetObjectReference(spc),
			},
		})
		require.Nil(t, err)
		require.Len(t, bySpace.Items, 1)
		assert.Equal(t, scoped.Metadata.Uid, bySpace.Items[0].Metadata.Uid)

		byTemplate, err := h.CordiumC().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{
			Filter: &cordiumv1.ListWorkspaceOptions_TemplateRef{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			},
		})
		require.Nil(t, err)
		require.Len(t, byTemplate.Items, 1)
		assert.Equal(t, scoped.Metadata.Uid, byTemplate.Items[0].Metadata.Uid)
	})

	t.Run("TheRegionMustAcceptWorkspaces", func(t *testing.T) {
		_, err := h.CordiumC().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
			Config: &cordiumv1.StartWorkspaceRequest_Config{
				RegionRef: &metav1.ObjectReference{Name: h.Name()},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))

		_, err = h.CordiumC().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
			Config: &cordiumv1.StartWorkspaceRequest_Config{
				RegionRef: &metav1.ObjectReference{
					Uid: "00000000-0000-4000-8000-000000000000",
				},
			},
		})
		require.NotNil(t, err)

		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED,
			h.GetWorkspace(t, ws).Status.State)
	})

	t.Run("AStoppedWorkspaceCannotBeStopped", func(t *testing.T) {
		_, err := h.CordiumC().StopWorkspace(ctx, &cordiumv1.StopWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("TheWorkspaceIsDeletable", func(t *testing.T) {
		doomed := h.NewWorkspace(t, &cordiumv1.Workspace_Spec{})

		h.DeleteWorkspace(t, doomed)

		_, err := h.CordiumC().GetWorkspace(ctx,
			&metav1.GetOptions{Uid: doomed.Metadata.Uid})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	})
}

func testWorkspaceValidation(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	create := func(spec *cordiumv1.Workspace_Spec) error {
		_, err := h.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     spec,
		})

		return err
	}

	t.Run("AnEmptyEnvVarKeyIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{envVar("", "value")},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("AnEmptyEnvVarValueIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{envVar("KEY", "")},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("ANonExistentSecretIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
					envVarFromSecret("KEY", h.Name()),
				},
			},
		})
		assert.NotNil(t, err)
	})

	t.Run("AnEmptyTaskCommandIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{postStartTask("empty", "")},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("ATaskWithoutATypeIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
					{
						Name: "no-type",
						Run:  "echo hello",
					},
				},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("AnUnnamedApplicationIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Applications: []*cordiumv1.Workspace_Spec_Application{application("", 8080, false)},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("AnApplicationWithoutAPortIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Applications: []*cordiumv1.Workspace_Spec_Application{application("web", 0, false)},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("DuplicateApplicationNamesAreRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Applications: []*cordiumv1.Workspace_Spec_Application{
				application("web", 8080, false),
				application("web", 9090, false),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("MoreThanOneDefaultApplicationIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Applications: []*cordiumv1.Workspace_Spec_Application{
				application("web", 8080, true),
				application("api", 9090, true),
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("AnEmptyImageURLIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Image: registryImage(""),
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})

	t.Run("AnInvalidCapabilityIsRefused", func(t *testing.T) {
		err := create(&cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Capabilities: &cordiumv1.Workspace_Spec_Runtime_Capabilities{
					Add: []string{"not a capability"},
				},
			},
		})
		require.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))
	})
}
