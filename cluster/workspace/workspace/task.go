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
	"io"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/octelium/cordium/cluster/common/broker"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type taskManager struct {
	tasks      []*task
	shellPath  string
	mu         sync.Mutex
	isClosed   bool
	userInfo   *userInfo
	doOnCreate bool
	isBuild    bool

	eventPublisher *eventPublisher

	startCancelFn context.CancelFunc
	req           *ccordiumv1.PrepareRequest
	srv           *Server

	runningTasks map[string]*task
}

func (s *Server) newTaskManager() (*taskManager, error) {
	req := s.initReq
	if req == nil {
		return nil, errors.Errorf("Could not initialize a task manager. No initReq")
	}

	ret := &taskManager{
		req:            req,
		shellPath:      s.shellPath,
		userInfo:       s.userInfo,
		doOnCreate:     s.isFreshRun,
		isBuild:        req.Workspace.Status.IsBuild,
		eventPublisher: s.eventPublisher,
		srv:            s,
		runningTasks:   make(map[string]*task),
	}

	zap.L().Debug("Created a new task manager")

	return ret, nil
}

func (t *taskManager) appendTask(tsk *cordiumv1.Workspace_Spec_Runtime_Task) error {
	task, err := t.newTask(tsk)
	if err != nil {
		return err
	}

	t.tasks = append(t.tasks, task)

	return nil
}

func (t *taskManager) run() error {
	if !t.req.Workspace.Status.IsBuild {
		connectTask, err := t.newTaskOcteliumConnect(t.req)
		if err != nil {
			return err
		}
		t.tasks = append(t.tasks, connectTask)
	}

	if ldflags.IsDev() {
		_ = t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "cat /etc/passwd",
		})

		_ = t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "ls -lah",
		})

		_ = t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "whoami",
		})

		_ = t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "users",
		})
	}

	spec := t.srv.spec
	if spec.Runtime != nil {
		for _, task := range spec.Runtime.Tasks {
			if err := t.appendTask(task); err != nil {
				zap.L().Warn("Could not append task", zap.Error(err))
			}
		}
	}

	if t.req.UserConfig != nil && len(t.req.UserConfig.Spec.Tasks) > 0 {
		for _, task := range t.req.UserConfig.Spec.Tasks {
			if err := t.appendTask(task); err != nil {
				zap.L().Warn("Could not append UserConfig task", zap.Error(err))
			}
		}
	}

	if t.req.Space != nil && t.req.Space.Spec.Runtime != nil && len(t.req.Space.Spec.Runtime.Tasks) > 0 {
		for _, task := range t.req.Space.Spec.Runtime.Tasks {
			if err := t.appendTask(task); err != nil {
				zap.L().Warn("Could not append Space task", zap.Error(err))
			}
		}
	}

	timeout := func() time.Duration {
		if t.isBuild || t.doOnCreate {
			return 60 * time.Minute
		}
		return 20 * time.Minute
	}()

	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	t.startCancelFn = cancelFn
	defer cancelFn()

	if t.doOnCreate {
		if err := t.runTasksByType(ctx, cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE); err != nil {
			return err
		}
	}

	if err := t.runTasksByType(ctx, cordiumv1.Workspace_Spec_Runtime_Task_POST_START); err != nil {
		return err
	}

	return nil
}

func (t *taskManager) runTasksByType(ctx context.Context, typ cordiumv1.Workspace_Spec_Runtime_Task_Type) error {
	tasks := t.filterByType(typ)

	zap.L().Debug("Starting running tasks with type", zap.String("type", typ.String()))
	for _, task := range tasks {
		if t.isBuild && task.isBackground {
			zap.L().Debug("Skipping background task since this is a prebuild", zap.String("name", task.name))
			continue
		}
		if err := task.run(ctx); err != nil {
			zap.L().Error("Could not run task",
				zap.String("name", task.name), zap.String("cmd", task.command), zap.Error(err))
			return err
		}
	}

	return nil
}

