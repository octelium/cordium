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

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkspaceNetwork(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{})

	supervisorIP := workspacePodIP(t, h, ws)
	clusterIP := workspaceClusterIP(t, h, ws)

	t.Run("TheProbeToolingIsAvailable", func(t *testing.T) {
		for _, tool := range []string{"bash", "timeout", "getent"} {
			res := h.Exec(t, ws, charness.ExecOpts{Command: "command -v " + tool})
			require.Equal(t, int32(0), res.Code,
				"the network probes need %s inside the Workspace", tool)
		}
	})

	t.Run("TheSandboxLoopbackIsNotFiltered", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{Command: tcpProbe("127.0.0.1", 2022)})
		assert.Equal(t, int32(1), res.Code,
			"the sandbox loopback is filtered instead of refusing the connection: %s",
			res.Stderr)
		assert.Contains(t, res.Stderr, "Connection refused")
	})

	t.Run("ThePublicInternetIsReachableByDefault", func(t *testing.T) {
		assertReachable(t, h, ws, publicDNSAddr, 53)
	})

	t.Run("TheSandboxResolvesPublicNames", func(t *testing.T) {
		res := h.Exec(t, ws, charness.ExecOpts{
			Command: "getent hosts one.one.one.one",
		})
		assert.Equal(t, int32(0), res.Code,
			"the sandbox cannot resolve public names: %s", res.Stderr)
	})

	t.Run("TheSupervisorIsNotReachableFromTheSandbox", func(t *testing.T) {
		require.NotEmpty(t, supervisorIP)

		assertBlocked(t, h, ws, supervisorIP, 8080)
	})

	t.Run("TheWorkspaceClusterIPIsNotReachableFromTheSandbox", func(t *testing.T) {
		require.NotEmpty(t, clusterIP)

		assertBlocked(t, h, ws, clusterIP, 8080)
	})

	t.Run("TheNodeMetadataAddressIsNotReachable", func(t *testing.T) {
		assertBlocked(t, h, ws, metadataAddr, 80)
	})

	t.Run("TheKubernetesAPIIsNotReachableFromTheSandbox", func(t *testing.T) {
		apiIP := kubernetesClusterIP(t, h)
		require.NotEmpty(t, apiIP)

		assertBlocked(t, h, ws, apiIP, 443)
	})

	t.Run("ThePrivateNetworksAreDeniedByDefault", func(t *testing.T) {
		assertBlocked(t, h, ws, unroutedRFC1918, 80)
	})

	t.Run("TheWorkspaceKeepsRunningUnderTheFirewall", func(t *testing.T) {
		cur := h.GetWorkspace(t, ws)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, cur.Status.State)
		assert.Nil(t, cur.Status.Failure)

		restarts, err := h.WorkspacePodRestarts(ctx, ws)
		require.Nil(t, err)
		assert.Zero(t, restarts)
	})
}

