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
	"context"
	"fmt"
	"time"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/sessionc"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/cluster/common/userctx"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"google.golang.org/protobuf/types/known/structpb"
)

func (s *Server) CreateWorkspace(ctx context.Context, req *cordiumv1.Workspace) (*cordiumv1.Workspace, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(req, &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{},
	}); err != nil {
		return nil, err
	}

	if req.Status == nil {
		req.Status = &cordiumv1.Workspace_Status{}
	}

	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	wsList, err := s.octeliumC.CordiumC().ListWorkspace(ctx, urscsrv.FilterByUser(i.User))
	if err != nil {
		return nil, err
	}

	if len(wsList.Items) >= int(s.getMaxWorkspacesPerUser(cc)) {
		return nil, serr.InvalidArg("Number of Workspaces per User has been exceeded")
	}

	wsName, err := s.genWorkspaceName(ctx)
	if err != nil {
		return nil, err
	}
	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name:        wsName,
			DisplayName: req.Metadata.DisplayName,
		},
		Spec: req.Spec,
		Status: &cordiumv1.Workspace_Status{
			UserRef:     umetav1.GetObjectReference(i.User),
			State:       cordiumv1.Workspace_Status_STOPPED,
			IsEphemeral: req.Status.IsEphemeral,
		},
	}

	ws.Status.Limit = &cordiumv1.Workspace_Status_Limit{}

	var template *cordiumv1.Template
	if req.Status.TemplateRef == nil {
		template, err = s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
			Name: fmt.Sprintf("default.default.%s", i.User.Metadata.Name),
		})
		if err != nil {
			if !grpcerr.IsNotFound(err) {
				return nil, err
			}

			spc, err := s.CreateSpace(ctx, &cordiumv1.Space{
				Metadata: &metav1.Metadata{
					Name:        fmt.Sprintf("default.%s", i.User.Metadata.Name),
					DisplayName: "Default Space",
				},
				Spec: &cordiumv1.Space_Spec{},
				Status: &cordiumv1.Space_Status{
					Type: cordiumv1.Space_Status_USER,
				},
			})
			if err != nil {
				return nil, err
			}

			template, err = s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
				Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
			})
			if err != nil {
				return nil, serr.K8sNotFoundOrInternalWithErr(err)
			}
		}
	} else {
		if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
			Uid:  req.Status.TemplateRef.Uid,
			Name: req.Status.TemplateRef.Name,
		}, &apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
			return nil, err
		}

		template, err = s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
			Uid:  req.Status.TemplateRef.Uid,
			Name: req.Status.TemplateRef.Name,
		})
		if err != nil {
			return nil, serr.K8sNotFoundOrInternalWithErr(err)
		}
	}

	ws.Status.TemplateRef = umetav1.GetObjectReference(template)
	ws.Status.SpaceRef = template.Status.SpaceRef

	if ws.Status.SpaceRef == nil {
		return nil, grpcutils.InvalidArg("Workspace must belong to a Space")
	}

	if ws.Status.TemplateRef == nil {
		return nil, grpcutils.InvalidArg("Workspace must belong to a Template")
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.SpaceRef.Uid,
	})
	if err != nil {
		return nil, err
	}

	ws.Status.SpaceType = org.Status.Type

	if err := s.validateWorkspace(ctx, ws); err != nil {
		return nil, err
	}

	if err := commonw.CheckIsMember(ctx, s.octeliumC, ws.Status.SpaceRef); err != nil {
		return nil, err
	}

	if err := s.setWorkspaceLimit(ctx, ws, org, cc); err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	ws, err = s.octeliumC.CordiumC().CreateWorkspace(ctx, ws)
	if err != nil {
		return nil, err
	}

	return ws, nil
}

