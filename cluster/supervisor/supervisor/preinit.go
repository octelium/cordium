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
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func (s *Server) runPreInitCommands(ctx context.Context) error {

	if s.isInner || ldflags.IsTest() {
		if err := s.runPreInitCommandsInner(ctx); err != nil {
			return err
		}
	} else {
		if err := s.runPreInitCommandsOuter(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) runPreInitCommandsInner(ctx context.Context) error {
	wgPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	s.wgPrivateKey = &wgPrivateKey
	s.wgPublicKey = s.wgPrivateKey.PublicKey().String()

	if ldflags.IsTest() {
		return nil
	}

	zap.L().Debug("Starting running inner init root cmds...")

	cmds := []string{
		`mkdir -p /sys/fs/cgroup/init`,
		`xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs || :`,
		`sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers > /sys/fs/cgroup/cgroup.subtree_control`,
		// "mkdir -p /home/octelium",
		// "chown -R octelium:octelium /home/octelium",
		// "podman --log-level=debug version",
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running inner init cmd", zap.String("cmd", cmdStr))
		cmd := getCommand(ctx, cmdStr)

		if ldflags.IsDev() {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if ldflags.IsTest() {
			continue
		}

		if err := cmd.Run(); err != nil {
			zap.S().Errorf("Could not run init cmd: %s: %+v", cmdStr, err)
		}
	}

	return nil
}

func (s *Server) runPreInitCommandsOuter(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	zap.L().Debug("Starting running outer init root cmds...")

	if err := s.runPreInitCommandsRoot(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Server) runPreInitCommandsRoot(ctx context.Context) error {

	cmds := []string{
		"setcap cap_setuid+ep /usr/bin/newuidmap",
		"setcap cap_setgid+ep /usr/bin/newgidmap",
		fmt.Sprintf("groupadd -g %d octelium", s.octeliumGID),
		fmt.Sprintf("useradd -g octelium -u %d -m -d /home/octelium octelium", s.octeliumUID),
		fmt.Sprintf("echo octelium:%d:65536 > /etc/subuid", s.octeliumUID+1),
		fmt.Sprintf("echo octelium:%d:65536 > /etc/subgid", s.octeliumGID+1),
		"mkdir -p /home/octelium/.local/share/containers",
		// "chown -R octelium:octelium /home/octelium",
		"mkdir -p /octelium/workspace",
		"mkdir -p /octelium/workspace/.octelium",
		"mkdir -p /octelium/workspace/repo",
		"mkdir -p /octelium/workspace/additional-repos",

		"mkdir -p /octelium/sockets",

		"mkdir -p /octelium/podman",
		"mkdir -p /octelium/podman/root",
		"mkdir -p /octelium/podman/tmp-cp",
		"mkdir -p /octelium/podman/tmp-storage",
		"mkdir -p /octelium/podman/tmp-dir",
		"mkdir -p /octelium/podman/libpod",
		"mkdir -p /octelium/podman/storage",
		"mkdir -p /octelium/podman/storage-rootless",
		"mkdir -p /octelium/podman/dind",
		"mkdir -p /octelium/podman/dind/docker",
		"mkdir -p /octelium/podman/dind/containers",

		"mkdir -p /octelium/podman/tmp",
		"mkdir -p /octelium/podman/tmp/tmp",
		"mkdir -p /octelium/podman/tmp/var",
		"mkdir -p /octelium/podman/tmp/var/tmp",

		"mkdir -p /octelium/outer/var/tmp",
		"mkdir -p /octelium/outer/tmp",
		"mkdir -p /octelium/outer/runtime",
		"mkdir -p /octelium/outer/home/octelium",
		"chmod 1777 /octelium/outer/var/tmp",
		"chmod 1777 /octelium/outer/tmp",
		"chmod 1777 /octelium/outer/runtime",

		"mkdir -p /octelium/home",

		"mkdir -p /tmp/octelium",
		"mkdir -p /tmp/podman-conf",
		"chmod 1777 /tmp/octelium",
		"chmod 1755 /tmp/podman-conf",
		"cp -r /etc/containers /tmp/podman-conf",

		fmt.Sprintf("mkdir -p %s", s.buildDir),

		// "chown -R octelium:octelium /octelium",

		"chmod 1777 /octelium/podman/tmp/tmp",
		"chmod 1777 /octelium/podman/tmp/var/tmp",

		fmt.Sprintf("mkdir -p %s", s.getCGroupRoot()),
		fmt.Sprintf(`echo "+memory +cpu +io +pids" > %s`, path.Join(s.getCGroupRoot(), "cgroup.subtree_control")),
		// fmt.Sprintf("mount -t cgroup2 none %s", s.getCGroupRoot()),

		// `echo "unqualified-search-registries = [\"docker.io\"]" >> /etc/containers/registries.conf`,

		"mkdir -p /dev/net",
		"mknod /dev/net/tun-octelium0 c 10 200",
		"chmod 666 /dev/net/tun-octelium0",

		/*
			"rm -rf /octelium/podman/tmp-storage/*",
			"rm -rf /octelium/podman/tmp-dir/*",

			"rm -rf /octelium/podman/storage-rootless/libpod",
			"rm -rf /octelium/podman/storage-rootless/userns.lock",
			"rm -rf /octelium/podman/storage-rootless/*.lock",

			"rm -rf /octelium/podman/storage-rootless/overlay-containers/containers.lock",
			"rm -rf /octelium/podman/storage-rootless/overlay-layers/layers.lock",
			"rm -rf /octelium/podman/storage-rootless/libpod/bolt_state.db.lock",
		*/

		/*
			`find /octelium/podman/storage-rootless -name "*.pid" -delete 2>/dev/null || true`,
			`find /octelium/podman/storage-rootless -name "*.lock" -delete 2>/dev/null || true`,
			`find /octelium/podman/storage-rootless -name "ctl" -delete 2>/dev/null || true`,
		*/
	}

	if ldflags.IsDev() {
		devCmds := []string{
			"podman --root=/octelium-root info",
			"find /bin -perm -4000",
			"find /usr/bin -perm -4000",
			"find /sbin -perm -4000",
			fmt.Sprintf("ls -la %s", s.getCGroupRoot()),
		}
		cmds = append(cmds, devCmds...)
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running init cmd", zap.String("cmd", cmdStr))
		cmd := getCommand(ctx, cmdStr)

		if ldflags.IsDev() {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if ldflags.IsTest() {
			continue
		}

		if err := cmd.Run(); err != nil {
			zap.S().Errorf("Could not run init cmd: %s: %+v", cmdStr, err)
		}
	}

	{
		if err := os.WriteFile("/tmp/podman-conf/containers/containers.conf", []byte(podmanConfContainers), 0644); err != nil {
			return err
		}

		if err := os.WriteFile("/tmp/podman-conf/containers/registries.conf", []byte(podmanConfRegistries), 0644); err != nil {
			return err
		}

		if err := os.WriteFile("/tmp/podman-conf/containers/storage.conf", []byte(podmanConfStorage), 0644); err != nil {
			return err
		}
	}

	if err := s.prepareTUN(); err != nil {
		return err
	}

	if err := s.prepareFuse(ctx); err != nil {
		return err
	}

	if err := s.setSeccompProfileOuter(); err != nil {
		return err
	}

	/*
		if err := s.replaceInFile(
			"/etc/containers/storage.conf",
			`# rootless_storage_path = "$HOME/.local/share/containers/storage"`,
			`rootless_storage_path = "/octelium/podman/root"`,
		); err != nil {
			return err
		}
	*/

	/*
		if err := s.setIPTablesRules(ctx); err != nil {
			zap.L().Error("Could not set iptables rules", zap.Error(err))
		}
	*/

	return nil
}

func getDefaultRoute() (*netlink.Route, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if (route.Dst == nil || route.Dst.String() == "0.0.0.0/0" || route.Dst.String() == "::/0") &&
			route.Src == nil {
			return &route, nil
		}
	}

	return nil, errors.Errorf("Could not find default interface")
}

func getDefaultIface() (netlink.Link, error) {
	defaultRoute, err := getDefaultRoute()
	if err != nil {
		return nil, err
	}

	return netlink.LinkByIndex(defaultRoute.LinkIndex)

}

func getDefaultIfaceAddr() (*net.IP, error) {
	iface, err := getDefaultIface()
	if err != nil {
		return nil, err
	}
	addrs, err := netlink.AddrList(iface, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}

	if len(addrs) < 1 {
		return nil, errors.Errorf("No addrs for the default interface")
	}

	return &addrs[0].IP, nil
}

func (s *Server) prepareTUN() error {
	const devNetTunScript = `
#!/bin/sh

mkdir -p /dev/net
mknod /dev/net/tun c 10 200
chmod 666 /dev/net/tun
`

	zap.L().Debug("Preparing tun dev")
	zap.S().Debugf("Checking whether /dev/net/tun exists")
	_, err := os.Stat("/dev/net/tun")
	if err == nil {
		zap.S().Debugf("/dev/net/tun exists. No mknod needed")
		if err := os.Chmod("/dev/net/tun", 0666); err != nil {
			zap.L().Warn("Could not chmod tun dev", zap.Error(err))
		}
		return nil
	}

	if !os.IsNotExist(err) {
		return err
	}

	zap.L().Debug("mknoding /dev/net/tun")

	if err := os.WriteFile("/tmp/install_dev_net_tun.sh", []byte(devNetTunScript), 0755); err != nil {
		return err
	}
	if out, err := exec.Command("/bin/sh", "-c", "/tmp/install_dev_net_tun.sh").CombinedOutput(); err != nil {
		zap.S().Debugf("command out: %s", string(out))
		return errors.Errorf("Could not install /dev/net/tun device: %+v", err)
	}

	zap.L().Debug("Successfully mknoded tun dev")

	return nil
}

func (s *Server) prepareFuse(ctx context.Context) error {

	zap.L().Debug("mknoding /dev/octelium-fuse")

	cmds := []string{
		"mknod /dev/octelium-fuse c 10 229",
		"chmod 666 /dev/octelium-fuse",
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running init cmd", zap.String("cmd", cmdStr))
		cmd := getCommand(ctx, cmdStr)

		if ldflags.IsDev() {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if ldflags.IsTest() {
			continue
		}

		if err := cmd.Run(); err != nil {
			zap.S().Errorf("Could not run init cmd: %s: %+v", cmdStr, err)
		}
	}

	zap.L().Debug("Successfully mknoded fuse dev")

	return nil
}

func (s *Server) runOuterPodman(ctx context.Context) error {
	argList := []string{
		// "-d",
		"--root=/octelium-root",
		"--name=inner",

		// `--entrypoint="tini","--"`,
		"--init",
		"--env=OCTELIUM_RUN_LAYER=INNER0",
		// "--env-host",

		"-v /usr:/usr:ro,nosuid",
		"-v /bin:/bin:ro,nosuid",
		"-v /sbin:/sbin:ro,nosuid",
		"-v /lib:/lib:ro,nosuid",
		"-v /lib64:/lib64:ro,nosuid",

		"-v /etc/passwd:/etc/passwd:ro",
		"-v /etc/shadow:/etc/shadow:ro",
		"-v /etc/group:/etc/group:ro",
		"-v /etc/subuid:/etc/subuid:ro",
		"-v /etc/subgid:/etc/subgid:ro",

		"-v /usr/bin/newuidmap:/usr/bin/newuidmap:ro",
		"-v /usr/bin/newgidmap:/usr/bin/newgidmap:ro",

		"-v /octelium/outer/var/tmp:/var/tmp:noexec,nosuid",
		"-v /octelium/outer/tmp:/tmp:noexec,nosuid",
		"-v /octelium/outer/home:/home/octelium:noexec,nosuid",
		// "-v /octelium/outer/runtime:/octelium-runtime",

		"-v /octelium:/octelium:nosuid",
		"-v /tmp/podman-conf/containers:/etc/containers:ro",

		"--device=/dev/net/tun",
		"--device=/dev/net/tun-octelium0",

		// "--device=/dev/octelium-fuse:/dev/fuse",

		// "--tmpfs /tmp:rw,mode=1777",

		// "--tmpfs /run:rw,mode=1777",

		"--net host",
		`--security-opt=unmask=/sys/fs/cgroup --security-opt=unmask="/proc/*"`,
		`--security-opt=seccomp=/etc/containers/seccomp-octelium.json`,
		`--security-opt=proc-opts=hidepid=2`,

		"--read-only",
		"--cap-add fowner,chown,kill,dac_override,fsetid,setuid,setgid",
		"--cap-drop ALL",
		"--runtime=crun",

		fmt.Sprintf("--cgroup-manager=cgroupfs --cgroup-parent=%s", s.getRelativePathOuterCgroup()),
	}

	if ldflags.IsDev() && ovutils.IsPrivateRegistry() && vutils.FSPathExists("/etc/regcred.json") {
		argList = append(argList, "-v /etc/regcred.json:/etc/regcred.json")
	}

	if ldflags.IsDev() {
		argList = append(argList, "--log-level=debug")
		argList = append(argList, "--env=OCTELIUM_DEV=true")
		argList = append(argList, "-v /etc/sudoers:/etc/sudoers:ro")
	}

	if ldflags.IsDev() {
		argList = append(argList, "--env=GRPC_GO_LOG_VERBOSITY_LEVEL=99")
		argList = append(argList, "--env=GRPC_GO_LOG_SEVERITY_LEVEL=info")
	}

	ctrCmd := "cordium-supervisor"

	cmdStr := fmt.Sprintf("podman run %s --rootfs /root/rootfs %s", strings.Join(argList, " "), ctrCmd)

	zap.L().Debug("Starting running outer podman", zap.String("cmd", cmdStr))

	cmd := s.getCommandAsRoot(ctx, cmdStr)
	if err := cmd.Start(); err != nil {
		return errors.Errorf("Could not start running outer podman run cmd: %+v", err)
	}

	go func() {
		err := cmd.Wait()
		zap.L().Debug("Outer podman exited", zap.Error(err))
		if exiterr, ok := err.(*exec.ExitError); ok {
			zap.L().Debug("Outer podman exited with code", zap.Int("code", exiterr.ExitCode()))
		}

		s.innerContainerCh <- struct{}{}
	}()

	zap.L().Debug("Successfully ran outer podman run")

	return nil
}

func (s *Server) setSeccompProfileOuter() error {
	dataBytes, err := os.ReadFile("/etc/containers/seccomp.json")
	if err != nil {
		return err
	}

	seccompSpec := &specs.LinuxSeccomp{}
	if err := json.Unmarshal(dataBytes, seccompSpec); err != nil {
		return err
	}

	seccompSpec.Syscalls = append(seccompSpec.Syscalls,
		specs.LinuxSyscall{
			Names: []string{
				"clone",
				"clone3",
				"mount",
				"umount2",
				"chroot",
				"pivot_root",
				"setdomainname",
				"sethostname",
				"unshare",
				"keyctl",
				"add_key",
				"request_key",
				"mknod",
				"mknodat",
			},
			Action: specs.ActAllow,
		},

		specs.LinuxSyscall{
			Names:  []string{"setns"},
			Action: specs.ActAllow,
		},
	)

	out, err := json.Marshal(seccompSpec)
	if err != nil {
		return err
	}

	if err := os.WriteFile("/etc/containers/seccomp-octelium.json", []byte(out), 0644); err != nil {
		return err
	}

	return nil
}
