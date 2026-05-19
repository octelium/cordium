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

package workspace

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"slices"

	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	mu      sync.RWMutex
	grpcSrv *grpc.Server

	ccordiumv1.UnimplementedWorkspaceServiceServer
	ccordiumv1.UnimplementedTerminalServiceServer
	grpc_health_v1.UnimplementedHealthServer

	state cordiumv1.Workspace_Status_State

	taskManager *taskManager

	initReq *ccordiumv1.PrepareRequest
	ws      *cordiumv1.Workspace

	spec *cordiumv1.Workspace_Spec

	shellPath string

	userInfo *userInfo

	terminalSrv terminalSrv

	// failurePublisher *failurePublisher

	lis            net.Listener
	eventPublisher *eventPublisher

	env []string

	repoDir string

	// fromBuild  bool
	isFreshRun bool

	gitStore gitStore

	startedPrepare bool

	statusSubscribersMap struct {
		mu             sync.RWMutex
		subscribersMap map[string]*statusSubscription
	}

	runningWG sync.WaitGroup

	buildDoneCh chan struct{}
}

type userInfo struct {
	name    string
	uid     int
	gid     int
	homeDir string
	group   string
}

func Run(ctx context.Context) error {

	usr, err := user.Current()
	if err != nil {
		return err
	}

	if usr.Username != "root" {
		return errors.Errorf("Workspace agent is not running as root")
	}

	srv, err := NewServer(ctx)
	if err != nil {
		return errors.Errorf("Could not initialize Workspace agent: %+v", err)
	}

	if err := srv.Run(ctx); err != nil {
		return errors.Errorf("Could not run Workspace agent: %+v", err)
	}

	zap.L().Info("Workspace agent is running...")

	select {
	case <-ctx.Done():
		zap.L().Debug("Workspace agent received TERM signal")
	case <-srv.buildDoneCh:
		zap.L().Debug("Received Build termination signal")
	}

	return srv.Close()
}

func NewServer(ctx context.Context) (*Server, error) {
	server := &Server{
		state: cordiumv1.Workspace_Status_STARTING_RUNTIME,
		eventPublisher: &eventPublisher{
			subMap: make(map[string]*eventSubscription),
		},
		env:     os.Environ(),
		repoDir: "/workspace/repo",
		gitStore: gitStore{
			entryMap: make(map[string]*gitStoreEntry),
		},
		statusSubscribersMap: struct {
			mu             sync.RWMutex
			subscribersMap map[string]*statusSubscription
		}{
			subscribersMap: make(map[string]*statusSubscription),
		},
		buildDoneCh: make(chan struct{}, 5),
	}
	server.terminalSrv.termMap = make(map[string]*terminal)

	return server, nil
}

// const srvPort = 35921

/*
func (s *Server) runInitCmds(ctx context.Context) error {

	cmds := []string{
		// "mkdir -p /dev/net",
		// "mknod /dev/net/tun c 10 200",
		// "chmod 666 /dev/net/tun",
	}

	for _, cmdStr := range cmds {
		zap.L().Debug("running init workspace cmd", zap.String("cmd", cmdStr))

		cmd := s.getCmd(ctx, cmdStr)
		if err := cmd.Run(); err != nil {
			zap.L().Warn("Could not run init workspace cmd", zap.Error(err))
		}
	}

	return nil
}
*/