func (s *Server) setWorkspaceLimit(ctx context.Context, ws *cordiumv1.Workspace, org *cordiumv1.Space,
	cc *cordiumv1.ClusterConfig) error {

	var err error

	var prj *cordiumv1.Template

	ws.Status.Limit = &cordiumv1.Workspace_Status_Limit{}

	if ws.Status.IsBuild && cc.Spec.Workspace != nil &&
		cc.Spec.Workspace.Limit != nil &&
		cc.Spec.Workspace.Limit.Resource != nil &&
		cc.Spec.Workspace.Limit.Resource.BuildLimit != nil {
		limit := cc.Spec.Workspace.Limit.Resource.BuildLimit

		if limit.Cpu != nil && limit.Cpu.Millicores != 0 {
			ws.Status.Limit.Cpu = &cordiumv1.Workspace_Status_Limit_CPU{
				Millicores: limit.Cpu.Millicores,
			}
		}

		if limit.Memory != nil && limit.Memory.Megabytes != 0 {
			ws.Status.Limit.Memory = &cordiumv1.Workspace_Status_Limit_Memory{
				Megabytes: limit.Memory.Megabytes,
			}
		}

		if limit.Storage != nil && limit.Storage.Megabytes != 0 {
			ws.Status.Limit.Storage = &cordiumv1.Workspace_Status_Limit_Storage{
				Megabytes: limit.Storage.Megabytes,
			}
		}

		if ws.Status.Limit.Cpu == nil || ws.Status.Limit.Cpu.Millicores <= 0 {
			ws.Status.Limit.Cpu = &cordiumv1.Workspace_Status_Limit_CPU{
				Millicores: 2000,
			}
		}

		if ws.Status.Limit.Memory == nil || ws.Status.Limit.Memory.Megabytes <= 0 {
			ws.Status.Limit.Memory = &cordiumv1.Workspace_Status_Limit_Memory{
				Megabytes: 6000,
			}
		}

		if ws.Status.Limit.Storage == nil || ws.Status.Limit.Storage.Megabytes <= 0 {
			ws.Status.Limit.Storage = &cordiumv1.Workspace_Status_Limit_Storage{
				Megabytes: 20000,
			}
		}

		return nil
	} else if ws.Status.TemplateRef != nil {
		prj, err = s.octeliumC.CordiumC().GetTemplate(ctx,
			apivalidation.ObjectReferenceToRGetOptions(ws.Status.TemplateRef))
		if err != nil {
			return err
		}

		if prj.Spec.Limit != nil {
			if prj.Spec.Limit.Cpu != nil && prj.Spec.Limit.Cpu.Millicores != 0 {
				ws.Status.Limit.Cpu = prj.Spec.Limit.Cpu
			}

			if prj.Spec.Limit.Memory != nil && prj.Spec.Limit.Memory.Megabytes != 0 {
				ws.Status.Limit.Memory = prj.Spec.Limit.Memory
			}

			if prj.Spec.Limit.Storage != nil && prj.Spec.Limit.Storage.Megabytes != 0 {
				ws.Status.Limit.Storage = prj.Spec.Limit.Storage
			}

		}
	} else if org.Status.Type == cordiumv1.Space_Status_ORGANIZATION &&
		org.Spec.Limit != nil && org.Spec.Limit.DefaultLimit != nil {
		if org.Spec.Limit.DefaultLimit.Cpu != nil && org.Spec.Limit.DefaultLimit.Cpu.Millicores != 0 {
			ws.Status.Limit.Cpu = org.Spec.Limit.DefaultLimit.Cpu
		}

		if org.Spec.Limit.DefaultLimit.Memory != nil && org.Spec.Limit.DefaultLimit.Memory.Megabytes != 0 {
			ws.Status.Limit.Memory = org.Spec.Limit.DefaultLimit.Memory
		}

		if org.Spec.Limit.DefaultLimit.Storage != nil && org.Spec.Limit.DefaultLimit.Storage.Megabytes != 0 {
			ws.Status.Limit.Storage = org.Spec.Limit.DefaultLimit.Storage
		}

	}

	doCCLimit := func(limit *cordiumv1.ClusterConfig_Spec_Workspace_Limit_Resource_Limit) {
		if ws.Status.Limit.Cpu == nil && limit.Cpu != nil && limit.Cpu.Millicores != 0 {
			ws.Status.Limit.Cpu = &cordiumv1.Workspace_Status_Limit_CPU{
				Millicores: limit.Cpu.Millicores,
			}
		}

		if ws.Status.Limit.Memory == nil && limit.Memory != nil && limit.Memory.Megabytes != 0 {
			ws.Status.Limit.Memory = &cordiumv1.Workspace_Status_Limit_Memory{
				Megabytes: limit.Memory.Megabytes,
			}
		}

		if ws.Status.Limit.Storage == nil && limit.Storage != nil && limit.Storage.Megabytes != 0 {
			ws.Status.Limit.Storage = &cordiumv1.Workspace_Status_Limit_Storage{
				Megabytes: limit.Storage.Megabytes,
			}
		}
	}

	switch org.Status.Type {
	case cordiumv1.Space_Status_USER:
		if cc.Spec.Workspace != nil &&
			cc.Spec.Workspace.Limit != nil &&
			cc.Spec.Workspace.Limit.Resource != nil &&
			cc.Spec.Workspace.Limit.Resource.DefaultPersonalSpaceLimit != nil {
			doCCLimit(cc.Spec.Workspace.Limit.Resource.DefaultPersonalSpaceLimit)
		}
	case cordiumv1.Space_Status_ORGANIZATION:
		if cc.Spec.Workspace != nil &&
			cc.Spec.Workspace.Limit != nil &&
			cc.Spec.Workspace.Limit.Resource != nil &&
			cc.Spec.Workspace.Limit.Resource.DefaultOrganizationSpaceLimit != nil {
			doCCLimit(cc.Spec.Workspace.Limit.Resource.DefaultOrganizationSpaceLimit)
		}
	default:
		return grpcutils.InvalidArg("Unset Space type")
	}

	if ws.Status.Limit.Cpu == nil || ws.Status.Limit.Cpu.Millicores <= 0 {
		ws.Status.Limit.Cpu = &cordiumv1.Workspace_Status_Limit_CPU{
			Millicores: 2000,
		}
	}

	if ws.Status.Limit.Memory == nil || ws.Status.Limit.Memory.Megabytes <= 0 {
		ws.Status.Limit.Memory = &cordiumv1.Workspace_Status_Limit_Memory{
			Megabytes: 6000,
		}
	}

	if ws.Status.Limit.Storage == nil || ws.Status.Limit.Storage.Megabytes <= 0 {
		ws.Status.Limit.Storage = &cordiumv1.Workspace_Status_Limit_Storage{
			Megabytes: 20000,
		}
	}

	if org.Spec.Limit != nil && org.Spec.Limit.MaxLimit != nil {
		if org.Spec.Limit.MaxLimit.Cpu != nil && org.Spec.Limit.MaxLimit.Cpu.Millicores != 0 &&
			ws.Status.Limit.Cpu.Millicores > org.Spec.Limit.MaxLimit.Cpu.Millicores {
			ws.Status.Limit.Cpu.Millicores = org.Spec.Limit.MaxLimit.Cpu.Millicores
		}

		if org.Spec.Limit.MaxLimit.Memory != nil && org.Spec.Limit.MaxLimit.Memory.Megabytes != 0 &&
			ws.Status.Limit.Memory.Megabytes > org.Spec.Limit.MaxLimit.Memory.Megabytes {
			ws.Status.Limit.Memory.Megabytes = org.Spec.Limit.MaxLimit.Memory.Megabytes
		}

		if org.Spec.Limit.MaxLimit.Storage != nil && org.Spec.Limit.MaxLimit.Storage.Megabytes != 0 &&
			ws.Status.Limit.Storage.Megabytes > org.Spec.Limit.MaxLimit.Storage.Megabytes {
			ws.Status.Limit.Storage.Megabytes = org.Spec.Limit.MaxLimit.Storage.Megabytes
		}

	}

	if cc.Spec.Workspace != nil && cc.Spec.Workspace.Limit != nil &&
		cc.Spec.Workspace.Limit.Resource != nil &&
		cc.Spec.Workspace.Limit.Resource.MaxLimit != nil {
		max := cc.Spec.Workspace.Limit.Resource.MaxLimit
		if max.Cpu != nil && max.Cpu.Millicores != 0 &&
			ws.Status.Limit.Cpu.Millicores > max.Cpu.Millicores {
			ws.Status.Limit.Cpu.Millicores = max.Cpu.Millicores
		}

		if max.Memory != nil && max.Memory.Megabytes != 0 &&
			ws.Status.Limit.Memory.Megabytes > max.Memory.Megabytes {
			ws.Status.Limit.Memory.Megabytes = max.Memory.Megabytes
		}

		if max.Storage != nil && max.Storage.Megabytes != 0 &&
			ws.Status.Limit.Storage.Megabytes > max.Storage.Megabytes {
			ws.Status.Limit.Storage.Megabytes = max.Storage.Megabytes
		}

	}

	return nil
}

