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

package wsutils

import (
	"fmt"
	"testing"
	"time"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestActivityCtl(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s-%s", usr.Usr.Metadata.Name, utilrand.GetRandomStringCanonical(6)),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			UserRef: umetav1.GetObjectReference(usr.Usr),
			State:   cordiumv1.Workspace_Status_RUNNING,
		},
	}

	ws, err = fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, ws)
	assert.Nil(t, err)

	ctl, err := NewActivityCtl(fakeC.OcteliumC)
	assert.Nil(t, err)

	now := time.Now()

	ctl.activityMap[ws.Metadata.Uid] = now

	err = ctl.doCheck(ctx)
	assert.Nil(t, err)

	ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
	assert.Nil(t, err)

	assert.True(t, pbutils.IsEqual(ws.Status.LastActivityAt, pbutils.Timestamp(now)))
	assert.Equal(t, 0, len(ctl.activityMap))
}
