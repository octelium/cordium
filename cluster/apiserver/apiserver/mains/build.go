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

	"slices"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
)

func (s *Server) BuildTemplate(ctx context.Context, req *cordiumv1.BuildTemplateRequest) (*cordiumv1.Template, error) {

	if req.TemplateRef == nil {
		return nil, grpcutils.InvalidArg("TemplateRef not defined")
	}

	if err := apivalidation.CheckGetOptions(
		getFullGetOptionsSpaceChild(ctx, apivalidation.ObjectReferenceToGetOptions(req.TemplateRef)),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
		return nil, err
	}

	if len(req.Tags) == 0 {
		req.Tags = []string{"latest"}
	} else {
		if len(req.Tags) > 10 {
			return nil, grpcutils.InvalidArg("Too many tags")
		}
		for _, tag := range req.Tags {
			if err := apivalidation.ValidateGenASCII(tag); err != nil {
				return nil, grpcutils.InvalidArg("Invalid Tag")
			}
			if len(tag) < 2 || len(tag) > 16 {
				return nil, grpcutils.InvalidArg("Invalid Tag")
			}
		}
	}

	tmpl, err := s.octeliumC.CordiumC().GetTemplate(ctx, apivalidation.ObjectReferenceToRGetOptions(req.TemplateRef))
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if err := commonw.CheckIsMemberAdmin(ctx, s.octeliumC, tmpl.Status.SpaceRef); err != nil {
		return nil, err
	}

	prebuild, err := s.getOrCreateBuild(ctx, req, tmpl)
	if err != nil {
		return nil, err
	}

	return prebuild, nil
}

func (s *Server) CancelBuildTemplate(ctx context.Context, req *cordiumv1.CancelBuildTemplateRequest) (*cordiumv1.Template, error) {
	if req.TemplateRef == nil {
		return nil, grpcutils.InvalidArg("TemplateRef not defined")
	}

	if err := apivalidation.CheckGetOptions(
		getFullGetOptionsSpaceChild(ctx, apivalidation.ObjectReferenceToGetOptions(req.TemplateRef)),
		&apivalidation.CheckGetOptionsOpts{
			ParentsMust: 2,
		}); err != nil {
		return nil, err
	}

	tmpl, err := s.GetTemplate(ctx, apivalidation.ObjectReferenceToGetOptions(req.TemplateRef))
	if err != nil {
		return nil, err
	}

	if tmpl.Status.BuildInfo.CurrentRunningBuildID == "" {
		return tmpl, nil
	}

	if err := s.doCancelBuild(ctx, tmpl); err != nil {
		return nil, err
	}

	tmpl, err = s.octeliumC.CordiumC().UpdateTemplate(ctx, tmpl)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return tmpl, nil
}

func (s *Server) doCancelBuild(ctx context.Context, tmpl *cordiumv1.Template) error {
	if tmpl.Status.BuildInfo.CurrentRunningBuildID == "" {
		return nil
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Name: fmt.Sprintf("build-%s-%s", tmpl.Metadata.Uid, tmpl.Status.BuildInfo.CurrentRunningBuildID),
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return grpcutils.InternalWithErr(err)
	}

	ws.Status.State = cordiumv1.Workspace_Status_STOPPING_REQUEST
	ws.Status.StoppingReason = cordiumv1.Workspace_Status_STOPPING_REASON_API

	if tmpl.Status.BuildInfo != nil {
		if idx := slices.IndexFunc(tmpl.Status.BuildInfo.Builds, func(t *cordiumv1.Template_Status_BuildInfo_Build) bool {
			return t.Id == tmpl.Status.BuildInfo.CurrentRunningBuildID
		}); idx >= 0 {
			bld := tmpl.Status.BuildInfo.Builds[idx]
			bld.IsCanceled = true
			bld.State = cordiumv1.Template_Status_BuildInfo_Build_STATE_FAILED
			bld.DoneAt = pbutils.Now()
		}
	}

	tmpl.Status.BuildInfo.CurrentRunningBuildID = ""

	_, err = s.octeliumC.CordiumC().UpdateWorkspace(ctx, ws)
	if err != nil {
		return serr.InternalWithErr(err)
	}

	return nil
}

func (s *Server) getOrCreateBuild(ctx context.Context, req *cordiumv1.BuildTemplateRequest, tmpl *cordiumv1.Template) (*cordiumv1.Template, error) {

	var err error
	if err := s.doCancelBuild(ctx, tmpl); err != nil {
		return nil, err
	}

	if err := s.createBuildRun(ctx, req, tmpl); err != nil {
		return nil, err
	}

	tmpl, err = s.octeliumC.CordiumC().UpdateTemplate(ctx, tmpl)
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	return tmpl, nil
}

func (s *Server) createBuildRun(ctx context.Context, req *cordiumv1.BuildTemplateRequest, tmpl *cordiumv1.Template) error {

	maxLen := 1000
	runID := utilrand.GetRandomStringCanonical(6)

	if tmpl.Status.BuildInfo == nil {
		tmpl.Status.BuildInfo = &cordiumv1.Template_Status_BuildInfo{}
	}

	if len(tmpl.Status.BuildInfo.Builds) >= maxLen {
		tmpl.Status.BuildInfo.Builds = tmpl.Status.BuildInfo.Builds[:maxLen-2]
	}

	tmpl.Status.BuildInfo.Builds = append([]*cordiumv1.Template_Status_BuildInfo_Build{{
		StartedAt: pbutils.Now(),
		Id:        runID,
		State:     cordiumv1.Template_Status_BuildInfo_Build_STATE_RUNNING,
		Tags:      req.Tags,
	}}, tmpl.Status.BuildInfo.Builds...)

	tmpl.Status.BuildInfo.CurrentRunningBuildID = runID

	cco, err := s.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return err
	}

	if err := s.checkIfCanStartWorkspace(ctx, i, cco); err != nil {
		return err
	}

	now := pbutils.Now()

	wsReq := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name:           fmt.Sprintf("build-%s-%s", tmpl.Metadata.Uid, runID),
			IsSystemHidden: true,
			IsUserHidden:   true,
			SystemLabels: map[string]string{
				"build-id": runID,
			},
		},
		Spec: &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{
			State:             cordiumv1.Workspace_Status_INIT_REQUEST,
			CurrentStateSetAt: now,
			LastInitializedAt: now,
			IsBuild:           true,

			TemplateRef: umetav1.GetObjectReference(tmpl),
			SpaceRef:    tmpl.Status.SpaceRef,
			Run: &cordiumv1.Workspace_Status_Run{
				Id:            utilrand.GetRandomStringCanonical(6),
				InitializedAt: now,
			},
		},
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: tmpl.Status.SpaceRef.Uid,
	})
	if err != nil {
		return err
	}

	if err := s.setWorkspaceLimit(ctx, wsReq, org, cco); err != nil {
		return grpcutils.InternalWithErr(err)
	}

	region, err := s.chooseRegion(ctx, wsReq, nil)
	if err != nil {
		return err
	}
	cc, err := s.octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return serr.InternalWithErr(err)
	}

	wsReq.Status.RegionRef = umetav1.GetObjectReference(region)
	wsReq.Status.Hostname = commonw.GetWorkspaceHostname(wsReq.Metadata.Name, region, cc)

	_, err = s.octeliumC.CordiumC().CreateWorkspace(ctx, wsReq)
	if err != nil {
		return serr.InternalWithErr(err)
	}

	return nil
}