func (s *Server) checkIfCanStartWorkspace(ctx context.Context, i *userctx.UserCtx, cc *cordiumv1.ClusterConfig) error {

	wsList, err := s.octeliumC.CordiumC().ListWorkspace(ctx, urscsrv.FilterByUser(i.User))
	if err != nil {
		return serr.InternalWithErr(err)
	}

	var activeWSList []*cordiumv1.Workspace
	for _, itm := range wsList.Items {
		if itm.Status.State != cordiumv1.Workspace_Status_STOPPED {
			activeWSList = append(activeWSList, itm)
		}
	}

	if len(activeWSList) >= s.getMaxActiveWorkspacesPerUser(cc) {
		return serr.Unauthorized("Maximum limit of active Workspaces exceeded")
	}

	return nil
}

func (s *Server) ListWorkspace(ctx context.Context, req *cordiumv1.ListWorkspaceOptions) (*cordiumv1.WorkspaceList, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	filters := []*rmetav1.ListOptions_Filter{
		urscsrv.FilterStatusUserUID(i.Session.Status.UserRef.Uid),
	}

	switch req.Filter.(type) {
	case *cordiumv1.ListWorkspaceOptions_TemplateRef:
		if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
			Uid:  req.GetTemplateRef().Uid,
			Name: req.GetTemplateRef().Name,
		}, &apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
			return nil, err
		}

		tmpl, err := s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
			Uid:  req.GetTemplateRef().Uid,
			Name: req.GetTemplateRef().Name,
		})
		if err != nil {
			return nil, grpcutils.K8sNotFoundOrInternalWithErr(err)
		}

		filters = append(filters, ourscsrv.FilterStatusTemplateUID(tmpl.Metadata.Uid))

	case *cordiumv1.ListWorkspaceOptions_SpaceRef:
		if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
			Uid:  req.GetSpaceRef().Uid,
			Name: req.GetSpaceRef().Name,
		}, &apivalidation.CheckGetOptionsOpts{
			ParentsMust: 1,
		}); err != nil {
			return nil, err
		}

		if _, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
			Uid:  req.GetSpaceRef().Uid,
			Name: req.GetSpaceRef().Name,
		}); err != nil {
			return nil, grpcutils.K8sNotFoundOrInternalWithErr(err)
		}

		filters = append(filters, ourscsrv.FilterStatusSpaceUID(req.GetSpaceRef().Uid))
	default:

	}

	wsList, err := s.octeliumC.CordiumC().ListWorkspace(ctx, urscsrv.GetUserPublicListOptions(req, filters...))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return wsList, nil
}

