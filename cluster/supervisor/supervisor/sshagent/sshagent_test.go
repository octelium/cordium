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

package sshagent

import (
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	utils_cert "github.com/octelium/octelium/pkg/utils/cert"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestSSHAgent(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	secretList := &cordiumv1.UserSecretList{}

	for i := 0; i < 10; i++ {

		k, err := utils_cert.GenerateECDSA()
		assert.Nil(t, err)

		kPrivatePEM, err := k.GetPrivateKeyPEM()
		assert.Nil(t, err)

		secretList.Items = append(secretList.Items, &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name: "ssh-key-main.user",
			},
			Spec: &cordiumv1.UserSecret_Spec{
				Type: cordiumv1.UserSecret_Spec_SSH_KEY,
			},
			Status: &cordiumv1.UserSecret_Status{},
			Data: &cordiumv1.UserSecret_Data{
				Type: &cordiumv1.UserSecret_Data_ValueBytes{
					ValueBytes: []byte(kPrivatePEM),
				},
			},
		})
	}

	srv, err := NewAgent(&Opts{
		UserSecretList: secretList,
	})
	assert.Nil(t, err)

	k, err := utils_cert.LoadECDSA(secretList.Items[0].Data.GetValueBytes())
	assert.Nil(t, err)
	signer, err := ssh.NewSignerFromKey(k.PrivateKey)
	assert.Nil(t, err)

	{
		_, err = srv.keyring.Sign(signer.PublicKey(), utilrand.GetRandomBytesMust(32))
		assert.Nil(t, err)
	}

	{
		keys, err := srv.keyring.List()
		assert.Nil(t, err)

		assert.Equal(t, len(secretList.Items), len(keys))

		for i := 0; i < len(keys); i++ {
			k, err := utils_cert.LoadECDSA(secretList.Items[i].Data.GetValueBytes())
			assert.Nil(t, err)
			signer, err := ssh.NewSignerFromKey(k.PrivateKey)
			assert.Nil(t, err)

			assert.Equal(t, signer.PublicKey().Marshal(), keys[i].Blob)
		}
	}

	{
		err = srv.keyring.Lock(utilrand.GetRandomBytesMust(32))
		assert.Equal(t, errUnsupported, err)
	}

	{
		err = srv.keyring.Unlock(utilrand.GetRandomBytesMust(32))
		assert.Equal(t, errUnsupported, err)
	}

	{
		err = srv.keyring.RemoveAll()
		assert.Equal(t, errUnsupported, err)
	}

	{
		err = srv.keyring.Remove(signer.PublicKey())
		assert.Equal(t, errUnsupported, err)
	}

	{

		k, err := utils_cert.GenerateECDSA()
		assert.Nil(t, err)

		err = srv.keyring.Add(agent.AddedKey{
			PrivateKey: k.PrivateKey,
			Comment:    "example",
		})
		assert.Equal(t, errUnsupported, err)
	}

	{
		_, err := NewAgent(nil)
		assert.Nil(t, err)
	}

	{
		_, err := NewAgent(&Opts{})
		assert.Nil(t, err)
	}

	{
		_, err := NewAgent(&Opts{
			UserSecretList: &cordiumv1.UserSecretList{},
		})
		assert.Nil(t, err)
	}
}
