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
