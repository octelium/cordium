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

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/common/watchers"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestWatcherCordium(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	watcher := watchers.NewCordiumV1(fakeC.OcteliumC)

	req := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(6),
		},
		Spec:   &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{},
	}

	didCreate := false
	didUpdate := false
	didDelete := false

	err = watcher.Workspace(ctx, nil, func(ctx context.Context, item *cordiumv1.Workspace) error {

		if item.Metadata.Name == req.Metadata.Name {
			didCreate = true
		}
		return nil
	},
		func(ctx context.Context, new, old *cordiumv1.Workspace) error {
			if new.Metadata.Name == req.Metadata.Name {
				assert.Equal(t, new.Metadata.Uid, old.Metadata.Uid)
				didUpdate = true
			}

			return nil
		},
		func(ctx context.Context, item *cordiumv1.Workspace) error {
			if item.Metadata.Name == req.Metadata.Name {
				assert.Equal(t, item.Metadata.Name, req.Metadata.Name)
				didDelete = true
			}

			return nil
		})
	assert.Nil(t, err)

	itm, err := fakeC.OcteliumC.CordiumC().CreateWorkspace(ctx, req)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	itm.Spec.Image = &cordiumv1.Workspace_Spec_Image{
		Type: &cordiumv1.Workspace_Spec_Image_Registry_{
			Registry: &cordiumv1.Workspace_Spec_Image_Registry{
				Url: utilrand.GetRandomStringCanonical(8),
			},
		},
	}
	itm, err = fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, itm)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)
	_, err = fakeC.OcteliumC.CordiumC().DeleteWorkspace(ctx, &rmetav1.DeleteOptions{Uid: itm.Metadata.Uid})
	assert.Nil(t, err)
	time.Sleep(3 * time.Second)

	assert.True(t, didCreate && didUpdate && didDelete)
	cancel()
}
