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
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const cgSystemRoot = "/sys/fs/cgroup"

const cgInitLeaf = "init"

var cgOuterControllers = []string{"cpu", "io", "memory", "pids"}

func (s *Server) getCGroupRoot() string {
	return "/sys/fs/cgroup/cordium.slice"
}

func (s *Server) moveSelfToWorkspaceLeafCgroup(ctx context.Context) error {

	if ldflags.IsTest() {
		return nil
	}

	if err := s.backupMyCgroup(ctx); err != nil {
		zap.L().Warn("Could not get my cgroup", zap.Error(err))
	}

	zap.L().Debug("Moving myPID to workspace leaf cgroup", zap.Int("pid", s.myPID))
	return writeCgroupFile(path.Join(s.getCgroupWorkspaceLeaf(), "cgroup.procs"), strconv.Itoa(s.myPID))
}

func (s *Server) returnToMyCgroup(ctx context.Context) error {
	zap.L().Debug("Returning to my cgroup")
	if s.myCgroup == "" {
		zap.L().Debug("my cgroup is not set. Nothing to be done")
		return nil
	}

	return writeCgroupFile(
		path.Join(cgSystemRoot, s.myCgroup, "cgroup.procs"), strconv.Itoa(s.myPID))
}

func writeCgroupFile(path, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}

func readCgroupFields(cgPath, name string) ([]string, error) {
	out, err := os.ReadFile(path.Join(cgPath, name))
	if err != nil {
		return nil, err
	}

	return strings.Fields(string(out)), nil
}

func isCgroupNamespaceRoot() (bool, error) {
	out, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 || parts[0] != "0" {
			continue
		}

		return parts[2] == "/", nil
	}

	return false, errors.Errorf("Could not find the cgroup v2 entry in /proc/self/cgroup")
}

func moveCgroupProcs(src, dst string) error {
	pids, err := readCgroupFields(src, "cgroup.procs")
	if err != nil {
		return err
	}

	if len(pids) == 0 {
		return nil
	}

	zap.L().Debug("Moving the cgroup processes to the leaf cgroup",
		zap.String("src", src), zap.String("dst", dst), zap.Int("pids", len(pids)))

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, pid := range pids {
		if err := writeCgroupFile(path.Join(dst, "cgroup.procs"), pid); err != nil {
			zap.L().Warn("Could not move the process to the leaf cgroup",
				zap.String("pid", pid), zap.Error(err))
		}
	}

	return nil
}

func delegateCgroupControllers(cgPath string) error {
	available, err := readCgroupFields(cgPath, "cgroup.controllers")
	if err != nil {
		return err
	}

	enabled, err := readCgroupFields(cgPath, "cgroup.subtree_control")
	if err != nil {
		return err
	}

	for _, controller := range available {
		if slices.Contains(enabled, controller) {
			continue
		}

		if err := writeCgroupFile(path.Join(cgPath, "cgroup.subtree_control"),
			fmt.Sprintf("+%s", controller)); err != nil {
			return errors.Errorf("Could not enable the cgroup controller %s at %s: %+v",
				controller, cgPath, err)
		}
	}

	for _, controller := range cgOuterControllers {
		if !slices.Contains(available, controller) {
			zap.L().Warn("The cgroup controller is not delegated by the parent cgroup",
				zap.String("cgPath", cgPath), zap.String("controller", controller))
		}
	}

	zap.L().Debug("Successfully delegated the cgroup controllers",
		zap.String("cgPath", cgPath), zap.Strings("controllers", available))

	return nil
}

func (s *Server) createInitCgroup(ctx context.Context) error {

	zap.L().Debug("Creating init cgroup")

	if err := s.getCommandAsOctelium(ctx,
		"mkdir", "-p", path.Join(s.getCgroupWorkspace(), "init")).Run(); err != nil {
		return err
	}

	zap.L().Debug("Successfully created init cgroup")

	return nil
}

func (s *Server) backupMyCgroup(ctx context.Context) error {
	var err error

	s.myCgroup, err = cgroup2.PidGroupPath(os.Getpid())
	if err != nil {
		return err
	}

	zap.L().Debug("My real cgroup", zap.String("cgPath", s.myCgroup))

	return nil
}

/*
func (s *Server) moveProcessToCgroup(ctx context.Context, pid int) error {

	zap.L().Debug("Moving pid to cgroup", zap.Int("pid", pid), zap.String("cgPath", s.getCgroupWorkspace()))
	if err := getCommand(ctx, fmt.Sprintf("echo %d > %s/init/cgroup.procs", pid, s.getCgroupWorkspace())).Run(); err != nil {
		return errors.Errorf("Could not move process to cgroup: %+v", err)
	}

	zap.L().Debug("Successfully moved pid to cgroup", zap.Int("pid", pid), zap.String("cgPath", s.getCgroupWorkspace()))

	return nil
}
*/

func (s *Server) getCgroupParentOuter() string {
	return path.Join(s.getCGroupRoot(), fmt.Sprintf("oct-%s", os.Getenv("OCTELIUM_WS_UID")))
}

