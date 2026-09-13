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

	charness "github.com/octelium/cordium/cluster/e2e/harness"
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

const (
	netProbeSeconds     = 8
	blockedProbeSeconds = 3
)

const timeoutExitCode = 124

const (
	publicDNSAddr   = "8.8.8.8"
	publicAltAddr   = "1.1.1.1"
	metadataAddr    = "169.254.169.254"
	unroutedRFC1918 = "10.213.77.91"
)

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

func egressRule(action cordiumv1.Workspace_Spec_Runtime_Network_Rule_Action,
	cidrs []string, ports ...uint32) *cordiumv1.Workspace_Spec_Runtime_Network_Rule {
	return &cordiumv1.Workspace_Spec_Runtime_Network_Rule{
		Action: action,
		Cidrs:  cidrs,
		Ports:  ports,
	}
}

func allowEgress(cidrs []string, ports ...uint32) *cordiumv1.Workspace_Spec_Runtime_Network_Rule {
	return egressRule(cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW, cidrs, ports...)
}

func denyEgress(cidrs []string, ports ...uint32) *cordiumv1.Workspace_Spec_Runtime_Network_Rule {
	return egressRule(cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY, cidrs, ports...)
}

func egressNetwork(defaultAction cordiumv1.Workspace_Spec_Runtime_Network_Egress_DefaultAction,
	rules ...*cordiumv1.Workspace_Spec_Runtime_Network_Rule) *cordiumv1.Workspace_Spec_Runtime_Network {
	return &cordiumv1.Workspace_Spec_Runtime_Network{
		Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
			DefaultAction: defaultAction,
			Rules:         rules,
		},
	}
}

func readOnlyFilesystem() *cordiumv1.Workspace_Spec_Runtime_Filesystem {
	return &cordiumv1.Workspace_Spec_Runtime_Filesystem{ReadOnly: true}
}

func capabilities(add, drop []string) *cordiumv1.Workspace_Spec_Runtime_Capabilities {
	return &cordiumv1.Workspace_Spec_Runtime_Capabilities{
		Add:  add,
		Drop: drop,
	}
}

func onCreateTask(name, run string) *cordiumv1.Workspace_Spec_Runtime_Task {
	return &cordiumv1.Workspace_Spec_Runtime_Task{
		Name: name,
		Run:  run,
		Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
	}
}

func tcpProbe(addr string, port int32) string {
	return tcpProbeWithin(netProbeSeconds, addr, port)
}

func tcpProbeWithin(seconds int, addr string, port int32) string {
	return fmt.Sprintf("timeout %d bash -c 'exec 3<>/dev/tcp/%s/%d'",
		seconds, addr, port)
}

func sandboxCmdlines(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) string {
	t.Helper()

	return h.MustExec(t, ws,
		"for f in /proc/[0-9]*/cmdline; do tr '\\0' ' ' < $f; echo; done")
}

func workspaceHome(elems ...string) string {
	ret := "${HOME}"
	for _, elem := range elems {
		ret = fmt.Sprintf("%s/%s", ret, elem)
	}

	return ret
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
