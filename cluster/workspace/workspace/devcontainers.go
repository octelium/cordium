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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/octelium/cordium/cluster/common/devcontainers"
	"github.com/octelium/cordium/cluster/common/devcontainers/dcfeatures"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (s *Server) getDevContainerSpec(ctx context.Context) (*devcontainers.Spec, error) {
	devContainerPath := s.getDevContainerJSONPath(ctx)
	if devContainerPath == "" {
		zap.L().Debug("No devcontainer file. Nothing to be done...")
		return nil, nil
	}

	zap.L().Debug("Found devcontainer path", zap.String("path", devContainerPath))

	devcBytes, err := os.ReadFile(devContainerPath)
	if err != nil {
		return nil, err
	}

	return devcontainers.LoadSpec(&devcontainers.LoadSpecOpts{
		Data:      devcBytes,
		Workspace: s.initReq.Workspace,
	})

}

func (s *Server) setDevContainer(ctx context.Context) error {
	zap.L().Debug("Starting setting up devcontainer...")
	if s.initReq == nil || s.ws == nil {
		return nil
	}

	dc, err := s.getDevContainerSpec(ctx)
	if err != nil {
		return err
	}

	if dc == nil {
		return nil
	}

	for k, v := range dc.ContainerEnv {
		setEnv(&s.env, k, v)
	}

	if s.isFreshRun && dc.OnCreateCommand != nil {

		for _, cmd := range dc.OnCreateCommand.Commands {
			if err := s.taskManager.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
				Name:       "devcontainers-on-create-cmd",
				WorkingDir: s.repoDir,
				Run:        cmd,
				Type:       cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
			}); err != nil {
				return err
			}
		}

	}

	if s.isFreshRun && dc.UpdateContentCommand != nil {
		for _, cmd := range dc.UpdateContentCommand.Commands {
			if err := s.taskManager.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
				Name:       "devcontainers-on-update-content-cmd",
				WorkingDir: s.repoDir,
				Run:        cmd,
				Type:       cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
			}); err != nil {
				return err
			}
		}

	}

	if s.isFreshRun && dc.PostCreateCommand != nil {

		for _, cmd := range dc.PostCreateCommand.Commands {
			if err := s.taskManager.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
				Name:       "devcontainers-post-create-cmd",
				WorkingDir: s.repoDir,
				Run:        cmd,
				Type:       cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
			}); err != nil {
				return err
			}
		}

	}

	if dc.PostStartCommand != nil {

		for _, cmd := range dc.PostStartCommand.Commands {
			if err := s.taskManager.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
				Name:       "devcontainers-post-start-cmd",
				WorkingDir: s.repoDir,
				Run:        cmd,
				Type:       cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
			}); err != nil {
				return err
			}
		}

	}

	if dc.Customizations != nil && dc.Customizations.VSCode != nil &&
		len(dc.Customizations.VSCode.Extensions) > 0 {
		zap.L().Debug("Found code extensions", zap.Strings("extensions", dc.Customizations.VSCode.Extensions))
		for _, ext := range dc.Customizations.VSCode.Extensions {
			if err := s.installVSCodExtension(ctx, ext); err != nil {
				zap.L().Warn("Could not install code extension", zap.String("extension", ext), zap.Error(err))
			}
		}
	}

	if len(dc.Extensions) > 0 {
		for _, ext := range dc.Extensions {
			if err := s.installVSCodExtension(ctx, ext); err != nil {
				zap.L().Warn("Could not install code extension", zap.String("extension", ext), zap.Error(err))
			}
		}
	}

	zap.L().Debug("Successfully set devcontainer")

	return nil
}

