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

package mains

import (
	"context"
	"fmt"
	"testing"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestShortName(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	_, err = NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC,
		adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	name := utilrand.GetRandomStringCanonical(8)

	spc := utilrand.GetRandomStringCanonical(8)

	{
		res := getFullGetOptionsSpaceChild(usr.Ctx(), &metav1.GetOptions{
			Name: name,
		})
		assert.NotNil(t, res)
		assert.Equal(t, res.Name, fmt.Sprintf("%s.default.%s", name, usr.Usr.Metadata.Name))
	}

	{
		res := getFullDeleteOptionsSpaceChild(usr.Ctx(), &metav1.DeleteOptions{
			Name: name,
		})
		assert.NotNil(t, res)
		assert.Equal(t, res.Name, fmt.Sprintf("%s.default.%s", name, usr.Usr.Metadata.Name))
	}

	{
		res := getFullGetOptionsSpaceChild(usr.Ctx(), &metav1.GetOptions{
			Name: fmt.Sprintf("%s.%s", name, spc),
		})
		assert.NotNil(t, res)
		assert.Equal(t, res.Name, fmt.Sprintf("%s.%s.%s", name, spc, usr.Usr.Metadata.Name))
	}

	{
		res := getFullGetOptionsSpaceChild(usr.Ctx(), &metav1.GetOptions{
			Name: fmt.Sprintf("%s.%s.%s", name, spc, usr.Usr.Metadata.Name),
		})
		assert.NotNil(t, res)
		assert.Equal(t, res.Name, fmt.Sprintf("%s.%s.%s", name, spc, usr.Usr.Metadata.Name))
	}

	{

		res := getFullResourceRefSpace(usr.Ctx(), &metav1.ObjectReference{
			Name: "default",
		})
		assert.Equal(t, fmt.Sprintf("default.%s", usr.Usr.Metadata.Name), res.Name)
	}

	{

		res := getFullResourceRefSpace(usr.Ctx(), &metav1.ObjectReference{
			Name: spc,
		})
		assert.Equal(t, fmt.Sprintf("%s.%s", spc, usr.Usr.Metadata.Name), res.Name)
	}

}
