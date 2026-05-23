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

package vigil

/*
import (
	"context"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests"
	"github.com/octelium/octelium/pkg/common/pbutils"
	utils_rand "github.com/octelium/octelium/pkg/utils/random"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})
	svc, err := adminSrv.CreateService(ctx, &corev1.Service{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
		},
		Spec: &corev1.Service_Spec{
			Port: 8080,
			Backend: &corev1.Service_Spec_Url{
				Url: "https://example.com",
			},
		},
	})
	assert.Nil(t, err)

	srv, err := NewServer(ctx, &Opts{
		OcteliumC: fakeC.OcteliumC,
		Service:   svc,
	})
	assert.Nil(t, err, "%+v", err)

	srv.server.Run(ctx)

	time.Sleep(1 * time.Second)

	svc2 := pbutils.Clone(svc).(*corev1.Service)

	svc2.Spec.Port = 8081
	svc2, err = adminSrv.UpdateService(ctx, svc2)
	assert.Nil(t, err)

	err = srv.svcCtl.FnOnUpdate(ctx, svc2, svc)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	svc3 := pbutils.Clone(svc2).(*corev1.Service)
	svc3.Spec.Backend = &corev1.Service_Spec_Url{
		Url: "postgres://localhost:5432",
	}

	svc3, err = adminSrv.UpdateService(ctx, svc3)
	assert.Nil(t, err)

	err = srv.svcCtl.FnOnUpdate(ctx, svc3, svc2)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	svc4 := pbutils.Clone(svc3).(*corev1.Service)
	svc4.Spec.Backend = &corev1.Service_Spec_Url{
		Url: "ssh://localhost:2022",
	}
	svc4.Spec.Mode = corev1.Service_Spec_SSH

	svc4, err = adminSrv.UpdateService(ctx, svc4)
	assert.Nil(t, err)

	err = srv.svcCtl.FnOnUpdate(ctx, svc4, svc3)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	svc5 := pbutils.Clone(svc4).(*corev1.Service)
	svc5.Spec.Backend = &corev1.Service_Spec_Url{
		Url: "dns://8.8.8.8",
	}

	svc5.Spec.Mode = corev1.Service_Spec_UNSET

	svc5, err = adminSrv.UpdateService(ctx, svc5)
	assert.Nil(t, err)

	err = srv.svcCtl.FnOnUpdate(ctx, svc5, svc4)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	err = srv.server.Close()
	assert.Nil(t, err)
}

*/
