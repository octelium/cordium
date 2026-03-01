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
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
)

const tmpImageLocation = "/tmp/octelium-workspace-image.tar.gz"

func (s *Server) setFailure(failure *cordiumv1.Workspace_Status_Failure) {
	s.failureWrp.mu.Lock()
	defer s.failureWrp.mu.Unlock()
	zap.L().Debug("Setting failure", zap.Any("failure", failure))
	s.failureWrp.failure = failure
}

func (s *Server) getFailure() *cordiumv1.Workspace_Status_Failure {
	s.failureWrp.mu.RLock()
	defer s.failureWrp.mu.RUnlock()
	return s.failureWrp.failure
}

func (s *Server) chownDirOctelium(ctx context.Context, path string) error {
	cmd := getCommand(ctx, fmt.Sprintf("chown -R octelium:octelium %s", path))
	return cmd.Run()
}

func (s *Server) chownFileOctelium(ctx context.Context, path string) error {

	cmd := getCommand(ctx, fmt.Sprintf("chown octelium:octelium %s", path))
	return cmd.Run()
}

func (s *Server) getCommandAsRoot(ctx context.Context, cmdStr string) *exec.Cmd {
	cmd := getCommand(ctx, cmdStr)
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"TERM=xterm",
		"NO_PROXY=localhost,127.0.0.0/8,::1",
		fmt.Sprintf("OCTELIUM_WS_NAME=%s", os.Getenv("OCTELIUM_WS_NAME")),
	}

	if ldflags.IsDev() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd
}

func (s *Server) runAllCommandsAsOctelium(ctx context.Context, cmds []string) {
	for _, cmdStr := range cmds {
		if err := s.getCommandAsOctelium(ctx, cmdStr).Run(); err != nil {
			zap.L().Warn("Could not exec", zap.String("cmd", cmdStr), zap.Error(err))
		}
	}
}

func (s *Server) getCommandAsOctelium(ctx context.Context, cmdStr string) *exec.Cmd {
	zap.L().Debug("Getting command as octelium", zap.String("cmd", cmdStr))
	cmd := getCommand(ctx, cmdStr)

	// setEnv(&cmd.Env, "HOME", "/home/octelium")
	// setEnv(&cmd.Env, "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	// setEnv(&cmd.Env, "XDG_RUNTIME_DIR", "/octelium-runtime")

	for k, v := range s.getDefaultCmdEnvAsOctelium() {
		setEnv(&cmd.Env, k, v)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(s.octeliumUID), Gid: uint32(s.octeliumGID)},
	}

	if ldflags.IsDev() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd
}

func (s *Server) getDefaultCmdEnvAsOctelium() map[string]string {
	return map[string]string{
		"PATH":                    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME":                    "/home/octelium",
		"TERM":                    "xterm",
		"NO_PROXY":                "localhost,127.0.0.0/8,::1",
		"CONTAINERS_STORAGE_CONF": "/etc/containers/storage.conf",
	}
}

func (s *Server) getCommandAsOcteliumNOStdout(ctx context.Context, cmdStr string) *exec.Cmd {
	cmd := getCommand(ctx, cmdStr)
	for k, v := range s.getDefaultCmdEnvAsOctelium() {
		setEnv(&cmd.Env, k, v)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(s.octeliumUID), Gid: uint32(s.octeliumGID)},
	}

	return cmd
}

func (s *Server) isStatusEqual(st cordiumv1.Workspace_Status_State) bool {
	s.status.mu.Lock()
	defer s.status.mu.Unlock()
	return s.status.status == st
}

type envVar struct {
	Key   string
	Value string
}

/*
func (s *Server) isWorkspaceAtStatus(ctx context.Context, st cordiumv1.Workspace_Status_State) (bool, error) {

	ctx, cancelFn := context.WithTimeout(ctx, 1000*time.Millisecond)
	defer cancelFn()
	zap.L().Debug("Checking for Workspace status", zap.String("status", st.String()))
	resp, err := s.wsC.GetState(ctx, &ccordiumv1.GetStateRequest{}, grpc.WaitForReady(true))
	if err != nil {
		switch status.Code(err) {
		case codes.Canceled, codes.DeadlineExceeded, codes.Aborted, codes.Unknown, codes.Unavailable:
			zap.L().Debug("workspace probably still initializing", zap.Error(err))
			return false, nil
		default:
			return false, err
		}
	}

	zap.L().Debug("Workspace status is now at", zap.String("status", st.String()))
	return resp.State == st, nil
}
*/