func (s *Server) Run(ctx context.Context) error {
	zap.L().Debug("Starting running Workspace agent")

	if err := s.setShell(); err != nil {
		return err
	}

	if err := s.setupCGroups(ctx); err != nil {
		zap.L().Warn("Could not setup cgroups", zap.Error(err))
	}

	if err := s.setSUIDBit(ctx); err != nil {
		zap.L().Warn("Could not set setSUID bit", zap.Error(err))
	}

	/*
		if err := s.runInitCmds(ctx); err != nil {
			zap.L().Warn("Could not run workspace init cmd", zap.Error(err))
		}
	*/

	var socketPath string
	if ldflags.IsTest() {
		socketPath = "/tmp/oct-ws.sock"
		os.Remove(socketPath)
	} else {
		socketPath = "/run/octelium/workspace.sock"
	}

	if err := func() error {
		var err error

		s.lis, err = net.Listen("unix", socketPath)
		if err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return err
	}

	go func() {
		time.Sleep(2 * time.Second)
		if err := os.Chmod(socketPath, 0777); err == nil {
			zap.L().Debug("Successfully chmoded socketPath")
		} else {
			zap.L().Warn("Could not chmod socketPath", zap.Error(err))
		}
	}()

	/*
		if err := func() error {
			var err error
			for i := 0; i < 100; i++ {
				s.lis, err = net.Listen("tcp", fmt.Sprintf(":%d", srvPort))
				if err == nil {
					zap.L().Debug("Workspace agent started listening on port", zap.Int("port", srvPort))
					return nil
				}
				time.Sleep(1 * time.Second)
				zap.L().Debug("Could not listen to Workspace port. Trying again...",
					zap.Error(err), zap.Int("listenPort", srvPort))
			}
			return errors.Errorf("Could not listen to Workspace port: %+v", err)
		}(); err != nil {
			return err
		}
	*/

	zap.L().Debug("Creating a new gRPC server")

	s.grpcSrv = grpc.NewServer(
		grpc.MaxConcurrentStreams(1000000),
	)

	grpc_health_v1.RegisterHealthServer(s.grpcSrv, s)
	ccordiumv1.RegisterWorkspaceServiceServer(s.grpcSrv, s)
	ccordiumv1.RegisterTerminalServiceServer(s.grpcSrv, s)

	go func() {

		zap.S().Debug("running Workspace agent gRPC server.")
		if err := s.grpcSrv.Serve(s.lis); err != nil {
			zap.L().Error("Workspace agent gRPC server exited", zap.Error(err))
			return
		}
	}()

	zap.L().Debug("Workspace agent is now running...")

	return nil
}

func (s *Server) Close() error {

	if s.taskManager != nil {
		if err := s.taskManager.close(); err != nil {
			zap.L().Error("Could not close task manager", zap.Error(err))
		}
	}

	if s.grpcSrv != nil {

		gracefulShutdownCh := make(chan struct{}, 10)

		go func() {
			s.grpcSrv.GracefulStop()
			gracefulShutdownCh <- struct{}{}
		}()

		select {
		case <-gracefulShutdownCh:
			zap.L().Debug("Workspace gRPC server gracefully stopped")
		case <-time.After(1200 * time.Millisecond):
			zap.L().Debug("Workspace gRPC server graceful shutdown timeout exceeded.")
		}
	}

	if s.lis != nil {
		if err := s.lis.Close(); err != nil {
			zap.L().Warn("Could not close listener", zap.Error(err))
		}
	}

	return nil
}

