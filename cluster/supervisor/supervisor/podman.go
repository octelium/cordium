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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/cordium/cluster/supervisor/supervisor/oproxy"
	"github.com/octelium/cordium/cluster/supervisor/supervisor/sshagent"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (s *Server) execWorkspaceBinary() error {
	if ldflags.IsTest() {
		return nil
	}

	if err := s.createInitCgroup(context.Background()); err != nil {
		return err
	}

	cmdArgs := []string{
		" --user 0",
		// fmt.Sprintf("--env-file %s", s.envFilePath),
	}

	cmdStr := fmt.Sprintf("podman exec %s workspace /bin/cordium-workspace serve", strings.Join(cmdArgs, " "))
	s.runCmd = s.getCommandAsOctelium(context.Background(), cmdStr)
	zap.L().Debug("Executing workspace agent in the container")
	if err := s.runCmd.Start(); err != nil {
		return err
	}

	/*
		if err := s.moveProcessToCgroup(context.Background(), s.runCmd.Process.Pid); err != nil {
			return err
		}
	*/

	zap.L().Debug("Workspace binary successfully started")

	return nil
}

/*
func (s *Server) execInWorkspaceAndGetOutput(ctx context.Context, command string) ([]byte, error) {

	cmdStr := fmt.Sprintf("podman exec --env-file %s workspace %s", s.envFilePath, command)
	cmd := s.getCommandAsOctelium(ctx, cmdStr)

	cmd.Stderr = nil
	cmd.Stdout = nil

	zap.L().Debug("Running command in Workspace container", zap.String("command", command))
	return cmd.CombinedOutput()
}
*/

func (s *Server) createPod(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	const wsPort = 35921
	tunPort := workspacecommon.GetWorkspaceTunnelPort()
	const eSSHPort = 2022

	args := []string{
		"--name ws",
		"--dns 8.8.8.8",
		"--dns-search=.",
		"--hostname cordium",
		fmt.Sprintf("-p %d:%d", wsPort, wsPort),
		fmt.Sprintf("-p %d:%d/udp", tunPort, tunPort),
		fmt.Sprintf("-p %d:%d", eSSHPort, eSSHPort),

		fmt.Sprintf("--memory=%dm", int(s.initReq.Workspace.Status.Limit.Memory.Megabytes*95/100)),
		"--network=slirp4netns",
		// "--network=slirp4netns:mtu=10000",
		// "--network=slirp4netns:port_handler=rootlesskit",
		// fmt.Sprintf("--network slirp4netns:outbound_addr=%s", brIP),
	}

	cmdStr := fmt.Sprintf("podman pod create %s", strings.Join(args, " "))
	cmd := s.getCommandAsOctelium(ctx, cmdStr)

	return cmd.Run()
}

/*
func (s *Server) pullImageFromLocal(ctx context.Context, imagePath string) error {

	if ldflags.IsTest() {
		return nil
	}

	var image string
	if imagePath == "" {
		image = fmt.Sprintf("%s/base", s.getContainerRegistryPrefix(s.initReq.LoadContainerRegistry))
	} else {
		image = fmt.Sprintf("%s/%s", s.getContainerRegistryPrefix(s.initReq.LoadContainerRegistry), imagePath)
	}

	s.setStatus(cordiumv1.Workspace_Status_PULLING_IMAGE)

	podmanRunArgs := []string{
		fmt.Sprintf("--creds=%s", s.getPodmanCreds(s.initReq.LoadContainerRegistry)),
	}

	if s.initReq.LoadContainerRegistry.InsecureSkipTLS {
		podmanRunArgs = append(podmanRunArgs, "--tls-verify=false")
	}

	cmdStr := fmt.Sprintf("podman pull %s %s", strings.Join(podmanRunArgs, " "), image)

	zap.L().Debug("Pulling image from local registry",
		zap.String("image", image), zap.String("cmd", cmdStr))
	cmd := s.getCommandAsOctelium(ctx, cmdStr)
	if err := s.cmdStdout(cmd, func(data []byte) {
		s.publishLog(data,
			cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDOUT)
	}, func(data []byte) {
		if ldflags.IsDev() {
			s.publishLog(data,
				cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDERR)
		}
	}); err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		zap.L().Debug("image pull cmd error", zap.Error(err))
		s.setFailure(&cordiumv1.Workspace_Status_Failure{
			Type: &cordiumv1.Workspace_Status_Failure_ImagePull_{
				ImagePull: &cordiumv1.Workspace_Status_Failure_ImagePull{},
			},
		})
		return err
	}
	zap.L().Debug("Image successfully pulled")

	{
		zap.L().Debug("Tagging image")
		cmdStr := fmt.Sprintf("podman tag %s workspace", image)
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		if err := cmd.Run(); err != nil {
			return err
		}
		zap.L().Debug("Successfully tagged the image")
	}

	return nil
}
*/

