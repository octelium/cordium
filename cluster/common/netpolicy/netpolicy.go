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
	"slices"
	"strings"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/pkg/errors"
)

const MaxRules = 64

const MaxCIDRsPerRule = 64

const MaxPortsPerRule = 64

type Action string

const (
	ActionAllow Action = "ALLOW"
	ActionDeny  Action = "DENY"
)

type DefaultAction string

const (
	DefaultActionAllowPublic DefaultAction = "ALLOW_PUBLIC"
	DefaultActionDeny        DefaultAction = "DENY"
)

type Rule struct {
	Action   Action         `json:"action"`
	Prefixes []netip.Prefix `json:"prefixes"`
	Ports    []uint32       `json:"ports"`
}

type SystemPort struct {
	Protocol string `json:"protocol"`
	Port     uint32 `json:"port"`
}

type Policy struct {
	UID               int            `json:"uid"`
	DefaultAction     DefaultAction  `json:"defaultAction"`
	Rules             []*Rule        `json:"rules"`
	Protected         []netip.Prefix `json:"protected"`
	NonPublic         []netip.Prefix `json:"nonPublic"`
	SystemSourcePorts []*SystemPort  `json:"systemSourcePorts"`
}

type CompileReq struct {
	Network           *cordiumv1.Workspace_Spec_Runtime_Network
	UID               int
	Protected         []netip.Prefix
	SystemSourcePorts []*SystemPort
}

func Compile(req *CompileReq) (*Policy, error) {
	if req == nil {
		return nil, errors.Errorf("Nil req")
	}

	if req.UID <= 0 {
		return nil, errors.Errorf("Invalid uid: %d", req.UID)
	}

	egress := req.Network.GetEgress()

	if err := ValidateEgress(egress); err != nil {
		return nil, err
	}

	ret := &Policy{
		UID:               req.UID,
		DefaultAction:     getDefaultAction(egress.GetDefaultAction()),
		Protected:         normalizePrefixes(append(getMandatoryProtectedPrefixes(), req.Protected...)),
		NonPublic:         normalizePrefixes(getNonPublicPrefixes()),
		SystemSourcePorts: req.SystemSourcePorts,
	}

	for _, rule := range egress.GetRules() {
		prefixes, err := parsePrefixes(rule.GetCidrs())
		if err != nil {
			return nil, err
		}

		ret.Rules = append(ret.Rules, &Rule{
			Action:   getAction(rule.GetAction()),
			Prefixes: normalizePrefixes(prefixes),
			Ports:    normalizePorts(rule.GetPorts()),
		})
	}

	slices.SortStableFunc(ret.Rules, func(a, b *Rule) int {
		if a.Action == b.Action {
			return 0
		}
		if a.Action == ActionDeny {
			return -1
		}
		return 1
	})

	return ret, nil
}

func (p *Policy) Check() error {
	if p == nil {
		return errors.Errorf("Nil Policy")
	}

	if p.UID <= 0 {
		return errors.Errorf("Invalid Policy uid: %d", p.UID)
	}

	switch p.DefaultAction {
	case DefaultActionAllowPublic, DefaultActionDeny:
	default:
		return errors.Errorf("Invalid Policy defaultAction: %s", p.DefaultAction)
	}

	if len(p.Rules) > MaxRules {
		return errors.Errorf("Too many Policy rules: %d", len(p.Rules))
	}

	for _, rule := range p.Rules {
		switch rule.Action {
		case ActionAllow, ActionDeny:
		default:
			return errors.Errorf("Invalid Policy rule action: %s", rule.Action)
		}

		if len(rule.Prefixes) == 0 {
			return errors.Errorf("Policy rule has no prefixes")
		}

		if len(rule.Prefixes) > MaxCIDRsPerRule {
			return errors.Errorf("Too many Policy rule prefixes: %d", len(rule.Prefixes))
		}

		if len(rule.Ports) > MaxPortsPerRule {
			return errors.Errorf("Too many Policy rule ports: %d", len(rule.Ports))
		}

		for _, prefix := range rule.Prefixes {
			if !prefix.IsValid() || prefix.Addr() != prefix.Masked().Addr() {
				return errors.Errorf("Invalid Policy rule prefix: %s", prefix.String())
			}
		}

		for _, port := range rule.Ports {
			if port < 1 || port > 65535 {
				return errors.Errorf("Invalid Policy rule port: %d", port)
			}
		}
	}

	for _, prefix := range append(slices.Clone(p.Protected), p.NonPublic...) {
		if !prefix.IsValid() || prefix.Addr() != prefix.Masked().Addr() {
			return errors.Errorf("Invalid Policy prefix: %s", prefix.String())
		}
	}

	for _, port := range p.SystemSourcePorts {
		switch port.Protocol {
		case "tcp", "udp":
		default:
			return errors.Errorf("Invalid Policy system port protocol: %s", port.Protocol)
		}

		if port.Port < 1 || port.Port > 65535 {
			return errors.Errorf("Invalid Policy system port: %d", port.Port)
		}
	}

	return nil
}