func (s *Server) setupCGroups(ctx context.Context) error {
	if ldflags.IsTest() {
		return nil
	}
	cmds := []string{
		`cat /proc/self/cgroup`,
		`mkdir -p /sys/fs/cgroup/init`,
		`xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs || :`,
		`sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers > /sys/fs/cgroup/cgroup.subtree_control`,
		`cat /proc/self/cgroup`,
	}

	zap.L().Debug("Setting cgroups")

	for _, cmdStr := range cmds {
		zap.L().Debug("running cmd", zap.String("cmd", cmdStr))

		cmd := s.getCmd(ctx, cmdStr)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	zap.L().Debug("Successfully set cgroups")

	return nil
}

func (s *Server) setUser(ctx context.Context) error {

	zap.L().Debug("Starting setting user")

	doSetUserInfo := func(usr *user.User) error {
		uid, err := strconv.Atoi(usr.Uid)
		if err != nil {
			return err
		}
		gid, err := strconv.Atoi(usr.Gid)
		if err != nil {
			return err
		}

		grp, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
		if err != nil {
			return err
		}

		s.userInfo = &userInfo{
			name:    usr.Username,
			uid:     uid,
			gid:     gid,
			homeDir: usr.HomeDir,
			group:   grp.Name,
		}
		return nil
	}

	overrideUser := func() string {
		/*
			if s.initReq != nil {
				return s.initReq.User
			}
		*/
		return ""
	}()

	if usr, err := user.Lookup(overrideUser); err == nil {
		zap.L().Debug("User is overridden in the prepare request", zap.String("user", overrideUser))
		if err := doSetUserInfo(usr); err != nil {
			return err
		}
	} else if usr, err := user.Lookup("octelium"); err == nil {
		zap.L().Debug("Found an already existent user with the name octelium")

		if err := doSetUserInfo(usr); err != nil {
			return err
		}

	} else if usr, err := user.LookupId("1000"); err == nil {
		zap.L().Debug("Found an existent user with a uid 1000")
		if err := doSetUserInfo(usr); err != nil {
			return err
		}
	} else {
		zap.L().Debug("Creating an octelium user with uid 1000")
		uid := 1000
		gid := 1000

		if _, err := exec.LookPath("useradd"); err == nil {
			cmds := []string{
				fmt.Sprintf("groupadd octelium --gid %d", gid),
				fmt.Sprintf("useradd octelium -g octelium -m -s %s --uid %d", s.shellPath, uid),
			}

			for _, cmdStr := range cmds {
				zap.L().Debug("running cmd", zap.String("cmd", cmdStr))

				cmd := s.getCmd(ctx, cmdStr)
				if err := cmd.Run(); err != nil {
					return err
				}
			}
		} else if _, err := exec.LookPath("adduser"); err == nil {
			cmds := []string{
				fmt.Sprintf("addgroup -g %d octelium", gid),
				fmt.Sprintf("adduser -G octelium -D -h /home/octelium -s %s -u %d octelium", s.shellPath, uid),
			}

			for _, cmdStr := range cmds {
				zap.L().Debug("running cmd", zap.String("cmd", cmdStr))

				cmd := s.getCmd(ctx, cmdStr)
				if err := cmd.Run(); err != nil {
					return err
				}
			}
		} else {
			return errors.Errorf("Cannot find either useradd or adduser commands. Cannot create octelium user")
		}

		usr, err := user.Lookup("octelium")
		if err != nil {
			return err
		}

		if err := doSetUserInfo(usr); err != nil {
			return err
		}
	}

	zap.L().Debug("User info is set",
		zap.Int("uid", s.userInfo.uid), zap.Int("gid", s.userInfo.gid),
		zap.String("name", s.userInfo.name), zap.String("group", s.userInfo.group),
		zap.String("homedir", s.userInfo.homeDir))

	if ldflags.IsTest() {
		return nil
	}

	if s.isFreshRun {
		if err := s.setSudoersFile(); err != nil {
			zap.L().Warn("Could not set sudoers file", zap.Error(err))
		}
	} else {
		zap.L().Debug("No need to add NOPASSWD to user sudoers")
	}

	if s.isFreshRun {

		/*
			zap.L().Debug("chowning home dir")
			if err := s.chownDirToUser(ctx, s.userInfo.homeDir); err != nil {
				return err
			}
		*/

		if vutils.FSPathExists("/workspace") {
			zap.L().Debug("chowning workspace dir")
			if err := s.chownDirToUser(ctx, "/workspace"); err != nil {
				zap.L().Warn("Could not chown workspace dir", zap.Error(err))
			}
		} else {
			zap.L().Debug("Could not find /workspace dir. Skipping chown")
		}

	} else {
		zap.L().Debug("No need to chown /workspace dir.")
	}

	zap.L().Debug("User successfully set")

	return nil
}

func (s *Server) setSudoersFile() error {
	sudoersDir := "/etc/sudoers.d"
	username := s.userInfo.name
	filePath := filepath.Join(sudoersDir, username)

	if _, err := os.Stat(sudoersDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sudoersDir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(filePath); err == nil {
		return nil
	}

	content := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: ALL\n", username)

	if err := os.WriteFile(filePath, []byte(content), 0440); err != nil {
		return err
	}

	return nil
}

func (s *Server) getCmd(ctx context.Context, cmdStr string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.shellPath, "-c", cmdStr)
	if ldflags.IsDev() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd
}

func (s *Server) getCmdAsUser(ctx context.Context, cmdStr string) *exec.Cmd {
	cmd := s.getCmd(ctx, cmdStr)
	if s.env != nil {
		cmd.Env = slices.Clone(s.env)
	} else {
		cmd.Env = os.Environ()
	}

	setEnv(&cmd.Env, "HOME", s.userInfo.homeDir)
	setEnv(&cmd.Env, "USERNAME", s.userInfo.name)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(s.userInfo.uid), Gid: uint32(s.userInfo.gid)},
	}
	return cmd
}

