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
)

var mandatoryProtectedCIDRs = []string{
	"0.0.0.0/8",
	"127.0.0.0/8",
	"169.254.0.0/16",

	"::/128",
	"::1/128",
	"fe80::/10",
}

var nonPublicCIDRs = []string{
	"10.0.0.0/8",
	"100.64.0.0/10",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",

	"64:ff9b:1::/48",
	"100::/64",
	"2001::/23",
	"2001:db8::/32",
	"fc00::/7",
	"ff00::/8",
}

func getMandatoryProtectedPrefixes() []netip.Prefix {
	return mustParsePrefixes(mandatoryProtectedCIDRs)
}

func getNonPublicPrefixes() []netip.Prefix {
	return mustParsePrefixes(nonPublicCIDRs)
}

func mustParsePrefixes(cidrs []string) []netip.Prefix {
	ret, err := parsePrefixes(cidrs)
	if err != nil {
		panic(err)
	}
	return ret
}
