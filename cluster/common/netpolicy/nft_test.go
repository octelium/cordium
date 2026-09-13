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

package netpolicy

import (
	"strings"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/stretchr/testify/assert"
)

func TestRenderNFTDefaults(t *testing.T) {
	p, err := Compile(&CompileReq{UID: 543210})
	assert.Nil(t, err, "%+v", err)

	out, err := RenderNFT(p)
	assert.Nil(t, err, "%+v", err)

	assert.True(t, strings.Contains(out, "table inet cordium\ndelete table inet cordium"))
	assert.True(t, strings.Contains(out, "type filter hook output priority filter; policy accept;"))
	assert.True(t, strings.Contains(out, "meta skuid != 543210 accept"))
	assert.True(t, strings.Contains(out, "jump egress"))

	assert.True(t, strings.Contains(out, "ip daddr { 0.0.0.0/8, 127.0.0.0/8, 169.254.0.0/16 } drop"))
	assert.True(t, strings.Contains(out, "ip6 daddr { ::/128, ::1/128, fe80::/10 } drop"))

	assert.True(t, strings.Contains(out, "10.0.0.0/8"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(out), "}\n}"))
	assert.True(t, strings.LastIndex(out, "\t\taccept\n") > indexOf(out, "10.0.0.0/8"))
}

func TestRenderNFTDenyByDefault(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
			},
		},
	})
	assert.Nil(t, err, "%+v", err)

	out, err := RenderNFT(p)
	assert.Nil(t, err, "%+v", err)

	assert.False(t, strings.Contains(out, "ip daddr { 10.0.0.0/8"))
	assert.True(t, strings.Contains(out, "\t\tdrop\n"))
	assert.False(t, strings.Contains(out, "\t\taccept\n"))
}

func TestRenderNFTDenyPrecedesAllow(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
					{
						Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
						Cidrs:  []string{"10.50.0.0/16"},
					},
					{
						Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY,
						Cidrs:  []string{"10.50.100.0/24"},
					},
				},
			},
		},
	})
	assert.Nil(t, err, "%+v", err)

	out, err := RenderNFT(p)
	assert.Nil(t, err, "%+v", err)

	assert.True(t, strings.Contains(out, "ip daddr { 10.50.100.0/24 } drop"))
	assert.True(t, strings.Contains(out, "ip daddr { 10.50.0.0/16 } accept"))
	assert.True(t, indexOf(out, "10.50.100.0/24 } drop") < indexOf(out, "10.50.0.0/16 } accept"))
}

func TestRenderNFTProtectedPrecedesRules(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
					{
						Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
						Cidrs:  []string{"0.0.0.0/0"},
					},
				},
			},
		},
	})
	assert.Nil(t, err, "%+v", err)

	out, err := RenderNFT(p)
	assert.Nil(t, err, "%+v", err)

	assert.True(t, indexOf(out, "127.0.0.0/8") < indexOf(out, "0.0.0.0/0 } accept"))
}

func TestRenderNFTPorts(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
					{
						Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
						Cidrs:  []string{"10.50.0.0/16", "fd00:50::/32"},
						Ports:  []uint32{443, 80},
					},
				},
			},
		},
	})
	assert.Nil(t, err, "%+v", err)

	out, err := RenderNFT(p)
	assert.Nil(t, err, "%+v", err)

	assert.True(t, strings.Contains(out,
		"ip daddr { 10.50.0.0/16 } meta l4proto { tcp, udp } th dport { 80, 443 } accept"))
	assert.True(t, strings.Contains(out,
		"ip6 daddr { fd00:50::/32 } meta l4proto { tcp, udp } th dport { 80, 443 } accept"))
}

func TestRenderNFTSystemPorts(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		SystemSourcePorts: []*SystemPort{
			{Protocol: "udp", Port: 45000},
			{Protocol: "tcp", Port: 2022},
		},
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
			},
		},
	})
	assert.Nil(t, err, "%+v", err)

	out, err := RenderNFT(p)
	assert.Nil(t, err, "%+v", err)

	assert.True(t, strings.Contains(out, "meta l4proto udp th sport 45000 accept"))
	assert.True(t, strings.Contains(out, "meta l4proto tcp th sport 2022 accept"))
	assert.True(t, indexOf(out, "th sport 45000") < indexOf(out, "127.0.0.0/8"))
}

func TestRenderNFTInvalidPolicy(t *testing.T) {
	_, err := RenderNFT(nil)
	assert.NotNil(t, err)

	_, err = RenderNFT(&Policy{UID: 543210, DefaultAction: "INVALID"})
	assert.NotNil(t, err)
}

func TestRenderNFTTeardown(t *testing.T) {
	out := RenderNFTTeardown()
	assert.Equal(t, "table inet cordium\ndelete table inet cordium\n", out)
}

func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}
