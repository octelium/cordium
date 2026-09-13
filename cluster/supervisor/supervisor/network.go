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

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/netpolicy"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const netPolicyDir = "/octelium-net"

const netPolicySocketPath = "/octelium-net/policy.sock"

const netPolicyTimeout = 30 * time.Second

const maxNetPolicyRequestBytes = 1024 * 1024

type reservedPort struct {
	protocol string
	port     int
}

func getReservedPorts() []*reservedPort {
	return []*reservedPort{
		{protocol: "tcp", port: 35921},
		{protocol: "tcp", port: 2022},
		{protocol: "udp", port: workspacecommon.GetWorkspaceTunnelPort()},
	}
}

func getSystemSourcePorts() []*netpolicy.SystemPort {
	var ret []*netpolicy.SystemPort
	for _, port := range getReservedPorts() {
		ret = append(ret, &netpolicy.SystemPort{
			Protocol: port.protocol,
			Port:     uint32(port.port),
		})
	}
	return ret
}

type netPolicyRequest struct {
	Network []byte `json:"network"`
}

type netPolicyResponse struct {
	Error string `json:"error"`
}

func (s *Server) runNetworkPolicyServer(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}
	return s.doRunNetworkPolicyServer(ctx)
}

func (s *Server) doRunNetworkPolicyServer(ctx context.Context) error {

	dir := path.Dir(s.netPolicyPath)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}

	if err := os.Remove(s.netPolicyPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	lis, err := net.Listen("unix", s.netPolicyPath)
	if err != nil {
		return errors.Errorf("Could not listen on the network policy socket: %+v", err)
	}

	if err := os.Chmod(s.netPolicyPath, 0600); err != nil {
		lis.Close()
		return err
	}

	s.netPolicyLis = lis

	zap.L().Debug("The network policy server is now listening",
		zap.String("path", s.netPolicyPath))

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				zap.L().Debug("The network policy listener exited", zap.Error(err))
				return
			}

			go s.handleNetworkPolicyConn(ctx, conn)
		}
	}()

	return nil
}

func (s *Server) handleNetworkPolicyConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(netPolicyTimeout))

	resp := &netPolicyResponse{}

	if err := s.doHandleNetworkPolicyConn(ctx, conn); err != nil {
		zap.L().Error("Could not handle the network policy request", zap.Error(err))
		resp.Error = err.Error()
	}

	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		zap.L().Error("Could not send the network policy response", zap.Error(err))
	}
}

func (s *Server) doHandleNetworkPolicyConn(ctx context.Context, conn net.Conn) error {
	req := &netPolicyRequest{}
	if err := json.NewDecoder(io.LimitReader(conn, maxNetPolicyRequestBytes)).Decode(req); err != nil {
		return errors.Errorf("Could not decode the network policy request: %+v", err)
	}

	policy, err := s.compileNetworkPolicy(req)
	if err != nil {
		return err
	}

	return s.applyNetworkPolicy(ctx, policy)
}

func (s *Server) compileNetworkPolicy(req *netPolicyRequest) (*netpolicy.Policy, error) {
	network := &cordiumv1.Workspace_Spec_Runtime_Network{}
	if len(req.Network) > 0 {
		if err := proto.Unmarshal(req.Network, network); err != nil {
			return nil, errors.Errorf("Could not unmarshal the network spec: %+v", err)
		}
	}

	return netpolicy.Compile(&netpolicy.CompileReq{
		Network:           network,
		UID:               s.octeliumUID,
		Protected:         getLocalProtectedPrefixes(),
		SystemSourcePorts: getSystemSourcePorts(),
	})
}

func (s *Server) applyNetworkPolicy(ctx context.Context, policy *netpolicy.Policy) error {
	script, err := netpolicy.RenderNFT(policy)
	if err != nil {
		return err
	}

	s.netPolicyMu.Lock()
	defer s.netPolicyMu.Unlock()

	zap.L().Debug("Applying the Workspace network policy",
		zap.String("defaultAction", string(policy.DefaultAction)),
		zap.Int("rules", len(policy.Rules)),
		zap.Int("protected", len(policy.Protected)))

	if err := s.runNetworkPolicyScript(ctx, script); err != nil {
		return err
	}

	s.netPolicyApplied = true

	zap.L().Debug("Successfully applied the Workspace network policy")

	return nil
}

