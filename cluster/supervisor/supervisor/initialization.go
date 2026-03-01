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
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/supervisor/supervisor/oproxy"
	"github.com/octelium/cordium/cluster/supervisor/supervisor/sshagent"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (s *Server) doInitialize() error {

	var err error
	ctx, cancelFn := context.WithTimeout(s.ctxMain, 180*time.Minute)
	defer cancelFn()

	zap.L().Debug("Starting doInitialize")
	zap.L().Debug("Env vars", zap.Strings("env", os.Environ()))

	if vutils.FSPathExists("/octelium/sockets/workspace.sock") {
		zap.L().Debug("Removing workspace.sock")
		os.Remove("/octelium/sockets/workspace.sock")
	}

	if s.wgPrivateKey == nil {
		return errors.Errorf("wg private key cannot be nil")
	}

	if s.initReq == nil || s.initReq.Workspace == nil {
		return errors.Errorf("Cannot doInitialize. No Workspace in req")
	}

	s.mu.Lock()
	req := s.initReq
	s.mu.Unlock()

	/*
		if req.LoadContainerRegistry == nil {
			return errors.Errorf("Cannot doInitialize. No LoadContainerRegistry in req")
		}

		if req.SaveContainerRegistry == nil {
			return errors.Errorf("Cannot doInitialize. No SaveContainerRegistry in req")
		}
	*/

	s.spec, err = wsutils.MergeSpec(&wsutils.MergeSpecReq{
		Workspace: req.Workspace,
		Template:  req.Template,
	})
	if err != nil {
		return errors.Errorf("Could not merge spec: %+v", err)
	}

	ws := s.initReq.Workspace

	s.isFreshRun = func() bool {
		if ws.Status.IsBuild {
			return true
		}
		return (ws.Status.IsEphemeral || ws.Status.SuccessfulRuns == 0) &&
			!ucordiumv1.ToTemplate(s.initReq.Template).HasReadyBuild()

	}()

	s.wsUID = ws.Metadata.Uid

	zap.L().Debug("Workspace", zap.Any("workspace", s.initReq.Workspace))
	zap.L().Debug("Space", zap.Any("space", s.initReq.Space))
	zap.L().Debug("Merged spec", zap.Any("spec", s.spec))

	if s.isFreshRun {
		if err := s.chownDirOctelium(ctx, "/octelium"); err != nil {
			zap.L().Warn("Could not chown /octelium", zap.Error(err))
		}

		if err := s.chownDirOctelium(ctx, "/home/octelium"); err != nil {
			zap.L().Warn("Could not chown /home/octelium", zap.Error(err))
		}
	}

	if s.isFreshRun {
		zap.L().Debug("Adding /workspace dir to be copied into Workspace container")
		s.cpToContainer = append(s.cpToContainer, &cpInfo{
			src: "/octelium/workspace",
			dst: "/workspace",
		})
	}

	if req == nil {
		return errors.Errorf("Cannot start doInitialize. Nil initReq")
	}

	if err := s.prepareCgroups(ctx); err != nil {
		zap.L().Error("Could not prepare cgroups", zap.Error(err))
	}

	if err := s.moveSelfToWorkspaceLeafCgroup(ctx); err != nil {
		zap.L().Error("Could not move self to Workspace leaf cgroup", zap.Error(err))
	}

	if !s.isFreshRun {

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmdStr := "podman inspect --type container workspace"
		cmd := s.getCommandAsOctelium(ctx, cmdStr)
		err := cmd.Run()
		if err != nil {
			zap.L().Warn("Could not inspect pre", zap.Error(err))
		}

	}

	if ldflags.IsDev() && !ldflags.IsTest() {

		if _, err := exec.LookPath("podman"); err != nil {
			zap.L().Warn("Could not find podman binary", zap.Error(err))
		}

		cmds := []string{
			"env",
			"cat /proc/mounts",
			"ls -la /octelium",
			"cat /etc/os-release",
			"podman --log-level=debug info",
			"cat /proc/mounts",
		}

		s.runAllCommandsAsOctelium(ctx, cmds)
	}

	if err := s.runOcteliumProxy(); err != nil {
		return errors.Errorf("Could not run Octelium Proxy: %+v", err)
	}

	if err := s.runSSHAgent(); err != nil {
		return errors.Errorf("Could not run SSH agent: %+v", err)
	}

	if s.isFreshRun {
		if err := s.createPod(ctx); err != nil {
			return errors.Errorf("Could not create workspace pod: %+v", err)
		}

		if err := s.prepareAndBuildImage(ctx); err != nil {
			return errors.Errorf("Could not prepare and build Image: %+v", err)
		}

	}

	s.setStatus(cordiumv1.Workspace_Status_STARTING_RUNTIME)

	if ldflags.IsDev() && !ldflags.IsTest() {
		if err := s.getCommandAsOctelium(ctx, "skopeo inspect containers-storage:localhost/workspace").Run(); err != nil {
			zap.L().Warn("Could not run skopeo inspect", zap.Error(err))
		}
	}

	if s.isFreshRun && !ldflags.IsTest() {
		// To set ownership for any files downloaded and restored by storage client
		zap.L().Debug("Chowning /workspace dir before running Workspace container")
		if err := s.chownDirOctelium(ctx, "/octelium/workspace"); err != nil {
			zap.L().Error("Could not chown workspace directory", zap.Error(err))
		}
	}

	if s.isFreshRun {
		if err := s.podmanRunImage(ctx); err != nil {
			return errors.Errorf("Could not run Workspace container image: %+v", err)
		}
	} else {
		if err := s.doStartContainer(ctx); err != nil {
			return errors.Errorf("Could not start Workspace container: %+v", err)
		}
	}

	if err := s.waitUntilContainerIsRunning(ctx); err != nil {
		return errors.Errorf("Could not wait until Workspace container is running: %+v", err)
	}

	go s.runStatsLoop()

	if err := s.copyToContainer(ctx); err != nil {
		return errors.Errorf("Could not copy to Workspace container: %+v", err)
	}

	{
		if err := s.execWorkspaceBinary(); err != nil {
			return errors.Errorf("Could not execute the Workspace agent inside the Workspace: %+v", err)
		}

		s.waitForPodmanRunAndShutdown()
	}

	if err := s.returnToMyCgroup(ctx); err != nil {
		zap.L().Warn("Could not return to my cgroup", zap.Error(err))
	}

	if err := s.waitUntilWorkspaceAgentReady(); err != nil {
		return errors.Errorf("Could not wait until Workspace agent is ready: %+v", err)
	}

	go s.runHealthCheckLoop()

	s.syncState()

	prepareReq := &ccordiumv1.PrepareRequest{
		Domain:              req.ClientInfo.Domain,
		TunnelPeerPublicKey: req.TunnelPeerPublicKey,
		TunnelPrivateKey:    s.wgPrivateKey.String(),

		Workspace:       s.initReq.Workspace,
		Space:           s.initReq.Space,
		Template:        s.initReq.Template,
		SecretList:      s.initReq.SecretList,
		UserSecretList:  s.initReq.UserSecretList,
		UserConfig:      s.initReq.UserConfig,
		GitProviderInfo: s.initReq.GitProviderInfo,

		Ssh: s.initReq.Ssh,
	}

	if ldflags.IsDev() {
		zap.L().Debug("sending prepare request", zap.Any("req", prepareReq))
	} else {
		zap.L().Debug("sending prepare request")
	}

	for i := range 30 {
		if _, err := s.wsC.Prepare(ctx, prepareReq); err == nil {
			break
		}
		zap.L().Error("Could not send prepare API call", zap.Error(err))
		if i >= 29 {
			return errors.Errorf("Could not send prepare call to Workspace: %+v", err)
		}
		time.Sleep(1 * time.Second)
	}

	s.wgPrivateKey = nil
	zap.L().Debug("Successfully sent prepare request")

	go s.runListenEventLoop()

	zap.L().Debug("Successfully finished initialization")

	return nil
}