func (s *Server) DeleteWorkspace(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckDeleteOptions(req, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if ws.Status.UserRef == nil {
		return nil, serr.InvalidArg("Workspace not owned by the User")
	}

	if ws.Status.UserRef.Uid != i.Session.Status.UserRef.Uid {
		return nil, serr.Unauthorized("Workspace is not owned by this User")
	}

	if _, err := s.octeliumC.CordiumC().DeleteWorkspace(ctx, &rmetav1.DeleteOptions{Uid: ws.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) StartWorkspace(ctx context.Context, req *cordiumv1.StartWorkspaceRequest) (*cordiumv1.StartWorkspaceResponse, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	cco, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.checkIfCanStartWorkspace(ctx, i, cco); err != nil {
		return nil, err
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if ws.Status.UserRef.Uid != i.Session.Status.UserRef.Uid {
		return nil, serr.Unauthorized("Workspace not owned by the User")
	}

	spc, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.SpaceRef.Uid,
	})
	if err != nil {
		return nil, grpcutils.K8sNotFoundOrInternalWithErr(err)
	}

	if !ucordiumv1.ToWorkspace(ws).IsStopped() {
		return nil, serr.InvalidArg("Workspace cannot be started as it is not stopped")
	}

	region, err := s.chooseRegion(ctx, ws, req.RegionRef)
	if err != nil {
		return nil, err
	}

	cc, err := s.octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	nowRFC3339 := pbutils.Now()

	ws.Status.State = cordiumv1.Workspace_Status_INIT_REQUEST
	ws.Status.CurrentStateSetAt = nowRFC3339
	ws.Status.LastInitializedAt = ws.Status.CurrentStateSetAt

	ws.Status.LastStoppingReason = ws.Status.StoppingReason
	ws.Status.StoppingReason = cordiumv1.Workspace_Status_STOPPING_REASON_UNSET

	ws.Status.RegionRef = umetav1.GetObjectReference(region)
	ws.Status.LastActivityAt = nowRFC3339
	ws.Status.Hostname = commonw.GetWorkspaceHostname(ws.Metadata.Name, region, cc)

	maxRuns := 1000

	if len(ws.Status.Runs) >= maxRuns {
		ws.Status.Runs = ws.Status.Runs[:maxRuns-2]
	}

	ws.Status.Runs = append([]*cordiumv1.Workspace_Status_Run{{
		Id:            utilrand.GetRandomStringCanonical(6),
		InitializedAt: ws.Status.LastInitializedAt,
	}}, ws.Status.Runs...)

	// run := ucordiumv1.ToWorkspace(ws).GetCurrentRun()

	/*
		if req.FromRunID != "" {
			if fromRun := ucordiumv1.ToWorkspace(ws).GeRunByID(req.FromRunID); fromRun != nil {
				run.FromID = fromRun.Id
			} else {
				return nil, grpcutils.InvalidArg("This run ID does not exit: %s", req.FromRunID)
			}
		} else if req.FromRunTag != "" {
			if fromRun := ucordiumv1.ToWorkspace(ws).GetRunByTag(req.FromRunTag); fromRun != nil {
				run.FromID = fromRun.Id
			} else {
				return nil, grpcutils.InvalidArg("This run ID does not exit: %s", req.FromRunID)
			}
		}
	*/

	if err := s.setWorkspaceLimit(ctx, ws, spc, cco); err != nil {
		return nil, err
	}

	/*
		if err := s.setWorkspaceStorage(ctx, ws); err != nil {
			return nil, grpcutils.InternalWithErr(err)
		}
	*/

	wsSession, err := s.createWorkspaceSession(ctx, i, ws)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	ws.Status.SessionRef = umetav1.GetObjectReference(wsSession)

	_, err = s.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &cordiumv1.StartWorkspaceResponse{}, nil
}

func (s *Server) createWorkspaceSession(ctx context.Context, i *userctx.UserCtx, ws *cordiumv1.Workspace) (*corev1.Session, error) {

	usr, err := s.octeliumC.CoreC().GetUser(ctx, &rmetav1.GetOptions{Uid: i.User.Metadata.Uid})
	if err != nil {
		return nil, err
	}
	sess, err := sessionc.NewSession(ctx,
		&sessionc.CreateSessionOpts{

			Usr:       usr,
			OcteliumC: s.octeliumC,
			SessType:  corev1.Session_Status_CLIENT,
			// ParentSession: umetav1.GetObjectReference(i.Session),
			ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
		})
	if err != nil {
		return nil, err
	}

	if sess.Status.Ext == nil {
		sess.Status.Ext = make(map[string]*structpb.Struct)
	}

	extInfo := &cordiumv1.SessionExtInfo{
		WorkspaceRef: umetav1.GetObjectReference(ws),
		SpaceRef:     ws.Status.SpaceRef,
		TemplateRef:  ws.Status.TemplateRef,
		SpaceType:    ws.Status.SpaceType,
	}
	extInfoStruct, err := pbutils.MessageToStruct(extInfo)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}
	sess.Status.Ext[ovutils.ExtInfoLabel] = extInfoStruct
	// sess.Status.Authentication = &corev1.Session_Status_Authentication{}

	sess, err = s.octeliumC.CoreC().CreateSession(ctx, sess)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return sess, nil
}

func (s *Server) StopWorkspace(ctx context.Context, req *cordiumv1.StopWorkspaceRequest) (*cordiumv1.StopWorkspaceResponse, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx,
		&rmetav1.GetOptions{
			Uid:  req.Uid,
			Name: req.Name,
		})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if ws.Status.UserRef == nil {
		return nil, serr.Unauthorized("Workspace does not belong to a User")
	}

	if ws.Status.UserRef.Uid != i.Session.Status.UserRef.Uid {
		return nil, serr.Unauthorized("Workspace not owned by the User")
	}

	if ucordiumv1.ToWorkspace(ws).IsStopped() {
		return nil, serr.InvalidArg("Workspace is already stopped")
	}

	if ucordiumv1.ToWorkspace(ws).IsStopping() {
		return &cordiumv1.StopWorkspaceResponse{}, nil
	}

	if len(req.Tags) > 0 {
		if len(req.Tags) > 10 {
			return nil, grpcutils.InvalidArg("Too many tags")
		}

		checkTag := func(arg string) error {
			if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
				Name: arg,
			}, nil); err != nil {
				return grpcutils.InvalidArg("Invalid tag: %s", arg)
			}

			if len(arg) > 16 {
				return grpcutils.InvalidArg("Tag: %s is too long", arg)
			}

			return nil
		}

		for _, tag := range req.Tags {
			if err := checkTag(tag); err != nil {
				return nil, err
			}
		}
	}

	// run := ucordiumv1.ToWorkspace(ws).GetCurrentRun()
	/*
		run.IsEphemeral = req.IsEphemeral
		run.Tags = req.Tags
	*/

	ws.Status.State = cordiumv1.Workspace_Status_STOPPING_REQUEST
	ws.Status.StoppingReason = cordiumv1.Workspace_Status_STOPPING_REASON_API

	_, err = s.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &cordiumv1.StopWorkspaceResponse{}, nil
}