func testWorkspaceNetworkPolicy(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	ws := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Network: egressNetwork(
				cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
				allowEgress([]string{"0.0.0.0/0", "::/0"}),
				denyEgress([]string{publicAltAddr + "/32"}),
				denyEgress([]string{publicDNSAddr + "/32"}, 443),
			),
		},
	})

	t.Run("ADenyRuleOverridesAnOverlappingAllowAllRule", func(t *testing.T) {
		assertBlocked(t, h, ws, publicAltAddr, 443)
		assertReachable(t, h, ws, publicDNSAddr, 53)
	})

	t.Run("ADenyRuleIsScopedToItsPorts", func(t *testing.T) {
		assertBlocked(t, h, ws, publicDNSAddr, 443)
		assertReachable(t, h, ws, publicDNSAddr, 53)
	})

	t.Run("TheProtectedNetworksSurviveAnAllowAllRule", func(t *testing.T) {
		assertBlocked(t, h, ws, workspacePodIP(t, h, ws), 8080)
		assertBlocked(t, h, ws, metadataAddr, 80)
	})

	h.StopWorkspace(t, ws)
	h.WaitWorkspaceStopped(t, ws)

	h.StartWorkspace(t, ws)
	restarted := h.WaitWorkspaceRunning(t, ws)

	t.Run("ThePolicyIsReappliedOnASubsequentRun", func(t *testing.T) {
		require.Equal(t, uint32(2), restarted.Status.SuccessfulRuns)

		assertBlocked(t, h, ws, publicAltAddr, 443)
		assertReachable(t, h, ws, publicDNSAddr, 53)
		assertBlocked(t, h, ws, workspacePodIP(t, h, ws), 8080)
	})

	h.StopWorkspace(t, ws)
	h.WaitWorkspaceStopped(t, ws)

	cur := h.GetWorkspace(t, ws)
	cur.Spec.Runtime.Network = egressNetwork(
		cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY)
	if _, err := h.CordiumC().UpdateWorkspace(ctx, cur); err != nil {
		t.Fatalf("Could not update the Workspace network policy: %+v", err)
	}

	h.StartWorkspace(t, ws)
	h.WaitWorkspaceRunning(t, ws)

	t.Run("AnUpdatedPolicyTakesEffectOnTheNextRun", func(t *testing.T) {
		assertBlocked(t, h, ws, publicDNSAddr, 53)
	})

	t.Run("AnAllowRulePunchesThroughTheDenyDefault", func(t *testing.T) {
		locked := h.RunWorkspace(t, &cordiumv1.Workspace_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Network: egressNetwork(
					cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
					allowEgress([]string{publicDNSAddr + "/32"}, 53),
				),
			},
		})

		assertReachable(t, h, locked, publicDNSAddr, 53)
		assertBlocked(t, h, locked, publicAltAddr, 443)

		cur := h.GetWorkspace(t, locked)
		assert.Equal(t, cordiumv1.Workspace_Status_RUNNING, cur.Status.State,
			"a Workspace with a locked-down egress policy did not stay running")
		assert.Nil(t, cur.Status.Failure)
	})

	t.Run("TheTemplateRulesAreEnforcedOnTheWorkspace", func(t *testing.T) {
		spc := h.CreateSpace(t, nil)
		tmpl := h.CreateTemplate(t, spc, &cordiumv1.Template_Spec{
			Runtime: &cordiumv1.Workspace_Spec_Runtime{
				Network: egressNetwork(
					cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
					denyEgress([]string{publicAltAddr + "/32"}),
				),
			},
		})

		merged := h.CreateWorkspace(t, &cordiumv1.Workspace{
			Spec: &cordiumv1.Workspace_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					Network: egressNetwork(
						cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
						allowEgress([]string{publicAltAddr + "/32"})),
				},
			},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			},
		})

		h.StartWorkspace(t, merged)
		h.WaitWorkspaceRunning(t, merged)

		assertBlocked(t, h, merged, publicAltAddr, 443)
		assertReachable(t, h, merged, publicDNSAddr, 53)
	})
}

