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

package workspace

import (
	"context"
	"fmt"
	"net"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type WgLink struct {
	attr     netlink.LinkAttrs
	linkType string
}

func newWgLink(name string, mtu int) WgLink {
	ret := WgLink{attr: netlink.NewLinkAttrs(), linkType: "wireguard"}
	ret.attr.Name = name
	ret.attr.MTU = mtu

	return ret
}

func (w WgLink) Attrs() *netlink.LinkAttrs {
	return &w.attr
}

func (w WgLink) Type() string {
	return w.linkType
}

const tunnelDevName = "wsdev"
const mtu = 1280

func runTunnel(ctx context.Context, req *ccordiumv1.PrepareRequest) error {
	if req.Workspace.Status.IsBuild {
		zap.L().Debug("This is a prebuild. No need to set run the tunnel")
		return nil
	}

	zap.L().Debug("Starting running ws tunnel")

	if err := startDev(); err != nil {
		return errors.Errorf("Could not start dev: %+v", err)
	}

	link, err := netlink.LinkByName(tunnelDevName)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return errors.Errorf("Could not set link up: %+v", err)
	}

	_, addr, err := net.ParseCIDR("10.100.100.1/32")
	if err != nil {
		return err
	}

	if err := netlink.AddrAdd(link, &netlink.Addr{
		IPNet: addr,
	}); err != nil {
		return errors.Errorf("Could not add addr: %+v", err)
	}

	mainTableIdx, err := func() (int, error) {
		routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
		if err != nil {
			return 0, err
		}

		for _, route := range routes {
			if (route.Dst == nil || route.Dst.String() == "0.0.0.0/0" || route.Dst.String() == "::/0") &&
				route.Src == nil {
				return route.Table, nil
			}
		}

		return 254, nil
	}()
	if err != nil {
		return err
	}

	_, route, err := net.ParseCIDR("10.100.100.0/30")
	if err != nil {
		return err
	}

	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: link.Attrs().Index,
		Scope:     netlink.SCOPE_LINK,
		Table:     mainTableIdx,
		Dst:       route,
	}); err != nil {
		return errors.Errorf("Could not add route: %+v", err)
	}

	privk, err := wgtypes.ParseKey(req.TunnelPrivateKey)
	if err != nil {
		return err
	}

	listenPort := workspacecommon.GetWorkspaceTunnelPort()

	peerPublicKey, err := wgtypes.ParseKey(req.TunnelPeerPublicKey)
	if err != nil {
		return err
	}

	_, peerAddr, err := net.ParseCIDR("10.100.100.2/32")
	if err != nil {
		return err
	}

	wgCfg := wgtypes.Config{
		PrivateKey:   &privk,
		ReplacePeers: true,
		ListenPort:   &listenPort,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: peerPublicKey,
				AllowedIPs: []net.IPNet{
					*peerAddr,
				},
			},
		},
	}

	wgC, err := wgctrl.New()
	if err != nil {
		return err
	}

	if err := wgC.ConfigureDevice(tunnelDevName, wgCfg); err != nil {
		return errors.Errorf("Could not configure wg dev: %+v", err)
	}

	zap.L().Debug("Workspace tunnel successfully running")

	return nil
}

func startDev() error {

	err := startKernelDev()
	if err == nil {
		return nil
	}

	zap.L().Warn("Could not start a kernel dev. Trying with tun", zap.Error(err))
	return startTunDev()
}

func startKernelDev() error {

	link := newWgLink(tunnelDevName, mtu)

	if err := netlink.LinkAdd(link); err != nil {
		return errors.Errorf("Could not add link: %+v", err)
	}

	return nil
}

func startTunDev() error {
	tundev, err := tun.CreateTUN(tunnelDevName, mtu)
	if err != nil {
		return errors.Errorf("Could not create TUN dev %+v", err)
	}

	logger := device.NewLogger(
		device.LogLevelSilent,
		fmt.Sprintf("(%s) ", tunnelDevName),
	)

	fileUAPI, err := ipc.UAPIOpen(tunnelDevName)
	if err != nil {
		return errors.Errorf("Could not open UAPI: %+v", err)
	}

	device := device.NewDevice(tundev, conn.NewDefaultBind(), logger)
	if err := device.Up(); err != nil {
		return err
	}

	uapi, err := ipc.UAPIListen(tunnelDevName, fileUAPI)
	if err != nil {
		return errors.Errorf("Could not listen UAPI: %+v", err)
	}

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := uapi.Accept()
				if err != nil {
					zap.S().Debugf("Could not accept UAPI conn: %+v", err)
					return
				}
				go device.IpcHandle(conn)
			}

		}
	}(context.Background())

	return nil
}