func (s *Server) getCgroupParent() string {
	return path.Join(cgSystemRoot, "octelium")
}

func (s *Server) getCgroupWorkspace() string {
	return path.Join(s.getCgroupParent(), "ws")
}

func (s *Server) getRelativePathOuterCgroup() string {
	return strings.TrimPrefix(s.getCgroupParentOuter(), "/sys/fs/cgroup/")
}

func (s *Server) getRelativePathCgroupWorkspace() string {
	return strings.TrimPrefix(path.Join(s.getCgroupParent(), "ws"), "/sys/fs/cgroup/")
}

func (s *Server) getCgroupWorkspaceLeaf() string {
	return path.Join(s.getCgroupWorkspace(), "leaf")
}

func (s *Server) prepareCgroups(ctx context.Context) error {

	if s.initReq == nil {
		zap.L().Warn("Cannot prepare cgroups. No Workspace supplied")
		return nil
	}

	zap.L().Debug("Preparing cgroups")

	cgParentPath := s.getCgroupParent()
	cgPath := s.getCgroupWorkspace()

	getFile := func(name string) string {
		return path.Join(cgParentPath, name)
	}

	limitMemoryBytes := s.getLimitMemoryMegabytes() * 1000 * 1000

	limitMillicores := s.getLimitMillicores()

	cmds := []string{

		fmt.Sprintf("mkdir -p %s", cgParentPath),
		fmt.Sprintf("mkdir -p %s", cgPath),
		fmt.Sprintf("mkdir -p %s", s.getCgroupWorkspaceLeaf()),
		// fmt.Sprintf("mkdir -p %s/init", cgPath),

		fmt.Sprintf(`echo "+memory +cpu +io +pids" > %s`, getFile("cgroup.subtree_control")),

		fmt.Sprintf(`echo "+memory +cpu +io +pids" > %s`, path.Join(cgPath, "cgroup.subtree_control")),

		fmt.Sprintf(`echo %d > %s`, limitMemoryBytes, getFile("memory.max")),
		fmt.Sprintf(`echo %d > %s`, limitMemoryBytes*90/100, getFile("memory.high")),
		fmt.Sprintf(`echo %d > %s`, limitMemoryBytes*70/100, getFile("memory.low")),

		fmt.Sprintf(`echo %d > %s`, 9000, getFile("pids.max")),

		fmt.Sprintf(`echo %d %d > %s`, limitMillicores*100, 100000, getFile("cpu.max")),
		fmt.Sprintf(`echo 98 > %s`, getFile("cpu.uclamp.max")),

		fmt.Sprintf("chown -R octelium:octelium %s", cgPath),
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running cmd", zap.String("cmd", cmdStr))
		cmd := getShellCommand(ctx, cmdStr)

		if ldflags.IsDev() {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if ldflags.IsTest() {
			continue
		}

		if err := cmd.Run(); err != nil {
			zap.S().Errorf("Could not run cmd: %s: %+v", cmdStr, err)
		}
	}

	zap.L().Debug("Done preparing cgroups")

	return nil
}

func (s *Server) prepareCgroupRootOuter(ctx context.Context) error {

	zap.L().Debug("Preparing the outer cgroup root")

	isOwnRoot, err := isCgroupNamespaceRoot()
	if err != nil {
		return err
	}

	if isOwnRoot {
		if err := moveCgroupProcs(cgSystemRoot, path.Join(cgSystemRoot, cgInitLeaf)); err != nil {
			return err
		}

		if err := delegateCgroupControllers(cgSystemRoot); err != nil {
			return err
		}
	} else {
		zap.L().Debug("The cgroup mount root is not owned by the supervisor. " +
			"Skipping the cgroup root delegation")
	}

	if err := os.MkdirAll(s.getCGroupRoot(), 0755); err != nil {
		return err
	}

	if err := delegateCgroupControllers(s.getCGroupRoot()); err != nil {
		return err
	}

	zap.L().Debug("Successfully prepared the outer cgroup root")

	return nil
}

func (s *Server) prepareCgroupsOuter(ctx context.Context) error {

	zap.L().Debug("Preparing outer cgroups")

	if ldflags.IsTest() {
		return nil
	}

	if err := s.prepareCgroupRootOuter(ctx); err != nil {
		return err
	}

	cgParentPath := s.getCgroupParentOuter()

	if err := os.MkdirAll(cgParentPath, 0755); err != nil {
		return err
	}

	if err := delegateCgroupControllers(cgParentPath); err != nil {
		return err
	}

	zap.L().Debug("Done preparing cgroups")

	return nil
}

func (s *Server) removeCgroupOuter() error {

	zap.L().Debug("Removing cgroup outer path")

	mgr, err := cgroup2.Load(fmt.Sprintf("/%s", s.getRelativePathOuterCgroup()))
	if err != nil {
		return err
	}

	if err := mgr.Delete(); err != nil {
		return err
	}

	zap.L().Debug("Successfully removed cgroup outer path")

	return nil
}
