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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func tstSetAgentConfig(t *testing.T, octeliumC octeliumc.ClientInterface, cfg *cordiumv1.ClusterConfig_Spec_Agent) {
	ctx := context.Background()

	cc, err := octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err, "%+v", err)

	cc.Spec.Agent = cfg

	_, err = octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err, "%+v", err)
}

func TestInitializeAgent(t *testing.T) {
	t.Setenv("OCTELIUM_DEV", "true")
	oldGitBranch := ldflags.GitBranch
	ldflags.GitBranch = ""
	t.Cleanup(func() {
		ldflags.GitBranch = oldGitBranch
	})

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

	{
		_, err := srv.InitializeAgent(usr.Ctx(), nil)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		_, err = srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{
			WorkspaceRef: &metav1.ObjectReference{},
		})
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	}

	{
		tstSetAgentConfig(t, fakeC.OcteliumC, &cordiumv1.ClusterConfig_Spec_Agent{
			IsDisabled: true,
		})

		_, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.True(t, grpcerr.IsFailedPrecondition(err), "%+v", err)

		tstSetAgentConfig(t, fakeC.OcteliumC, nil)
	}

	var agentWS *cordiumv1.Workspace

	{
		res, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.NotNil(t, res.Status.AgentWorkspaceRef)

		agentWS, err = srv.GetWorkspace(usr.Ctx(), &metav1.GetOptions{Uid: res.Status.AgentWorkspaceRef.Uid})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPED, agentWS.Status.State)
		assert.Equal(t, usr.Usr.Metadata.Uid, agentWS.Status.UserRef.Uid)
		assert.Equal(t, fmt.Sprintf("octelium.%s", usr.Usr.Metadata.Name), agentWS.Status.SpaceRef.Name)
		assert.Equal(t, fmt.Sprintf("cordium-agent.octelium.%s", usr.Usr.Metadata.Name), agentWS.Status.TemplateRef.Name)
		assert.Equal(t, cordiumv1.Space_Status_USER, agentWS.Status.SpaceType)
		assert.Equal(t, agentDisplayName, agentWS.Metadata.DisplayName)
		assert.False(t, agentWS.Spec.IsEphemeral)
		assert.Len(t, agentWS.Spec.Applications, 1)
		assert.Equal(t, agentApplicationName, agentWS.Spec.Applications[0].Name)
		assert.Equal(t, int32(agentApplicationPort), agentWS.Spec.Applications[0].Port)
		assert.True(t, agentWS.Spec.Applications[0].IsDefault)
		assert.NotZero(t, agentWS.Status.Limit.Cpu.Millicores)

		spc, err := fakeC.OcteliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{Uid: agentWS.Status.SpaceRef.Uid})
		assert.Nil(t, err, "%+v", err)
		assert.True(t, spc.Metadata.IsSystem)
		assert.Equal(t, "true", spc.Metadata.SystemLabels[agentSystemLabelSpace])
		assert.Equal(t, "user", spc.Metadata.SystemLabels["type"])
		assert.Equal(t, cordiumv1.Space_Status_USER, spc.Status.Type)
		assert.Equal(t, usr.Usr.Metadata.Uid, spc.Status.UserRef.Uid)

		mem, err := fakeC.OcteliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
			Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(spc), umetav1.GetObjectReference(usr.Usr)),
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, cordiumv1.Membership_Spec_OWNER, mem.Spec.Role)

		tmpl, err := fakeC.OcteliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{Uid: agentWS.Status.TemplateRef.Uid})
		assert.Nil(t, err, "%+v", err)
		assert.True(t, tmpl.Metadata.IsSystem)
		assert.Equal(t, "true", tmpl.Metadata.SystemLabels[agentSystemLabel])
		assert.Equal(t, spc.Metadata.Uid, tmpl.Status.SpaceRef.Uid)
		assert.Len(t, tmpl.Spec.Runtime.Tasks, 2)
		assert.Equal(t, cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE, tmpl.Spec.Runtime.Tasks[0].Type)
		assert.True(t, tmpl.Spec.Runtime.Tasks[0].RunAsRoot)
		assert.Equal(t, cordiumv1.Workspace_Spec_Runtime_Task_POST_START, tmpl.Spec.Runtime.Tasks[1].Type)
		assert.True(t, tmpl.Spec.Runtime.Tasks[1].IsBackground)
		assert.Equal(t, fmt.Sprintf("exec npx --yes --prefer-online %s@%s serve", agentPackage, defaultAgentVersion),
			tmpl.Spec.Runtime.Tasks[1].Run)
		assert.Len(t, tmpl.Spec.Runtime.EnvVars, 0)

		usrCfg, err := srv.GetUserConfig(usr.Ctx(), &cordiumv1.GetUserConfigRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, agentWS.Metadata.Uid, usrCfg.Status.AgentWorkspaceRef.Uid)
	}

	{
		res, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, agentWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)

		wsList, err := srv.ListWorkspace(usr.Ctx(), &cordiumv1.ListWorkspaceOptions{})
		assert.Nil(t, err, "%+v", err)
		assert.Len(t, wsList.Items, 1)
	}

	{
		res, err := srv.UpdateUserConfig(usr.Ctx(), &cordiumv1.UserConfig{
			Spec: &cordiumv1.UserConfig_Spec{},
			Status: &cordiumv1.UserConfig_Status{
				AgentWorkspaceRef: &metav1.ObjectReference{
					Uid:  "00000000-0000-4000-8000-000000000000",
					Name: "abc",
				},
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, agentWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)
	}

	{
		tstSetAgentConfig(t, fakeC.OcteliumC, &cordiumv1.ClusterConfig_Spec_Agent{
			Version: "0.2.0",
			Llm: &cordiumv1.ClusterConfig_Spec_Agent_LLM{
				Service: "llm.default",
				Model:   "gpt-5.1",
			},
			Config: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"llm": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"thinkingLevel": structpb.NewStringValue("high"),
						},
					}),
				},
			},
		})

		res, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, agentWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)

		tmpl, err := fakeC.OcteliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{Uid: agentWS.Status.TemplateRef.Uid})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, fmt.Sprintf("exec npx --yes --prefer-online %s@0.2.0 serve", agentPackage),
			tmpl.Spec.Runtime.Tasks[1].Run)
		assert.Len(t, tmpl.Spec.Runtime.EnvVars, 1)
		assert.Equal(t, agentConfigEnvVar, tmpl.Spec.Runtime.EnvVars[0].Key)

		cfg := map[string]any{}
		assert.Nil(t, json.Unmarshal([]byte(tmpl.Spec.Runtime.EnvVars[0].GetValue()), &cfg))
		assert.Equal(t, map[string]any{
			"llm": map[string]any{
				"provider":      "octelium",
				"service":       "llm.default",
				"model":         "gpt-5.1",
				"thinkingLevel": "high",
			},
		}, cfg)

		tstSetAgentConfig(t, fakeC.OcteliumC, nil)
	}

	var otherWS *cordiumv1.Workspace

	{
		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status:   &cordiumv1.Workspace_Status{},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, fmt.Sprintf("default.%s", usr.Usr.Metadata.Name), ws.Status.SpaceRef.Name)

		_, err = srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		otherWS, err = srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{},
			Spec:     &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: agentWS.Status.TemplateRef,
			},
		})
		assert.Nil(t, err, "%+v", err)

		res, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{
			WorkspaceRef: &metav1.ObjectReference{Name: otherWS.Metadata.Name},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, otherWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)

		wsList, err := srv.ListWorkspace(usr.Ctx(), &cordiumv1.ListWorkspaceOptions{
			Filter: &cordiumv1.ListWorkspaceOptions_SpaceRef{
				SpaceRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("octelium.%s", usr.Usr.Metadata.Name),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Len(t, wsList.Items, 2)

		wsList, err = srv.ListWorkspace(usr.Ctx(), &cordiumv1.ListWorkspaceOptions{
			Filter: &cordiumv1.ListWorkspaceOptions_SpaceRef{
				SpaceRef: ws.Status.SpaceRef,
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Len(t, wsList.Items, 1)
		assert.Equal(t, ws.Metadata.Uid, wsList.Items[0].Metadata.Uid)

		wsList, err = srv.ListWorkspace(usr.Ctx(), &cordiumv1.ListWorkspaceOptions{
			Filter: &cordiumv1.ListWorkspaceOptions_TemplateRef{
				TemplateRef: &metav1.ObjectReference{
					Name: agentWS.Status.TemplateRef.Name,
				},
			},
		})
		assert.Nil(t, err, "%+v", err)
		assert.Len(t, wsList.Items, 2)
	}

	{
		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.InitializeAgent(usr2.Ctx(), &cordiumv1.InitializeAgentRequest{
			WorkspaceRef: umetav1.GetObjectReference(otherWS),
		})
		assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)
	}

	{
		_, err := srv.DeleteWorkspace(usr.Ctx(), &metav1.DeleteOptions{Uid: agentWS.Metadata.Uid})
		assert.Nil(t, err, "%+v", err)

		usrCfg, err := srv.GetUserConfig(usr.Ctx(), &cordiumv1.GetUserConfigRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, otherWS.Metadata.Uid, usrCfg.Status.AgentWorkspaceRef.Uid)

		_, err = srv.DeleteWorkspace(usr.Ctx(), &metav1.DeleteOptions{Uid: otherWS.Metadata.Uid})
		assert.Nil(t, err, "%+v", err)

		usrCfg, err = srv.GetUserConfig(usr.Ctx(), &cordiumv1.GetUserConfigRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, usrCfg.Status.AgentWorkspaceRef)

		res, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.NotNil(t, res.Status.AgentWorkspaceRef)
		assert.NotEqual(t, agentWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)
		assert.NotEqual(t, otherWS.Metadata.Uid, res.Status.AgentWorkspaceRef.Uid)

		_, err = fakeC.OcteliumC.CordiumC().DeleteWorkspace(ctx, &rmetav1.DeleteOptions{
			Uid: res.Status.AgentWorkspaceRef.Uid,
		})
		assert.Nil(t, err, "%+v", err)

		res2, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.NotEqual(t, res.Status.AgentWorkspaceRef.Uid, res2.Status.AgentWorkspaceRef.Uid)

		spc, err := srv.GetSpace(usr.Ctx(), &metav1.GetOptions{Name: fmt.Sprintf("octelium.%s", usr.Usr.Metadata.Name)})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.DeleteSpace(usr.Ctx(), &metav1.DeleteOptions{Uid: spc.Metadata.Uid})
		assert.Nil(t, err, "%+v", err)

		usrCfg, err = srv.GetUserConfig(usr.Ctx(), &cordiumv1.GetUserConfigRequest{})
		assert.Nil(t, err, "%+v", err)
		assert.Nil(t, usrCfg.Status.AgentWorkspaceRef)
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: "octelium",
			},
			Spec: &cordiumv1.Space_Spec{},
		})
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)

		_, err = fakeC.OcteliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("octelium.%s", usr.Usr.Metadata.Name),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				UserRef: umetav1.GetObjectReference(usr.Usr),
				Type:    cordiumv1.Space_Status_USER,
			},
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.True(t, grpcerr.AlreadyExists(err), "%+v", err)
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := fakeC.OcteliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name:     fmt.Sprintf("octelium.%s", usr.Usr.Metadata.Name),
				IsSystem: true,
				SystemLabels: map[string]string{
					"type":                "user",
					agentSystemLabelSpace: "true",
				},
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				UserRef: umetav1.GetObjectReference(usr.Usr),
				Type:    cordiumv1.Space_Status_USER,
			},
		})
		assert.Nil(t, err, "%+v", err)

		_, err = fakeC.OcteliumC.CordiumC().CreateMembership(ctx, &cordiumv1.Membership{
			Metadata: &metav1.Metadata{
				Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(spc), umetav1.GetObjectReference(usr.Usr)),
			},
			Spec: &cordiumv1.Membership_Spec{
				Role: cordiumv1.Membership_Spec_OWNER,
			},
			Status: &cordiumv1.Membership_Status{
				UserRef:  umetav1.GetObjectReference(usr.Usr),
				SpaceRef: umetav1.GetObjectReference(spc),
			},
		})
		assert.Nil(t, err, "%+v", err)

		res, err := srv.InitializeAgent(usr.Ctx(), &cordiumv1.InitializeAgentRequest{})
		assert.Nil(t, err, "%+v", err)

		ws, err := srv.GetWorkspace(usr.Ctx(), &metav1.GetOptions{Uid: res.Status.AgentWorkspaceRef.Uid})
		assert.Nil(t, err, "%+v", err)
		assert.Equal(t, spc.Metadata.Uid, ws.Status.SpaceRef.Uid)
	}
}

