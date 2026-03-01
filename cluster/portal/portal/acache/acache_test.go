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

package acache

import (
	"testing"

	"github.com/google/uuid"
	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	acache, err := NewCache()
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  uuid.New().String(),
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			UserRef: &metav1.ObjectReference{
				Name: utilrand.GetRandomStringCanonical(8),
				Uid:  uuid.New().String(),
			},
		},
	}

	err = acache.SetWorkspace(ws)
	assert.Nil(t, err)

	res, err := acache.GetWorkspace(ws.Status.UserRef.Uid, ws.Metadata.Name)
	assert.Nil(t, err)
	assert.Equal(t, res.Metadata.Uid, ws.Metadata.Uid)
}