func (s *Server) teardownNetworkPolicy(ctx context.Context) error {
	s.netPolicyMu.Lock()
	defer s.netPolicyMu.Unlock()

	if !s.netPolicyApplied {
		return nil
	}

	zap.L().Debug("Tearing down the Workspace network policy")

	if err := s.runNetworkPolicyScript(ctx, netpolicy.RenderNFTTeardown()); err != nil {
		return err
	}

	s.netPolicyApplied = false

	return nil
}

func (s *Server) runNetworkPolicyScript(ctx context.Context, script string) error {
	if s.netPolicyScriptFn != nil {
		return s.netPolicyScriptFn(ctx, script)
	}
	return runNFT(ctx, script)
}

func (s *Server) closeNetworkPolicyServer() {
	if s.netPolicyLis == nil {
		return
	}

	s.netPolicyLis.Close()
	os.Remove(s.netPolicyPath)
}

func runNFT(ctx context.Context, script string) error {
	if _, err := exec.LookPath("nft"); err != nil {
		return errors.Errorf("Could not find the nft binary: %+v", err)
	}

	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)

	out, err := cmd.CombinedOutput()
	if err != nil {
		zap.L().Error("Could not run the nft cmd",
			zap.String("script", script), zap.String("out", string(out)), zap.Error(err))
		return errors.Errorf("Could not run the nft cmd: %s: %+v", string(out), err)
	}

	return nil
}

func (s *Server) setNetworkPolicy(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}
	return s.doSetNetworkPolicy(ctx)
}

func (s *Server) doSetNetworkPolicy(ctx context.Context) error {

	network := s.spec.GetRuntime().GetNetwork()
	if network == nil {
		network = &cordiumv1.Workspace_Spec_Runtime_Network{}
	}

	networkBytes, err := proto.Marshal(network)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, netPolicyTimeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", s.netPolicyPath)
	if err != nil {
		return errors.Errorf("Could not dial the network policy socket: %+v", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	if err := json.NewEncoder(conn).Encode(&netPolicyRequest{
		Network: networkBytes,
	}); err != nil {
		return errors.Errorf("Could not send the network policy request: %+v", err)
	}

	resp := &netPolicyResponse{}
	if err := json.NewDecoder(io.LimitReader(conn, maxNetPolicyRequestBytes)).Decode(resp); err != nil {
		return errors.Errorf("Could not read the network policy response: %+v", err)
	}

	if resp.Error != "" {
		return errors.Errorf("Could not set the network policy: %s", resp.Error)
	}

	zap.L().Debug("The Workspace network policy is now enforced")

	return nil
}

func getLocalProtectedPrefixes() []netip.Prefix {
	var ret []netip.Prefix

	iface, err := getDefaultIface()
	if err != nil {
		zap.L().Warn("Could not get the default interface", zap.Error(err))
		return ret
	}

	addrs, err := netlink.AddrList(iface, netlink.FAMILY_ALL)
	if err != nil {
		zap.L().Warn("Could not list the default interface addrs", zap.Error(err))
		return ret
	}

	for _, addr := range addrs {
		if ip, ok := netip.AddrFromSlice(addr.IP); ok {
			ip = ip.Unmap()
			ret = append(ret, netip.PrefixFrom(ip, ip.BitLen()))
		}
	}

	if route, err := getDefaultRoute(); err == nil && route.Gw != nil {
		if ip, ok := netip.AddrFromSlice(route.Gw); ok {
			ip = ip.Unmap()
			ret = append(ret, netip.PrefixFrom(ip, ip.BitLen()))
		}
	}

	zap.L().Debug("Local protected prefixes", zap.Any("prefixes", ret))

	return ret
}