func (s *Server) cmdStdout(cmd *exec.Cmd, stdoutFn, stderrFn func([]byte)) error {
	cmd.Stdout = nil
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Errorf("Could not get pull image cmd stdout: %+v", err)
	}
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			if stdoutFn != nil {
				dataI := scanner.Bytes()
				data := make([]byte, len(dataI))
				copy(data, dataI)
				zap.L().Debug("cmd stdout", zap.String("data", string(data)))
				stdoutFn(data)
			}
		}
	}()

	cmd.Stderr = nil
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return errors.Errorf("Could not get pull image cmd stdout: %+v", err)
	}
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			if stderrFn != nil {
				dataI := scanner.Bytes()
				data := make([]byte, len(dataI))
				copy(data, dataI)
				zap.L().Debug("cmd stderr", zap.String("data", string(data)))
				stderrFn(data)
			}
		}
	}()

	return nil
}

func (s *Server) pullImageFromExternal(ctx context.Context, image string, auth *cordiumv1.Workspace_Spec_Image_Registry_Authentication) error {

	if ldflags.IsTest() {
		return nil
	}

	image = strings.TrimSpace(image)

	if image == "" {
		image = components.GetImage(components.Workspace, "")
		zap.L().Debug("No image provided. Switching to loading the base image")
		// image = "mcr.microsoft.com/vscode/devcontainers/base:ubuntu"
	}

	podmanRunArgs := []string{}
	if auth != nil {
		if auth.Username != "" && auth.Password != nil && auth.Password.GetFromSecret() != "" {
			if sec, err := ucordiumv1.ToSecretList(s.initReq.SecretList).GetByName(auth.Password.GetFromSecret()); err == nil {
				podmanRunArgs = append(podmanRunArgs,
					fmt.Sprintf("--creds=%s:%s", auth.Username, ucordiumv1.ToSecret(sec).GetValueStr()))
			}
		}
	}

	s.setStatus(cordiumv1.Workspace_Status_PULLING_IMAGE)

	if ldflags.IsDev() && ovutils.IsPrivateRegistry() &&
		vutils.FSPathExists("/etc/regcred.json") &&
		image == components.GetImage(components.Workspace, "") {
		podmanRunArgs = append(podmanRunArgs, "--authfile /etc/regcred.json")
	}

	if ldflags.IsDev() {
		podmanRunArgs = append(podmanRunArgs, "--log-level=debug")
	}

	cmdStr := fmt.Sprintf("podman pull %s %s", strings.Join(podmanRunArgs, " "), image)

	zap.L().Debug("Pulling external image", zap.String("image", image))
	cmd := s.getCommandAsOctelium(ctx, cmdStr)
	if err := s.cmdStdout(cmd, func(data []byte) {
		s.publishLog(data,
			cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDOUT)
	}, func(data []byte) {
		if ldflags.IsDev() {
			s.publishLog(data,
				cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDERR)
		}
	}); err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		zap.L().Debug("image pull cmd error", zap.Error(err))
		s.setFailure(&cordiumv1.Workspace_Status_Failure{
			Type: &cordiumv1.Workspace_Status_Failure_ImagePull_{
				ImagePull: &cordiumv1.Workspace_Status_Failure_ImagePull{},
			},
		})
		return err
	}
	zap.L().Debug("Image successfully pulled")

	{
		zap.L().Debug("Tagging image")
		cmdStr := fmt.Sprintf("podman tag %s workspace", image)
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		if err := cmd.Run(); err != nil {
			return err
		}
		zap.L().Debug("Successfully tagged the image")
	}

	return nil
}

