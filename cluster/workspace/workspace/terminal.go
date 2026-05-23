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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"slices"

	"github.com/creack/pty"
	"github.com/moby/term"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const maxSizeStdout = 3 * 1024
const maxSizeTermBuf = 500 * 1000

type terminal struct {
	id        string
	cmd       *exec.Cmd
	createdAt time.Time

	pty *os.File
	tty *os.File

	stdinCh  chan []byte
	stdoutCh chan []byte

	subscribers struct {
		subscribersMap map[string]*terminalSubscription
		mu             sync.RWMutex
	}

	mu sync.RWMutex

	isClosed bool

	closeCh struct {
		ch   chan struct{}
		mu   sync.Mutex
		done bool
	}

	stdinCloseCh  chan struct{}
	stdoutCloseCh chan struct{}

	cancelFn context.CancelFunc

	buf []byte

	winSize winSize
}

type winSize struct {
	width  uint16
	height uint16
}

func (s *Server) genTerminalID() string {
	wsName := func() string {
		if s.ws != nil {
			return s.ws.Metadata.Name
		}
		return os.Getenv("CORDIUM_NAME")
	}()

	return fmt.Sprintf("%s-%s", wsName, utilrand.GetRandomStringLowercase(4))
}

func (s *Server) newTerminal(req *ccordiumv1.CreateTerminalRequest) (*terminal, error) {

	ret := &terminal{
		id:        s.genTerminalID(),
		createdAt: time.Now(),

		stdinCh:       make(chan []byte, 1000),
		stdoutCh:      make(chan []byte, 1000),
		stdinCloseCh:  make(chan struct{}, 2),
		stdoutCloseCh: make(chan struct{}, 2),
	}
	ret.closeCh.ch = make(chan struct{})
	ret.subscribers.subscribersMap = map[string]*terminalSubscription{}

	if s.userInfo == nil {
		return nil, errors.Errorf("Terminal user owner is not available. Please trying again later")
	}

	zap.L().Debug("Creating a new terminal", zap.String("id", ret.id))

	var err error

	shellPath, err := s.getShellPath()
	if err != nil {
		return nil, err
	}

	ret.pty, ret.tty, err = pty.Open()
	if err != nil {
		return nil, err
	}

	if err := ret.setWinSize(uint16(req.Cols), uint16(req.Rows)); err != nil {
		return nil, err
	}

	zap.L().Debug("Initial win size set",
		zap.Int("width", int(ret.winSize.width)),
		zap.Int("height", int(ret.winSize.height)))

	cmd := &exec.Cmd{
		Path: shellPath,
		Dir: func() string {
			if ldflags.IsTest() {
				dir, err := os.UserHomeDir()
				if err != nil {
					return "/tmp"
				}

				return dir
			}
			return s.userInfo.homeDir
		}(),
	}

	cmd.Stdin = ret.tty
	cmd.Stdout = ret.tty
	cmd.Stderr = ret.tty

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:     true,
		Setctty:    true,
		Credential: &syscall.Credential{Uid: uint32(s.userInfo.uid), Gid: uint32(s.userInfo.gid)},
	}

	if ldflags.IsTest() {
		cmd.SysProcAttr.Credential = nil
	}

	domain := s.initReq.Domain

	cmd.Env = s.getTerminalEnv(domain, shellPath)

	ret.cmd = cmd

	zap.L().Debug("terminal successfully created", zap.String("id", ret.id))

	return ret, nil
}

func (s *Server) getShellPath() (string, error) {
	res, err := parsePasswdFile()
	if err != nil {
		zap.L().Debug("Could nto get shellPath from passwdFile", zap.Error(err))
		return getShellPath()
	}

	ret := res[s.userInfo.name].Shell

	zap.L().Debug("Found shell in passwd file", zap.String("shell", ret))
	return ret, nil
}

func getShellPath() (string, error) {
	shells := []string{"zsh", "bash", "sh"}

	for _, sh := range shells {
		path, err := exec.LookPath(sh)
		if err == nil {
			zap.L().Debug("Found shell", zap.String("shell", sh), zap.String("path", path))
			return path, nil
		}
	}
	return "", errors.Errorf("Could not find shell path")
}

