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

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/celengine"

	wswatchers "github.com/octelium/cordium/cluster/common/watchers"
)

type Server struct {
	octeliumC octeliumc.ClientInterface
	cordiumv1.UnimplementedMainServiceServer
	celEngine  *celengine.CELEngine
	wsWatchMan *watchWorkspaceSubscriptionManager
}

func NewServer(ctx context.Context, octeliumC octeliumc.ClientInterface) (*Server, error) {

	celEngine, err := celengine.New(ctx, &celengine.Opts{})
	if err != nil {
		return nil, err
	}

	return &Server{
		octeliumC: octeliumC,
		celEngine: celEngine,
		wsWatchMan: &watchWorkspaceSubscriptionManager{
			mp: make(map[string]*watchWorkspaceSubscription),
		},
	}, nil
}

func (s *Server) Run(ctx context.Context) error {

	ctl := s.wsWatchMan
	if err := wswatchers.NewCordiumV1(s.octeliumC).Workspace(ctx, nil,
		ctl.onCreate, ctl.onUpdate, ctl.onDelete); err != nil {
		return err
	}

	return nil
}
