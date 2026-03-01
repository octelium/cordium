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
	"os"
	"path/filepath"
	"time"

	"github.com/octelium/cordium/cluster/common/gitutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"go.uber.org/zap"
)

func (s *Server) setupDotFiles(ctx context.Context) error {

	if s.initReq == nil {
		return nil
	}

	if s.ws.Status.IsBuild {
		zap.L().Debug("No need to setup dotfiles. This is a prebuild")
		return nil
	}

	if !s.ws.Status.IsEphemeral && s.ws.Status.SuccessfulRuns > 0 {
		zap.L().Debug("No need to setup dotfiles. This is not a first run")
		return nil
	}

	if s.initReq.UserConfig == nil || s.initReq.UserConfig.Spec.Dotfiles == nil ||
		s.initReq.UserConfig.Spec.Dotfiles.Url == "" {
		zap.L().Debug("No need to setup dotfiles. No dotfiles url supplied")
		return nil
	}

	spec := s.initReq.UserConfig.Spec.Dotfiles

	zap.L().Debug("Setting up dotfiles from URL", zap.String("url", spec.Url))

	dotfilesDir := filepath.Join(s.userInfo.homeDir, "dotfiles")

	{
		ctx, cancelFn := context.WithTimeout(ctx, 1*time.Minute)
		defer cancelFn()

		opts := &gitutils.CloneOpts{
			Dir: dotfilesDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: spec.Url,
			},
		}

		if spec.Branch != "" {
			opts.Repo.CloneOptions = &cordiumv1.Workspace_Spec_Repository_CloneOptions{
				Branch: spec.Branch,
			}
		}

		if spec.Authentication != nil {

			opts.Repo.Authentication = &cordiumv1.Workspace_Spec_Repository_Authentication{}

			secretList := &cordiumv1.SecretList{}
			for _, sec := range s.initReq.UserSecretList.Items {
				secretList.Items = append(secretList.Items, &cordiumv1.Secret{
					Metadata: sec.Metadata,
					Data: &cordiumv1.Secret_Data{
						Type: &cordiumv1.Secret_Data_Value{
							Value: ucordiumv1.ToUserSecret(sec).GetValueStr(),
						},
					},
				})
			}

			opts.SecretList = secretList

			switch spec.Authentication.Type.(type) {
			case *cordiumv1.UserConfig_Spec_Dotfiles_Authentication_Http:
				if spec.Authentication.GetHttp().Password != nil {
					opts.Repo.Authentication = &cordiumv1.Workspace_Spec_Repository_Authentication{
						Type: &cordiumv1.Workspace_Spec_Repository_Authentication_Http{
							Http: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP{
								Username: spec.Authentication.GetHttp().Username,
								Password: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password{
									Type: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password_FromSecret{
										FromSecret: spec.Authentication.GetHttp().Password.GetFromUserSecret(),
									},
								},
							},
						},
					}
				}

			}
		}

		if err := gitutils.Clone(ctx, opts); err != nil {
			return err
		}
	}

	if err := s.chownDirToUser(ctx, dotfilesDir); err != nil {
		return err
	}

	var installFiles = []string{
		"install.sh",
		"install",
		"bootstrap.sh",
		"bootstrap",
		"script/bootstrap",
		"setup.sh",
		"setup",
		"script/setup",
	}

	var installFilePath string
	for _, fileName := range installFiles {
		filePath := filepath.Join(dotfilesDir, fileName)
		stat, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}

		if stat.Mode()&0100 == 0 {
			continue
		}

		installFilePath = filePath
		break
	}

	if installFilePath == "" {
		return nil
	}

	{
		ctx, cancelFn := context.WithTimeout(ctx, 3*time.Minute)
		defer cancelFn()
		cmd := s.getCmdAsUser(ctx, installFilePath)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}
