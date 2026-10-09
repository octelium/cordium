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
	"time"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rlockv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	agentSpaceBaseName    = "octelium"
	agentTemplateBaseName = "cordium-agent"

	agentSystemLabelSpace = "octelium-agent"
	agentSystemLabel      = "cordium-agent"

	agentDisplayName = "Cordium Agent"

	agentLockTTLSeconds  = 60
	agentLockWaitSeconds = 30
)

func getAgentSpaceName(usr *corev1.User) string {
	return fmt.Sprintf("%s.%s", agentSpaceBaseName, usr.Metadata.Name)
}

func getAgentTemplateName(usr *corev1.User) string {
	return fmt.Sprintf("%s.%s", agentTemplateBaseName, getAgentSpaceName(usr))
}

func (s *Server) InitializeAgent(ctx context.Context, req *cordiumv1.InitializeAgentRequest) (*cordiumv1.UserConfig, error) {
	if req == nil {
		return nil, grpcutils.InvalidArg("Nil request")
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if req.WorkspaceRef != nil {
		if err := apivalidation.CheckObjectRef(req.WorkspaceRef, &apivalidation.CheckGetOptionsOpts{}); err != nil {
			return nil, err
		}
	}

	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	cfg := cc.GetSpec().GetAgent()
	if cfg.GetIsDisabled() {
		return nil, status.Error(codes.FailedPrecondition, "The agent is disabled in this Cluster")
	}

	tmplSpec, err := getAgentTemplateSpec(cfg)
	if err != nil {
		return nil, err
	}

	unlock, err := s.lockAgent(ctx, i.User)
	if err != nil {
		return nil, err
	}
	defer unlock()

	usrCfg, err := s.GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
	if err != nil {
		return nil, err
	}

	var ws *cordiumv1.Workspace

	if req.WorkspaceRef != nil {
		ws, err = s.getAgentWorkspace(ctx, i.User, req.WorkspaceRef)
		if err != nil {
			return nil, err
		}
		if ws == nil {
			return nil, grpcutils.InvalidArg("The Workspace does not belong to the agent's Template")
		}
	}

	spc, err := s.getOrCreateAgentSpace(ctx, i.User)
	if err != nil {
		return nil, err
	}

	tmpl, err := s.setAgentTemplate(ctx, i.User, spc, tmplSpec)
	if err != nil {
		return nil, err
	}

	if ws == nil && usrCfg.Status.AgentWorkspaceRef != nil {
		ws, err = s.getAgentWorkspace(ctx, i.User, usrCfg.Status.AgentWorkspaceRef)
		if err != nil && !grpcerr.IsNotFound(err) && !grpcerr.IsUnauthorized(err) {
			return nil, err
		}
	}

	if ws == nil {
		ws, err = s.createAgentWorkspace(ctx, tmpl)
		if err != nil {
			return nil, err
		}
	}

	if usrCfg.Status.AgentWorkspaceRef.GetUid() != ws.Metadata.Uid {
		usrCfg.Status.AgentWorkspaceRef = umetav1.GetObjectReference(ws)
		usrCfg, err = s.octeliumC.CordiumC().UpdateUserConfig(ctx, usrCfg)
		if err != nil {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	return usrCfg, nil
}

func (s *Server) lockAgent(ctx context.Context, usr *corev1.User) (func(), error) {
	key := []byte(fmt.Sprintf("cordium.apiserver.agent.%s", usr.Metadata.Uid))

	res, err := s.octeliumC.LockC().Lock(ctx, &rlockv1.LockRequest{
		Key: key,
		Ttl: &metav1.Duration{
			Type: &metav1.Duration_Seconds{
				Seconds: agentLockTTLSeconds,
			},
		},
		Wait: &metav1.Duration{
			Type: &metav1.Duration_Seconds{
				Seconds: agentLockWaitSeconds,
			},
		},
	})
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}
	if !res.Acquired {
		return nil, status.Error(codes.Aborted, "The agent is currently being initialized. Try again later")
	}

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if _, err := s.octeliumC.LockC().Unlock(ctx, &rlockv1.UnlockRequest{
			Key:     key,
			LeaseID: res.LeaseID,
		}); err != nil {
			zap.L().Warn("Could not release the agent lock", zap.Error(err))
		}
	}, nil
}

func (s *Server) getAgentSpace(ctx context.Context, usr *corev1.User) (*cordiumv1.Space, error) {
	spc, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Name: getAgentSpaceName(usr),
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return nil, nil
		}
		return nil, grpcutils.InternalWithErr(err)
	}

	if spc.Metadata.SystemLabels[agentSystemLabelSpace] != "true" ||
		spc.GetStatus().GetUserRef().GetUid() != usr.Metadata.Uid {
		return nil, grpcutils.AlreadyExists(
			"The Space %s already exists and it is not managed by the Cluster", spc.Metadata.Name)
	}

	return spc, nil
}