func (t *terminal) setWinSize(w, h uint16) error {
	cur := t.getWinSize()
	if cur.width == w && cur.height == h {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := term.SetWinsize(t.pty.Fd(), &term.Winsize{
		Width:  w,
		Height: h,
	}); err != nil {
		zap.L().Warn("Could not set terminal winSize", zap.String("id", t.id), zap.Error(err))
		return grpcutils.Internal("Could not set terminal size")
	}

	winSize, err := term.GetWinsize(t.pty.Fd())
	if err != nil {
		zap.L().Warn("Could not get winSize", zap.String("id", t.id), zap.Error(err))
		t.winSize.width = w
		t.winSize.height = h
	} else {
		t.winSize.width = winSize.Width
		t.winSize.height = winSize.Height
	}

	return nil
}

func (s *Server) getTerminalEnv(domain string, shellPath string) []string {
	editor := func() string {
		if _, err := exec.LookPath("vim"); err == nil {
			return "vim"
		}
		return "nano"
	}()

	env := slices.Clone(s.env)

	setEnv(&env, "USERNAME", s.userInfo.name)
	setEnv(&env, "TERM", "xterm-256color")
	setEnv(&env, "COLORTERM", "truecolor")
	setEnv(&env, "HOME", s.userInfo.homeDir)
	setEnv(&env, "LANG", "en_US.utf8")
	// setEnv(&env, "LANGUAGE", "en_US:en")
	// setEnv(&env, "LC_ALL", "en_US.utf8")
	setEnv(&env, "SHELL", shellPath)
	setEnv(&env, "OCTELIUM_DOMAIN", domain)
	setEnv(&env, "EDITOR", editor)
	setEnv(&env, "VISUAL", editor)
	setEnv(&env, "OCTELIUM_AUTH_PROXY_SOCKET", "/var/run/octelium-proxy.sock")
	setEnv(&env, "SSH_AUTH_SOCK", "/var/run/octelium-ssh-agent.sock")
	// setEnv(&env, "CONTAINER_HOST", "unix:///var/run/docker.sock")

	return env
}

func (t *terminal) run(ctx context.Context) error {
	zap.S().Debugf("Starting running terminal for id: %s", t.id)

	ctx, cancelFn := context.WithCancel(ctx)
	t.cancelFn = cancelFn

	err := t.cmd.Start()
	if err != nil {
		return err
	}

	// go t.startRecvLoop(ctx)
	go t.startSendLoop(ctx)

	go t.startStdinLoop(ctx)
	go t.startStdoutLoop(ctx)

	t.tty.Close()
	t.tty = nil

	go t.waitAndClose(ctx)

	zap.L().Debug("Terminal is now running", zap.String("id", t.id))

	return nil
}

func (t *terminal) startStdinLoop(ctx context.Context) {
	defer zap.L().Debug("Exiting terminal stdin loop", zap.String("uid", t.id))
	defer close(t.stdinCloseCh)
	for {
		select {
		case <-ctx.Done():
			return
		case buf := <-t.stdinCh:
			_, err := t.pty.Write(buf)
			if err != nil {
				zap.L().Debug("Could not write to pty", zap.Error(err))
				return
			}
		}
	}
}

func (t *terminal) startStdoutLoop(ctx context.Context) {

	defer zap.L().Debug("Exiting terminal stdoutLoop", zap.String("uid", t.id))
	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, maxSizeStdout)
			n, err := t.pty.Read(buf)
			if err != nil {
				return
			}
			t.stdoutCh <- buf[:n]
		}
	}
}

type terminalSubscription struct {
	id    string
	msgCh chan *ccordiumv1.ListenTerminalResponse
}

func (t *terminal) subscribe() *terminalSubscription {
	ret := &terminalSubscription{
		id:    vutils.UUIDv4(),
		msgCh: make(chan *ccordiumv1.ListenTerminalResponse, 1000),
	}
	t.subscribers.mu.Lock()
	t.subscribers.subscribersMap[ret.id] = ret
	t.subscribers.mu.Unlock()

	return ret
}

func (t *terminal) unsubscribe(id string) {

	t.subscribers.mu.Lock()
	delete(t.subscribers.subscribersMap, id)
	t.subscribers.mu.Unlock()

}

/*
func (t *terminal) startRecvLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := t.stream.Recv()
			if err != nil {
				continue
			}

			if err := t.handleRecv(msg); err != nil {
				zap.L().Debug("Could not handle rcv msg", zap.Error(err))
			}
		}
	}
}

*/

func (t *terminal) startSendLoop(ctx context.Context) {
	defer zap.L().Debug("Exiting terminal sendLoop", zap.String("uid", t.id))
	defer close(t.stdoutCloseCh)
	for {
		select {
		case <-ctx.Done():
			return
		case buf, ok := <-t.stdoutCh:
			if !ok {
				return
			}
			t.setBuf(buf[:])
			t.publishMsg(&ccordiumv1.ListenTerminalResponse{
				Type: &ccordiumv1.ListenTerminalResponse_Stdout_{
					Stdout: &ccordiumv1.ListenTerminalResponse_Stdout{
						Data: buf[:],
					},
				},
			})
		}
	}
}