func ValidateEgress(egress *cordiumv1.Workspace_Spec_Runtime_Network_Egress) error {
	if egress == nil {
		return nil
	}

	if len(egress.Rules) > MaxRules {
		return errors.Errorf("Too many egress rules")
	}

	switch egress.DefaultAction {
	case cordiumv1.Workspace_Spec_Runtime_Network_Egress_DEFAULT_ACTION_UNSET,
		cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC,
		cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY:
	default:
		return errors.Errorf("Invalid egress defaultAction")
	}

	for _, rule := range egress.Rules {
		switch rule.Action {
		case cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
			cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY:
		default:
			return errors.Errorf("The action of an egress rule must be set to either ALLOW or DENY")
		}

		if len(rule.Cidrs) == 0 {
			return errors.Errorf("An egress rule must have at least one CIDR")
		}

		if len(rule.Cidrs) > MaxCIDRsPerRule {
			return errors.Errorf("Too many CIDRs in an egress rule")
		}

		if len(rule.Ports) > MaxPortsPerRule {
			return errors.Errorf("Too many ports in an egress rule")
		}

		if _, err := parsePrefixes(rule.Cidrs); err != nil {
			return err
		}

		for _, port := range rule.Ports {
			if port < 1 || port > 65535 {
				return errors.Errorf("Invalid egress rule port: %d", port)
			}
		}
	}

	return nil
}

func parsePrefixes(cidrs []string) ([]netip.Prefix, error) {
	var ret []netip.Prefix

	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			return nil, errors.Errorf("Invalid CIDR: %s", cidr)
		}

		if prefix.Addr().Is4In6() {
			return nil, errors.Errorf("IPv4-mapped IPv6 CIDRs are not supported: %s", cidr)
		}

		ret = append(ret, prefix.Masked())
	}

	return ret, nil
}

func normalizePrefixes(prefixes []netip.Prefix) []netip.Prefix {
	var ret []netip.Prefix

	for _, prefix := range prefixes {
		if !prefix.IsValid() {
			continue
		}
		prefix = prefix.Masked()
		if !slices.Contains(ret, prefix) {
			ret = append(ret, prefix)
		}
	}

	slices.SortFunc(ret, func(a, b netip.Prefix) int {
		return strings.Compare(a.String(), b.String())
	})

	return ret
}

func normalizePorts(ports []uint32) []uint32 {
	var ret []uint32

	for _, port := range ports {
		if port < 1 || port > 65535 {
			continue
		}
		if !slices.Contains(ret, port) {
			ret = append(ret, port)
		}
	}

	slices.Sort(ret)

	return ret
}

func getAction(arg cordiumv1.Workspace_Spec_Runtime_Network_Rule_Action) Action {
	switch arg {
	case cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW:
		return ActionAllow
	default:
		return ActionDeny
	}
}

func getDefaultAction(arg cordiumv1.Workspace_Spec_Runtime_Network_Egress_DefaultAction) DefaultAction {
	switch arg {
	case cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY:
		return DefaultActionDeny
	default:
		return DefaultActionAllowPublic
	}
}

func FilterV4(prefixes []netip.Prefix) []netip.Prefix {
	var ret []netip.Prefix
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			ret = append(ret, prefix)
		}
	}
	return ret
}

func FilterV6(prefixes []netip.Prefix) []netip.Prefix {
	var ret []netip.Prefix
	for _, prefix := range prefixes {
		if prefix.Addr().Is6() && !prefix.Addr().Is4In6() {
			ret = append(ret, prefix)
		}
	}
	return ret
}