func (s *Server) chownDirToUser(ctx context.Context, dir string) error {
	cmdStr := fmt.Sprintf("chown -R %s:%s %s", s.userInfo.name, s.userInfo.group, dir)
	cmd := s.getCmd(ctx, cmdStr)
	if err := cmd.Run(); err != nil {
		zap.L().Error("Could not chown dir", zap.String("dir", dir), zap.Error(err))
	}
	return nil
}

func (s *Server) Prepare(ctx context.Context, req *ccordiumv1.PrepareRequest) (*ccordiumv1.PrepareResponse, error) {

	go s.startPrepare(req)

	return &ccordiumv1.PrepareResponse{}, nil
}

func (s *Server) startPrepare(req *ccordiumv1.PrepareRequest) {
	zap.L().Debug("status is now set to preparing")

	if err := s.doPrepare(context.Background(), req); err != nil {
		zap.L().Error("Could not doPrepare", zap.Error(err))
	}
}

func (s *Server) startWaitAndSetRunning() {
	if ldflags.IsTest() {
		s.setState(cordiumv1.Workspace_Status_RUNNING)
		return
	}
	zap.L().Debug("starting waitAndSetRunning")
	s.runningWG.Wait()
	zap.L().Debug("status is now set to running")
	s.setState(cordiumv1.Workspace_Status_RUNNING)
	s.initReq.SecretList = nil
	time.Sleep(1000 * time.Millisecond)
	if s.initReq.Workspace.Status.IsBuild || s.initReq.Workspace.Spec.AutoStop {
		s.buildDoneCh <- struct{}{}
	}
}

func (s *Server) doPrepare(ctx context.Context, req *ccordiumv1.PrepareRequest) error {

	s.mu.RLock()
	startedPrepare := s.startedPrepare
	s.mu.RUnlock()

	if startedPrepare {
		return nil
	}

	s.mu.Lock()
	s.startedPrepare = true
	s.mu.Unlock()

	var err error
	zap.L().Debug("Starting preparing the Workspace", zap.Any("req", req))
	s.initReq = req
	s.ws = req.Workspace

	s.ws, err = wsutils.Merge(&wsutils.MergeSpecReq{
		Workspace: req.Workspace,
		Template:  req.Template,
	})
	if err != nil {
		return err
	}
	s.spec = s.ws.Spec

	{

		s.isFreshRun = func() bool {
			ws := s.ws

			if ws.Status.IsBuild {
				return true
			}
			return (ws.Spec.IsEphemeral || ws.Status.SuccessfulRuns == 0) &&
				!(ucordiumv1.ToTemplate(s.initReq.Template).HasReadyBuild() &&
					s.initReq.TemplateHasSnapshot)
		}()
	}

	if err := s.setUser(ctx); err != nil {
		return errors.Errorf("Could not set user: %+v", err)
	}

	if err := s.doShallowCloneMainRepository(ctx); err != nil {
		zap.L().Warn("Could not doShallowCloneMainRepository", zap.Error(err))
	}

	if err := s.setWorkspaceFile(ctx); err != nil {
		zap.L().Warn("Could not set workspace.yaml file spec", zap.Error(err))
	}

	s.setEnvVars(ctx)

	if ldflags.IsTest() {
		s.taskManager, err = s.newTaskManager()
		if err != nil {
			return err
		}
		s.setState(cordiumv1.Workspace_Status_RUNNING)
		return nil
	}

	if err := s.setSSHKeys(ctx); err != nil {
		zap.L().Warn("Could not set SSH keys", zap.Error(err))
	}

	if err := s.setHostsFile(ctx); err != nil {
		zap.L().Warn("Could not set hosts file", zap.Error(err))
	}

	if err := runTunnel(ctx, req); err != nil {
		zap.L().Warn("Could not run Workspace tunnel", zap.Error(err))
	}

	s.setState(cordiumv1.Workspace_Status_PREPARING)

	s.taskManager, err = s.newTaskManager()
	if err != nil {
		return err
	}

	if err := s.setDevContainer(ctx); err != nil {
		zap.L().Warn("Could not setDevContainer", zap.Error(err))
	}
	if err := s.setupDevContainersFeatures(ctx); err != nil {
		zap.L().Warn("Could not setDevContainers features", zap.Error(err))
	}

	if err := s.setupGit(ctx); err != nil {
		zap.L().Warn("Could not setupGit", zap.Error(err))
	}

	if err := s.setupDotFiles(ctx); err != nil {
		zap.L().Warn("Could not setup dotfiles", zap.Error(err))
	}

	if err := s.completeRepoClone(ctx); err != nil {
		zap.L().Error("Could not complete repo clone", zap.Error(err))
	}

	s.runningWG.Add(1)

	go s.startWaitAndSetRunning()

	if err := s.taskManager.run(); err != nil {
		return err
	}

	s.runningWG.Done()

	return nil
}