func (t *terminal) publishMsg(msg *ccordiumv1.ListenTerminalResponse) {
	t.subscribers.mu.RLock()
	for _, sub := range t.subscribers.subscribersMap {
		sub.msgCh <- msg
	}
	t.subscribers.mu.RUnlock()
}

/*
func (t *terminal) handleRecv(msg *ccordiumv1.StartTerminalRequest) error {
	switch msg.Type.(type) {
	case *ccordiumv1.StartTerminalRequest_ChangeWindow_:
		return t.setWinSize(uint16(msg.GetChangeWindow().Width), uint16(msg.GetChangeWindow().Height))
	case *ccordiumv1.StartTerminalRequest_Stdin_:
		t.stdinCh <- msg.GetStdin().Data
	}
	return nil
}
*/

func (t *terminal) waitAndClose(ctx context.Context) {
	zap.S().Debugf("Starting waiting for shell to exit: %s", t.id)

	waitCh := make(chan error)
	go func() {
		err := t.cmd.Wait()
		if err != nil {
			zap.S().Debugf("cmd wait err: %+v", err)
		}
		waitCh <- err
	}()

	skipKillProcess := false

	zap.S().Debugf("waiting for cmd to close")
	select {
	case <-ctx.Done():
	case <-waitCh:
		zap.S().Debugf("cmd wait returned")
		skipKillProcess = true
	case <-t.stdinCloseCh:
	case <-t.stdoutCloseCh:
	case <-t.closeCh.ch:
		zap.S().Debugf("terminal closed. Killing cmd")

	}
	t.publishMsg(&ccordiumv1.ListenTerminalResponse{
		Type: &ccordiumv1.ListenTerminalResponse_Close_{
			Close: &ccordiumv1.ListenTerminalResponse_Close{},
		},
	})

	if !skipKillProcess {
		if err := t.cmd.Process.Kill(); err != nil {
			zap.S().Debugf("cmd kill err: %+v", err)
		}
	}

	if err := t.close(); err != nil {
		zap.L().Debug("Could not close terminal", zap.String("id", t.id), zap.Error(err))
	}
}

func (t *terminal) killAndClose() error {
	t.closeCh.mu.Lock()
	defer t.closeCh.mu.Unlock()
	if t.closeCh.done {
		return nil
	}
	zap.L().Debug("sending closeCh for terminal", zap.String("id", t.id))
	t.closeCh.done = true
	close(t.closeCh.ch)
	return nil
}

func (t *terminal) close() error {

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.isClosed {
		return nil
	}

	t.isClosed = true
	t.cancelFn()

	t.subscribers.mu.Lock()
	for _, sub := range t.subscribers.subscribersMap {
		close(sub.msgCh)
	}
	t.subscribers.mu.Unlock()

	zap.S().Debugf("closing terminal for uid: %s", t.id)
	if t.tty != nil {
		t.tty.Close()
		t.tty = nil
	}

	if t.pty != nil {
		t.pty.Close()
		t.pty = nil
	}

	return nil
}

func (t *terminal) setBuf(arg []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf = append(t.buf, arg...)

	if len(t.buf) > maxSizeTermBuf {
		t.buf = t.buf[len(t.buf)-maxSizeTermBuf:]
	}
}

func (s *Server) CreateTerminal(ctx context.Context, req *ccordiumv1.CreateTerminalRequest) (*ccordiumv1.CreateTerminalResponse, error) {

	if s.terminalSrv.len() > 150 {
		return nil, grpcutils.InvalidArg("Too many Terminals")
	}

	term, err := s.newTerminal(req)
	if err != nil {
		zap.L().Warn("Could not create new terminal", zap.Error(err))
		return nil, err
	}

	if err := term.run(context.Background()); err != nil {
		zap.L().Warn("Could not run terminal", zap.Error(err))
		return nil, err
	}

	s.terminalSrv.mu.Lock()
	s.terminalSrv.termMap[term.id] = term
	s.terminalSrv.mu.Unlock()

	return &ccordiumv1.CreateTerminalResponse{
		Id: term.id,
	}, nil
}

