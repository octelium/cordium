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
	"fmt"
	"testing"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestValidateAndSetWorkspace(t *testing.T) {

	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(ctx, fakeC.OcteliumC)
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

	tmpl, err := srv.GetTemplate(usr.Ctx(), &metav1.GetOptions{
		Name: fmt.Sprintf("default.%s", org.Metadata.Name),
	})
	assert.Nil(t, err)

	ctx = usr.Ctx()

	{
		req := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		}
		err := srv.validateAndSetWorkspace(ctx, req)
		assert.Nil(t, err, "%+v", err)
	}

	{
		req := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Workspace_Spec{
				Image: &cordiumv1.Workspace_Spec_Image{},
			},
			Status: &cordiumv1.Workspace_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		}
		err := srv.validateAndSetWorkspace(ctx, req)
		assert.Nil(t, err, "%+v", err)
	}

	{
		req := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Workspace_Spec{
				Runtime: &cordiumv1.Workspace_Spec_Runtime{},
			},
			Status: &cordiumv1.Workspace_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		}
		err := srv.validateAndSetWorkspace(ctx, req)
		assert.Nil(t, err)
	}

	{

		validEnvVarList := [][]*cordiumv1.Workspace_Spec_Runtime_EnvVar{
			nil,
			[]*cordiumv1.Workspace_Spec_Runtime_EnvVar{},
			[]*cordiumv1.Workspace_Spec_Runtime_EnvVar{
				{
					Key: "key1",
					Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{
						Value: "val",
					},
				},
			},
		}

		invalidEnvVarList := [][]*cordiumv1.Workspace_Spec_Runtime_EnvVar{

			[]*cordiumv1.Workspace_Spec_Runtime_EnvVar{
				{
					Key: "key1",
				},

				{
					Key: "key1",
					Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret{
						FromSecret: "non-existen-secret",
					},
				},
			},
		}

		for _, envVars := range validEnvVarList {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						EnvVars: envVars,
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.Nil(t, err)
		}

		for _, envVars := range invalidEnvVarList {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						EnvVars: envVars,
					},
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.NotNil(t, err)
		}
	}

	{
		req := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Workspace_Spec{
				Image: &cordiumv1.Workspace_Spec_Image{
					Type: &cordiumv1.Workspace_Spec_Image_Registry_{
						Registry: &cordiumv1.Workspace_Spec_Image_Registry{},
					},
				},
			},
			Status: &cordiumv1.Workspace_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		}
		err := srv.validateAndSetWorkspace(ctx, req)
		assert.NotNil(t, err)
	}

	{

		validCmds := [][]*cordiumv1.Workspace_Spec_Runtime_Task{
			nil,
			[]*cordiumv1.Workspace_Spec_Runtime_Task{},
			[]*cordiumv1.Workspace_Spec_Runtime_Task{
				{
					Name: "cmd-1",
					Run:  "ls -la",
					Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
				},
				{
					Name: "cmd-2",
					Run:  "podman version",
					Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
				},
			},
		}

		invalidCmds := [][]*cordiumv1.Workspace_Spec_Runtime_Task{
			[]*cordiumv1.Workspace_Spec_Runtime_Task{
				{},
			},
			[]*cordiumv1.Workspace_Spec_Runtime_Task{
				{
					Run: "ls -la",
				},
			},
			[]*cordiumv1.Workspace_Spec_Runtime_Task{
				{
					Name: "cmd-1",
					Run:  "ls -la",
				},
			},
		}

		for _, validCmd := range validCmds {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						Tasks: validCmd,
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.Nil(t, err)
		}

		for _, validCmd := range invalidCmds {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						Tasks: validCmd,
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.NotNil(t, err)
		}
	}

	{

		validURLs := []string{
			"ubuntu",
			"example.com/org/image",
		}

		for _, imageURL := range validURLs {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Image: &cordiumv1.Workspace_Spec_Image{
						Type: &cordiumv1.Workspace_Spec_Image_Registry_{
							Registry: &cordiumv1.Workspace_Spec_Image_Registry{
								Url: imageURL,
							},
						},
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.Nil(t, err)
		}

	}

	{

		validURLs := []string{
			"github.com/example/example",
			"https://github.com/example/example",
			"https://github.com/example/example.git",
		}

		invalidURLs := []string{
			"",
		}

		for _, gitURL := range validURLs {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Image: &cordiumv1.Workspace_Spec_Image{
						Type: &cordiumv1.Workspace_Spec_Image_Git_{
							Git: &cordiumv1.Workspace_Spec_Image_Git{
								Url: gitURL,
							},
						},
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.Nil(t, err)
		}

		for _, gitURL := range invalidURLs {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Image: &cordiumv1.Workspace_Spec_Image{
						Type: &cordiumv1.Workspace_Spec_Image_Git_{
							Git: &cordiumv1.Workspace_Spec_Image_Git{
								Url: gitURL,
							},
						},
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.NotNil(t, err)
		}
	}

	{

		validURLs := []string{
			"https://github.com/example/example",
			"https://github.com/example/example.git",
		}

		invalidURLs := []string{
			"http://example.com/example/example",
			"github.com/example/example",
		}

		for _, pURL := range validURLs {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Repository: &cordiumv1.Workspace_Spec_Repository{
						Url: pURL,
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.Nil(t, err)
		}

		for _, pURL := range invalidURLs {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Repository: &cordiumv1.Workspace_Spec_Repository{
						Url: pURL,
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef:    umetav1.GetObjectReference(org),
					TemplateRef: umetav1.GetObjectReference(tmpl),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.NotNil(t, err)
		}
	}

	{

		validApps := [][]*cordiumv1.Workspace_Spec_Application{
			nil,
			{},
			{
				{
					Name: "app1",
					Port: 80,
				},
			},
			{
				{
					Name:      "app1",
					Port:      80,
					IsDefault: true,
				},
			},
			{
				{
					Name: "app1",
					Port: 80,
				},
				{
					Name: "app2",
					Port: 8080,
				},
				{
					Name: "app3",
					Port: 80,
				},
			},
		}

		invalidApps := [][]*cordiumv1.Workspace_Spec_Application{
			{
				{
					Name: "app1",
					Port: 80,
				},
				{
					Name: "app1",
					Port: 8080,
				},
			},
			{
				{
					Name: "app1",
				},
			},
			{
				{
					Name: "app1",
					Port: 80,
				},
				{
					Name: "app2",
					Port: -80,
				},
			},
			{
				{
					Name:      "app1",
					Port:      80,
					IsDefault: true,
				},
				{
					Name:      "app2",
					Port:      8080,
					IsDefault: true,
				},
			},
		}

		for _, validApp := range validApps {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Applications: validApp,
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef:    umetav1.GetObjectReference(org),
					TemplateRef: umetav1.GetObjectReference(tmpl),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.Nil(t, err, "%+v", err)
		}

		for _, invalidApp := range invalidApps {
			req := &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(8),
				},
				Spec: &cordiumv1.Workspace_Spec{
					Applications: invalidApp,
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: umetav1.GetObjectReference(org),
				},
			}
			err := srv.validateAndSetWorkspace(ctx, req)
			assert.NotNil(t, err)
		}
	}
}