func (t *taskManager) filterByType(typ cordiumv1.Workspace_Spec_Runtime_Task_Type) []*task {
	var ret []*task

	for _, task := range t.tasks {
		if task.typ == typ {
			ret = append(ret, task)
		}
	}

	return ret
}

func (t *taskManager) addRunningTask(tsk *task) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runningTasks[tsk.tUID] = tsk
}

func (t *taskManager) removeRunningTask(tsk *task) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.runningTasks, tsk.tUID)
}

func (t *taskManager) getTaskByName(name string) (*task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, task := range t.runningTasks {
		if task.name == name {
			return task, nil
		}
	}

	for _, task := range t.tasks {
		if task.name == name {
			return task, nil
		}
	}

	return nil, errors.Errorf("Could not find the task: %s", name)
}

func (t *taskManager) close() error {
	t.mu.Lock()
	if t.isClosed {
		t.mu.Unlock()
		return nil
	}
	t.isClosed = true
	if t.startCancelFn != nil {
		t.startCancelFn()
	}
	t.mu.Unlock()

	t.srv.setState(cordiumv1.Workspace_Status_STOPPING)

	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelFn()

	var retErr error
	if !t.isBuild {
		if err := t.runTasksByType(ctx, cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP); err != nil {
			retErr = err
		}
	} else {
		zap.L().Debug("Skipping PRE_STOP commands for prebuild runs")
	}

	t.closeRunningTasks()

	return retErr
}

func (t *taskManager) closeRunningTasks() {
	t.mu.Lock()
	tasks := make([]*task, 0, len(t.runningTasks))
	for _, task := range t.runningTasks {
		tasks = append(tasks, task)
	}
	t.mu.Unlock()

	for _, task := range tasks {
		if err := task.close(); err != nil {
			zap.L().Warn("Could not close task", zap.String("name", task.name), zap.Error(err))
		}
	}
}

type task struct {
	tUID       string
	name       string
	uid        uint32
	gid        uint32
	command    string
	env        map[string]string
	shellPath  string
	workingDir string

	user    string
	homeDir string

	isBackground bool

	cmd *exec.Cmd
	mu  sync.Mutex

	typ cordiumv1.Workspace_Spec_Runtime_Task_Type

	onFailure cordiumv1.Workspace_Spec_Runtime_Task_OnFailure

	eventPublisher *eventPublisher

	parentEnv []string

	isExec    bool
	isClosed  bool
	hasExited bool
	hasStdin  bool

	listenBroker *broker.Broker[*cordiumv1.ExecResponse]

	stdinPipe io.WriteCloser

	ctx         context.Context
	ctxCancelFn context.CancelFunc
	cmdCancelFn context.CancelFunc

	listenExecResponse []*cordiumv1.ExecResponse
	respMu             sync.Mutex

	taskExecListener      <-chan *cordiumv1.ExecResponse
	taskExecListenerUnsub func()

	taskManager *taskManager
}

func (t *taskManager) newTask(in *cordiumv1.Workspace_Spec_Runtime_Task) (*task, error) {
	ctx, cancel := context.WithCancel(context.Background())

	ret := &task{
		tUID:           vutils.UUIDv4(),
		ctx:            ctx,
		ctxCancelFn:    cancel,
		name:           in.Name,
		command:        in.Run,
		workingDir:     in.WorkingDir,
		isBackground:   in.IsBackground,
		typ:            in.Type,
		shellPath:      t.shellPath,
		uid:            uint32(t.userInfo.uid),
		gid:            uint32(t.userInfo.gid),
		user:           t.userInfo.name,
		homeDir:        t.userInfo.homeDir,
		eventPublisher: t.eventPublisher,
		onFailure:      in.OnFailure,
		parentEnv:      slices.Clone(t.srv.env),
		listenBroker:   broker.NewBroker(ctx, broker.BrokerConfig[*cordiumv1.ExecResponse]{}),
		taskManager:    t,
	}

	ret.taskExecListener, ret.taskExecListenerUnsub = ret.listenBroker.Subscribe()

	if in.RunAsRoot {
		ret.uid = 0
		ret.gid = 0
		ret.user = "root"
		ret.homeDir = "/root"
	}

	if len(in.EnvVars) > 0 {
		ret.env = make(map[string]string)
		for _, env := range in.EnvVars {
			ret.env[env.Key] = env.Value
		}
	}

	return ret, nil
}