func (s *Server) GetState(ctx context.Context, req *ccordiumv1.GetStateRequest) (*ccordiumv1.GetStateResponse, error) {
	zap.L().Debug("New getState request")
	s.mu.RLock()
	defer s.mu.RUnlock()

	zap.L().Debug("GetStatus request", zap.String("currentStatus", s.state.String()))

	return &ccordiumv1.GetStateResponse{
		State: s.state,
	}, nil
}

func (s *Server) setShell() error {

	shells := []string{"bash", "sh", "zsh"}

	for _, sh := range shells {
		path, err := exec.LookPath(sh)
		if err == nil {
			zap.L().Debug("Found shell", zap.String("shell", sh), zap.String("path", path))
			s.shellPath = path
			return nil
		}
	}

	return errors.Errorf("Could not find shell path")
}

func (s *Server) setSSHKeys(ctx context.Context) error {

	if s.ws.Status.IsBuild {
		zap.L().Debug("This is a prebuild. No need to set SSH keys")
		return nil
	}

	zap.L().Debug("Starting setting SSH config")
	/*
		if err := os.Chown("/var/run/octelium-ssh-agent.sock", s.userInfo.uid, s.userInfo.gid); err != nil {
			zap.L().Error("Could not chown ssh agent socket", zap.Error(err))
		}
	*/

	sshDir := path.Join(s.userInfo.homeDir, ".ssh")
	if _, err := os.Stat(sshDir); err == nil {
		zap.L().Debug("SSH dir already exists. Nothing to be done")
		return nil
	} else if !os.IsNotExist(err) {
		return errors.Errorf("Could not stat ssh dir: %+v", err)
	}

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}

	if f, err := os.OpenFile(path.Join(sshDir, "known_hosts"), os.O_RDONLY|os.O_CREATE, 0600); err != nil {
		return err
	} else {
		f.Close()
	}

	if f, err := os.OpenFile(path.Join(sshDir, "authorized_keys"), os.O_RDONLY|os.O_CREATE, 0600); err != nil {
		return err
	} else {
		f.Close()
	}

	if err := s.chownDirToUser(ctx, sshDir); err != nil {
		return err
	}

	zap.L().Debug("Successfully set SSH config")

	return nil
}

type terminalSrv struct {
	termMap map[string]*terminal
	mu      sync.RWMutex
}

func (s *terminalSrv) get(id string) (*terminal, bool) {
	if err := wsutils.CheckTerminalID(id); err != nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret, ok := s.termMap[id]
	return ret, ok
}

func (s *terminalSrv) delete(id string) {
	if err := wsutils.CheckTerminalID(id); err != nil {
		return
	}
	s.mu.Lock()
	delete(s.termMap, id)
	s.mu.Unlock()
}

