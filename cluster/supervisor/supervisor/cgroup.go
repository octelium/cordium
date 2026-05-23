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
	"strings"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
)

const cgSystemRoot = "/sys/fs/cgroup"

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
	return s.getCommandAsRoot(ctx, fmt.Sprintf("echo %d > %s/cgroup.procs", s.myPID, s.getCgroupWorkspaceLeaf())).Run()
}

func (s *Server) returnToMyCgroup(ctx context.Context) error {
	zap.L().Debug("Returning to my cgroup")
	if s.myCgroup == "" {
		zap.L().Debug("my cgroup is not set. Nothing to be done")
		return nil
	}

	return s.getCommandAsRoot(ctx, fmt.Sprintf("echo %d > %s/cgroup.procs", s.myPID, path.Join(cgSystemRoot, s.myCgroup))).Run()
}

func (s *Server) createInitCgroup(ctx context.Context) error {

	zap.L().Debug("Creating init cgroup")

	if err := s.getCommandAsOctelium(ctx, fmt.Sprintf(`mkdir -p %s/init`, s.getCgroupWorkspace())).Run(); err != nil {
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

	limit := s.initReq.Workspace.Status.Limit

	limitMemoryBytes := func() int64 {
		if limit == nil || limit.Memory == nil || limit.Memory.Megabytes < 256 {
			return 256 * 1000 * 1000
		}

		if limit.Memory.Megabytes > 128*1000 {
			return 128 * 1000 * 1000 * 1000
		}

		return int64(limit.Memory.Megabytes) * 1000 * 1000
	}()

	limitMillicores := func() int64 {
		if limit == nil || limit.Cpu == nil || limit.Cpu.Millicores < 100 {
			return 100
		}

		if limit.Cpu.Millicores > 10000*1000 {
			return 10000 * 1000
		}

		return int64(limit.Cpu.Millicores)
	}()

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
		cmd := getCommand(ctx, cmdStr)

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

func (s *Server) prepareCgroupsOuter(ctx context.Context) error {

	zap.L().Debug("Preparing outer cgroups")

	cgParentPath := s.getCgroupParentOuter()

	getFile := func(name string) string {
		return path.Join(cgParentPath, name)
	}

	cmds := []string{
		fmt.Sprintf("mkdir -p %s", cgParentPath),
		fmt.Sprintf(`echo "+memory +cpu +io +pids" > %s`, getFile("cgroup.subtree_control")),
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running cmd", zap.String("cmd", cmdStr))
		cmd := getCommand(ctx, cmdStr)

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