func (t *task) isUserRoot() bool {
	return t.uid == 0 && t.gid == 0
}

func (t *task) run(parentCtx context.Context) error {
	var err error

	go t.captureExecResponses()

	zap.L().Debug("Starting running task",
		zap.String("name", t.name),
		zap.String("cmd", t.command),
		zap.String("shellPath", t.shellPath))

	baseCtx := parentCtx
	if t.isBackground || t.isExec {
		baseCtx = context.Background()
	}

	cmdCtx, cmdCancelFn := context.WithCancel(baseCtx)
	t.cmdCancelFn = cmdCancelFn

	go func() {
		select {
		case <-t.ctx.Done():
			cmdCancelFn()
		case <-cmdCtx.Done():
		}
	}()

	t.cmd = exec.CommandContext(cmdCtx, t.shellPath, "-c", t.command)

	t.stdinPipe, err = t.cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := t.startStdoutLoop(); err != nil {
		_ = t.close()
		return err
	}
	if err := t.startStderrLoop(); err != nil {
		_ = t.close()
		return err
	}

	if t.workingDir != "" {
		t.cmd.Dir = t.workingDir
	}

	if !ldflags.IsTest() {
		t.cmd.SysProcAttr = t.sysProcAttr()
	}

	t.cmd.Env = t.getEnv()

	if err := t.cmd.Start(); err != nil {
		zap.L().Warn("Could not start task cmd", zap.String("name", t.name), zap.Error(err))
		_ = t.close()

		if t.shouldAbortOnFailure() {
			t.publishFailure(err)
			return errors.Errorf("Running task: %s failed: %+v", t.name, err)
		}

		return nil
	}

	t.taskManager.addRunningTask(t)

	if t.isBackground || t.isExec {
		go func() {
			if err := t.wait(); err != nil {
				zap.L().Warn("Background task exited with error",
					zap.String("name", t.name),
					zap.Error(err))

				if t.shouldAbortOnFailure() {
					t.publishFailure(err)
				}
			}
		}()

		return nil
	}

	err = t.wait()
	if err != nil {
		zap.L().Warn("Task exited with error",
			zap.String("name", t.name),
			zap.Error(err))

		if t.shouldAbortOnFailure() {
			t.publishFailure(err)
			return errors.Errorf("Running task: %s failed: %+v", t.name, err)
		}
	}

	return nil
}

func (t *task) sysProcAttr() *syscall.SysProcAttr {
	ret := &syscall.SysProcAttr{
		Setpgid: true,
	}

	if !t.isUserRoot() {
		ret.Credential = &syscall.Credential{
			Uid: t.uid,
			Gid: t.gid,
		}
	}

	return ret
}

func (t *task) shouldAbortOnFailure() bool {
	return t.onFailure == cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT
}

func (t *task) captureExecResponses() {
	for msg := range t.taskExecListener {
		if msg == nil {
			return
		}

		t.respMu.Lock()
		t.listenExecResponse = append(t.listenExecResponse, msg)
		t.respMu.Unlock()
	}
}

func (t *task) wait() error {
	err := t.cmd.Wait()

	exitCode := t.exitCodeFromError(err)

	t.listenBroker.Publish(&cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Exit_{
			Exit: &cordiumv1.ExecResponse_Exit{
				Code: int32(exitCode),
			},
		},
	})

	t.mu.Lock()
	t.hasExited = true
	t.mu.Unlock()

	_ = t.close()

	zap.L().Debug("Task wait done", zap.Int("exitCode", exitCode))

	return err
}

func (t *task) exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	exitError, ok := err.(*exec.ExitError)
	if !ok {
		return -1
	}

	waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
	if !ok {
		return exitError.ExitCode()
	}

	switch {
	case waitStatus.Exited():
		return waitStatus.ExitStatus()
	case waitStatus.Signaled():
		return -1
	default:
		return -1
	}
}