func (s *terminalSrv) len() int {

	s.mu.RLock()
	ret := len(s.termMap)
	s.mu.RUnlock()
	return ret
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

/*
func (s *Server) ListenFailure(req *ccordiumv1.ListenFailureRequest, srv ccordiumv1.WorkspaceService_ListenFailureServer) error {

	ctx := srv.Context()

	zap.L().Debug("Starting ListenFailure loop")

	sub := s.failurePublisher.subscribe()
	defer s.failurePublisher.unsubscribe(sub.id)

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting ListenFailure. ctx done")
			return nil
		case msg, ok := <-sub.failure:
			if !ok {
				zap.L().Debug("Exiting ListenTerminal. Subscription ended")
				return nil
			}

			if err := srv.Send(&ccordiumv1.ListenFailureResponse{
				Failure: msg,
			}); err != nil {
				zap.L().Error("Could not send listenFailureResp",
					zap.Error(err))
			}
		}
	}

}
*/

/*
type failurePublisher struct {
	mu     sync.RWMutex
	subMap map[string]*failureSubscription
}

func (p *failurePublisher) subscribe() *failureSubscription {
	ret := &failureSubscription{
		id:      vutils.UUIDv4(),
		failure: make(chan *cordiumv1.Workspace_Status_Failure, 1000),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subMap[ret.id] = ret
	return ret
}

func (p *failurePublisher) unsubscribe(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.subMap, id)
}

func (p *failurePublisher) publish(failure *cordiumv1.Workspace_Status_Failure) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	zap.L().Debug("Publishing failure", zap.Any("failure", failure))
	for _, itm := range p.subMap {
		itm.failure <- failure
	}
}


type failureSubscription struct {
	id      string
	failure chan *cordiumv1.Workspace_Status_Failure
}
*/

func (s *Server) setSUIDBit(ctx context.Context) error {

	if ldflags.IsTest() {
		return nil
	}
	binList := []string{
		"sudo", "su", "ping",
	}

	zap.L().Debug("Setting suid bit for sudo to make sure it was not lost by podman")

	for _, bin := range binList {
		if binaryPath, err := exec.LookPath(bin); err == nil {
			if err := s.getCmd(ctx, fmt.Sprintf("chmod 4755 %s", binaryPath)).Run(); err != nil {
				zap.L().Warn("Could not set setuid bit", zap.String("file", binaryPath), zap.Error(err))
			}
		}
	}

	return nil
}

func (s *Server) setEnvVars(_ context.Context) {
	if s.initReq == nil || s.ws == nil {
		return
	}

	s.env = os.Environ()

	if s.initReq.UserConfig != nil {
		cfg := s.initReq.UserConfig
		for _, env := range cfg.Spec.EnvVars {
			switch env.Type.(type) {
			case *cordiumv1.UserConfig_Spec_EnvVar_Value:
				setEnv(&s.env, env.Key, env.GetValue())
			case *cordiumv1.UserConfig_Spec_EnvVar_FromUserSecret:
				if s.initReq.UserSecretList != nil {
					sec, err := ucordiumv1.ToUserSecretList(s.initReq.UserSecretList).GetByName(env.GetFromUserSecret())
					if err == nil {
						setEnv(&s.env, env.Key, ucordiumv1.ToUserSecret(sec).GetValueStr())
					}
				}
			}
		}
	}

	if s.initReq.Space != nil && s.initReq.Space.Spec.Runtime != nil {
		for _, env := range s.initReq.Space.Spec.Runtime.EnvVars {
			switch env.Type.(type) {
			case *cordiumv1.Workspace_Spec_Runtime_EnvVar_Value:
				setEnv(&s.env, env.Key, env.GetValue())
			case *cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret:
				if s.initReq.SecretList != nil {
					sec, err := ucordiumv1.ToSecretList(s.initReq.SecretList).GetByName(env.GetFromSecret())
					if err == nil {
						setEnv(&s.env, env.Key, ucordiumv1.ToSecret(sec).GetValueStr())
					}
				}
			}
		}
	}

	if s.spec != nil {
		spec := s.spec

		if spec.Runtime != nil {
			for _, env := range spec.Runtime.EnvVars {
				switch env.Type.(type) {
				case *cordiumv1.Workspace_Spec_Runtime_EnvVar_Value:
					setEnv(&s.env, env.Key, env.GetValue())
				case *cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret:
					if s.initReq.SecretList != nil {
						sec, err := ucordiumv1.ToSecretList(s.initReq.SecretList).GetByName(env.GetFromSecret())
						if err == nil {
							setEnv(&s.env, env.Key, ucordiumv1.ToSecret(sec).GetValueStr())
						}
					}
				}
			}
		}
	}

	setEnv(&s.env, "OCTELIUM_AUTH_PROXY_SOCKET", "/var/run/octelium-proxy.sock")
	setEnv(&s.env, "OCTELIUM_DOMAIN", s.initReq.Domain)

	if s.ws != nil {
		setEnv(&s.env, "CORDIUM_NAME", s.ws.Metadata.Name)
		setEnv(&s.env, "CORDIUM_HOSTNAME", s.ws.Status.Hostname)
	}

	setEnv(&s.env, "SSH_AUTH_SOCK", "/var/run/octelium-ssh-agent.sock")
	setEnv(&s.env, "OCTELIUM_HOME", "mem")
	// setEnv(&s.env, "CONTAINER_HOST", "unix:///var/run/docker.sock")

	zap.L().Debug("Env vars set to", zap.Strings("env", s.env))

}

func (s *Server) getDevContainerJSONPath(ctx context.Context) string {

	spec := s.spec

	repoDir := s.repoDir

	if !vutils.FSPathExists(repoDir) {
		return ""
	}

	if spec.Repository == nil {
		return ""
	}

	if spec.Image != nil {
		switch spec.Image.Type.(type) {
		case *cordiumv1.Workspace_Spec_Image_Dockerfile_,
			*cordiumv1.Workspace_Spec_Image_Git_,
			*cordiumv1.Workspace_Spec_Image_Registry_:
			return ""

		case *cordiumv1.Workspace_Spec_Image_Repository_:
			switch spec.Image.GetRepository().Type.(type) {
			case *cordiumv1.Workspace_Spec_Image_Repository_Devcontainer_:
				dPath := path.Join(repoDir, spec.Image.GetRepository().GetDevcontainer().DirPath)
				if vutils.FSPathExists(dPath) {
					if dPath == repoDir {
						return path.Join(repoDir, ".devcontainer.json")
					} else {
						return path.Join(repoDir, "devcontainer.json")
					}
				}

			default:
				return ""
			}
		}
	}

	if vutils.FSPathExists(path.Join(repoDir, ".devcontainer/devcontainer.json")) {
		return path.Join(repoDir, ".devcontainer/devcontainer.json")
	} else if vutils.FSPathExists(path.Join(repoDir, ".devcontainer.json")) {
		return path.Join(repoDir, ".devcontainer.json")
	}

	return ""
}

func (s *Server) setWorkspaceFile(ctx context.Context) error {

	if ldflags.IsTest() {
		return nil
	}

	/*
		var filePath string

		zap.L().Debug("Searching for a workspace.yaml file to be merged with the Workspace spec")
		if exists, _ := vutils.FSPathExistent(path.Join(repoDir, ".octelium/workspace.yaml")); exists {
			filePath = path.Join(repoDir, ".octelium/workspace.yaml")
		} else if exists, _ := vutils.FSPathExistent(path.Join(repoDir, ".octelium/workspace.yml")); exists {
			filePath = path.Join(repoDir, ".octelium/workspace.yml")
		} else {
			zap.L().Debug("No workspace.yaml in this repo. Nothing to be done...")
			return nil
		}
		if filePath == "" {
			return nil
		}

		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		ws := &cordiumv1.Workspace{}
		if err := pbutils.UnmarshalYAML(contentBytes, ws); err != nil {
			return err
		}

		if ws.Spec == nil {
			return nil
		}
	*/

	if ws, err := wsutils.LoadWorkspaceFile(ctx, &wsutils.LoadWorkspaceFileRequest{
		Parent:         s.ws,
		BaseDir:        s.repoDir,
		Space:          s.initReq.Space,
		SecretList:     s.initReq.SecretList,
		UserSecretList: s.initReq.UserSecretList,
	}); err == nil {
		zap.L().Debug("Found a workspace yaml file", zap.Any("spec", ws.Spec))

		s.ws, err = wsutils.Merge(&wsutils.MergeSpecReq{
			Workspace:      s.initReq.Workspace,
			Template:       s.initReq.Template,
			ChildWorkspace: ws,
		})
		if err != nil {
			return err
		}
		s.spec = s.ws.Spec
		zap.L().Debug("Merged spec after adding the workspac.yaml", zap.Any("spec", s.spec))
	}

	return nil
}

func (s *Server) setHostsFile(ctx context.Context) error {
	contents := `
127.0.0.1 localhost octelium
::1 localhost ip6-localhost ip6-loopback
`
	if err := os.WriteFile("/etc/hosts", []byte(contents), 0644); err != nil {
		return err
	}

	return nil
}

/*
func (s *Server) WaitUntilReady(req *ccordiumv1.WaitUntilReadyRequest, srv ccordiumv1.WorkspaceService_WaitUntilReadyServer) error {

	ctx := srv.Context()

	doFn := sync.OnceFunc(func() {
		zap.L().Debug("Workspace agent is now ready for prepare call")
		if err := srv.Send(&ccordiumv1.WaitUntilReadyResponse{}); err != nil {
			zap.L().Error("Could not send waitUntilReadyResponse", zap.Error(err))
		}
	})

	s.mu.RLock()
	isReady := s.isReady
	s.mu.RUnlock()

	if isReady {
		zap.L().Debug("Workspace is already ready. No need to wait")
		doFn()
		return nil
	}

	zap.L().Debug("Starting for waitUntilReady signal")
	select {
	case <-ctx.Done():
	case <-s.waitReadyCh:
		doFn()
	}

	return nil
}
*/

func (s *Server) setState(st cordiumv1.Workspace_Status_State) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()

	s.statusSubscribersMap.mu.RLock()
	defer s.statusSubscribersMap.mu.RUnlock()
	for _, sub := range s.statusSubscribersMap.subscribersMap {
		sub.statusCh <- st
	}
}

