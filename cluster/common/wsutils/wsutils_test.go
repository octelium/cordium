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

package wsutils

import (
	"testing"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/stretchr/testify/assert"
)

func TestMergeSpec(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	{
		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status:   &cordiumv1.Workspace_Status{},
		}

		spec, err := MergeSpec(&MergeSpecReq{
			Workspace: ws,
		})
		assert.Nil(t, err)
		assert.True(t, pbutils.IsEqual(ws.Spec, spec))
	}

	{
		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status:   &cordiumv1.Workspace_Status{},
		}

		env := &cordiumv1.Template{
			Metadata: &metav1.Metadata{},
			Spec: &cordiumv1.Template_Spec{
				Repository: &cordiumv1.Workspace_Spec_Repository{
					Url: "https://github.com/a/env",
					CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
						Depth: 10,
					},
				},
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
						{
							Key: "K01",
							Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{
								Value: "V01",
							},
						},
					},
				},
			},
			Status: &cordiumv1.Template_Status{},
		}

		{
			spec, err := MergeSpec(&MergeSpecReq{
				Workspace: ws,
				Template:  env,
			})
			assert.Nil(t, err)
			assert.Equal(t, spec.Repository.Url, "https://github.com/a/env")
		}

	}

	/*
		t.Run("project", func(t *testing.T) {
			usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
			assert.Nil(t, err)

			spc, err := MergeSpec(&MergeSpecReq{
				Workspace: ,
			})
			assert.Nil(t, err)

			prj, err := srv.CreateTemplate(usr.Ctx(), &cordiumv1.Template{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), spc.Metadata.Name),
				},
				Spec: &cordiumv1.Template_Spec{
					Repository: &cordiumv1.Workspace_Spec_Repository{
						Url: "https://github.com/linux/linux",
					},
				},
				Status: &cordiumv1.Template_Status{
					SpaceRef: umetav1.GetObjectReference(spc),
				},
			})
			assert.Nil(t, err)

			ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name:        "rrr",
					DisplayName: "My Workspace",
				},
				Spec: &cordiumv1.Workspace_Spec{},
				Status: &cordiumv1.Workspace_Status{
					TemplateRef: umetav1.GetObjectReference(prj),
				},
			})
			assert.Nil(t, err, "%+v", err)

			assert.True(t, pbutils.IsEqual(ws.Status.TemplateRef, umetav1.GetObjectReference(prj)))

			assert.Equal(t, prj.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)

			ws.Spec.Repository = &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/linux2/linux2",
			}

			ws, err = srv.UpdateWorkspace(usr.Ctx(), ws)
			assert.Nil(t, err)

			assert.Equal(t, ws.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)
		})

		t.Run("template", func(t *testing.T) {
			usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
			assert.Nil(t, err)

			spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Space_Spec{},
				Status: &cordiumv1.Space_Status{
					Type: cordiumv1.Space_Status_ORGANIZATION,
				},
			})
			assert.Nil(t, err)

			prj, err := srv.CreateTemplate(usr.Ctx(), &cordiumv1.Template{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), spc.Metadata.Name),
				},
				Spec: &cordiumv1.Template_Spec{
					Repository: &cordiumv1.Workspace_Spec_Repository{
						Url: "https://github.com/linux/linux",
					},
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						Entrypoint: "/usr/bin/init",
					},
				},
				Status: &cordiumv1.Template_Status{},
			})
			assert.Nil(t, err)

			tmpl, err := srv.CreateTemplate(usr.Ctx(), &cordiumv1.Template{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), prj.Metadata.Name),
				},
				Spec: &cordiumv1.Template_Spec{
					Repository: &cordiumv1.Workspace_Spec_Repository{
						Url: "https://github.com/linux2/linux2",
					},
				},
				Status: &cordiumv1.Template_Status{},
			})
			assert.Nil(t, err)

			assert.Equal(t, tmpl.Spec.Repository.Url, ucordiumv1.ToTemplate(tmpl).GetMergedSpec().Repository.Url)
			assert.Equal(t, "/usr/bin/init", ucordiumv1.ToTemplate(tmpl).GetMergedSpec().Runtime.Entrypoint)

			ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name:        "rrr",
					DisplayName: "My Workspace",
				},
				Spec: &cordiumv1.Workspace_Spec{},
				Status: &cordiumv1.Workspace_Status{
					TemplateRef: umetav1.GetObjectReference(tmpl),
				},
			})
			assert.Nil(t, err, "%+v", err)

			assert.True(t, pbutils.IsEqual(ws.Status.TemplateRef, umetav1.GetObjectReference(prj)))
			assert.True(t, pbutils.IsEqual(ws.Status.TemplateRef, umetav1.GetObjectReference(tmpl)))

			assert.Equal(t, tmpl.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)

			ws.Spec.Repository = &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/linux3/linux3",
			}

			ws, err = srv.UpdateWorkspace(usr.Ctx(), ws)
			assert.Nil(t, err)

			assert.Equal(t, ws.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)
			assert.Equal(t, "/usr/bin/init", ucordiumv1.ToWorkspace(ws).GetMergedSpec().Runtime.Entrypoint)
		})
	*/

}