func (t *task) close() error {
	t.mu.Lock()
	if t.isClosed {
		t.mu.Unlock()
		return nil
	}
	t.isClosed = true

	stdinPipe := t.stdinPipe
	ctxCancelFn := t.ctxCancelFn
	cmdCancelFn := t.cmdCancelFn
	cmd := t.cmd
	hasExited := t.hasExited
	t.mu.Unlock()

	if stdinPipe != nil {
		_ = stdinPipe.Close()
	}

	if cmdCancelFn != nil {
		cmdCancelFn()
	}

	if ctxCancelFn != nil {
		ctxCancelFn()
	}

	if cmd != nil && cmd.Process != nil && !hasExited {
		t.terminateProcessGroup(cmd.Process.Pid)
	}

	if t.taskExecListenerUnsub != nil {
		t.taskExecListenerUnsub()
	}

	t.taskManager.removeRunningTask(t)

	zap.L().Debug("task closed", zap.String("name", t.name))

	return nil
}

func (t *task) terminateProcessGroup(pid int) {
	if pid <= 0 {
		return
	}

	_ = syscall.Kill(-pid, syscall.SIGTERM)

	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		<-timer.C

		t.mu.Lock()
		hasExited := t.hasExited
		t.mu.Unlock()

		if !hasExited {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	}()
}

func (t *task) publishFailure(err error) {
	failure := &cordiumv1.Workspace_Status_Failure{
		Type: &cordiumv1.Workspace_Status_Failure_Task_{
			Task: &cordiumv1.Workspace_Status_Failure_Task{
				Name: t.name,
				ExitCode: func() int32 {
					if exiterr, ok := err.(*exec.ExitError); ok {
						return int32(exiterr.ExitCode())
					}
					return 0
				}(),
			},
		},
	}

	if t.eventPublisher != nil {
		t.eventPublisher.publish(&ccordiumv1.ListenEventResponse{
			Type: &ccordiumv1.ListenEventResponse_Failure{
				Failure: failure,
			},
		})
	}
}

func (t *task) getEnv() []string {
	ret := slices.Clone(t.parentEnv)

	for k, v := range t.env {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}

	setEnv(&ret, "HOME", t.homeDir)

	return ret
}

func (t *taskManager) newTaskOcteliumConnect(req *ccordiumv1.PrepareRequest) (*task, error) {
	args := []string{"octelium", "connect"}

	if t.srv.spec != nil && t.srv.spec.Runtime != nil && t.srv.spec.Runtime.Octelium != nil {
		if t.srv.spec.Runtime.Octelium.ServeAll {
			args = append(args, "--serve-all")
		}

		for _, svc := range t.srv.spec.Runtime.Octelium.ServeServices {
			if err := apivalidation.ValidateName(svc, 0, 1); err != nil {
				return nil, err
			}
			args = append(args, "--serve", svc)
		}
	}

	return t.newTask(&cordiumv1.Workspace_Spec_Runtime_Task{
		Name:         "octelium-connect",
		Run:          shellJoin(args),
		IsBackground: true,
		RunAsRoot:    true,
		Type:         cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
		EnvVars: func() []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar {
			var ret []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar

			envMap := map[string]string{
				"OCTELIUM_DOMAIN":            req.Domain,
				"OCTELIUM_HOME":              "mem",
				"OCTELIUM_USER_HOME":         t.userInfo.homeDir,
				"OCTELIUM_AUTH_PROXY_SOCKET": "/var/run/octelium-proxy.sock",
				"OCTELIUM_ESSH":              "true",
				"OCTELIUM_ESSH_USER":         t.userInfo.name,
				"OCTELIUM_ESSH_IP_ADDRS":     "0.0.0.0",
				"OCTELIUM_ESSH_PORT":         "2022",
				"OCTELIUM_LOCAL_DNS_SERVER":  "true",
				"OCTELIUM_ESSH_SFTP_USER":    "true",
			}

			for k, v := range envMap {
				ret = append(ret, &cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{
					Key:   k,
					Value: v,
				})
			}

			return ret
		}(),
	})
}