func (s *Server) podmanRunImage(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	zap.L().Debug("Starting Workspace container")

	ws := s.initReq.Workspace

	containerSpec := s.spec.Runtime
	containerCmd := func() string {
		if s.doNotOverrideCmd {
			return ""
		}
		if containerSpec != nil && containerSpec.Cmd != "" {
			return strings.TrimSpace(containerSpec.Cmd)
		}
		return "sleep infinity"
	}()

	podmanRunArgs := []string{
		"--name workspace",
		"--pod ws",
		// "--cap-add mknod,net_admin,sys_admin,net_raw,sys_ptrace",
		"-d",
		// "--privileged",
		"--dns 8.8.8.8",
		"--dns-search=.",
		"--http-proxy=false",

		"--no-hosts",
		// "--network=slirp4netns:mtu=10000",
		// "--network=slirp4netns:port_handler=rootlesskit",

		"--volume /octelium/podman/dind/docker:/var/lib/docker",
		"--volume /octelium/podman/dind/containers:/var/lib/containers",
		"--volume /octelium/sockets:/run/octelium",
		"--volume /octelium/podman/tmp/var/tmp:/var/tmp",
		"--volume /octelium/podman/tmp/tmp:/tmp",

		"--device=/dev/net/tun-octelium0:/dev/net/tun",
		// fmt.Sprintf("--env-file %s", s.envFilePath),
		fmt.Sprintf("--log-level=%s", func() string {
			if ldflags.IsDev() {
				return "debug"
			}
			return "info"
		}()),
		// "--cgroups=disabled",
		// "--oom-kill-disable",

		fmt.Sprintf("--memory=%dm", int(s.initReq.Workspace.Status.Limit.Memory.Megabytes*95/100)),
		fmt.Sprintf("--memory-reservation=%dm", int(s.initReq.Workspace.Status.Limit.Memory.Megabytes*85/100)),

		`--security-opt=seccomp=/etc/containers/seccomp.json`,
		"--runtime=crun",
	}
	if ldflags.IsDev() {
		podmanRunArgs = append(podmanRunArgs, "--env=OCTELIUM_DEV=true")
	}
	if s.spec.Runtime != nil && s.spec.Runtime.Filesystem != nil && s.spec.Runtime.Filesystem.ReadOnly {
		podmanRunArgs = append(podmanRunArgs, "--read-only")
	}

	for _, bin := range s.mountBinaries {
		podmanRunArgs = append(podmanRunArgs, fmt.Sprintf("--volume %s:%s:ro,exec,nosuid", bin, bin))
	}

	/*
		if ldflags.IsDev() {
			podmanRunArgs = append(podmanRunArgs, "--env GRPC_GO_LOG_VERBOSITY_LEVEL=99")
			podmanRunArgs = append(podmanRunArgs, "--env GRPC_GO_LOG_SEVERITY_LEVEL=info")
		}
	*/

	if s.containerInitProcess || containerSpec == nil || (containerSpec != nil && !containerSpec.DisableInit) {
		podmanRunArgs = append(podmanRunArgs, "--init")
	} else {
		// podmanRunArgs = append(podmanRunArgs, "--systemd always")
	}

	if !ws.Status.IsBuild {
		podmanRunArgs = append(podmanRunArgs,
			fmt.Sprintf("--volume %s:/var/run/octelium-proxy.sock", oproxy.SocketPath))

		podmanRunArgs = append(podmanRunArgs,
			fmt.Sprintf("--volume %s:/var/run/octelium-ssh-agent.sock", sshagent.SocketPath))
	}

	if containerSpec != nil && containerSpec.Entrypoint != "" {
		podmanRunArgs = append(podmanRunArgs, fmt.Sprintf("--entrypoint %s", strings.TrimSpace(containerSpec.Entrypoint)))
	}

	if err := s.doRunContainer(ctx, podmanRunArgs, containerCmd); err != nil {

		s.setFailure(&cordiumv1.Workspace_Status_Failure{
			Type: &cordiumv1.Workspace_Status_Failure_RunContainer_{
				RunContainer: &cordiumv1.Workspace_Status_Failure_RunContainer{},
			},
		})

		return err
	}

	return nil
	/*
		cmdStr := fmt.Sprintf("podman run %s workspace", strings.Join(podmanRunArgs, " "))

		if containerCmd != "" {
			cmdStr = fmt.Sprintf("%s %s", cmdStr, containerCmd)
		}

		zap.L().Debug("running podman run cmd", zap.String("cmd", cmdStr))
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		if err := cmd.Run(); err != nil {
			zap.S().Errorf("Could not run podman run cmd: %s: %+v", cmdStr, err)
			return err
		}

		return nil
	*/
}

