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
	"fmt"
	"net/netip"
	"strings"
)

const TableName = "cordium"

const chainOutput = "output"

const chainEgress = "egress"

func RenderNFT(p *Policy) (string, error) {
	if err := p.Check(); err != nil {
		return "", err
	}

	var egress []string

	for _, port := range p.SystemSourcePorts {
		egress = append(egress, fmt.Sprintf("meta l4proto %s th sport %d accept",
			port.Protocol, port.Port))
	}

	egress = append(egress, renderMatches(p.Protected, nil, "drop")...)

	for _, rule := range p.Rules {
		verdict := "accept"
		if rule.Action == ActionDeny {
			verdict = "drop"
		}
		egress = append(egress, renderMatches(rule.Prefixes, rule.Ports, verdict)...)
	}

	switch p.DefaultAction {
	case DefaultActionDeny:
		egress = append(egress, "drop")
	default:
		egress = append(egress, renderMatches(p.NonPublic, nil, "drop")...)
		egress = append(egress, "accept")
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "table inet %s\n", TableName)
	fmt.Fprintf(&sb, "delete table inet %s\n\n", TableName)
	fmt.Fprintf(&sb, "table inet %s {\n", TableName)
	fmt.Fprintf(&sb, "\tchain %s {\n", chainOutput)
	fmt.Fprintf(&sb, "\t\ttype filter hook output priority filter; policy accept;\n")
	fmt.Fprintf(&sb, "\t\tmeta skuid != %d accept\n", p.UID)
	fmt.Fprintf(&sb, "\t\tjump %s\n", chainEgress)
	fmt.Fprintf(&sb, "\t}\n\n")
	fmt.Fprintf(&sb, "\tchain %s {\n", chainEgress)
	for _, rule := range egress {
		fmt.Fprintf(&sb, "\t\t%s\n", rule)
	}
	fmt.Fprintf(&sb, "\t}\n")
	fmt.Fprintf(&sb, "}\n")

	return sb.String(), nil
}

func RenderNFTTeardown() string {
	return fmt.Sprintf("table inet %s\ndelete table inet %s\n", TableName, TableName)
}

func renderMatches(prefixes []netip.Prefix, ports []uint32, verdict string) []string {
	var ret []string

	for _, family := range []struct {
		keyword  string
		prefixes []netip.Prefix
	}{
		{keyword: "ip", prefixes: FilterV4(prefixes)},
		{keyword: "ip6", prefixes: FilterV6(prefixes)},
	} {
		if len(family.prefixes) == 0 {
			continue
		}

		match := fmt.Sprintf("%s daddr %s", family.keyword, renderPrefixSet(family.prefixes))
		if len(ports) > 0 {
			match = fmt.Sprintf("%s meta l4proto { tcp, udp } th dport %s",
				match, renderPortSet(ports))
		}

		ret = append(ret, fmt.Sprintf("%s %s", match, verdict))
	}

	return ret
}

func renderPrefixSet(prefixes []netip.Prefix) string {
	var elems []string
	for _, prefix := range prefixes {
		elems = append(elems, prefix.String())
	}
	return fmt.Sprintf("{ %s }", strings.Join(elems, ", "))
}

func renderPortSet(ports []uint32) string {
	var elems []string
	for _, port := range ports {
		elems = append(elems, fmt.Sprintf("%d", port))
	}
	return fmt.Sprintf("{ %s }", strings.Join(elems, ", "))
}