type statusSubscription struct {
	id       string
	statusCh chan cordiumv1.Workspace_Status_State
}

func (s *Server) subscribeStatusListener() *statusSubscription {
	statusSubscription := &statusSubscription{
		id:       utilrand.GetRandomString(12),
		statusCh: make(chan cordiumv1.Workspace_Status_State, 1000),
	}
	s.statusSubscribersMap.mu.Lock()
	defer s.statusSubscribersMap.mu.Unlock()
	s.statusSubscribersMap.subscribersMap[statusSubscription.id] = statusSubscription

	return statusSubscription
}

func (s *Server) unsubscribeStatusListener(sub *statusSubscription) {

	s.statusSubscribersMap.mu.Lock()
	defer s.statusSubscribersMap.mu.Unlock()
	delete(s.statusSubscribersMap.subscribersMap, sub.id)
}

func (s *Server) ListenState(req *ccordiumv1.ListenStateRequest, srv ccordiumv1.WorkspaceService_ListenStateServer) error {

	ctx := srv.Context()

	zap.L().Debug("ListenState request")

	s.mu.RLock()
	initState := s.state
	s.mu.RUnlock()

	zap.L().Debug("Sending init status in ListenState", zap.String("status", initState.String()))
	if err := srv.Send(&ccordiumv1.ListenStateResponse{
		State: initState,
	}); err != nil {
		return err
	}

	sub := s.subscribeStatusListener()

	defer s.unsubscribeStatusListener(sub)

	zap.L().Debug("Starting ListenState loop")

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting ListenState. ctx done")
			return nil
		case status, ok := <-sub.statusCh:
			if !ok {
				return nil
			}

			zap.L().Debug("Sending state on change", zap.String("state", status.String()))
			if err := srv.Send(&ccordiumv1.ListenStateResponse{
				State: status,
			}); err != nil {
				return err
			}
		}
	}

}
