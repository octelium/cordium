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
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorkspaceTemplateMerge(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	spc := h.CreateSpace(t, nil)
	sec := h.CreateSecret(t, spc, "template-secret-value")

	taskMarker := workspacePath("e2e-template-task-" + h.Name())

	tmpl := h.CreateTemplate(t, spc, &cordiumv1.Template_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
				envVar("E2E_TEMPLATE", "from-template"),
				envVar("E2E_RENDERED", "${{ vars.GREETING }}"),
				envVarFromSecret("E2E_TEMPLATE_SECRET", sec.Metadata.Name),
			},
			Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
				postStartTask("e2e-template-task",
					fmt.Sprintf("echo ${{ vars.GREETING }} > %s", taskMarker)),
			},
		},
		Vars: []*cordiumv1.Workspace_Spec_Var{
			{Name: "GREETING", Value: "template-value"},
		},
	})

	ws := h.CreateWorkspace(t, &cordiumv1.Workspace{
		Spec: &cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
					envVar("E2E_WORKSPACE", "from-workspace"),
				},
			},
			Vars: []*cordiumv1.Workspace_Spec_Var{
				{Name: "GREETING", Value: "workspace-value"},
			},
		},
		Status: &cordiumv1.Workspace_Status{
			TemplateRef: umetav1.GetObjectReference(tmpl),
		},
	})

	h.StartWorkspaceWithConfig(t, ws, &cordiumv1.StartWorkspaceRequest_Config{
		Vars: []*cordiumv1.Workspace_Spec_Var{
			{Name: "GREETING", Value: "run-value"},
		},
	})

	running := h.WaitWorkspaceRunning(t, ws)

	t.Run("TheRunKeepsTheRequestedConfig", func(t *testing.T) {
		require.NotNil(t, running.Status.Run)
		require.NotNil(t, running.Status.Run.Config)
		require.Len(t, running.Status.Run.Config.Vars, 1)
		assert.Equal(t, "GREETING", running.Status.Run.Config.Vars[0].Name)
		assert.Equal(t, "run-value", running.Status.Run.Config.Vars[0].Value)
	})

	t.Run("TheTemplateAndWorkspaceEnvVarsAreMerged", func(t *testing.T) {
		assert.Equal(t, "from-template", h.MustExec(t, ws, "printenv E2E_TEMPLATE"))
		assert.Equal(t, "from-workspace", h.MustExec(t, ws, "printenv E2E_WORKSPACE"))
	})

	t.Run("TheTemplateSecretIsResolved", func(t *testing.T) {
		assert.Equal(t, "template-secret-value",
			h.MustExec(t, ws, "printenv E2E_TEMPLATE_SECRET"))
	})

	t.Run("TheRunVarsOverrideTheTemplateAndWorkspaceVars", func(t *testing.T) {
		assert.Equal(t, "run-value", h.MustExec(t, ws, "printenv E2E_RENDERED"))
	})

	t.Run("TheVarsAreRenderedInTasks", func(t *testing.T) {
		waitLineCount(t, h, ws, taskMarker, 1)
		assert.Equal(t, "run-value", h.MustExec(t, ws, "cat "+taskMarker))
	})
}