func TestGetAgentVersion(t *testing.T) {
	oldGitBranch, oldGitTag, oldSemVer := ldflags.GitBranch, ldflags.GitTag, ldflags.SemVer
	t.Cleanup(func() {
		ldflags.GitBranch, ldflags.GitTag, ldflags.SemVer = oldGitBranch, oldGitTag, oldSemVer
	})

	tstVersion := func(cfg *cordiumv1.ClusterConfig_Spec_Agent) string {
		ret, err := getAgentVersion(cfg)
		assert.Nil(t, err, "%+v", err)
		return ret
	}

	{
		t.Setenv("OCTELIUM_DEV", "true")
		t.Setenv("OCTELIUM_PRODUCTION", "")

		ldflags.GitBranch = ""
		assert.Equal(t, defaultAgentVersion, tstVersion(nil))

		ldflags.GitBranch = "b-w01"
		assert.Equal(t, "b-w01", tstVersion(nil))
		assert.Equal(t, "next", tstVersion(&cordiumv1.ClusterConfig_Spec_Agent{Version: "next"}))
	}

	{
		t.Setenv("OCTELIUM_DEV", "")
		t.Setenv("OCTELIUM_PRODUCTION", "true")

		for _, tc := range []struct {
			semVer string
			gitTag string
			want   string
		}{
			{want: "latest"},
			{gitTag: "v0.17.0", want: "0.17.0"},
			{gitTag: "0.17.0", want: "0.17.0"},
			{semVer: "v1.2.3-rc.1", gitTag: "v0.17.0", want: "1.2.3-rc.1"},
			{gitTag: "v01.2.3", want: "latest"},
			{gitTag: "v1.2.3-01", want: "latest"},
			{gitTag: "release", want: "latest"},
		} {
			ldflags.SemVer, ldflags.GitTag = tc.semVer, tc.gitTag
			assert.Equal(t, tc.want, tstVersion(nil), "%+v", tc)
		}

		ldflags.SemVer, ldflags.GitTag = "", "v0.17.0"
		assert.Equal(t, "0.18.0", tstVersion(&cordiumv1.ClusterConfig_Spec_Agent{Version: "0.18.0"}))
	}

	{
		_, err := getAgentVersion(&cordiumv1.ClusterConfig_Spec_Agent{Version: "-bad"})
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v", err)
	}
}