type cpInfo struct {
	src string
	dst string
}

/*
func (s *Server) waitUntilStatusAndSet(ctx context.Context, status cordiumv1.Workspace_Status_State) error {
	zap.L().Debug("Waiting for Workspace status until", zap.String("status", status.String()))
	for i := 0; i < 1000; i++ {
		isEqual, err := s.isWorkspaceAtStatus(ctx, status)
		if err != nil {
			return errors.Errorf("Could not check if Workspace is ready: %+v", err)
		}
		if isEqual {
			s.setStatus(status)
			return nil
		}
		zap.L().Debug("Workspace is not at status yet. Trying again...", zap.String("status", status.String()))
		time.Sleep(1 * time.Second)
	}

	return errors.Errorf("Could not check if Workspace is ready after so many attempts")
}
*/

func (s *Server) syncState() {
	go func() {
		if err := s.doSyncState(); err != nil {
			zap.L().Error("Could not sync state", zap.Error(err))
		}
	}()
}
func (s *Server) doSyncState() error {
	strm, err := s.wsC.ListenState(s.ctxMain, &ccordiumv1.ListenStateRequest{})
	if err != nil {
		return err
	}

	for {
		select {
		case <-s.ctxMain.Done():
			return nil
		default:
			msg, err := strm.Recv()
			if err != nil {
				if grpcerr.IsCanceled(err) || grpcerr.IsDeadlineExceeded(err) {
					return nil
				}
				return err
			}

			zap.L().Debug("Syncing state from the Workspace agent", zap.String("state", msg.State.String()))
			s.setStatus(msg.State)
		}
	}
}

func getCommand(ctx context.Context, cmdStr string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", cmdStr)
}

func (s *Server) setStatus(st cordiumv1.Workspace_Status_State) {
	s.status.mu.Lock()
	defer s.status.mu.Unlock()
	zap.L().Debug("Setting status", zap.String("status", st.String()))
	s.status.status = st

	s.statusSubscribersMap.mu.RLock()
	defer s.statusSubscribersMap.mu.RUnlock()
	for _, sub := range s.statusSubscribersMap.subscribersMap {
		sub.statusCh <- st
	}
}

func (s *Server) getStatus() cordiumv1.Workspace_Status_State {
	s.status.mu.RLock()
	defer s.status.mu.RUnlock()
	return s.status.status
}

func (s *Server) replaceInFile(filePath string, old, new string) error {

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	newContent := strings.ReplaceAll(string(content), old, new)
	return os.WriteFile(filePath, []byte(newContent), 0)
}

func (s *Server) getUploadUID() string {

	if s.initReq.Workspace.Status.IsBuild {
		return s.initReq.Workspace.Status.TemplateRef.Uid
	}

	return s.wsUID
}

func setEnvExistingKey(envVars []string, key, val string) bool {
	for idx, envVar := range envVars {
		strs := strings.SplitAfterN(envVar, "=", 2)

		if len(strs) < 1 {
			continue
		}

		if strings.TrimSuffix(strs[0], "=") == key {
			envVars[idx] = fmt.Sprintf("%s=%s", key, val)
			return true
		}
	}
	return false
}

func setEnv(envVars *[]string, key, val string) {
	if !setEnvExistingKey(*envVars, key, val) {
		*envVars = append(*envVars, fmt.Sprintf("%s=%s", key, val))
	}
}

func (s *Server) publishLog(data []byte, typ cordiumv1.ListenLogResponse_Type, mode cordiumv1.ListenLogResponse_Mode) {
	s.eventPublisher.publish(&ccordiumv1.ListenEventResponse{
		Type: &ccordiumv1.ListenEventResponse_ListenLogResponse{
			ListenLogResponse: &cordiumv1.ListenLogResponse{
				CreatedAt: pbutils.Now(),
				Data:      data,
				Type:      typ,
				Mode:      mode,
			},
		},
	})
}

func getTagByID(id string) string {
	return fmt.Sprintf("id-%s", id)
}
