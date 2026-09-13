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
	"net/netip"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/stretchr/testify/assert"
)

func TestCompileDefaults(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
	})
	assert.Nil(t, err, "%+v", err)

	assert.Equal(t, DefaultActionAllowPublic, p.DefaultAction)
	assert.Equal(t, 0, len(p.Rules))
	assert.Nil(t, p.Check())

	assert.True(t, hasPrefix(p.Protected, "127.0.0.0/8"))
	assert.True(t, hasPrefix(p.Protected, "169.254.0.0/16"))
	assert.True(t, hasPrefix(p.Protected, "::1/128"))

	assert.True(t, hasPrefix(p.NonPublic, "10.0.0.0/8"))
	assert.True(t, hasPrefix(p.NonPublic, "192.168.0.0/16"))
	assert.True(t, hasPrefix(p.NonPublic, "fc00::/7"))

	assert.False(t, hasPrefix(p.NonPublic, "127.0.0.0/8"))
}

func TestCompileInvalidUID(t *testing.T) {
	_, err := Compile(&CompileReq{})
	assert.NotNil(t, err)

	_, err = Compile(&CompileReq{UID: -1})
	assert.NotNil(t, err)
}

func TestCompileProtected(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Protected: []netip.Prefix{
			netip.MustParsePrefix("10.244.3.7/32"),
			netip.MustParsePrefix("10.244.3.7/32"),
		},
	})
	assert.Nil(t, err, "%+v", err)

	assert.True(t, hasPrefix(p.Protected, "10.244.3.7/32"))
	assert.Equal(t, 1, countPrefix(p.Protected, "10.244.3.7/32"))
}

func TestCompileRules(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
				Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
					{
						Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
						Cidrs:  []string{"10.50.0.0/16"},
						Ports:  []uint32{443, 443, 80},
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

	assert.Equal(t, DefaultActionDeny, p.DefaultAction)
	assert.Equal(t, 2, len(p.Rules))

	assert.Equal(t, ActionDeny, p.Rules[0].Action)
	assert.Equal(t, ActionAllow, p.Rules[1].Action)

	assert.Equal(t, []uint32{80, 443}, p.Rules[1].Ports)
	assert.Nil(t, p.Check())
}

func TestCompileMasksCIDR(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{
			Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
				Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
					{
						Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY,
						Cidrs:  []string{" 10.50.1.7/16 "},
					},
				},
			},
		},
	})
	assert.Nil(t, err, "%+v", err)
	assert.Equal(t, "10.50.0.0/16", p.Rules[0].Prefixes[0].String())
}

func TestValidateEgress(t *testing.T) {
	assert.Nil(t, ValidateEgress(nil))

	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Cidrs: []string{"10.0.0.0/8"},
			},
		},
	}))

	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
			},
		},
	}))

	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
				Cidrs:  []string{"not-a-cidr"},
			},
		},
	}))

	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
				Cidrs:  []string{"10.0.0.0/8"},
				Ports:  []uint32{0},
			},
		},
	}))

	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
				Cidrs:  []string{"10.0.0.0/8"},
				Ports:  []uint32{65536},
			},
		},
	}))

	assert.Nil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
				Cidrs:  []string{"10.0.0.0/8", "fd00::/8"},
				Ports:  []uint32{443, 65535},
			},
		},
	}))
}

func TestValidateEgressTooMany(t *testing.T) {
	var rules []*cordiumv1.Workspace_Spec_Runtime_Network_Rule
	for i := 0; i < MaxRules+1; i++ {
		rules = append(rules, &cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY,
			Cidrs:  []string{"10.0.0.0/8"},
		})
	}

	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: rules,
	}))
}

func TestPolicyCheck(t *testing.T) {
	p := &Policy{
		UID:           543210,
		DefaultAction: DefaultActionAllowPublic,
	}
	assert.Nil(t, p.Check())

	p.UID = 0
	assert.NotNil(t, p.Check())

	p = &Policy{UID: 543210, DefaultAction: "INVALID"}
	assert.NotNil(t, p.Check())

	p = &Policy{
		UID:           543210,
		DefaultAction: DefaultActionDeny,
		Rules: []*Rule{
			{
				Action: ActionAllow,
			},
		},
	}
	assert.NotNil(t, p.Check())

	p = &Policy{
		UID:           543210,
		DefaultAction: DefaultActionDeny,
		SystemSourcePorts: []*SystemPort{
			{Protocol: "sctp", Port: 100},
		},
	}
	assert.NotNil(t, p.Check())
}

func TestFilterFamilies(t *testing.T) {
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("fc00::/7"),
	}

	assert.Equal(t, 1, len(FilterV4(prefixes)))
	assert.Equal(t, 1, len(FilterV6(prefixes)))
	assert.Equal(t, "10.0.0.0/8", FilterV4(prefixes)[0].String())
	assert.Equal(t, "fc00::/7", FilterV6(prefixes)[0].String())
}

func hasPrefix(prefixes []netip.Prefix, arg string) bool {
	return countPrefix(prefixes, arg) > 0
}

func countPrefix(prefixes []netip.Prefix, arg string) int {
	var ret int
	for _, prefix := range prefixes {
		if prefix.String() == arg {
			ret = ret + 1
		}
	}
	return ret
}