func testWorkspaceNetworkValidation(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	ctx := t.Context()

	newWorkspace := func(network *cordiumv1.Workspace_Spec_Runtime_Network) *cordiumv1.Workspace {
		return &cordiumv1.Workspace{
			Spec: &cordiumv1.Workspace_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{Network: network},
			},
		}
	}

	t.Run("ARuleWithoutAnActionIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateWorkspace(ctx, newWorkspace(egressNetwork(
			cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
			&cordiumv1.Workspace_Spec_Runtime_Network_Rule{
				Cidrs: []string{"10.0.0.0/8"},
			})))
		assert.NotNil(t, err)
	})

	t.Run("ARuleWithoutACIDRIsRefused", func(t *testing.T) {
		_, err := h.CordiumC().CreateWorkspace(ctx, newWorkspace(egressNetwork(
			cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
			denyEgress(nil, 443))))
		assert.NotNil(t, err)
	})

	t.Run("AnInvalidCIDRIsRefused", func(t *testing.T) {
		for _, cidr := range []string{"10.0.0.0", "not-a-cidr", "10.0.0.0/64", ""} {
			_, err := h.CordiumC().CreateWorkspace(ctx, newWorkspace(egressNetwork(
				cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
				denyEgress([]string{cidr}))))
			assert.NotNil(t, err, "the CIDR %q was accepted", cidr)
		}
	})

	t.Run("AnInvalidPortIsRefused", func(t *testing.T) {
		for _, port := range []uint32{0, 65536, 1 << 20} {
			_, err := h.CordiumC().CreateWorkspace(ctx, newWorkspace(egressNetwork(
				cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
				allowEgress([]string{"203.0.113.0/24"}, port))))
			assert.NotNil(t, err, "the port %d was accepted", port)
		}
	})

	t.Run("AValidPolicyIsStored", func(t *testing.T) {
		ws := h.CreateWorkspace(t, newWorkspace(egressNetwork(
			cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
			allowEgress([]string{"203.0.113.0/24", "fd00::/8"}, 443, 8443),
			denyEgress([]string{"203.0.113.128/25"}),
		)))

		cur := h.GetWorkspace(t, ws)
		require.NotNil(t, cur.Spec.Runtime.Network.GetEgress())

		egress := cur.Spec.Runtime.Network.Egress
		assert.Equal(t, cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
			egress.DefaultAction)
		require.Len(t, egress.Rules, 2)
		assert.Equal(t, cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
			egress.Rules[0].Action)
		assert.Equal(t, []uint32{443, 8443}, egress.Rules[0].Ports)
		assert.Equal(t, cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY,
			egress.Rules[1].Action)
		assert.Empty(t, egress.Rules[1].Ports)
	})
}

func assertReachable(t *testing.T, h *charness.H, ws *cordiumv1.Workspace,
	addr string, port int32) {
	t.Helper()

	res := h.Exec(t, ws, charness.ExecOpts{Command: tcpProbe(addr, port)})
	assert.Equal(t, int32(0), res.Code,
		"the Workspace %s cannot reach %s:%d. stderr: %q",
		ws.Metadata.Name, addr, port, res.Stderr)
}

func assertBlocked(t *testing.T, h *charness.H, ws *cordiumv1.Workspace,
	addr string, port int32) {
	t.Helper()

	res := h.Exec(t, ws, charness.ExecOpts{
		Command: tcpProbeWithin(blockedProbeSeconds, addr, port),
	})
	assert.Equal(t, int32(timeoutExitCode), res.Code,
		"the Workspace %s did not have its packets to %s:%d dropped. stderr: %q",
		ws.Metadata.Name, addr, port, res.Stderr)
}

func workspacePodIP(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) string {
	t.Helper()

	var ret string

	h.Eventually(t, "the Workspace to have a single live pod", charness.StopBudget,
		func(ctx context.Context) error {
			pods, err := h.WorkspacePods(ctx, ws)
			if err != nil {
				return err
			}

			var ips []string
			for _, pod := range pods {
				if pod.DeletionTimestamp != nil ||
					pod.Status.Phase != k8scorev1.PodRunning ||
					pod.Status.PodIP == "" {
					continue
				}

				ips = append(ips, pod.Status.PodIP)
			}

			if len(ips) != 1 {
				return errors.Errorf("the Workspace %s has %d live pods",
					ws.Metadata.Name, len(ips))
			}

			ret = ips[0]

			return nil
		})

	return ret
}

func kubernetesClusterIP(t *testing.T, h *charness.H) string {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	svc, err := h.K8sC().CoreV1().Services("default").
		Get(ctx, "kubernetes", k8smetav1.GetOptions{})
	require.Nil(t, err)

	return svc.Spec.ClusterIP
}

func workspaceClusterIP(t *testing.T, h *charness.H, ws *cordiumv1.Workspace) string {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	svc, err := h.K8sC().CoreV1().Services(charness.WorkspaceNamespace).
		Get(ctx, workspaceK8sName(ws), k8smetav1.GetOptions{})
	require.Nil(t, err)

	return svc.Spec.ClusterIP
}
