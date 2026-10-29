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

package wsutils

import (
	"os"
	"path/filepath"
	"testing"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
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
					Url: "https://github.com/${{vars.ORG}}/env",
					CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
						Depth: 10,
					},
				},
				Vars: []*cordiumv1.Workspace_Spec_Var{
					{
						Name:  "ORG",
						Value: "my_org",
					},
					{
						Name:  "KEY",
						Value: "my_val",
					},
				},
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
						{
							Key: "K01",
							Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{
								Value: "MY_${{ vars.KEY }}_VAL",
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
			assert.Equal(t, spec.Repository.Url, "https://github.com/my_org/env")

			assert.Equal(t, spec.Runtime.EnvVars[0].GetValue(), "MY_my_val_VAL")
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

func TestLoadWorkspaceFileCandidatesAndParent(t *testing.T) {
	for _, candidate := range []string{".cordium/workspace.yaml", ".cordium/workspace.yml", ".cordium.yaml", ".cordium.yml"} {
		t.Run(candidate, func(t *testing.T) {
			base := t.TempDir()
			file := filepath.Join(base, candidate)
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("metadata:\n  name: ignored\nspec:\n  runtime:\n    cmd: echo loaded\n"), 0600); err != nil {
				t.Fatal(err)
			}
			req := workspaceValidationReq()
			got, err := LoadWorkspaceFile(t.Context(), &LoadWorkspaceFileRequest{Parent: req.Workspace, Space: req.Space, BaseDir: base})
			if !assert.Nil(t, err) {
				return
			}
			assert.Equal(t, req.Workspace.Metadata, got.Metadata)
			assert.Equal(t, req.Workspace.Status, got.Status)
			assert.Equal(t, "echo loaded", got.Spec.Runtime.Cmd)
			assert.Nil(t, req.Workspace.Spec.Runtime)
		})
	}
}

func TestLoadWorkspaceFilePrecedenceAndErrors(t *testing.T) {
	base := t.TempDir()
	req := workspaceValidationReq()
	opts := &LoadWorkspaceFileRequest{Parent: req.Workspace, Space: req.Space, BaseDir: base}
	_, err := LoadWorkspaceFile(t.Context(), opts)
	assert.NotNil(t, err)
	assert.Nil(t, os.WriteFile(filepath.Join(base, ".cordium.yaml"), []byte("spec:\n  runtime:\n    cmd: fallback\n"), 0600))
	assert.Nil(t, os.MkdirAll(filepath.Join(base, ".cordium"), 0700))
	primary := filepath.Join(base, ".cordium", "workspace.yaml")
	assert.Nil(t, os.WriteFile(primary, []byte("spec:\n  runtime:\n    cmd: primary\n"), 0600))
	got, err := LoadWorkspaceFile(t.Context(), opts)
	if assert.Nil(t, err) {
		assert.Equal(t, "primary", got.Spec.Runtime.Cmd)
	}
	assert.Nil(t, os.WriteFile(primary, []byte("spec: [unterminated"), 0600))
	_, err = LoadWorkspaceFile(t.Context(), opts)
	assert.NotNil(t, err)
	assert.Nil(t, os.WriteFile(primary, []byte("{}"), 0600))
	got, err = LoadWorkspaceFile(t.Context(), opts)
	if assert.Nil(t, err) {
		assert.NotNil(t, got.Spec)
	}
	_, err = LoadWorkspaceFile(t.Context(), nil)
	assert.NotNil(t, err)
}

func TestMergeSpecDoesNotMutateInputs(t *testing.T) {
	ws := workspaceValidationReq().Workspace
	ws.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: "KEY", Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: "workspace"}}}}
	tmpl := &cordiumv1.Template{Spec: &cordiumv1.Template_Spec{Repository: &cordiumv1.Workspace_Spec_Repository{Url: "https://example.com/repo"}}}
	beforeWS, beforeTemplate := proto.Clone(ws), proto.Clone(tmpl)
	merged, err := Merge(&MergeSpecReq{Workspace: ws, Template: tmpl})
	if !assert.Nil(t, err) {
		return
	}
	merged.Spec.Repository.Url = "https://example.com/changed"
	merged.Spec.Runtime.EnvVars[0].Key = "OTHER"
	merged.Metadata.Name = "changed"
	assert.True(t, proto.Equal(beforeWS, ws))
	assert.True(t, proto.Equal(beforeTemplate, tmpl))
}

func TestCheckTerminalIDBoundaries(t *testing.T) {
	for _, id := range []string{"abc-1234", "abcdefg-1abc", "a12-0000"} {
		assert.Nil(t, CheckTerminalID(id), "%s", id)
	}
	for _, id := range []string{"", "ab-1234", "abcdefgh-1234", "1ab-1234", "Abc-1234", "abc-123", "abc-12345", "abc-12_4", "abc-1234\n", "../abc-1234"} {
		assert.NotNil(t, CheckTerminalID(id), "%s", id)
	}
}
