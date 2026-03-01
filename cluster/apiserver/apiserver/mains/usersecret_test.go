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
		assert.NotNil(t, err)

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

		assert.Equal(t, sec.Data.GetValue(), req.Data.GetValue())

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