func (s *Server) RemoveTerminal(ctx context.Context, req *ccordiumv1.RemoveTerminalRequest) (*ccordiumv1.RemoveTerminalResponse, error) {

	zap.L().Debug("RemoveTerminal request", zap.Any("req", req))

	term, ok := s.terminalSrv.get(req.Id)
	if !ok {
		zap.L().Debug("No terminal found to be deleted. Nothing to be done", zap.String("id", req.Id))
		return &ccordiumv1.RemoveTerminalResponse{}, nil
	}

	go func() {
		if err := term.killAndClose(); err != nil {
			zap.L().Warn("Could not killAndclose terminal", zap.String("uid", term.id), zap.Error(err))
		}
	}()

	s.terminalSrv.delete(req.Id)

	return &ccordiumv1.RemoveTerminalResponse{}, nil
}

func (s *Server) ListTerminal(ctx context.Context, req *ccordiumv1.ListTerminalRequest) (*ccordiumv1.ListTerminalResponse, error) {

	termList := s.getTerminalList()
	ret := &ccordiumv1.ListTerminalResponse{}

	for _, term := range termList {
		ret.Items = append(ret.Items, &ccordiumv1.Terminal{
			Id: term.id,
		})
	}

	zap.L().Debug("ListTerminal request", zap.Any("resp", ret))

	return ret, nil
}

func (s *Server) getTerminalList() []*terminal {
	s.terminalSrv.mu.RLock()
	defer s.terminalSrv.mu.RUnlock()

	var ret []*terminal

	for _, term := range s.terminalSrv.termMap {
		ret = append(ret, term)
	}

	slices.SortStableFunc(ret, func(a, b *terminal) int {
		if a.createdAt.After(b.createdAt) {
			return 1
		}
		return -1
	})

	return ret
}

func (s *Server) ListenTerminal(req *ccordiumv1.ListenTerminalRequest, srv ccordiumv1.TerminalService_ListenTerminalServer) error {

	ctx := srv.Context()
	term, ok := s.terminalSrv.get(req.Id)
	if !ok {
		return grpcutils.InvalidArg("Terminal not found")
	}

	if err := term.sendInitListenMsg(srv); err != nil {
		return err
	}

	sub := term.subscribe()
	defer term.unsubscribe(sub.id)

	zap.L().Debug("Starting ListenTerminal loop",
		zap.String("id", term.id),
		zap.String("subID", sub.id))

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting ListenTerminal. ctx done",
				zap.String("id", term.id))
			return nil
		case msg, ok := <-sub.msgCh:
			if !ok {
				zap.L().Debug("Exiting ListenTerminal. Subscription ended",
					zap.String("id", term.id),
					zap.String("subID", sub.id))
				return nil
			}

			if err := srv.Send(msg); err != nil {
				zap.L().Error("Could not send terminal stdout",
					zap.String("id", term.id), zap.Error(err))
			}
		}
	}

}

func (term *terminal) sendInitListenMsg(srv ccordiumv1.TerminalService_ListenTerminalServer) error {
	term.mu.RLock()
	defer term.mu.RUnlock()

	if err := srv.Send(&ccordiumv1.ListenTerminalResponse{
		Type: &ccordiumv1.ListenTerminalResponse_WindowSize_{
			WindowSize: &ccordiumv1.ListenTerminalResponse_WindowSize{
				Cols: uint32(term.winSize.width),
				Rows: uint32(term.winSize.height),
			},
		},
	}); err != nil {
		zap.L().Error("Could not send initial terminal winSize",
			zap.String("id", term.id), zap.Error(err))
		return grpcutils.InternalWithErr(err)
	}
	if len(term.buf) > 0 {
		if err := srv.Send(&ccordiumv1.ListenTerminalResponse{
			Type: &ccordiumv1.ListenTerminalResponse_Stdout_{
				Stdout: &ccordiumv1.ListenTerminalResponse_Stdout{
					Data: term.buf[:],
				},
			},
		}); err != nil {
			zap.L().Error("Could not send initial terminal stdout",
				zap.String("id", term.id), zap.Error(err))
			return grpcutils.InternalWithErr(err)
		}
	}

	return nil
}

func (s *Server) WriteDataTerminal(ctx context.Context, req *ccordiumv1.WriteDataTerminalRequest) (*ccordiumv1.WriteDataTerminalResponse, error) {
	term, ok := s.terminalSrv.get(req.Id)
	if !ok {
		return nil, grpcutils.InvalidArg("Terminal does not exist")
	}

	dataLen := len(req.Data)
	if dataLen == 0 {
		return &ccordiumv1.WriteDataTerminalResponse{}, nil
	}
	if dataLen > 5000 {
		return nil, grpcutils.InvalidArg("Data buffer size is too large")
	}

	term.stdinCh <- req.Data

	return &ccordiumv1.WriteDataTerminalResponse{}, nil
}