func shellJoin(args []string) string {
	var ret []string
	for _, arg := range args {
		ret = append(ret, shellQuote(arg))
	}
	return strings.Join(ret, " ")
}

func shellQuote(in string) string {
	if in == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(in, "'", "'\"'\"'") + "'"
}

func (t *task) startStdoutLoop() error {
	if t.cmd == nil {
		return errors.Errorf("Could not start stdout loop. Nil cmd")
	}

	stdoutPipe, err := t.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	go t.readOutputLoop(stdoutPipe, cordiumv1.ListenLogResponse_MODE_STDOUT)

	return nil
}

func (t *task) startStderrLoop() error {
	if t.cmd == nil {
		return errors.Errorf("Could not start stderr loop. Nil cmd")
	}

	stderrPipe, err := t.cmd.StderrPipe()
	if err != nil {
		return err
	}

	go t.readOutputLoop(stderrPipe, cordiumv1.ListenLogResponse_MODE_STDERR)

	return nil
}

func (t *task) readOutputLoop(r io.Reader, mode cordiumv1.ListenLogResponse_Mode) {
	zap.L().Debug("Starting task output loop",
		zap.String("name", t.name),
		zap.String("uid", t.tUID),
		zap.String("mode", mode.String()))

	buf := make([]byte, 32*1024)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			data := cloneBytes(buf[:n])
			if mode == cordiumv1.ListenLogResponse_MODE_STDOUT {
				t.publishStdout(data)
			} else {
				t.publishStderr(data)
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				zap.L().Warn("Task output loop ended with error",
					zap.String("name", t.name),
					zap.String("uid", t.tUID),
					zap.String("mode", mode.String()),
					zap.Error(err))
			}
			break
		}
	}

	zap.L().Debug("Task output loop ended",
		zap.String("name", t.name),
		zap.String("uid", t.tUID),
		zap.String("mode", mode.String()))
}

func (t *task) publishStdout(buf []byte) {
	data := cloneBytes(buf)

	t.listenBroker.Publish(&cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Stdout_{
			Stdout: &cordiumv1.ExecResponse_Stdout{
				Data: data,
			},
		},
	})

	if t.eventPublisher != nil {
		if ldflags.IsDev() {
			zap.L().Debug("Task stdout", zap.String("data", string(buf)), zap.String("task", t.name))
		}

		t.eventPublisher.publish(&ccordiumv1.ListenEventResponse{
			Type: &ccordiumv1.ListenEventResponse_ListenLogResponse{
				ListenLogResponse: &cordiumv1.ListenLogResponse{
					CreatedAt: pbutils.Now(),
					Type:      cordiumv1.ListenLogResponse_TYPE_TASK,
					Mode:      cordiumv1.ListenLogResponse_MODE_STDOUT,
					Data:      cloneBytes(buf),
				},
			},
		})
	}
}

func (t *task) publishStderr(buf []byte) {
	data := cloneBytes(buf)

	t.listenBroker.Publish(&cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Stderr_{
			Stderr: &cordiumv1.ExecResponse_Stderr{
				Data: data,
			},
		},
	})

	if t.eventPublisher != nil {
		if ldflags.IsDev() {
			zap.L().Debug("Task stderr", zap.String("data", string(buf)), zap.String("task", t.name))
		}

		t.eventPublisher.publish(&ccordiumv1.ListenEventResponse{
			Type: &ccordiumv1.ListenEventResponse_ListenLogResponse{
				ListenLogResponse: &cordiumv1.ListenLogResponse{
					CreatedAt: pbutils.Now(),
					Type:      cordiumv1.ListenLogResponse_TYPE_TASK,
					Mode:      cordiumv1.ListenLogResponse_MODE_STDERR,
					Data:      cloneBytes(buf),
				},
			},
		})
	}
}

func cloneBytes(in []byte) []byte {
	if in == nil {
		return nil
	}

	ret := make([]byte, len(in))
	copy(ret, in)
	return ret
}
