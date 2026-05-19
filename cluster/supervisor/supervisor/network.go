/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package supervisor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type egressCtl struct {
	wsName    string
	chainName string
	cgroupID  uint64
	s         *Server

	blockedIPv4CIDRs []string
	blockedIPv6CIDRs []string
}

func newEgessCtl(s *Server) *egressCtl {
	return &egressCtl{
		s:      s,
		wsName: s.initReq.Workspace.Metadata.Name,
	}
}

func (e *egressCtl) needsSetup() bool {
	return len(e.blockedIPv4CIDRs) > 0 || len(e.blockedIPv6CIDRs) > 0
}

func (e *egressCtl) setup(ctx context.Context, containerID string) error {
	if !e.needsSetup() {
		return nil
	}

	e.chainName = fmt.Sprintf("CRD-%s", e.wsName)

	pastaPID, err := findNetworkProxyPID(containerID)
	if err != nil {
		return errors.Errorf("finding pasta pid for container %s: %+v", containerID, err)
	}

	zap.L().Debug("Found pasta process",
		zap.String("workspace", e.wsName),
		zap.Int("pid", pastaPID),
	)

	cgroupPath, err := getCgroupV2Path(pastaPID)
	if err != nil {
		return errors.Errorf("getting cgroup path for pid %d: %+v", pastaPID, err)
	}

	cgroupID, err := getCgroupID(cgroupPath)
	if err != nil {
		return errors.Errorf("getting cgroup id for %s: %+v", cgroupPath, err)
	}
	e.cgroupID = cgroupID

	zap.L().Debug("Got cgroup id for workspace pasta process",
		zap.String("workspace", e.wsName),
		zap.String("cgroupPath", cgroupPath),
		zap.Uint64("cgroupID", cgroupID),
	)

	return e.applyRules(ctx)
}

func findNetworkProxyPID(containerID string) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := 0
		if _, err := fmt.Sscanf(entry.Name(), "%d", &pid); err != nil || pid == 0 {
			continue
		}

		cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}

		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")

		if (strings.Contains(cmdline, "pasta") ||
			strings.Contains(cmdline, "slirp4netns")) &&
			strings.Contains(cmdline, containerID[:12]) {
			return pid, nil
		}
	}

	return 0, errors.Errorf("no pasta/slirp4netns process found for container %s", containerID[:12])
}

func getCgroupV2Path(pid int) (string, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			return parts[2], nil
		}
	}

	return "", errors.Errorf("no cgroup v2 entry found for pid %d", pid)
}

func getCgroupID(cgroupPath string) (uint64, error) {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
	var stat syscall.Stat_t
	if err := syscall.Stat(fullPath, &stat); err != nil {
		return 0, err
	}

	return stat.Ino, nil
}

func (e *egressCtl) applyRules(ctx context.Context) error {
	tableName := fmt.Sprintf("cordium_%s", e.wsName)

	script := fmt.Sprintf(`
table inet %s {
    chain output {
        type filter hook output priority filter; policy accept;

        meta cgroup != %d accept

        ct state established,related accept

        ip daddr { %s } drop

        ip6 daddr { %s } drop

        accept
    }
}
`, tableName, e.cgroupID,
		strings.Join(e.blockedIPv4CIDRs, ", "),
		strings.Join(e.blockedIPv6CIDRs, ", "),
	)

	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Errorf("Could not run nft cmd: %+v", err)
	}

	zap.L().Debug("Applied egress rules for Workspace",
		zap.String("workspace", e.wsName),
		zap.String("table", tableName),
		zap.Uint64("cgroupID", e.cgroupID),
	)

	return nil
}

func (e *egressCtl) teardown(ctx context.Context) error {
	if !e.needsSetup() {
		return nil
	}

	tableName := fmt.Sprintf("cordium_%s", e.wsName)
	cmd := exec.CommandContext(ctx, "nft", "delete", "table", "inet", tableName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		zap.L().Warn("Failed to delete nftables table", zap.String("table", tableName), zap.Error(err))
	}

	return nil
}
