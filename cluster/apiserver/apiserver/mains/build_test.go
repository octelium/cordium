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
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestBuild(t *testing.T) {

	ctx := context.Background()
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

		tmplReq := &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec:   &cordiumv1.Template_Spec{},
			Status: &cordiumv1.Template_Status{},
		}

		tmpl, err := srv.CreateTemplate(usr.Ctx(), tmplReq)
		assert.Nil(t, err, "%+v", err)

		{

			prebuild, err := srv.BuildTemplate(usr.Ctx(), &cordiumv1.BuildTemplateRequest{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			})
			assert.Nil(t, err, "%+v", err)

			assert.Equal(t, tmpl.Status.SpaceRef.Uid, prebuild.Status.SpaceRef.Uid)
			assert.Equal(t, 1, len(prebuild.Status.BuildInfo.Builds))
			assert.Equal(t, prebuild.Status.BuildInfo.CurrentRunningBuildID, prebuild.Status.BuildInfo.Builds[0].Id)

			ws, err := srv.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
				Name: fmt.Sprintf("build-%s-%s", prebuild.Metadata.Uid, prebuild.Status.BuildInfo.Builds[0].Id),
			})
			assert.Nil(t, err)

			assert.True(t, ws.Status.IsBuild)
			assert.Equal(t, ws.Metadata.SystemLabels["build-id"], prebuild.Status.BuildInfo.Builds[0].Id)
			assert.Equal(t, ws.Status.State, cordiumv1.Workspace_Status_INIT_REQUEST)
		}

		{

			prebuild, err := srv.BuildTemplate(usr.Ctx(), &cordiumv1.BuildTemplateRequest{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			})
			assert.Nil(t, err, "%+v", err)

			assert.Equal(t, tmpl.Status.SpaceRef.Uid, prebuild.Status.SpaceRef.Uid)
			assert.Equal(t, 2, len(prebuild.Status.BuildInfo.Builds))
			assert.Equal(t, prebuild.Status.BuildInfo.CurrentRunningBuildID, prebuild.Status.BuildInfo.Builds[0].Id)

			{
				ws, err := srv.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
					Name: fmt.Sprintf("build-%s-%s", prebuild.Metadata.Uid, prebuild.Status.BuildInfo.Builds[0].Id),
				})
				assert.Nil(t, err)

				assert.True(t, ws.Status.IsBuild)
				assert.Equal(t, ws.Metadata.SystemLabels["build-id"], prebuild.Status.BuildInfo.Builds[0].Id)
				assert.Equal(t, ws.Status.State, cordiumv1.Workspace_Status_INIT_REQUEST)
			}

			{
				ws, err := srv.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
					Name: fmt.Sprintf("build-%s-%s", prebuild.Metadata.Uid, prebuild.Status.BuildInfo.Builds[1].Id),
				})
				assert.Nil(t, err)

				assert.True(t, ws.Status.IsBuild)
				assert.Equal(t, ws.Metadata.SystemLabels["build-id"], prebuild.Status.BuildInfo.Builds[1].Id)
				assert.Equal(t, ws.Status.State, cordiumv1.Workspace_Status_STOPPING_REQUEST)
			}
		}

	}

}
