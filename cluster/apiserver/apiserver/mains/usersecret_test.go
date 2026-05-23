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
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestUserSecret(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		req := &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), usr.Usr.Metadata.Name),
			},
			Spec:   &cordiumv1.UserSecret_Spec{},
			Status: &cordiumv1.UserSecret_Status{},
		}
		_, err = srv.CreateUserSecret(usr.Ctx(), req)
		assert.NotNil(t, err)

		req = &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), usr.Usr.Metadata.Name),
			},
			Spec:   &cordiumv1.UserSecret_Spec{},
			Status: &cordiumv1.UserSecret_Status{},
			Data:   &cordiumv1.UserSecret_Data{},
		}
		_, err = srv.CreateUserSecret(usr.Ctx(), req)
		assert.NotNil(t, err)

		req = &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), utilrand.GetRandomStringCanonical(8)),
			},
			Spec:   &cordiumv1.UserSecret_Spec{},
			Status: &cordiumv1.UserSecret_Status{},
			Data: &cordiumv1.UserSecret_Data{
				Type: &cordiumv1.UserSecret_Data_Value{
					Value: utilrand.GetRandomString(20),
				},
			},
		}
		_, err = srv.CreateUserSecret(usr.Ctx(), req)
		assert.NotNil(t, err)

		req = &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec:   &cordiumv1.UserSecret_Spec{},
			Status: &cordiumv1.UserSecret_Status{},
			Data: &cordiumv1.UserSecret_Data{
				Type: &cordiumv1.UserSecret_Data_Value{
					Value: utilrand.GetRandomString(20),
				},
			},
		}
		_, err = srv.CreateUserSecret(usr.Ctx(), req)
		assert.Nil(t, err, "%+v", err)
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		req := &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), usr.Usr.Metadata.Name),
			},
			Spec:   &cordiumv1.UserSecret_Spec{},
			Status: &cordiumv1.UserSecret_Status{},
			Data: &cordiumv1.UserSecret_Data{
				Type: &cordiumv1.UserSecret_Data_Value{
					Value: utilrand.GetRandomString(20),
				},
			},
		}
		sec, err := srv.CreateUserSecret(usr.Ctx(), req)
		assert.Nil(t, err)

		assert.Equal(t, "", ucordiumv1.ToUserSecret(sec).GetValueStr())

		_, err = srv.GetUserSecret(usr2.Ctx(), &metav1.GetOptions{
			Name: sec.Metadata.Name,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		_, err = srv.DeleteUserSecret(usr2.Ctx(), &metav1.DeleteOptions{
			Name: sec.Metadata.Name,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err))

		secN, err := srv.GetUserSecret(usr.Ctx(), &metav1.GetOptions{
			Name: sec.Metadata.Name,
		})
		assert.Nil(t, err)
		assert.Nil(t, secN.Data)

		_, err = srv.DeleteUserSecret(usr.Ctx(), &metav1.DeleteOptions{
			Name: sec.Metadata.Name,
		})
		assert.Nil(t, err)

		_, err = srv.DeleteUserSecret(usr.Ctx(), &metav1.DeleteOptions{
			Name: sec.Metadata.Name,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsNotFound(err))
	}
}