func (s *Server) runOcteliumProxy() error {

	if ldflags.IsTest() {
		return nil
	}

	if s.initReq.Workspace.Status.IsBuild {
		zap.L().Debug("No need to run Octelium proxy. This is a prebuild")
		return nil
	}

	if s.initReq.ClientInfo == nil {
		return errors.Errorf("No clientInfo in the request. Cannot run Octelium proxy")
	}

	var err error
	s.octeliumProxy, err = oproxy.NewOcteliumProxy(&oproxy.Opts{
		Domain:     s.initReq.ClientInfo.Domain,
		ClientInfo: s.initReq.ClientInfo,
	})
	if err != nil {
		return err
	}
	return s.octeliumProxy.Run(s.ctxMain)
}

func (s *Server) runSSHAgent() error {
	var err error
	if ldflags.IsTest() {
		return nil
	}

	if s.initReq.Workspace.Status.IsBuild {
		zap.L().Debug("No need to run SSH agent. This is a prebuild")
		return nil
	}

	s.sshAgent, err = sshagent.NewAgent(&sshagent.Opts{
		UserSecretList: s.initReq.UserSecretList,
	})
	if err != nil {
		return err
	}

	return s.sshAgent.Run(s.ctxMain)
}

func (s *Server) waitForPodmanRunAndShutdown() {
	if ldflags.IsTest() {
		return
	}

	go func() {
		if s.runCmd == nil {
			zap.L().Error("run cmd is nil. Cannot wait for cmd to exit...")
			return
		}

		zap.L().Debug("Starting waiting for the Workspace agent to exit")
		err := s.runCmd.Wait()
		zap.L().Debug("Workspace agent inside container exited...", zap.Error(err))

		if exiterr, ok := err.(*exec.ExitError); ok {
			zap.L().Debug("Workspace agent exit status code", zap.Int("code", exiterr.ExitCode()))
		}
		s.wsAgentExitCh <- err
	}()
}
