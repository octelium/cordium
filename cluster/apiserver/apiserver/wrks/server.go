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

package wrks

import (
	"context"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/cluster/common/watchers"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/celengine"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
)

type Server struct {
	octeliumC octeliumc.ClientInterface
	cordiumv1.UnimplementedWorkspaceServiceServer
	celEngine *celengine.CELEngine
	regionRef *metav1.ObjectReference

	activityCtl  *wsutils.ActivityCtl
	wsCtl        *wsCtl
	supClientMap *suputils.SupervisorCMap
}

type wsCtl struct {
	srv *Server
}

func (c *wsCtl) onCreate(ctx context.Context, ws *cordiumv1.Workspace) error {
	return c.set(ws)
}

func (c *wsCtl) onUpdate(ctx context.Context, new, old *cordiumv1.Workspace) error {
	if ucordiumv1.ToWorkspace(new).IsStoppingOrStopped() && !ucordiumv1.ToWorkspace(old).IsStoppingOrStopped() {
		return c.srv.supClientMap.Remove(new)
	}
	return c.set(new)
}

func (c *wsCtl) onDelete(ctx context.Context, ws *cordiumv1.Workspace) error {
	return c.srv.supClientMap.Remove(ws)
}

func (c *wsCtl) shouldSet(ws *cordiumv1.Workspace) bool {
	if ws.Status.RegionRef == nil {
		return false
	}
	if ws.Status.RegionRef.Uid != c.srv.regionRef.Uid {
		return false
	}
	if ws.Status.UserRef == nil {
		return false
	}
	return ucordiumv1.ToWorkspace(ws).IsActive()
}

func (c *wsCtl) set(ws *cordiumv1.Workspace) error {
	if !c.shouldSet(ws) {
		return nil
	}
	return c.srv.supClientMap.Set(ws)
}

func NewServer(ctx context.Context, octeliumC octeliumc.ClientInterface) (*Server, error) {

	celEngine, err := celengine.New(ctx, &celengine.Opts{})
	if err != nil {
		return nil, err
	}

	region, err := octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
		Name: vutils.GetMyRegionName(),
	})
	if err != nil {
		return nil, err
	}

	activityCtl, err := wsutils.NewActivityCtl(octeliumC)
	if err != nil {
		return nil, err
	}

	ret := &Server{
		octeliumC:   octeliumC,
		celEngine:   celEngine,
		regionRef:   umetav1.GetObjectReference(region),
		activityCtl: activityCtl,
		wsCtl:       &wsCtl{},
	}

	ret.wsCtl.srv = ret
	ret.supClientMap = suputils.NewSupervisorCtxMap(umetav1.GetObjectReference(region))

	return ret, nil
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.activityCtl.Run(ctx); err != nil {
		return err
	}

	if err := watchers.NewCordiumV1(s.octeliumC).Workspace(ctx, nil,
		s.wsCtl.onCreate, s.wsCtl.onUpdate, s.wsCtl.onDelete,
	); err != nil {
		return err
	}

	return nil
}
