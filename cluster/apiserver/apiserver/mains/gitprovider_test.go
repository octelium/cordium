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
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestGitProvider(t *testing.T) {

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
				Value: utilrand.GetRandomStringCanonical(8),
			},
		},
	})
	assert.Nil(t, err)

	{
		gp, err := srv.CreateGitProvider(usr.Ctx(), &cordiumv1.GitProvider{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.GitProvider_Spec{
				Type: &cordiumv1.GitProvider_Spec_Github_{
					Github: &cordiumv1.GitProvider_Spec_Github{
						ClientID: utilrand.GetRandomStringCanonical(8),
						ClientSecret: &cordiumv1.GitProvider_Spec_Github_ClientSecret{
							Type: &cordiumv1.GitProvider_Spec_Github_ClientSecret_FromSecret{
								FromSecret: sec.Metadata.Name,
							},
						},
					},
				},
			},
			Status: &cordiumv1.GitProvider_Status{},
		})
		assert.Nil(t, err)

		_, err = srv.DeleteGitProvider(usr.Ctx(), &metav1.DeleteOptions{
			Uid: gp.Metadata.Uid,
		})
		assert.Nil(t, err)
	}

	{
		gp, err := srv.CreateGitProvider(usr.Ctx(), &cordiumv1.GitProvider{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.GitProvider_Spec{
				Type: &cordiumv1.GitProvider_Spec_Gitlab_{
					Gitlab: &cordiumv1.GitProvider_Spec_Gitlab{
						ClientID: utilrand.GetRandomStringCanonical(8),
						ClientSecret: &cordiumv1.GitProvider_Spec_Gitlab_ClientSecret{
							Type: &cordiumv1.GitProvider_Spec_Gitlab_ClientSecret_FromSecret{
								FromSecret: sec.Metadata.Name,
							},
						},
					},
				},
			},
			Status: &cordiumv1.GitProvider_Status{},
		})
		assert.Nil(t, err)

		_, err = srv.DeleteGitProvider(usr.Ctx(), &metav1.DeleteOptions{
			Uid: gp.Metadata.Uid,
		})
		assert.Nil(t, err)
	}
}