func (s *Server) doStartContainer(ctx context.Context) error {

	{

		/*
			if err := s.getCommandAsOctelium(ctx, "podman network disconnect --help").Run(); err != nil {
				zap.L().Warn("podman container disconnect err", zap.Error(err))
			}

			if err := s.getCommandAsOctelium(ctx, "podman network disconnect -f podman workspace").Run(); err != nil {
				zap.L().Warn("podman container disconnect err", zap.Error(err))
			}

			if err := s.getCommandAsOctelium(ctx, "podman init workspace").Run(); err != nil {
				zap.L().Warn("podman init err", zap.Error(err))
			}
		*/

		if err := s.podmanMigrate(ctx); err != nil {
			zap.L().Warn("Could not migrate podman", zap.Error(err))
		}

		if err := s.getCommandAsOctelium(ctx, "podman init workspace").Run(); err != nil {
			zap.L().Warn("podman init err", zap.Error(err))
		}

		return s.getCommandAsOctelium(ctx, "podman start workspace").Run()
	}

	/*
		if err := s.podmanMigrate(ctx); err != nil {
			zap.L().Warn("Could not migrate podman", zap.Error(err))
		}

		if err := s.getCommandAsOctelium(ctx, "podman start workspace").Run(); err != nil {
			zap.L().Warn("Could not run podman start workspace. Migrating and re-starting the container", zap.Error(err))
			if err := s.podmanMigrate(ctx); err != nil {
				return errors.Errorf("Could not migrate podman: %+v", err)
			}
			return s.getCommandAsOctelium(ctx, "podman start workspace").Run()
		}
	*/

	return nil
}

func (s *Server) doRunContainer(ctx context.Context, commonArgs []string, containerCmd string) error {

	aCPUs := fmt.Sprintf("--cpus=%.2f",
		float32(float32(s.initReq.Workspace.Status.Limit.Cpu.Millicores*97)/float32(100*1000)))
	// aCAP := "--cap-add net_admin,sys_admin,net_raw,sys_ptrace,net_bind_service"
	aCAP := "--cap-add net_admin,sys_admin,net_raw,net_bind_service"

	aSecFS := "--tmpfs /sys/kernel/security:rw,size=100k,mode=1755"
	aSecOpts := "--security-opt=unmask=/sys/fs/cgroup --security-opt=unmask=/proc/sys"
	aCG := fmt.Sprintf("--cgroup-manager=cgroupfs --cgroup-parent=%s", s.getRelativePathCgroupWorkspace())
	// noSeccomp := "--security-opt=seccomp=unconfined"

	if ldflags.IsDev() {
		if err := s.getCommandAsOctelium(ctx,
			fmt.Sprintf(`ls -la %s`, s.getCgroupWorkspace())).Run(); err != nil {
			return err
		}

		if err := s.getCommandAsOctelium(ctx,
			fmt.Sprintf(`cat %s/cgroup.subtree_control`, s.getCgroupWorkspace())).Run(); err != nil {
			return err
		}
	}

	argsList := [][]string{

		{aCAP, aCPUs, aSecOpts, aSecFS, aCG},

		/*
			{aCAP, aCPUs, aSecOpts, aSecFS, fmt.Sprintf("--cgroup-parent=%s", s.getRelativePathCgroupWorkspace())},
			{aCAP, aSecOpts, aSecFS, aCG},
			{aCAP, aSecOpts, aSecFS, fmt.Sprintf("--cgroup-manager=cgroupfs --cgroup-parent=%s", s.getCgroupWorkspace())},
			{aCAP, aSecOpts, aSecFS, fmt.Sprintf("--cgroup-parent=%s", s.getCgroupWorkspace())},
			{aCAP, aCPUs, aSecOpts, aSecFS, aCG, "--runtime=runc"},

			{aCAP, aCPUs, aSecOpts, aSecFS, aCG, "--cgroups=no-conmon"},
			{aCAP, aCPUs, aSecOpts, aSecFS, aCG, "--cgroups=no-conmon", "--runtime=runc"},

			{aCAP, aCPUs, aSecOpts},
			{"--privileged", aCPUs},
			{aCAP, aCPUs, "--security-opt=unmask=/proc/sys"},
			{aCAP, aCPUs, "--security-opt=unmask=/proc/sys", "--runtime=runc"},

			{"--privileged", aCPUs},
			{"--privileged"},
			{"--privileged", "--runtime=runc"},
		*/
	}

	for _, args := range argsList {
		podmanRunArgs := args
		podmanRunArgs = append(podmanRunArgs, commonArgs...)

		cmdStr := fmt.Sprintf("podman run %s workspace", strings.Join(podmanRunArgs, " "))

		if containerCmd != "" {
			cmdStr = fmt.Sprintf("%s %s", cmdStr, containerCmd)
		}

		zap.L().Debug("running podman run cmd", zap.String("cmd", cmdStr))

		cmd := s.getCommandAsOctelium(ctx, cmdStr)

		err := cmd.Run()
		if err == nil {
			zap.L().Debug("Successfully ran podman run cmd", zap.String("cmd", cmdStr))
			return nil
		}
		zap.S().Errorf("Could not run podman run cmd: %s: %+v", cmdStr, err)

		{
			cmd := s.getCommandAsOctelium(ctx, "podman container rm -f workspace")
			if err := cmd.Run(); err == nil {
				zap.L().Debug("Successfully removed Workspace container")
			}
		}
		time.Sleep(1 * time.Second)
	}
	return errors.Errorf("Could not run any of the podman run cmds")
}