func (s *Server) setupDevContainersFeatures(ctx context.Context) error {
	if s.initReq == nil || s.ws == nil {
		return nil
	}

	zap.L().Debug("Starting setting up devcontainers features")

	dc, err := s.getDevContainerSpec(ctx)
	if err != nil {
		return err
	}

	var devcFeatureMap map[string]any
	if dc != nil {
		devcFeatureMap = dc.Features
	}

	basePath := "/workspace/.octelium/devcontainers/features"

	if s.isFreshRun {
		zap.L().Debug("This is a fresh run. Downloading features")
		if err := dcfeatures.DownloadFeatures(ctx, &dcfeatures.GetFeaturesOpts{
			FeaturesMap: devcFeatureMap,
			DirBase:     basePath,
			Workspace:   s.ws,
		}); err != nil {
			return err
		}

		if !vutils.FSPathExists(basePath) {
			zap.L().Debug("No downloaded features found in the base dir. Nothing to be done",
				zap.String("baseDir", basePath))
			return nil
		}

		if err := s.chownDirToUser(ctx, basePath); err != nil {
			zap.L().Error("Could not chown devctonaienrs features dir", zap.Error(err))
		}
	}

	if !vutils.FSPathExists(basePath) {
		zap.L().Debug("No installed features found in the base dir. Nothing to be done",
			zap.String("baseDir", basePath))
		return nil
	}

	features, err := dcfeatures.GetSortedFeatures(&dcfeatures.GetSortedFeaturesOpts{
		BasePath:     basePath,
		ContainerEnv: s.env,
		Workspace:    s.ws,
	})
	if err != nil {
		return err
	}

	envCommon := []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{
		{
			Key:   "_REMOTE_USER",
			Value: s.userInfo.name,
		},
		{
			Key:   "_REMOTE_USER_HOME",
			Value: s.userInfo.homeDir,
		},
	}

	for _, ftr := range features {

		env := []*cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{}
		env = append(env, envCommon...)
		varMap := getFeatureVariableMap(ftr.Spec.Options)

		for k, v := range varMap {
			if k != "" && v != "" {
				env = append(env, &cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{
					Key:   strings.ToUpper(k),
					Value: v,
				})
			}
		}

		if s.isFreshRun {
			zap.L().Debug("Adding install task for feature", zap.String("name", ftr.Name))
			if err := s.taskManager.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
				Name:       fmt.Sprintf("devcontainer-feature-install-%s", ftr.Name),
				WorkingDir: ftr.Path,
				Run:        filepath.Join(ftr.Path, "install.sh"),
				Type:       cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
				RunAsRoot:  true,
				EnvVars:    env,
			}); err != nil {
				return err
			}

			dc := ftr.Spec
			if dc.Customizations != nil && dc.Customizations.VSCode != nil &&
				len(dc.Customizations.VSCode.Extensions) > 0 {
				zap.L().Debug("Found code extensions in feature",
					zap.String("feature", dc.Name),
					zap.Strings("extensions", dc.Customizations.VSCode.Extensions))
				for _, ext := range dc.Customizations.VSCode.Extensions {
					if err := s.installVSCodExtension(ctx, ext); err != nil {
						zap.L().Warn("Could not install code extension", zap.String("extension", ext), zap.Error(err))
					}
				}
			}
		}

		if len(ftr.Spec.ContainerEnv) > 0 {
			for k, v := range ftr.Spec.ContainerEnv {
				setEnv(&s.env, k, v)
			}
		}

		if !s.ws.Status.IsBuild {
			if ftr.Spec.EntryPoint != "" {
				zap.L().Debug("Adding entrypoint task for feature", zap.String("name", ftr.Name))
				if err := s.taskManager.appendTask(&cordiumv1.Workspace_Spec_Runtime_Task{
					Name:       fmt.Sprintf("devcontainer-feature-entrypoint-%s", ftr.Name),
					WorkingDir: ftr.Path,
					Run:        ftr.Spec.EntryPoint,
					Type:       cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					EnvVars:    envCommon,
				}); err != nil {
					return err
				}
			}
		}

	}

	zap.L().Debug("Successfully set devcontainers features")

	return nil
}

func (s *Server) installVSCodExtension(ctx context.Context, ext string) error {
	zap.L().Debug("Installing code extension", zap.String("extension", ext))
	codeBinary, err := getCodeBinary()
	if err != nil {
		zap.L().Debug("Could not find code binary. Nothing to be done")
		return nil
	}

	cmd := s.getCmdAsUser(ctx, fmt.Sprintf("%s --install-extension %s", codeBinary, ext))

	return cmd.Run()
}

func getCodeBinary() (string, error) {
	candidates := []string{
		"code",
		"code-server",
		"openvscode-server",
	}

	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c, nil
		}
	}

	return "", errors.Errorf("Could not find code binary")
}

func getFeatureVariableMap(in map[string]any) map[string]string {
	ret := make(map[string]string)

	for k, v := range in {
		switch v.(type) {
		case string:
			ret[k] = v.(string)
		case int:
			ret[k] = fmt.Sprintf("%d", v.(int))
		case bool:
			ret[k] = fmt.Sprintf("%t", v.(bool))
		}
	}

	return ret
}
