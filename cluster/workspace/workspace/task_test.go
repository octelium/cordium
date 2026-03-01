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

	{
		tm.tasks = append(tm.tasks, &task{
			name:    "ls",
			command: "ls",
			user:    tm.userInfo.name,
			homeDir: tm.userInfo.homeDir,

			typ:            cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			shellPath:      tm.shellPath,
			eventPublisher: tm.eventPublisher,
		})
	}

	/*
		{
			tm.tasks = append(tm.tasks, &task{
				name:    "ls",
				command: "ls",
				user:    tm.userInfo.name,
				homeDir: tm.userInfo.homeDir,

				typ:            cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP,
				shellPath:      tm.shellPath,
				eventPublisher: tm.eventPublisher,
			})
		}
	*/
	{
		tm.tasks = append(tm.tasks, &task{
			name: "ls",
			command: `
ls
uname -a
			`,
			user:    tm.userInfo.name,
			homeDir: tm.userInfo.homeDir,

			typ:            cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			shellPath:      tm.shellPath,
			eventPublisher: tm.eventPublisher,
			onFailure:      cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
		})
	}
	err = tm.run()
	assert.Nil(t, err)

	err = tm.close()
	assert.Nil(t, err)

	{
		tm.tasks = append(tm.tasks, &task{
			name: "bash",
			command: `
#!/bin/bash
if false; then
	echo "True"
else
	echo "False"
fi
			`,
			user:    tm.userInfo.name,
			homeDir: tm.userInfo.homeDir,

			typ:            cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			shellPath:      tm.shellPath,
			eventPublisher: tm.eventPublisher,
			onFailure:      cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
		})
		err = tm.run()
		assert.Nil(t, err)

	}

	{
		tm.tasks = append(tm.tasks, &task{
			name: "ls",
			command: `
ls -la
command-that-does-not-exist
			`,
			user:    tm.userInfo.name,
			homeDir: tm.userInfo.homeDir,

			typ:            cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			shellPath:      tm.shellPath,
			eventPublisher: tm.eventPublisher,
			onFailure:      cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT,
		})
		err = tm.run()
		assert.NotNil(t, err)

	}

}