func (s *Server) waitUntilContainerIsRunning(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	for i := 0; i < 5000; i++ {
		isRunning, err := s.isContainerIsRunning(ctx)
		if err != nil {
			return errors.Errorf("Could not check if container is running: %+v", err)
		}
		if isRunning {
			zap.L().Debug("Container is now ready and running")
			return nil
		}
		zap.L().Debug("Container is not ready yet. Trying again...")
		time.Sleep(250 * time.Millisecond)
	}
	return errors.Errorf("Could not get container status after 1000 attempts")
}

/*
func (s *Server) createVolume(ctx context.Context, name string) error {

	if ldflags.IsTest() {
		return nil
	}

	cmdStr := fmt.Sprintf("podman volume create %s", name)
	zap.L().Debug("running run cmd", zap.String("cmd", cmdStr))
	cmd := s.getCommandAsOctelium(ctx, cmdStr)
	if err := cmd.Run(); err != nil {
		zap.S().Errorf("Could not run podman run cmd: %s: %+v", cmdStr, err)
		return err
	}

	return nil
}
*/

func (s *Server) inspectWorkspaceContainer(ctx context.Context) (*inspectContainer, error) {
	cmdStr := "podman inspect --type container workspace"
	cmd := s.getCommandAsOctelium(ctx, cmdStr)

	cmd.Stderr = nil
	cmd.Stdout = nil

	zap.L().Debug("Starting running inspect command")
	resp, err := cmd.CombinedOutput()
	if err != nil {
		zap.L().Error("Could not run inspect command", zap.String("output", string(resp)), zap.Error(err))
		return nil, err
	}

	zap.L().Debug("Inspect container output", zap.String("out", string(resp)))

	var containers []inspectContainer
	if err := json.Unmarshal(resp, &containers); err != nil {
		zap.L().Error("Could not unmarshal inspect data", zap.Error(err))
		return nil, err
	}

	if len(containers) == 0 {
		zap.L().Debug("No containers found in the inspect data")
		return nil, errors.Errorf("Could not find containers to inspect")
	}

	container := containers[0]
	zap.L().Debug("Found container inspect data", zap.Any("data", container))
	return &container, nil
}

func (s *Server) isContainerIsRunning(ctx context.Context) (bool, error) {
	container, err := s.inspectWorkspaceContainer(ctx)
	if err != nil {
		zap.L().Debug("Could not inspect container", zap.Error(err))
		return false, nil
	}

	if container.State == nil {
		zap.L().Debug("No state found in the inspect data")
		return false, nil
	}

	if container.State.Status == "exited" {
		return false, errors.Errorf("Workspace container exited: state %+v", container.State)
	}

	return container.State.Running && container.State.Status == "running", nil
}

