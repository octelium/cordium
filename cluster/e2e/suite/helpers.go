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
	"slices"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const capRootTUN = scenario.CapRootTUN

const (
	alpineImage = "docker.io/library/alpine:3.21"
	ubuntuImage = "docker.io/library/ubuntu:24.04"
)

const workspaceDir = "/workspace"

func registryImage(url string) *cordiumv1.Workspace_Spec_Image {
	return &cordiumv1.Workspace_Spec_Image{
		Type: &cordiumv1.Workspace_Spec_Image_Registry_{
			Registry: &cordiumv1.Workspace_Spec_Image_Registry{
				Url: url,
			},
		},
	}
}

func envVar(key, value string) *cordiumv1.Workspace_Spec_Runtime_EnvVar {
	return &cordiumv1.Workspace_Spec_Runtime_EnvVar{
		Key:  key,
		Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: value},
	}
}

func envVarFromSecret(key, secret string) *cordiumv1.Workspace_Spec_Runtime_EnvVar {
	return &cordiumv1.Workspace_Spec_Runtime_EnvVar{
		Key:  key,
		Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret{FromSecret: secret},
	}
}

func postStartTask(name, run string) *cordiumv1.Workspace_Spec_Runtime_Task {
	return &cordiumv1.Workspace_Spec_Runtime_Task{
		Name: name,
		Run:  run,
		Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
	}
}

func application(name string, port int32, isDefault bool) *cordiumv1.Workspace_Spec_Application {
	return &cordiumv1.Workspace_Spec_Application{
		Name:      name,
		Port:      port,
		IsDefault: isDefault,
	}
}

func workspacePath(elems ...string) string {
	ret := workspaceDir
	for _, elem := range elems {
		ret = fmt.Sprintf("%s/%s", ret, elem)
	}

	return ret
}

func workspaceK8sName(ws *cordiumv1.Workspace) string {
	return fmt.Sprintf("ws-%s", ws.Metadata.Name)
}

func workspacePVCName(ws *cordiumv1.Workspace) string {
	return fmt.Sprintf("ws-%s", ws.Metadata.Uid)
}

var workspaceStateOrder = []cordiumv1.Workspace_Status_State{
	cordiumv1.Workspace_Status_INIT_REQUEST,
	cordiumv1.Workspace_Status_INITIALIZING,
	cordiumv1.Workspace_Status_PULLING_IMAGE,
	cordiumv1.Workspace_Status_BUILDING_IMAGE,
	cordiumv1.Workspace_Status_STARTING_RUNTIME,
	cordiumv1.Workspace_Status_PREPARING,
	cordiumv1.Workspace_Status_RUNNING,
	cordiumv1.Workspace_Status_STOPPING_REQUEST,
	cordiumv1.Workspace_Status_STOPPING,
	cordiumv1.Workspace_Status_STOPPED,
}

func assertStateOrder(t *testing.T, states []cordiumv1.Workspace_Status_State) {
	t.Helper()

	last := -1
	for i, state := range states {
		if i == 0 && state == cordiumv1.Workspace_Status_STOPPED {
			continue
		}

		idx := slices.Index(workspaceStateOrder, state)
		require.GreaterOrEqual(t, idx, 0,
			"the Workspace reported the unexpected state %s in %v", state, states)
		assert.Greater(t, idx, last,
			"the Workspace reported the state %s out of order in %v", state, states)
		last = idx
	}
}

func statesAfter(all, before []cordiumv1.Workspace_Status_State) []cordiumv1.Workspace_Status_State {
	if len(all) <= len(before) {
		return nil
	}

	return all[len(before):]
}

func containsAny(states []cordiumv1.Workspace_Status_State,
	wanted ...cordiumv1.Workspace_Status_State) bool {
	for _, state := range wanted {
		if slices.Contains(states, state) {
			return true
		}
	}

	return false
}
