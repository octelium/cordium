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