func (s *Server) copyToContainer(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}

	for _, cpInfo := range s.cpToContainer {
		cmdStr := fmt.Sprintf("podman cp %s workspace:%s", cpInfo.src, cpInfo.dst)

		zap.L().Debug("Copying binary to the container",
			zap.String("src", cpInfo.src), zap.String("dst", cpInfo.dst))

		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		if err := cmd.Run(); err != nil {
			return err
		}

	}

	zap.L().Debug("Successfully copied Octelium binaries to the container")

	return nil
}

func (s *Server) buildImage(ctx context.Context, workdir, dockerfilePath, contextDir string, args map[string]string) error {
	ctx, cancelFn := context.WithTimeout(ctx, 10*time.Minute)
	defer cancelFn()

	zap.L().Debug("Starting building image",
		zap.String("workDir", workdir),
		zap.String("dockerFilePath", dockerfilePath),
		zap.String("contextDir", contextDir),
		zap.Any("args", args))

	podmanRunArgs := []string{
		"--tag workspace",
	}

	if dockerfilePath != "" {
		podmanRunArgs = append(podmanRunArgs, fmt.Sprintf("-f %s", dockerfilePath))
	}

	if len(args) > 0 {
		for k, v := range args {
			if k != "" && v != "" {
				podmanRunArgs = append(podmanRunArgs,
					fmt.Sprintf("--build-arg=%s=%s", k, v))
			}
		}
	}

	cmdStr := fmt.Sprintf("podman build %s %s", strings.Join(podmanRunArgs, " "), contextDir)

	cmd := s.getCommandAsOctelium(ctx, cmdStr)
	cmd.Dir = workdir

	s.setStatus(cordiumv1.Workspace_Status_BUILDING_IMAGE)

	if err := s.cmdStdout(cmd, func(data []byte) {
		s.publishLog(data,
			cordiumv1.ListenLogResponse_TYPE_BUILDING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDOUT)
	}, func(data []byte) {
		if ldflags.IsDev() {
			s.publishLog(data,
				cordiumv1.ListenLogResponse_TYPE_BUILDING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDERR)
		}
	}); err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		zap.S().Errorf("Could not run podman build cmd: %s: %+v", cmdStr, err)
		s.setFailure(&cordiumv1.Workspace_Status_Failure{
			Type: &cordiumv1.Workspace_Status_Failure_ImageBuild_{
				ImageBuild: &cordiumv1.Workspace_Status_Failure_ImageBuild{},
			},
		})
		return err
	}

	zap.L().Debug("Image built successfully")

	return nil
}

func (s *Server) exportContainer(ctx context.Context) error {
	cmdStr := fmt.Sprintf("podman export workspace | gzip > %s", tmpImageLocation)

	zap.L().Debug("Exporting Workspace container", zap.String("cmd", cmdStr))

	cmd := s.getCommandAsOctelium(ctx, cmdStr)

	if err := cmd.Run(); err != nil {
		zap.S().Errorf("Could not run podman export cmd: %s: %+v", cmdStr, err)
		return err
	}

	return nil
}

/*
func (s *Server) importImage(ctx context.Context) error {
	zap.L().Debug("Importing Workspace image", zap.String("path", tmpImageLocation))
	cmdStr := fmt.Sprintf("podman import %s workspace", tmpImageLocation)

	cmd := s.getCommandAsOctelium(ctx, cmdStr)

	if err := s.cmdStdout(cmd, func(data []byte) {
		s.publishLog(data,
			cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDOUT)
	}, func(data []byte) {
		if ldflags.IsDev() {
			s.publishLog(data,
				cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE, cordiumv1.ListenLogResponse_MODE_STDERR)
		}
	}); err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		zap.S().Errorf("Could not run podman import cmd: %s: %+v", cmdStr, err)
		return err
	}

	return nil
}
*/

func (s *Server) commitContainer(ctx context.Context) error {

	args := []string{
		"--format oci",
	}

	if s.initReq != nil && s.initReq.Workspace.Status.IsBuild {
		args = append(args, "--squash")
	}

	cmdStr := fmt.Sprintf("podman commit %s workspace workspace", strings.Join(args, " "))

	zap.L().Debug("Committing Workspace container", zap.String("cmd", cmdStr))

	cmd := s.getCommandAsOctelium(ctx, cmdStr)

	if err := cmd.Run(); err != nil {
		zap.S().Errorf("Could not run podman commit cmd: %s: %+v", cmdStr, err)
		return err
	}

	return nil
}

