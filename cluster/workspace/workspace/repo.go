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

package workspace

import (
	"context"
	"io"
	"os"
	"path"

	"github.com/octelium/cordium/cluster/common/gitutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"go.uber.org/zap"
)

func (s *Server) doShallowCloneMainRepository(ctx context.Context) error {

	if !s.isFreshRun {
		zap.L().Debug("Not a fresh run. No need to clone the main repo")
		return nil
	}

	if s.spec.Repository == nil || s.spec.Repository.Url == "" {
		return nil
	}

	repo := s.spec.Repository

	repoPath := "/workspace/repo"

	if err := os.MkdirAll(repoPath, 0700); err != nil {
		return err
	}

	if isEmpty, _ := isDirEmpty(repoPath); !isEmpty {
		return nil
	}
	if err := gitutils.Clone(ctx, &gitutils.CloneOpts{
		Dir:        repoPath,
		Repo:       repo,
		SecretList: s.initReq.SecretList,
		UserUID:    s.userInfo.uid,
	}); err != nil {
		return err
	}

	zap.L().Debug("Successfully shallow cloned main repo", zap.String("url", repo.GetUrl()))

	if err := s.chownDirToUser(ctx, repoPath); err != nil {
		return err
	}

	return nil
}

func (s *Server) completeRepoClone(ctx context.Context) error {

	if !s.isFreshRun {
		zap.L().Debug("Not a fresh run. No need to complete repo clone")
		return nil
	}

	if s.initReq.Workspace.Status.IsBuild {
		if err := s.completeFetchMainRepo(ctx); err != nil {
			return err
		}

		if err := s.cloneAdditionalRepos(ctx); err != nil {
			return err
		}

	} else {
		s.runningWG.Add(2)
		go func() {
			defer s.runningWG.Done()
			if err := s.completeFetchMainRepo(ctx); err != nil {
				zap.L().Error("Could not complete fetch main repo", zap.Error(err))
			}

		}()

		go func() {
			defer s.runningWG.Done()
			if err := s.cloneAdditionalRepos(ctx); err != nil {
				zap.L().Error("Could not clone additional repos", zap.Error(err))
			}
		}()
	}

	return nil
}

func (s *Server) completeFetchMainRepo(ctx context.Context) error {
	zap.L().Debug("Starting completeFetchMainRepo")
	if s.spec.Repository == nil || s.spec.Repository.Url == "" {
		return nil
	}

	repoPath := "/workspace/repo"

	isEmpty, _ := isDirEmpty(repoPath)
	if isEmpty {

		return nil
	}

	if err := gitutils.Unshallow(ctx, &gitutils.CloneOpts{
		Dir:        repoPath,
		Repo:       s.spec.Repository,
		SecretList: s.initReq.SecretList,
		UserUID:    s.userInfo.uid,
	}); err != nil {
		return err
	}

	zap.L().Debug("Successfully fetched main repo")

	if err := s.chownDirToUser(ctx, repoPath); err != nil {
		return err
	}

	return nil
}

func (s *Server) cloneAdditionalRepos(ctx context.Context) error {

	if !s.isFreshRun {
		zap.L().Debug("Not a fresh run. No need to clone additional projects")
		return nil
	}

	if len(s.spec.AdditionalRepositories) == 0 {
		zap.L().Debug("No additional repos. Nothing to be done")
		return nil
	}

	for _, project := range s.spec.AdditionalRepositories {
		zap.L().Debug("Cloning additional repo", zap.String("name", project.Name), zap.Any("repo", project))
		if err := s.cloneAdditionalRepositoryGit(ctx, project); err != nil {
			zap.L().Error("Could not clone additional repo", zap.String("name", project.Name))
		}
	}

	return nil
}

func (s *Server) cloneAdditionalRepositoryGit(ctx context.Context,
	repo *cordiumv1.Workspace_Spec_AdditionalRepository) error {

	repoPath := path.Join("/workspace/additional-repos", repo.Name)

	if err := os.MkdirAll(repoPath, 0700); err != nil {
		return err
	}

	if err := gitutils.Clone(ctx, &gitutils.CloneOpts{
		Dir:        repoPath,
		Repo:       repo.Repository,
		SecretList: s.initReq.SecretList,
		UserUID:    s.userInfo.uid,
	}); err != nil {
		return err
	}

	if err := s.chownDirToUser(ctx, repoPath); err != nil {
		return err
	}

	return nil
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
