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

package supervisor

import (
	"context"

	"github.com/octelium/cordium/cluster/common/gitutils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"go.uber.org/zap"
)

type cloneRepoWriter struct {
	p *eventPublisher
}

func (w *cloneRepoWriter) Write(data []byte) (int, error) {
	w.p.publish(&ccordiumv1.ListenEventResponse{
		Type: &ccordiumv1.ListenEventResponse_ListenLogResponse{
			ListenLogResponse: &cordiumv1.ListenLogResponse{
				Type: cordiumv1.ListenLogResponse_TYPE_CLONING_REPO,
				Mode: cordiumv1.ListenLogResponse_MODE_STDOUT,
				Data: data,
			},
		},
	})
	return len(data), nil
}

func (s *Server) doShallowCloneMainRepository(ctx context.Context, project *cordiumv1.Workspace_Spec_Repository) error {

	if project == nil {
		return nil
	}

	repoPath := "/octelium/workspace/repo"
	if err := gitutils.Clone(ctx, &gitutils.CloneOpts{
		Repo:       project,
		Dir:        repoPath,
		SecretList: s.initReq.SecretList,
		UserUID:    s.octeliumUID,
	}); err != nil {
		return err
	}

	zap.L().Debug("Successfully cloned main repo", zap.String("url", project.GetUrl()))

	if err := s.chownDirOctelium(ctx, repoPath); err != nil {
		return err
	}

	return nil
}

func (s *Server) shallowCloneMainRepository(ctx context.Context) error {

	if !s.isFreshRun {
		zap.L().Debug("Not first run. No need to clone main repo")
		return nil
	}

	if s.spec.Repository != nil && s.spec.Repository.Url != "" {
		// s.setStatus(ccordiumv1.GetStatusResponse_CLONING_REPO)
		if err := s.doShallowCloneMainRepository(ctx, s.spec.Repository); err != nil {
			s.setFailure(&cordiumv1.Workspace_Status_Failure{
				Type: &cordiumv1.Workspace_Status_Failure_RepoClone_{
					RepoClone: &cordiumv1.Workspace_Status_Failure_RepoClone{},
				},
			})
			return err
		}
	}

	return nil
}