func (s *Server) getContainerStats(ctx context.Context) ([]containerStats, error) {
	// zap.L().Debug("Fetching Workspace container stats")
	cmdStr := "podman stats workspace --no-stream --format json"
	if ldflags.IsTest() {
		return []containerStats{}, nil
	}

	cmd := s.getCommandAsOcteliumNOStdout(ctx, cmdStr)

	out, err := cmd.Output()
	if err != nil {
		zap.S().Errorf("Could not run podman stats cmd: %s: %+v", cmdStr, err)
		return nil, err
	}

	var ret []containerStats
	if err := json.Unmarshal(out, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *Server) podmanMigrate(ctx context.Context) error {
	cmd := s.getCommandAsOctelium(ctx, "podman system migrate")
	if err := cmd.Run(); err != nil {
		zap.L().Warn("Could not podman system migrate", zap.Error(err))
	}

	return nil
}

type containerStats struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	CPUTime    string `json:"cpu_time"`
	CpuPercent string `json:"cpu_percent"`
	AverageCPU string `json:"avg_cpu"`
	MemUsage   string `json:"mem_usage"`
	MemPerc    string `json:"mem_percent"`
	NetIO      string `json:"net_io"`
	BlockIO    string `json:"block_io"`
	Pids       string `json:"pids"`
}

/*
func (s *Server) runAdditionalContainer(ctx context.Context, ctr *cordiumv1.Workspace_Spec_Container) error {
	if ldflags.IsTest() {
		return nil
	}

	runArgs := []string{
		"-d",
		"--pod ws",
		"--dns 8.8.8.8",
	}

	for _, itm := range ctr.EnvVars {
		runArgs = append(runArgs, fmt.Sprintf("--env %s=%s", itm.Key, itm.Value))
	}

	cmdStr := fmt.Sprintf("podman run %s %s", strings.Join(runArgs, " "), ctr.Image)

	if ctr.Cmd != "" {
		cmdStr = fmt.Sprintf("%s %s", cmdStr, ctr.Cmd)
	}

	zap.L().Debug("running run cmd", zap.String("cmd", cmdStr))
	cmd := s.getCommandAsOctelium(ctx, cmdStr)
	if err := cmd.Run(); err != nil {
		zap.S().Errorf("Could not run podman run cmd: %s: %+v", cmdStr, err)
		return err
	}

	return nil
}
*/

/*
func (s *Server) waitForInnerPodman(ctx context.Context) error {
	cmdStr := "podman wait --root=/octelium-root inner"
	if err := s.getCommandAsRoot(ctx, cmdStr).Run(); err != nil {
		zap.L().Error("inner podman exited with error", zap.Error(err))
	}

	zap.L().Debug("Signaling inner container exit")

	s.innerContainerCh <- struct{}{}
	return nil
}
*/

func (s *Server) podmanStopOuter(ctx context.Context) error {
	cmdStr := "podman stop --root=/octelium-root -t 5000 inner"
	zap.L().Debug("Stopping outer container")
	if err := s.getCommandAsRoot(ctx, cmdStr).Run(); err != nil {
		zap.L().Error("outer podman exited with error", zap.Error(err))
	}

	zap.L().Debug("outer container stopped")
	return nil
}

type inspectContainer struct {
	State  *inspectContainerState  `json:"State"`
	Config *inspectContainerConfig `json:"Config"`
}

