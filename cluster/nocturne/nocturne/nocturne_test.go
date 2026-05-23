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

package nocturne

/*
import (
	"testing"

	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	utils_rand "github.com/octelium/octelium/pkg/utils/random"
	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/stretchr/testify/assert"
	"context"
)

func TestSrv(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	accessKeySec, err := fakeC.OcteliumC.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec:   &cordiumv1.Secret_Spec{},
		Status: &cordiumv1.Secret_Status{},
		Data: &cordiumv1.Secret_Data{
			Type: &cordiumv1.Secret_Data_Value{
				Value: utilrand.GetRandomString(20),
			},
		},
	})
	assert.Nil(t, err)

	cp, err := fakeC.OcteliumC.CordiumC().CreateCloudProvider(ctx, &cordiumv1.CloudProvider{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &cordiumv1.CloudProvider_Spec{
			Type: &cordiumv1.CloudProvider_Spec_S3_{
				S3: &cordiumv1.CloudProvider_Spec_S3{
					AccessKeyID: utilrand.GetRandomString(8),
					SecretAccessKey: &cordiumv1.CloudProvider_Spec_S3_SecretAccessKey{
						Type: &cordiumv1.CloudProvider_Spec_S3_SecretAccessKey_FromSecret{
							FromSecret: accessKeySec.Metadata.Name,
						},
					},
				},
			},
		},
		Status: &cordiumv1.CloudProvider_Status{},
	})
	assert.Nil(t, err)

	cc, err := fakeC.OcteliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err)

	// cc.Spec.StorageCloudProvider = cp.Metadata.Name

	cc, err = fakeC.OcteliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err)



}
*/
