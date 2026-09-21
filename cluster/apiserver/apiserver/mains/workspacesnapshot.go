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
	"strings"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
)

const maxWorkspaceSnapshotsPerUser = 100

func (s *Server) CreateWorkspaceSnapshot(ctx context.Context,
	req *cordiumv1.WorkspaceSnapshot) (*cordiumv1.WorkspaceSnapshot, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(getFullNamResourceUserChildWithUserCtx(i, req),
		&apivalidation.ValidateCommonOpts{
			ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
				RequireName: true,
				ParentsMust: 1,
			},
		}); err != nil {
		return nil, err
	}

	nameArgs := strings.Split(req.Metadata.Name, ".")
	if len(nameArgs) != 2 {
		return nil, serr.InvalidArg("Invalid name: %s", req.Metadata.Name)
	}
	if nameArgs[1] != i.User.Metadata.Name {
		return nil, serr.Unauthorized("Invalid User")
	}

	if req.Status == nil || req.Status.WorkspaceRef == nil {
		return nil, serr.InvalidArg("WorkspaceRef is not set")
	}

	if err := apivalidation.CheckObjectRef(req.Status.WorkspaceRef,
		&apivalidation.CheckGetOptionsOpts{}); err != nil {
		return nil, err
	}

	cc, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	{
		itmList, err := s.octeliumC.CordiumC().ListWorkspaceSnapshot(ctx,
			ourscsrv.SetCountOnly(urscsrv.FilterByUser(i.User)))
		if err != nil {
			return nil, serr.InternalWithErr(err)
		}
		if itmList.GetListResponseMeta().GetTotalCount() >=
			uint32(s.getMaxWorkspaceSnapshotsPerUser(cc)) {
			return nil, serr.Unauthorized("Number of WorkspaceSnapshots per User has been exceeded")
		}
	}

	{
		_, err := s.octeliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
			Name: req.Metadata.Name,
		})
		if err == nil {
			return nil, grpcutils.InvalidArg("This WorkspaceSnapshot name already exists")
		} else if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx,
		apivalidation.ObjectReferenceToRGetOptions(req.Status.WorkspaceRef))
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if ws.Status.UserRef == nil || ws.Status.UserRef.Uid != i.User.Metadata.Uid {
		return nil, serr.Unauthorized("Workspace not owned by the User")
	}

	if ws.Status.IsBuild {
		return nil, serr.InvalidArg("Build Workspaces cannot be snapshotted")
	}

	if ws.Spec.IsEphemeral && !ucordiumv1.ToWorkspace(ws).IsActive() {
		return nil, serr.InvalidArg(
			"Ephemeral Workspaces do not have a persistent storage to be snapshotted while they are stopped")
	}

	regionRef := getWorkspaceStorageRegionRef(ws)
	if regionRef == nil {
		return nil, serr.InvalidArg("The Workspace does not have a persistent storage to be snapshotted yet")
	}

	{
		itmList, err := s.octeliumC.CordiumC().ListWorkspaceSnapshot(ctx,
			ourscsrv.FilterByWorkspace(ws))
		if err != nil {
			return nil, serr.InternalWithErr(err)
		}

		for _, itm := range itmList.Items {
			if ucordiumv1.ToWorkspaceSnapshot(itm).IsCreating() {
				return nil, grpcutils.AlreadyExists(
					"The Workspace already has a WorkspaceSnapshot that is still being taken: %s",
					itm.Metadata.Name)
			}
		}
	}

	item := &cordiumv1.WorkspaceSnapshot{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec:     &cordiumv1.WorkspaceSnapshot_Spec{},
		Status: &cordiumv1.WorkspaceSnapshot_Status{
			State:        cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING,
			WorkspaceRef: umetav1.GetObjectReference(ws),
			UserRef:      umetav1.GetObjectReference(i.User),
			SpaceRef:     ws.Status.SpaceRef,
			TemplateRef:  ws.Status.TemplateRef,
			RegionRef:    regionRef,
			Consistency: func() cordiumv1.WorkspaceSnapshot_Status_Consistency {
				if ucordiumv1.ToWorkspace(ws).IsStopped() {
					return cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CLEAN
				}
				return cordiumv1.WorkspaceSnapshot_Status_CONSISTENCY_CRASH
			}(),
		},
	}

	ret, err := s.octeliumC.CordiumC().CreateWorkspaceSnapshot(ctx, item)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return ret, nil
}

func (s *Server) GetWorkspaceSnapshot(ctx context.Context,
	req *metav1.GetOptions) (*cordiumv1.WorkspaceSnapshot, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(getFullNameGetOptionsUserChild(ctx, req),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 1,
		}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if item.Status.UserRef == nil || item.Status.UserRef.Uid != i.User.Metadata.Uid {
		return nil, serr.Unauthorized("This WorkspaceSnapshot is not owned by the User")
	}

	return item, nil
}

