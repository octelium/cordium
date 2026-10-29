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
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
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

func TestFullNameParentDepth(t *testing.T) {
	user := &userctx.UserCtx{User: &corev1.User{Metadata: &metav1.Metadata{Name: "alice"}}}
	for _, tc := range []struct {
		name    string
		parents int
		want    string
	}{
		{name: "workspace.default", parents: 2, want: "workspace.default.alice"},
		{name: "workspace.team", parents: 2, want: "workspace.team.alice"},
		{name: "workspace.team.bob", parents: 2, want: "workspace.team.bob"},
		{name: "secret", parents: 1, want: "secret.alice"},
		{name: "secret.bob", parents: 1, want: "secret.bob"},
	} {
		t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.want, getFullGetOptionsUserCtx(user, tc.name, tc.parents)) })
	}
	assert.Nil(t, getFullGetOptionsSpaceChild(t.Context(), nil))
	assert.Nil(t, getFullDeleteOptionsSpaceChild(t.Context(), nil))
	assert.Nil(t, getFullNamResourceRefSpaceChild(t.Context(), nil))
	assert.Nil(t, getFullGetOptionsSpace(t.Context(), nil))
	assert.Nil(t, getFullResourceRefSpace(t.Context(), nil))
	for _, ref := range []*metav1.ObjectReference{{Uid: "uid-only"}, {Name: "item.team.bob", Uid: "uid"}} {
		before := proto.Clone(ref)
		assert.Same(t, ref, getFullNamResourceRefSpaceChild(t.Context(), ref))
		assert.True(t, proto.Equal(before, ref))
	}
}

func TestParseSpaceResourceNames(t *testing.T) {
	got, err := parseSpaceResource("item.team.alice")
	if assert.Nil(t, err) {
		assert.Equal(t, "item", got.name)
		assert.Equal(t, "team.alice", got.space)
	}
	for _, name := range []string{"", "item", "item.team", "item.team.alice.extra", ".team.alice", "item..alice", "item.team."} {
		t.Run(name, func(t *testing.T) {
			_, err := parseSpaceResource(name)
			assert.NotNil(t, err)
		})
	}
}

func TestDefaultResourceRequiresDefaultName(t *testing.T) {
	srv := &Server{}
	for _, resource := range []*cordiumv1.Space{nil, {}, {Metadata: &metav1.Metadata{}}, {Metadata: &metav1.Metadata{Name: "team.alice"}}} {
		assert.NotNil(t, srv.checkResourceDefault(resource))
	}
	assert.Nil(t, srv.checkResourceDefault(&cordiumv1.Space{Metadata: &metav1.Metadata{Name: "default.alice"}}))
}