func (s *Server) getOrCreateAgentSpace(ctx context.Context, usr *corev1.User) (*cordiumv1.Space, error) {
	spc, err := s.getAgentSpace(ctx, usr)
	if err != nil {
		return nil, err
	}

	if spc == nil {
		spc, err = s.octeliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name:        getAgentSpaceName(usr),
				DisplayName: "Octelium",
				Description: "The personal Space of the Octelium AI agents",
				IsSystem:    true,
				SystemLabels: map[string]string{
					"type":                "user",
					agentSystemLabelSpace: "true",
				},
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				UserRef: umetav1.GetObjectReference(usr),
				Type:    cordiumv1.Space_Status_USER,
			},
		})
		if err != nil {
			if !grpcerr.AlreadyExists(err) {
				return nil, grpcutils.InternalWithErr(err)
			}

			spc, err = s.getAgentSpace(ctx, usr)
			if err != nil {
				return nil, err
			}
			if spc == nil {
				return nil, grpcutils.Internal("Could not get the agent Space")
			}
		}
	}

	if _, err := s.octeliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
		Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(spc), umetav1.GetObjectReference(usr)),
	}); err == nil {
		return spc, nil
	} else if !grpcerr.IsNotFound(err) {
		return nil, grpcutils.InternalWithErr(err)
	}

	if _, err := s.octeliumC.CordiumC().CreateMembership(ctx, &cordiumv1.Membership{
		Metadata: &metav1.Metadata{
			Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(spc), umetav1.GetObjectReference(usr)),
		},
		Spec: &cordiumv1.Membership_Spec{
			Role: cordiumv1.Membership_Spec_OWNER,
		},
		Status: &cordiumv1.Membership_Status{
			UserRef:  umetav1.GetObjectReference(usr),
			SpaceRef: umetav1.GetObjectReference(spc),
			UserInfo: workspacecommon.GetMembershipUserInfo(usr),
		},
	}); err != nil && !grpcerr.AlreadyExists(err) {
		return nil, grpcutils.InternalWithErr(err)
	}

	return spc, nil
}

func (s *Server) setAgentTemplate(ctx context.Context,
	usr *corev1.User, spc *cordiumv1.Space, spec *cordiumv1.Template_Spec) (*cordiumv1.Template, error) {
	tmpl, err := s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Name: getAgentTemplateName(usr),
	})
	if err != nil {
		if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}

		tmpl = &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name:        getAgentTemplateName(usr),
				DisplayName: agentDisplayName,
				Description: "The Template of the Cordium AI agent Workspaces",
				IsSystem:    true,
				SystemLabels: map[string]string{
					agentSystemLabel: "true",
				},
			},
			Spec: spec,
			Status: &cordiumv1.Template_Status{
				SpaceRef:  umetav1.GetObjectReference(spc),
				UserRef:   umetav1.GetObjectReference(usr),
				BuildInfo: &cordiumv1.Template_Status_BuildInfo{},
			},
		}

		if err := s.validateAndSetTemplate(ctx, tmpl); err != nil {
			return nil, err
		}

		tmpl, err = s.octeliumC.CordiumC().CreateTemplate(ctx, tmpl)
		if err != nil {
			return nil, grpcutils.InternalWithErr(err)
		}

		return tmpl, nil
	}

	if tmpl.GetStatus().GetSpaceRef().GetUid() != spc.Metadata.Uid {
		return nil, grpcutils.AlreadyExists(
			"The Template %s already exists and it does not belong to the agent Space", tmpl.Metadata.Name)
	}

	if pbutils.IsEqual(tmpl.Spec, spec) {
		return tmpl, nil
	}

	tmpl.Spec = spec
	if err := s.validateAndSetTemplate(ctx, tmpl); err != nil {
		return nil, err
	}

	tmpl, err = s.octeliumC.CordiumC().UpdateTemplate(ctx, tmpl)
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	return tmpl, nil
}

func (s *Server) getAgentWorkspace(ctx context.Context,
	usr *corev1.User, ref *metav1.ObjectReference) (*cordiumv1.Workspace, error) {
	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, apivalidation.ObjectReferenceToRGetOptions(ref))
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if ws.GetStatus().GetUserRef().GetUid() != usr.Metadata.Uid {
		return nil, grpcutils.Unauthorized("This Workspace is not owned by the User")
	}

	if ws.GetStatus().GetIsBuild() ||
		ws.GetStatus().GetTemplateRef().GetName() != getAgentTemplateName(usr) {
		return nil, nil
	}

	return ws, nil
}

func (s *Server) createAgentWorkspace(ctx context.Context, tmpl *cordiumv1.Template) (*cordiumv1.Workspace, error) {
	return s.CreateWorkspace(ctx, &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			DisplayName: agentDisplayName,
		},
		Spec: &cordiumv1.Workspace_Spec{
			Applications: []*cordiumv1.Workspace_Spec_Application{
				{
					Name:        agentApplicationName,
					DisplayName: agentDisplayName,
					Port:        agentApplicationPort,
					IsDefault:   true,
				},
			},
		},
		Status: &cordiumv1.Workspace_Status{
			TemplateRef: umetav1.GetObjectReference(tmpl),
		},
	})
}

func (s *Server) unsetAgentWorkspace(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.GetStatus().GetUserRef() == nil {
		return nil
	}

	usrCfg, err := s.octeliumC.CordiumC().GetUserConfig(ctx, &rmetav1.GetOptions{
		Name: ovutils.GetUserConfigName(ws.Status.UserRef),
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return grpcutils.InternalWithErr(err)
	}

	if usrCfg.GetStatus().GetAgentWorkspaceRef().GetUid() != ws.Metadata.Uid {
		return nil
	}

	usrCfg.Status.AgentWorkspaceRef = nil
	if _, err := s.octeliumC.CordiumC().UpdateUserConfig(ctx, usrCfg); err != nil {
		return grpcutils.InternalWithErr(err)
	}

	return nil
}