func (s *Server) ListWorkspaceSnapshot(ctx context.Context,
	req *cordiumv1.ListWorkspaceSnapshotOptions) (*cordiumv1.WorkspaceSnapshotList, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	filters := []*rmetav1.ListOptions_Filter{
		urscsrv.FilterStatusUserUID(i.User.Metadata.Uid),
	}

	switch req.Filter.(type) {
	case *cordiumv1.ListWorkspaceSnapshotOptions_WorkspaceRef:
		if err := apivalidation.CheckObjectRef(req.GetWorkspaceRef(),
			&apivalidation.CheckGetOptionsOpts{}); err != nil {
			return nil, err
		}

		ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx,
			apivalidation.ObjectReferenceToRGetOptions(req.GetWorkspaceRef()))
		if err != nil {
			return nil, serr.K8sNotFoundOrInternalWithErr(err)
		}

		filters = append(filters, ourscsrv.FilterStatusWorkspaceUID(ws.Metadata.Uid))
	case *cordiumv1.ListWorkspaceSnapshotOptions_SpaceRef:
		org, err := s.getMemberSpaceFromSpaceRef(ctx, getFullResourceRefSpace(ctx, req.GetSpaceRef()))
		if err != nil {
			return nil, err
		}

		filters = append(filters, ourscsrv.FilterStatusSpaceUID(org.Metadata.Uid))
	default:
	}

	itmList, err := s.octeliumC.CordiumC().ListWorkspaceSnapshot(ctx,
		urscsrv.GetUserPublicListOptions(req, filters...))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return itmList, nil
}

func (s *Server) DeleteWorkspaceSnapshot(ctx context.Context,
	req *metav1.DeleteOptions) (*metav1.OperationResult, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckDeleteOptions(getFullNameDeleteOptionsUserChild(ctx, req),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 1,
		}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetWorkspaceSnapshot(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if item.Status.UserRef == nil || item.Status.UserRef.Uid != i.User.Metadata.Uid {
		return nil, serr.Unauthorized("This WorkspaceSnapshot is not owned by the User")
	}

	{
		wsList, err := s.octeliumC.CordiumC().ListWorkspace(ctx, &rmetav1.ListOptions{
			Filters: []*rmetav1.ListOptions_Filter{
				ourscsrv.FilterStatusWorkspaceSnapshotUID(item.Metadata.Uid),
			},
		})
		if err != nil {
			return nil, serr.InternalWithErr(err)
		}

		for _, ws := range wsList.Items {
			if !workspaceRestoresFromSnapshot(ws) {
				continue
			}

			if ws.Spec.IsEphemeral {
				return nil, grpcutils.InvalidArg(
					"The WorkspaceSnapshot cannot be deleted while the ephemeral Workspace: %s restores its storage from it on every run",
					ws.Metadata.Name)
			}

			return nil, grpcutils.InvalidArg(
				"The WorkspaceSnapshot cannot be deleted while the Workspace: %s that is restored from it has not run yet",
				ws.Metadata.Name)
		}
	}

	if _, err := s.octeliumC.CordiumC().DeleteWorkspaceSnapshot(ctx, &rmetav1.DeleteOptions{
		Uid: item.Metadata.Uid,
	}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) getMaxWorkspaceSnapshotsPerUser(cc *cordiumv1.ClusterConfig) int {
	if cc.Spec.Workspace != nil && cc.Spec.Workspace.Limit != nil &&
		cc.Spec.Workspace.Limit.MaxSnapshotsPerUser != 0 &&
		cc.Spec.Workspace.Limit.MaxSnapshotsPerUser < 1000000 {
		return int(cc.Spec.Workspace.Limit.MaxSnapshotsPerUser)
	}

	return maxWorkspaceSnapshotsPerUser
}

func workspaceRestoresFromSnapshot(ws *cordiumv1.Workspace) bool {
	if ws.Status.WorkspaceSnapshotRef == nil {
		return false
	}

	return ws.Spec.IsEphemeral || ws.Status.SuccessfulRuns == 0
}

func getWorkspaceStorageRegionRef(ws *cordiumv1.Workspace) *metav1.ObjectReference {
	if ws.Status.RegionRef != nil {
		return ws.Status.RegionRef
	}

	return ws.Status.LastRegionRef
}

func (s *Server) getWorkspaceSnapshotForRestore(ctx context.Context,
	ref *metav1.ObjectReference) (*cordiumv1.WorkspaceSnapshot, error) {

	snapshot, err := s.GetWorkspaceSnapshot(ctx, apivalidation.ObjectReferenceToGetOptions(ref))
	if err != nil {
		return nil, err
	}

	switch snapshot.Status.State {
	case cordiumv1.WorkspaceSnapshot_Status_STATE_READY:
	case cordiumv1.WorkspaceSnapshot_Status_STATE_CREATING:
		return nil, grpcutils.InvalidArg(
			"The WorkspaceSnapshot: %s is still being taken", snapshot.Metadata.Name)
	default:
		return nil, grpcutils.InvalidArg(
			"The WorkspaceSnapshot: %s cannot be restored from", snapshot.Metadata.Name)
	}

	return snapshot, nil
}

func (s *Server) setWorkspaceRestoreLimit(ws *cordiumv1.Workspace,
	snapshot *cordiumv1.WorkspaceSnapshot) error {

	if snapshot.Status.RestoreSizeBytes == 0 {
		return nil
	}

	restoreMegabytes := uint32((snapshot.Status.RestoreSizeBytes + 1000*1000 - 1) / (1000 * 1000))

	if ws.Status.Limit == nil {
		ws.Status.Limit = &cordiumv1.Workspace_Spec_Limit{}
	}

	if ws.Status.Limit.Storage == nil {
		ws.Status.Limit.Storage = &cordiumv1.Workspace_Spec_Limit_Storage{}
	}

	if ws.Status.Limit.Storage.Megabytes >= restoreMegabytes {
		return nil
	}

	return grpcutils.InvalidArg(
		"The WorkspaceSnapshot: %s needs at least %d MB of storage while the Workspace is limited to %d MB",
		snapshot.Metadata.Name, restoreMegabytes, ws.Status.Limit.Storage.Megabytes)
}