func (s *Server) UpdateWorkspace(ctx context.Context, req *cordiumv1.Workspace) (*cordiumv1.Workspace, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(req, &apivalidation.ValidateCommonOpts{
		ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{},
	}); err != nil {
		return nil, err
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: req.Metadata.Uid})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if ws.Status.UserRef == nil {
		return nil, serr.Unauthorized("Workspace does not belong to a User")
	}

	if ws.Status.UserRef.Uid != i.Session.Status.UserRef.Uid {
		return nil, serr.Unauthorized("Workspace not owned by the User")
	}

	ws.Metadata.DisplayName = req.Metadata.DisplayName
	ws.Spec = req.Spec

	if err := s.validateWorkspace(ctx, ws); err != nil {
		return nil, err
	}

	ret, err := s.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *Server) validateWorkspace(ctx context.Context, ws *cordiumv1.Workspace) error {

	req := &cordiumv1.Workspace{
		Metadata: ws.Metadata,
		Spec:     ws.Spec,
		Status:   ws.Status,
	}
	if err := s.validateAndSetWorkspace(ctx, req); err != nil {
		return err
	}

	return nil
}

func (s *Server) GetWorkspace(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Workspace, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(req, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if item.Status.UserRef != nil && item.Status.UserRef.Uid == i.User.Metadata.Uid {
		return item, nil
	}

	return nil, grpcutils.Unauthorized("This Workspace is not owned by the User")
}

func (s *Server) ShareWorkspacePort(ctx context.Context, req *cordiumv1.ShareWorkspacePortRequest) (*cordiumv1.ShareWorkspacePortResponse, error) {

	if req.Mode == cordiumv1.ShareWorkspacePortRequest_UNSET {
		return nil, grpcutils.InvalidArg("Mode cannot be UNSET")
	}

	if req.ApplicationName == "" {
		return nil, grpcutils.InvalidArg("Application name must be set")
	}

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	ws, err := s.GetWorkspace(ctx, &metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	app := ucordiumv1.ToWorkspace(ws).GetApplicationByName(req.ApplicationName)
	if app == nil {
		return nil, grpcutils.InvalidArg("The application: %s does not exist", req.ApplicationName)
	}

	var isFound bool
	for _, sharedPort := range ws.Status.SharedPorts {
		if sharedPort.ApplicationName == app.Name {
			isFound = true
			sharedPort.Mode = cordiumv1.Workspace_Status_SharedPort_Mode(req.Mode)
		}
	}

	if !isFound {
		sharedPort := &cordiumv1.Workspace_Status_SharedPort{
			ApplicationName: app.Name,
			Mode:            cordiumv1.Workspace_Status_SharedPort_Mode(req.Mode),
		}

		ws.Status.SharedPorts = append(ws.Status.SharedPorts, sharedPort)
	}

	_, err = s.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	return &cordiumv1.ShareWorkspacePortResponse{}, nil
}

func (s *Server) UnshareWorkspacePort(ctx context.Context, req *cordiumv1.UnshareWorkspacePortRequest) (*cordiumv1.UnshareWorkspacePortResponse, error) {

	if req.ApplicationName == "" {
		return nil, grpcutils.InvalidArg("Application name must be set")
	}

	if err := apivalidation.CheckGetOptions(&metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	}, &apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	ws, err := s.GetWorkspace(ctx, &metav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	var requiresUpdate bool
	sps := ws.Status.SharedPorts
	for idx, port := range sps {
		if port.ApplicationName == req.ApplicationName {
			sps = append(sps[:idx], sps[idx+1:]...)
			requiresUpdate = true
			break
		}
	}
	ws.Status.SharedPorts = sps
	if requiresUpdate {
		_, err = s.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
		if err != nil {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	return &cordiumv1.UnshareWorkspacePortResponse{}, nil
}

func (s *Server) getMaxWorkspacesPerUser(cc *cordiumv1.ClusterConfig) int {

	if cc.Spec.Workspace != nil && cc.Spec.Workspace.Limit != nil &&
		cc.Spec.Workspace.Limit.MaxPerUser != 0 && cc.Spec.Workspace.Limit.MaxPerUser < 1000000 {
		return int(cc.Spec.Workspace.Limit.MaxPerUser)
	}

	return 1000
}

func (s *Server) getMaxActiveWorkspacesPerUser(cc *cordiumv1.ClusterConfig) int {

	if cc.Spec.Workspace != nil && cc.Spec.Workspace.Limit != nil &&
		cc.Spec.Workspace.Limit.MaxActivePerUser != 0 && cc.Spec.Workspace.Limit.MaxActivePerUser < 1000000 {
		return int(cc.Spec.Workspace.Limit.MaxActivePerUser)
	}

	return 32
}

func (s *Server) genWorkspaceName(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for i := 0; i < 10000; i++ {
		var name string

		if i < 50 {
			name = utilrand.GetRandomStringCanonical(3)
		} else if i < 500 {
			name = utilrand.GetRandomStringCanonical(4)
		} else if i < 1000 {
			name = utilrand.GetRandomStringCanonical(5)
		} else {
			name = utilrand.GetRandomStringCanonical(6)
		}

		if _, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
			Name: name,
		}); err != nil {

			if grpcerr.IsNotFound(err) {
				return name, nil
			}

			return "", grpcutils.InternalWithErr(err)
		}
	}

	return "", grpcutils.Internal("Could not generate Workspace name")
}

func (s *Server) setWorkspaceStorage(ctx context.Context, ws *cordiumv1.Workspace) error {

	run := ucordiumv1.ToWorkspace(ws).GetCurrentRun()
	if run == nil {
		return grpcutils.InvalidArg("No current run")
	}

	/*
		if run.FromID != "" {
			if fromRun := ucordiumv1.ToWorkspace(ws).GeRunByID(run.FromID); fromRun != nil && fromRun.CloudProviderRef != nil {
				if _, err := s.octeliumC.CordiumC().GetCloudProvider(ctx, &rmetav1.GetOptions{
					Uid: fromRun.CloudProviderRef.Uid,
				}); err == nil {
					run.CloudProviderRef = fromRun.CloudProviderRef
					return nil
				}
			}
		}

		if fromRun := ucordiumv1.ToWorkspace(ws).GetRunByTag("latest"); fromRun != nil && fromRun.CloudProviderRef != nil {
			if _, err := s.octeliumC.CordiumC().GetCloudProvider(ctx, &rmetav1.GetOptions{
				Uid: fromRun.CloudProviderRef.Uid,
			}); err == nil {
				run.CloudProviderRef = fromRun.CloudProviderRef
				return nil
			}
		}
	*/

	/*
		cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
		if err != nil {
			return err
		}
	*/

	/*
		if cc.Spec.Workspace != nil && cc.Spec.Workspace.Storage.GetExternalContainerRegistry() != nil &&
			cc.Spec.Workspace.Storage.GetExternalContainerRegistry().CloudProvider != "" {

			if provider, err := s.octeliumC.CordiumC().GetCloudProvider(ctx, &rmetav1.GetOptions{
				Name: cc.Spec.Workspace.Storage.GetExternalContainerRegistry().CloudProvider,
			}); err == nil {
				run.CloudProviderRef = umetav1.GetObjectReference(provider)
				return nil
			}
		}
	*/

	/*
		provider, err := s.octeliumC.CordiumC().GetCloudProvider(ctx, &rmetav1.GetOptions{
			Name: "sys:internal-registry",
		})
		if err != nil {
			return grpcutils.InternalWithErr(err)
		}

		run.CloudProviderRef = umetav1.GetObjectReference(provider)
	*/

	return nil
}
