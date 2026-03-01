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
