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
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"slices"

	"github.com/octelium/cordium/cluster/common/broker"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
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

		t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "cat /etc/passwd",
		})

		t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "ls -lah",
		})

		t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			Run:  "whoami",
		})

		t.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
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

func (t *taskManager) getTaskByName(name string) (*task, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
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
	t.mu.Unlock()
	t.srv.setState(cordiumv1.Workspace_Status_STOPPING)

	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelFn()

	if !t.isBuild {
		if err := t.runTasksByType(ctx, cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP); err != nil {
			return err
		}
		return nil
	} else {
		zap.L().Debug("Skipping PRE_STOP commands for prebuild runs")
	}

	return nil
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

	isExec   bool
	isClosed bool
	hasStdin bool

	listenBroker *broker.Broker[*cordiumv1.ExecResponse]

	stdinPipe io.WriteCloser

	ctx                context.Context
	ctxCancelFn        context.CancelFunc
	listenExecResponse []*cordiumv1.ExecResponse

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

func (t *task) run(ctx context.Context) error {
	var err error

	go func() {
		for msg := range t.taskExecListener {
			if msg == nil {
				return
			}

			t.listenExecResponse = append(t.listenExecResponse, msg)
		}
	}()

	zap.L().Debug("Starting running task",
		zap.String("name", t.name),
		zap.String("cmd", t.command),
		zap.String("shellPath", t.shellPath))

	if t.isBackground {
		ctx = context.Background()
	}

	t.cmd = exec.CommandContext(ctx, t.shellPath, "-c", t.command)

	t.stdinPipe, err = t.cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := t.startStdoutLoop(); err != nil {
		return err
	}
	if err := t.startStderrLoop(); err != nil {
		return err
	}

	t.cmd.Dir = t.workingDir

	if !ldflags.IsTest() {
		if !t.isUserRoot() {
			t.cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{Uid: t.uid, Gid: t.gid},
			}
		}
	}

	t.cmd.Env = t.getEnv()

	if err := t.cmd.Start(); err != nil {
		zap.L().Warn("Could not start task cmd", zap.String("name", t.name), zap.Error(err))

		if t.onFailure == cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT {
			t.publishFailure(err)
			return errors.Errorf("Running task: %s failed: %+v", t.name, err)
		}
		return err
	}

	t.taskManager.mu.Lock()
	t.taskManager.tasks = append(t.taskManager.tasks, t)
	t.taskManager.mu.Unlock()

	if t.isBackground || t.isExec {
		go t.wait()
	} else {

		t.wait()
		/*
			err := t.cmd.Run()
			if err != nil {
				zap.L().Warn("Could not run task cmd", zap.String("name", t.name), zap.Error(err))
				if t.onFailure == cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT {
					t.publishFailure(err)
					return errors.Errorf("Running task: %s failed: %+v", t.name, err)
				}
			}

		*/

		// t.publishTaskDone(err)

	}

	return nil
}

func (t *task) wait() error {
	err := t.cmd.Wait()

	var exitCode int
	if err != nil {

		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus := exitError.Sys().(syscall.WaitStatus)

			switch {
			case waitStatus.Exited():
				exitCode = waitStatus.ExitStatus()
			case waitStatus.Signaled():
				exitCode = -1
			default:
				exitCode = -1
			}
		}
	}

	t.listenBroker.Publish(&cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Exit_{
			Exit: &cordiumv1.ExecResponse_Exit{
				Code: int32(exitCode),
			},
		},
	})

	// t.publishTaskDone(err)

	t.close()

	zap.L().Debug("Task wait done", zap.Int("exitCode", exitCode))

	return err
}

func (t *task) close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.isClosed {
		return nil
	}
	t.isClosed = true

	if t.stdinPipe != nil {
		t.stdinPipe.Close()
	}

	if t.ctxCancelFn != nil {
		t.ctxCancelFn()
	}

	if t.taskExecListenerUnsub != nil {
		t.taskExecListenerUnsub()
	}

	t.taskManager.mu.Lock()
	t.taskManager.tasks = slices.DeleteFunc(t.taskManager.tasks, func(tsk *task) bool {
		return tsk.tUID == t.tUID
	})
	t.taskManager.mu.Unlock()

	zap.L().Debug("task closed", zap.String("name", t.name))

	return nil
}

/*
func (t *task) publishTaskDone(err error) {
	var exitCode int
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			exitCode = exiterr.ExitCode()
		}
	}


		t.eventPublisher.publish(&cordiumv1.ListenEventResponse{
			Type: &cordiumv1.ListenEventResponse_TaskDone_{
				TaskDone: &cordiumv1.ListenEventResponse_TaskDone{
					Uid:      t.tUID,
					ExitCode: int64(exitCode),
				},
			},
		})

}
*/

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
	ret := t.parentEnv

	for k, v := range t.env {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}

	setEnv(&ret, "HOME", t.homeDir)
	// setEnv(&ret, "CONTAINER_HOST", "unix:///var/run/docker.sock")

	return ret
}

func (t *taskManager) newTaskOcteliumConnect(req *ccordiumv1.PrepareRequest) (*task, error) {

	cmd := "octelium connect"

	if t.srv.spec != nil && t.srv.spec.Runtime != nil && t.srv.spec.Runtime.Octelium != nil {
		if t.srv.spec.Runtime.Octelium.ServeAll {
			cmd = fmt.Sprintf("%s --serve-all", cmd)
		}

		for _, svc := range t.srv.spec.Runtime.Octelium.ServeServices {
			cmd = fmt.Sprintf("%s --serve %s", cmd, svc)
		}
	}

	return t.newTask(&cordiumv1.Workspace_Spec_Runtime_Task{
		Name:         "octelium-connect",
		Run:          cmd,
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

				"OCTELIUM_ESSH_SFTP_USER": "true",
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

func (t *task) startStdoutLoop() error {
	if t.cmd == nil {
		return errors.Errorf("Could not start stdout loop. Nil cmd")
	}

	stdoutPipe, err := t.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	go func() {
		zap.L().Debug("Starting task stdoutLoop", zap.String("name", t.name), zap.String("uid", t.tUID))
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			t.publishStdout(scanner.Bytes())
		}
		zap.L().Debug("Task stdoutLoop ended", zap.String("name", t.name), zap.String("uid", t.tUID))
	}()

	return nil
}

func (t *task) publishStdout(buf []byte) {

	t.listenBroker.Publish(&cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Stdout_{
			Stdout: &cordiumv1.ExecResponse_Stdout{
				Data: buf,
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
					Data:      buf,
				},
			},
		})
	}
}

func (t *task) startStderrLoop() error {
	if t.cmd == nil {
		return errors.Errorf("Could not start stderr loop. Nil cmd")
	}

	stderrPipe, err := t.cmd.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		zap.L().Debug("Starting task stderrLoop", zap.String("name", t.name), zap.String("uid", t.tUID))
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			t.publishStderr(scanner.Bytes())
		}
		zap.L().Debug("Task stderrLoop ended", zap.String("name", t.name), zap.String("uid", t.tUID))
	}()

	return nil
}

func (t *task) publishStderr(buf []byte) {

	t.listenBroker.Publish(&cordiumv1.ExecResponse{
		Type: &cordiumv1.ExecResponse_Stderr_{
			Stderr: &cordiumv1.ExecResponse_Stderr{
				Data: buf,
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
					Data:      buf,
				},
			},
		})
	}

}
