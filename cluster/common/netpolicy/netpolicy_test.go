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

func TestValidateEgressLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		set   func(*cordiumv1.Workspace_Spec_Runtime_Network_Rule, int)
		limit int
	}{
		{name: "CIDRs", limit: MaxCIDRsPerRule, set: func(rule *cordiumv1.Workspace_Spec_Runtime_Network_Rule, n int) {
			rule.Cidrs = nil
			for i := 0; i < n; i++ {
				rule.Cidrs = append(rule.Cidrs, "203.0.113.0/24")
			}
		}},
		{name: "ports", limit: MaxPortsPerRule, set: func(rule *cordiumv1.Workspace_Spec_Runtime_Network_Rule, n int) {
			for i := 0; i < n; i++ {
				rule.Ports = append(rule.Ports, uint32(i+1))
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{tc.limit, tc.limit + 1} {
				rule := &cordiumv1.Workspace_Spec_Runtime_Network_Rule{Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW, Cidrs: []string{"203.0.113.0/24"}}
				tc.set(rule, n)
				err := ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{rule}})
				if n == tc.limit {
					assert.Nil(t, err)
				} else {
					assert.NotNil(t, err)
				}
			}
		})
	}
}

func TestValidateEgressUnknownEnumsAndMappedIPv6(t *testing.T) {
	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DefaultAction(99)}))
	assert.NotNil(t, ValidateEgress(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{{Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_Action(99), Cidrs: []string{"203.0.113.0/24"}}}}))
	for _, cidr := range []string{"::ffff:192.0.2.1/128", "::ffff:192.0.2.0/120", "192.0.2.1/33", "2001:db8::/129"} {
		_, err := parsePrefixes([]string{cidr})
		assert.NotNil(t, err, "%s", cidr)
	}
}

func TestCompileStableDenyOrderAndNormalization(t *testing.T) {
	p, err := Compile(&CompileReq{
		UID: 543210,
		Network: &cordiumv1.Workspace_Spec_Runtime_Network{Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW, Cidrs: []string{"203.0.113.129/24", "203.0.113.0/24"}, Ports: []uint32{65535, 1, 1}},
			{Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY, Cidrs: []string{"2001:db8:1::1/64"}},
			{Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY, Cidrs: []string{"192.0.2.1/24"}},
		}}},
	})
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, p.Check())
	assert.Equal(t, "2001:db8:1::/64", p.Rules[0].Prefixes[0].String())
	assert.Equal(t, "192.0.2.0/24", p.Rules[1].Prefixes[0].String())
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}, p.Rules[2].Prefixes)
	assert.Equal(t, []uint32{1, 65535}, p.Rules[2].Ports)
	_, err = Compile(nil)
	assert.NotNil(t, err)
}

func TestPolicyCheckInvalidValues(t *testing.T) {
	var nilPolicy *Policy
	assert.NotNil(t, nilPolicy.Check())
	for _, tc := range []struct {
		name   string
		policy *Policy
	}{
		{name: "unknown rule action", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, Rules: []*Rule{{Action: "INVALID", Prefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}}}}},
		{name: "invalid prefix", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, Rules: []*Rule{{Action: ActionDeny, Prefixes: []netip.Prefix{{}}}}}},
		{name: "noncanonical prefix", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, Rules: []*Rule{{Action: ActionAllow, Prefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")}}}}},
		{name: "invalid protected prefix", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, Protected: []netip.Prefix{{}}}},
		{name: "invalid nonpublic prefix", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, NonPublic: []netip.Prefix{{}}}},
		{name: "zero system port", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, SystemSourcePorts: []*SystemPort{{Protocol: "tcp"}}}},
		{name: "large system port", policy: &Policy{UID: 1, DefaultAction: DefaultActionDeny, SystemSourcePorts: []*SystemPort{{Protocol: "udp", Port: 65536}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { assert.NotNil(t, tc.policy.Check()) })
	}
}

func TestNormalizeAndFilterPrefixes(t *testing.T) {
	input := []netip.Prefix{{}, netip.MustParsePrefix("2001:db8::1/64"), netip.MustParsePrefix("192.0.2.1/24"), netip.MustParsePrefix("192.0.2.0/24")}
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8::/64")}, normalizePrefixes(input))
	assert.Equal(t, []uint32{1, 443, 65535}, normalizePorts([]uint32{65536, 443, 0, 1, 443, 65535}))
	assert.Empty(t, FilterV4(nil))
	assert.Empty(t, FilterV6([]netip.Prefix{{}, netip.MustParsePrefix("::ffff:192.0.2.0/120")}))
}
