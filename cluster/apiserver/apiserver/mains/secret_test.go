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
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestSecret(t *testing.T) {

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

		org, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		sec, err := srv.CreateSecret(usr.Ctx(), &cordiumv1.Secret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec:   &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{
					Value: utilrand.GetRandomString(200),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)

		{
			_, err := srv.CreateSecret(usr.Ctx(), &cordiumv1.Secret{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("%s.%s", sec.Metadata.Name, org.Metadata.Name),
				},
				Spec:   &cordiumv1.Secret_Spec{},
				Status: &cordiumv1.Secret_Status{},
				Data: &cordiumv1.Secret_Data{
					Type: &cordiumv1.Secret_Data_Value{
						Value: utilrand.GetRandomString(200),
					},
				},
			})
			assert.NotNil(t, err, "%+v", err)
			assert.True(t, grpcerr.IsInvalidArg(err))
		}

		{
			secList, err := srv.ListSecret(usr.Ctx(), &cordiumv1.ListSecretOptions{
				SpaceRef: umetav1.GetObjectReference(org),
			})
			assert.Nil(t, err, "%+v", err)

			assert.Equal(t, 1, len(secList.Items))
			assert.Equal(t, sec.Metadata.Uid, secList.Items[0].Metadata.Uid)
			assert.Nil(t, secList.Items[0].Data)
		}

		_, err = srv.DeleteSecret(usr.Ctx(), &metav1.DeleteOptions{Uid: sec.Metadata.Uid})
		assert.Nil(t, err)
	}
}

func TestValidateSecret(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	ctx := context.Background()

	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	org, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
		},
		Spec: &cordiumv1.Space_Spec{},
		Status: &cordiumv1.Space_Status{
			Type: cordiumv1.Space_Status_ORGANIZATION,
		},
	})
	assert.Nil(t, err)

	valids := []*cordiumv1.Secret{
		{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec:   &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{
					Value: utilrand.GetRandomString(1000),
				},
			},
		},
		{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec:   &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_ValueBytes{
					ValueBytes: utilrand.GetRandomBytesMust(300),
				},
			},
		},
	}

	for _, valid := range valids {
		err := srv.checkSecretData(ctx, valid)
		assert.Nil(t, err)
	}

	invalids := []*cordiumv1.Secret{

		/*
			{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Status: &cordiumv1.Secret_Status{},
				Data: &cordiumv1.Secret_Data{
					Type: &cordiumv1.Secret_Data_ValueBytes{
						ValueBytes: utilrand.GetRandomBytesMust(300),
					},
				},
			},

			{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec:   &cordiumv1.Secret_Spec{},
				Status: &cordiumv1.Secret_Status{},
				Data: &cordiumv1.Secret_Data{
					Type: &cordiumv1.Secret_Data_ValueBytes{
						ValueBytes: utilrand.GetRandomBytesMust(300),
					},
				},
			},

			{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec:   &cordiumv1.Secret_Spec{},
				Status: &cordiumv1.Secret_Status{},
				Data: &cordiumv1.Secret_Data{
					Type: &cordiumv1.Secret_Data_ValueBytes{
						ValueBytes: utilrand.GetRandomBytesMust(300),
					},
				},
			},
		*/

		{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		},

		{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{},
		},
		{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_ValueBytes{},
			},
		},
		{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{},
			},
		},

		{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{
					Value: utilrand.GetRandomString(1600),
				},
			},
		},
		{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_ValueBytes{
					ValueBytes: utilrand.GetRandomBytesMust(160000),
				},
			},
		},
	}

	for _, invalid := range invalids {
		err := srv.checkSecretData(ctx, invalid)
		assert.NotNil(t, err)
	}
}