func TestValidateAgentConfig(t *testing.T) {
	assert.Nil(t, ValidateAgentConfig(nil))
	assert.Nil(t, ValidateAgentConfig(&cordiumv1.ClusterConfig_Spec_Agent{}))

	assert.Nil(t, ValidateAgentConfig(&cordiumv1.ClusterConfig_Spec_Agent{
		Version: "0.1.0",
		Llm: &cordiumv1.ClusterConfig_Spec_Agent_LLM{
			Service: "llm.default",
			Model:   "claude-opus-4-8",
		},
		Image: &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Registry_{
				Registry: &cordiumv1.Workspace_Spec_Image_Registry{
					Url: "node:24",
				},
			},
		},
		Limit: &cordiumv1.Workspace_Spec_Limit{
			Cpu: &cordiumv1.Workspace_Spec_Limit_CPU{
				Millicores: 4000,
			},
		},
	}))

	for _, cfg := range []*cordiumv1.ClusterConfig_Spec_Agent{
		{Version: "bad version"},
		{Llm: &cordiumv1.ClusterConfig_Spec_Agent_LLM{Service: "Invalid Service"}},
		{Llm: &cordiumv1.ClusterConfig_Spec_Agent_LLM{Model: strings.Repeat("a", maxAgentModelLen+1)}},
		{Llm: &cordiumv1.ClusterConfig_Spec_Agent_LLM{Model: "model\n"}},
		{Image: &cordiumv1.Workspace_Spec_Image{}},
		{Image: &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Registry_{
				Registry: &cordiumv1.Workspace_Spec_Image_Registry{
					Url: "node 24",
				},
			},
		}},
		{Image: &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_{
				Dockerfile: &cordiumv1.Workspace_Spec_Image_Dockerfile{
					Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_Url{
						Url: "http://example.com/Dockerfile",
					},
				},
			},
		}},
		{Limit: &cordiumv1.Workspace_Spec_Limit{
			Memory: &cordiumv1.Workspace_Spec_Limit_Memory{
				Megabytes: maxAgentLimitMegabytes + 1,
			},
		}},
		{Config: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"a": structpb.NewStringValue(strings.Repeat("a", maxAgentConfigBytes)),
			},
		}},
	} {
		err := ValidateAgentConfig(cfg)
		assert.True(t, grpcerr.IsInvalidArg(err), "%+v: %+v", cfg, err)
	}
}

func TestGetAgentConfig(t *testing.T) {
	{
		ret, err := getAgentConfig(nil)
		assert.Nil(t, err)
		assert.Equal(t, "", ret)
	}

	{
		ret, err := getAgentConfig(&cordiumv1.ClusterConfig_Spec_Agent{
			Llm: &cordiumv1.ClusterConfig_Spec_Agent_LLM{
				Model: "gpt-5.1",
			},
			Config: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"llm": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"provider": structpb.NewStringValue("anthropic"),
						},
					}),
					"approvals": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"api": structpb.NewStringValue("write"),
						},
					}),
				},
			},
		})
		assert.Nil(t, err)

		cfg := map[string]any{}
		assert.Nil(t, json.Unmarshal([]byte(ret), &cfg))
		assert.Equal(t, map[string]any{
			"llm": map[string]any{
				"provider": "anthropic",
				"model":    "gpt-5.1",
			},
			"approvals": map[string]any{
				"api": "write",
			},
		}, cfg)
	}
}