func (s *Server) SetWindowSize(ctx context.Context, req *ccordiumv1.SetWindowSizeRequest) (*ccordiumv1.SetWindowSizeResponse, error) {
	term, ok := s.terminalSrv.get(req.Id)
	if !ok {
		return nil, grpcutils.InvalidArg("Terminal does not exist: %s", req.Id)
	}

	if err := term.setWinSize(uint16(req.Cols), uint16(req.Rows)); err != nil {
		return nil, err
	}

	winSize := term.getWinSize()
	msg := &ccordiumv1.ListenTerminalResponse{
		Type: &ccordiumv1.ListenTerminalResponse_WindowSize_{
			WindowSize: &ccordiumv1.ListenTerminalResponse_WindowSize{
				Cols: uint32(winSize.width),
				Rows: uint32(winSize.height),
			},
		},
	}

	term.publishMsg(msg)

	return &ccordiumv1.SetWindowSizeResponse{}, nil
}

func (t *terminal) getWinSize() *winSize {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return &winSize{
		width:  t.winSize.width,
		height: t.winSize.height,
	}
}

type passwdEntry struct {
	Pass  string
	Uid   string
	Gid   string
	Gecos string
	Home  string
	Shell string
}

/*
func getShellFromPasswdFile(usr string) (string, error) {
	fileMap, err := parsePasswdFile()
	if err != nil {
		return "", err
	}
	ret, ok := fileMap[usr]
	if !ok {
		return "", errors.Errorf("Could not find passwd entry for user: %s", usr)
	}
	return ret.Shell, nil
}
*/

func parsePasswdFile() (map[string]passwdEntry, error) {

	parseLine := func(line string) (string, passwdEntry, error) {
		fs := strings.Split(line, ":")
		if len(fs) != 7 {
			return "", passwdEntry{}, errors.Errorf("Invalid number of fields")
		}
		return fs[0], passwdEntry{fs[1], fs[2], fs[3], fs[4], fs[5], fs[6]}, nil
	}

	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}

	defer file.Close()
	lines := bufio.NewReader(file)
	entries := make(map[string]passwdEntry)
	for {
		line, _, err := lines.ReadLine()
		if err != nil {
			break
		}
		name, entry, err := parseLine(string(line))
		if err != nil {
			return nil, err
		}
		entries[name] = entry
	}
	return entries, nil
}

func (s *Server) Exec(srv cordiumv1.WorkspaceService_ExecServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	reqI, err := srv.Recv()
	if err != nil {
		return err
	}

	if reqI.GetRequest() == nil {
		return grpcutils.InvalidArg("Initial message must be a request")
	}

	req := reqI.GetRequest()

	task, err := s.taskManager.newTask(&cordiumv1.Workspace_Spec_Runtime_Task{
		Name:       s.genTerminalID(),
		Run:        req.Command,
		WorkingDir: req.WorkingDir,
		RunAsRoot:  req.RunAsRoot,
		EnvVars: func() []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar {
			var ret []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar

			for _, env := range req.EnvVars {
				ret = append(ret, &cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{
					Key:   env.Key,
					Value: env.Value,
				})
			}

			return ret
		}(),
	})
	if err != nil {
		return err
	}

	task.eventPublisher = nil
	task.hasStdin = req.HasStdin
	task.isExec = true

	listenCh, unsub := task.listenBroker.Subscribe()
	defer unsub()

	if err := task.run(ctx); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-listenCh:
				if !ok {
					continue
				}
				if err := srv.Send(resp); err != nil {
					zap.L().Debug("Could not send stdout", zap.Error(err))
					continue
				}

				switch resp.Type.(type) {
				case *cordiumv1.ExecResponse_Exit_:
					return
				}

			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := srv.Recv()
				if err != nil {
					return
				}

				switch msg.Type.(type) {
				case *cordiumv1.ExecRequest_WriteData_:
					if task.stdinPipe != nil {
						if _, err := task.stdinPipe.Write(msg.GetWriteData().Data); err != nil {
							zap.L().Debug("Could not write data", zap.Error(err))
						}
					}
				case *cordiumv1.ExecRequest_Kill_:
					if err := task.cmd.Process.Kill(); err != nil {
						zap.L().Debug("Could not kill task", zap.Error(err))
					} else {
						zap.L().Debug("Successfully killed task", zap.String("name", task.name))
					}
					return
				}
			}
		}
	}()

	<-ctx.Done()

	return nil
}