type inspectContainerConfig struct {
	// Container hostname
	Hostname string `json:"Hostname"`
	// Container domain name - unused at present
	DomainName string `json:"Domainname"`
	// User the container was launched with
	User string `json:"User"`
	// Unused, at present
	AttachStdin bool `json:"AttachStdin"`
	// Unused, at present
	AttachStdout bool `json:"AttachStdout"`
	// Unused, at present
	AttachStderr bool `json:"AttachStderr"`
	// Whether the container creates a TTY
	Tty bool `json:"Tty"`
	// Whether the container leaves STDIN open
	OpenStdin bool `json:"OpenStdin"`
	// Whether STDIN is only left open once.
	// Presently not supported by Podman, unused.
	StdinOnce bool `json:"StdinOnce"`
	// Container environment variables
	Env []string `json:"Env"`
	// Container command
	Cmd []string `json:"Cmd"`
	// Container image
	Image string `json:"Image"`
	// Unused, at present. I've never seen this field populated.
	Volumes map[string]struct{} `json:"Volumes"`
	// Container working directory
	WorkingDir string `json:"WorkingDir"`
	// Container entrypoint
	Entrypoint []string `json:"Entrypoint"`
	// On-build arguments - presently unused. More of Buildah's domain.
	OnBuild *string `json:"OnBuild"`
	// Container labels
	Labels map[string]string `json:"Labels"`
	// Container annotations
	Annotations map[string]string `json:"Annotations"`
	// Container stop signal
	StopSignal string `json:"StopSignal"`

	// HealthcheckOnFailureAction defines an action to take once the container turns unhealthy.
	HealthcheckOnFailureAction string `json:"HealthcheckOnFailureAction,omitempty"`
	// HealthLogDestination defines the destination where the log is stored
	HealthLogDestination string `json:"HealthLogDestination,omitempty"`
	// HealthMaxLogCount is maximum number of attempts in the HealthCheck log file.
	// ('0' value means an infinite number of attempts in the log file)
	HealthMaxLogCount uint `json:"HealthcheckMaxLogCount,omitempty"`
	// HealthMaxLogSize is the maximum length in characters of stored HealthCheck log
	// ("0" value means an infinite log length)
	HealthMaxLogSize uint `json:"HealthcheckMaxLogSize,omitempty"`
	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string `json:"CreateCommand,omitempty"`
	// Timezone is the timezone inside the container.
	// Local means it has the same timezone as the host machine
	Timezone string `json:"Timezone,omitempty"`
	// SystemdMode is whether the container is running in systemd mode. In
	// systemd mode, the container configuration is customized to optimize
	// running systemd in the container.
	SystemdMode bool `json:"SystemdMode,omitempty"`
	// Umask is the umask inside the container.
	Umask string `json:"Umask,omitempty"`

	// Timeout is time before container is killed by conmon
	Timeout uint `json:"Timeout"`
	// StopTimeout is time before container is stopped when calling stop
	StopTimeout uint `json:"StopTimeout"`
	// Passwd determines whether or not podman can add entries to /etc/passwd and /etc/group
	Passwd *bool `json:"Passwd,omitempty"`
	// ChrootDirs is an additional set of directories that need to be
	// treated as root directories. Standard bind mounts will be mounted
	// into paths relative to these directories.
	ChrootDirs []string `json:"ChrootDirs,omitempty"`
	// SdNotifyMode is the sd-notify mode of the container.
	SdNotifyMode string `json:"sdNotifyMode,omitempty"`
	// SdNotifySocket is the NOTIFY_SOCKET in use by/configured for the container.
	SdNotifySocket string `json:"sdNotifySocket,omitempty"`
	// ExposedPorts includes ports the container has exposed.
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`

	// V4PodmanCompatMarshal indicates that the json marshaller should
	// use the old v4 inspect format to keep API compatibility.
	V4PodmanCompatMarshal bool `json:"-"`
}

type inspectContainerState struct {
	OciVersion string    `json:"OciVersion"`
	Status     string    `json:"Status"`
	Running    bool      `json:"Running"`
	Paused     bool      `json:"Paused"`
	Restarting bool      `json:"Restarting"` // TODO
	OOMKilled  bool      `json:"OOMKilled"`
	Dead       bool      `json:"Dead"`
	Pid        int       `json:"Pid"`
	ConmonPid  int       `json:"ConmonPid,omitempty"`
	ExitCode   int32     `json:"ExitCode"`
	Error      string    `json:"Error"` // TODO
	StartedAt  time.Time `json:"StartedAt"`
	FinishedAt time.Time `json:"FinishedAt"`

	Checkpointed   bool      `json:"Checkpointed,omitempty"`
	CgroupPath     string    `json:"CgroupPath,omitempty"`
	CheckpointedAt time.Time `json:"CheckpointedAt,omitempty"`
	RestoredAt     time.Time `json:"RestoredAt,omitempty"`
	CheckpointLog  string    `json:"CheckpointLog,omitempty"`
	CheckpointPath string    `json:"CheckpointPath,omitempty"`
	RestoreLog     string    `json:"RestoreLog,omitempty"`
	Restored       bool      `json:"Restored,omitempty"`
	StoppedByUser  bool      `json:"StoppedByUser,omitempty"`
}
