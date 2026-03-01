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
	"fmt"
	"os/exec"
	"sync"

	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"go.uber.org/zap"
)

func (s *Server) setupGit(ctx context.Context) error {

	if s.initReq == nil {
		return nil
	}
	if s.initReq.Workspace.Status.IsBuild {
		zap.L().Debug("This is a prebuild. No need to set Git cred helper")
		return nil
	}

	zap.L().Debug("Setting git cred helper config")

	if _, err := exec.LookPath("git"); err != nil {
		zap.L().Warn("No git binary installed in this Workspace container. Setting git credential helper is getting skipped...")
		return nil
	}

	{
		cmdStr := `git config --global credential.helper "/bin/cordium-git-cred-helper"`
		cmd := s.getCmdAsUser(ctx, cmdStr)

		if err := cmd.Run(); err != nil {
			return err
		}

		zap.L().Debug("git cred helper config successfully set")
	}

	if s.initReq.GitProviderInfo != nil &&
		s.initReq.GitProviderInfo.Email != "" &&
		s.initReq.GitProviderInfo.Username != "" {
		zap.L().Debug("Found gitProviderInfo",
			zap.String("username", s.initReq.GitProviderInfo.Username),
			zap.String("email", s.initReq.GitProviderInfo.Email))
		{
			cmdStr := fmt.Sprintf(`git config --global user.email "%s"`, s.initReq.GitProviderInfo.Email)
			cmd := s.getCmdAsUser(ctx, cmdStr)

			if err := cmd.Run(); err != nil {
				return err
			}

			zap.L().Debug("git user.email config successfully set",
				zap.String("value", s.initReq.GitProviderInfo.Email))
		}

		{
			cmdStr := fmt.Sprintf(`git config --global user.name "%s"`, s.initReq.GitProviderInfo.Username)
			cmd := s.getCmdAsUser(ctx, cmdStr)

			if err := cmd.Run(); err != nil {
				return err
			}

			zap.L().Debug("git user.name config successfully set",
				zap.String("value", s.initReq.GitProviderInfo.Username))
		}
	} else {
		zap.L().Debug("Skipping setting git user.email and user.name configs")
	}

	return nil
}

func (s *Server) GetGitCreds(ctx context.Context, req *ccordiumv1.GetGitCredsRequest) (*ccordiumv1.GetGitCredsResponse, error) {
	zap.L().Debug("New GetGitCreds Request", zap.Any("req", req))

	if req.Request == nil {
		return nil, grpcutils.NotFound("Nil request")
	}

	s.gitStore.Lock()
	defer s.gitStore.Unlock()

	if entry, ok := s.gitStore.entryMap[req.Request["host"]]; ok {
		zap.L().Debug("Found entry stored", zap.Any("username", entry.Username))
		resp := req.Request
		resp["username"] = entry.Username
		resp["password"] = entry.Password
		return &ccordiumv1.GetGitCredsResponse{
			Response: resp,
		}, nil
	}

	if s.initReq != nil && s.initReq.GitProviderInfo != nil {
		zap.L().Debug("Found gitProviderInfo", zap.Any("username", s.initReq.GitProviderInfo.Username))
		resp := req.Request
		resp["username"] = s.initReq.GitProviderInfo.Username
		resp["password"] = s.initReq.GitProviderInfo.AccessToken
		return &ccordiumv1.GetGitCredsResponse{
			Response: resp,
		}, nil
	}

	zap.L().Debug("Could not find git creds neither in the map nor the gitProviderInfo")

	return nil, grpcutils.NotFound("No git creds found")
}

func (s *Server) StoreGitCreds(ctx context.Context, req *ccordiumv1.StoreGitCredsRequest) (*ccordiumv1.StoreGitCredsResponse, error) {
	zap.L().Debug("New StoreGitCreds Request", zap.Any("req", req))

	s.gitStore.Lock()
	defer s.gitStore.Unlock()

	if req.Request == nil || req.Request["host"] == "" || req.Request["username"] == "" || req.Request["password"] == "" {
		zap.L().Debug("Invalid request. Nothing to be stored", zap.Any("req", req))
		return nil, grpcutils.InvalidArg("host, username and password fields must be set")
	}

	s.gitStore.entryMap[req.Request["host"]] = &gitStoreEntry{
		Host:     req.Request["host"],
		Username: req.Request["username"],
		Password: req.Request["password"],
		Protocol: req.Request["protocol"],
		Path:     req.Request["path"],
		URL:      req.Request["url"],
	}

	return &ccordiumv1.StoreGitCredsResponse{}, nil
}

func (s *Server) EraseGitCreds(ctx context.Context, req *ccordiumv1.EraseGitCredsRequest) (*ccordiumv1.EraseGitCredsResponse, error) {
	zap.L().Debug("New EraseGitCreds Request", zap.Any("req", req))

	if req.Request == nil {
		return nil, grpcutils.InvalidArg("Nil request")
	}

	s.gitStore.Lock()
	defer s.gitStore.Unlock()

	if req.Request["host"] == "" {
		zap.L().Debug("Invalid request. Nothing to be deleted")
		return nil, grpcutils.InvalidArg("host, username and password fields must be set")
	}

	if len(s.gitStore.entryMap) == 0 {
		return &ccordiumv1.EraseGitCredsResponse{}, nil
	}

	delete(s.gitStore.entryMap, req.Request["host"])

	return &ccordiumv1.EraseGitCredsResponse{}, nil
}

type gitStore struct {
	sync.Mutex
	entryMap map[string]*gitStoreEntry
}

type gitStoreEntry struct {
	Host     string
	Username string
	Password string
	Protocol string
	Path     string
	URL      string
}
