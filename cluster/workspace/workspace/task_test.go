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
	"os/user"
	"strconv"
	"testing"

	"context"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestTask(t *testing.T) {

	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	srv, err := NewServer(ctx)
	assert.Nil(t, err)

	err = srv.setShell()
	assert.Nil(t, err)

	defer srv.Close()

	curUsr, err := user.Current()
	assert.Nil(t, err)

	uid, _ := strconv.Atoi(curUsr.Uid)
	gid, _ := strconv.Atoi(curUsr.Gid)

	grp, _ := user.LookupGroupId(curUsr.Gid)

	srv.userInfo = &userInfo{
		name:    curUsr.Name,
		uid:     uid,
		gid:     gid,
		homeDir: curUsr.HomeDir,
		group:   grp.Name,
	}

	srv.initReq = &ccordiumv1.PrepareRequest{
		Workspace: &cordiumv1.Workspace{
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		},
		Space: &cordiumv1.Space{
			Spec:   &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{},
		},
	}

	srv.spec, err = wsutils.MergeSpec(&wsutils.MergeSpecReq{
		Workspace: srv.initReq.Workspace,
		Template:  srv.initReq.Template,
	})
	assert.Nil(t, err)

	tm, err := srv.newTaskManager()
	assert.Nil(t, err)

	tm.tasks = nil

	zap.L().Debug("tm shellPath", zap.String("val", tm.shellPath))

	task, err := tm.newTask(&cordiumv1.Workspace_Spec_Runtime_Task{
		Name: "ls",
		Run: `
ls
uname -a`,
		Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
	})
	assert.Nil(t, err)
	tm.tasks = append(tm.tasks, task)

	err = tm.run()
	assert.Nil(t, err)

	err = tm.close()
	assert.Nil(t, err)

	{
		task, err := tm.newTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Name: "bash",
			Run: `
#!/bin/bash
if false; then
	echo "True"
else
	echo "False"
fi`,
			Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
		})
		assert.Nil(t, err)
		tm.tasks = append(tm.tasks, task)

		err = tm.run()
		assert.Nil(t, err)

	}

	{
		task, err := tm.newTask(&cordiumv1.Workspace_Spec_Runtime_Task{
			Name: "ls2",
			Run: `
ls -la
command-that-does-not-exist`,
			Type:      cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			OnFailure: cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
		})
		assert.Nil(t, err)
		tm.tasks = append(tm.tasks, task)

		err = task.run(ctx)
		assert.NotNil(t, err)

	}

}
