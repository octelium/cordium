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

package supervisor

/*
import (
	"context"
	"fmt"
	"os"

	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
)


func (s *Server) setIPTablesRules(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	{
		zap.L().Debug("Setting iptables rules")

		blockedIPv4Ranges := []string{
			"10.0.0.0/8",
			"192.168.0.0/16",
			"172.16.0.0/12",
			"100.64.0.0/10",
			"169.254.0.0/16",
		}

		cmds := []string{
			fmt.Sprintf("iptables -A OUTPUT -m owner --uid %d -m state --state RELATED,ESTABLISHED -j ACCEPT",
				s.octeliumUID),
			fmt.Sprintf("ip6tables -A OUTPUT -m owner --uid %d -m state --state RELATED,ESTABLISHED -j ACCEPT",
				s.octeliumUID),

			fmt.Sprintf("ip6tables -A OUTPUT -m owner --uid %d -d fc00::/7 -j DROP", s.octeliumUID),
		}

		if defaultRoute, err := getDefaultRoute(); err == nil {
			cmd := fmt.Sprintf("iptables -A OUTPUT -m owner --uid %d -d %s -j ACCEPT",
				s.octeliumUID, defaultRoute.Gw.String())
			cmds = append(cmds, cmd)
		}

		for _, ipv4Range := range blockedIPv4Ranges {
			cmd := fmt.Sprintf("iptables -A OUTPUT -m owner --uid %d -d %s -j DROP",
				s.octeliumUID, ipv4Range)
			cmds = append(cmds, cmd)
		}

		{
			gwIPAddr, err := getDefaultIfaceAddr()
			if err == nil {
				cmd := fmt.Sprintf("iptables -I OUTPUT -m owner --uid %d -d %s -j ACCEPT",
					s.octeliumUID, gwIPAddr.String())
				cmds = append(cmds, cmd)
			}
		}

		for _, cmdStr := range cmds {
			zap.L().Debug("running iptables cmd", zap.String("cmd", cmdStr))
			cmd := getCommand(ctx, cmdStr)

			if ldflags.IsDev() {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}

			if err := cmd.Run(); err != nil {
				zap.S().Errorf("Could not run iptables cmd: %s: %+v", cmdStr, err)
			}
		}

	}

	return nil
}

func (s *Server) unsetIPTablesRules(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	{
		zap.L().Debug("Setting iptables rules")

		blockedIPv4Ranges := []string{
			"10.0.0.0/8",
			"192.168.0.0/16",
			"172.16.0.0/12",
			"100.64.0.0/10",
			"169.254.0.0/16",
		}

		cmds := []string{
			fmt.Sprintf("iptables -D OUTPUT -m owner --uid %d -m state --state RELATED,ESTABLISHED -j ACCEPT",
				s.octeliumUID),
			fmt.Sprintf("ip6tables -D OUTPUT -m owner --uid %d -m state --state RELATED,ESTABLISHED -j ACCEPT",
				s.octeliumUID),

			fmt.Sprintf("ip6tables -D OUTPUT -m owner --uid %d -d fc00::/7 -j DROP", s.octeliumUID),
		}

		if defaultRoute, err := getDefaultRoute(); err == nil {
			cmd := fmt.Sprintf("iptables -D OUTPUT -m owner --uid %d -d %s -j ACCEPT",
				s.octeliumUID, defaultRoute.Gw.String())
			cmds = append(cmds, cmd)
		}

		for _, ipv4Range := range blockedIPv4Ranges {
			cmd := fmt.Sprintf("iptables -D OUTPUT -m owner --uid %d -d %s -j DROP",
				s.octeliumUID, ipv4Range)
			cmds = append(cmds, cmd)
		}

		{
			gwIPAddr, err := getDefaultIfaceAddr()
			if err == nil {
				cmd := fmt.Sprintf("iptables -D OUTPUT -m owner --uid %d -d %s -j ACCEPT",
					s.octeliumUID, gwIPAddr.String())
				cmds = append(cmds, cmd)
			}
		}

		for _, cmdStr := range cmds {
			zap.L().Debug("running iptables cmd", zap.String("cmd", cmdStr))
			cmd := getCommand(ctx, cmdStr)

			if ldflags.IsDev() {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}

			if err := cmd.Run(); err != nil {
				zap.S().Errorf("Could not run iptables cmd: %s: %+v", cmdStr, err)
			}
		}

	}

	return nil
}
*/
