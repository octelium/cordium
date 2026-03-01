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

package supervisor

import (
	"context"
	"os"
	"path"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/octelium/cordium/cluster/common/devcontainers"
	"github.com/octelium/cordium/cluster/common/gitutils"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (s *Server) prepareAndBuildImage(ctx context.Context) error {

	doFn := func() error {
		imageSpec := s.spec.Image
		if imageSpec == nil {
			if s.spec.Repository != nil {
				return s.prepareImageFromRepository(ctx)
			}
			return s.pullImageFromExternal(ctx, "", nil)
		}

		switch imageSpec.Type.(type) {
		case *cordiumv1.Workspace_Spec_Image_Dockerfile_:
			return s.prepareImageContainerfile(ctx)
		case *cordiumv1.Workspace_Spec_Image_Registry_:
			if imageSpec.GetRegistry().Url != "" {
				return s.pullImageFromExternal(ctx, imageSpec.GetRegistry().Url, imageSpec.GetRegistry().Authentication)
			}
		case *cordiumv1.Workspace_Spec_Image_Git_:
			return s.prepareImageFromGit(ctx)
		case *cordiumv1.Workspace_Spec_Image_Repository_:
			if s.spec.Repository != nil {
				return s.prepareImageFromRepository(ctx)
			}
		default:
			if s.spec.Repository != nil {
				return s.prepareImageFromRepository(ctx)
			}
		}

		// pull base image
		return s.pullImageFromExternal(ctx, "", nil)
	}

	if s.initReq.Workspace.Status.IsBuild {
		zap.L().Debug("This is a Build. Getting image from scratch...")
		return doFn()
	}

	if !s.isFreshRun {
		zap.L().Debug("Not a fresh run. No need to pull or build image.")
		return nil
	}
	// first run

	zap.L().Debug("First run. Getting image from scratch...")
	return doFn()
}

func (s *Server) prepareImageContainerfile(ctx context.Context) error {

	zap.L().Debug("Preparing image from Containerfile")
	var fPath string

	spec := s.spec.GetImage().GetDockerfile()

	switch spec.Type.(type) {
	case *cordiumv1.Workspace_Spec_Image_Dockerfile_Inline:
		fPath = path.Join(s.buildDir, "Dockerfile")
		if err := os.WriteFile(fPath,
			[]byte(spec.GetInline()), 0644); err != nil {
			return err
		}

		if err := s.chownFileOctelium(ctx, fPath); err != nil {
			return err
		}

	case *cordiumv1.Workspace_Spec_Image_Dockerfile_Url:
		fPath = spec.GetUrl()
	}

	if err := s.buildImage(ctx, s.buildDir, fPath, ".", nil); err != nil {
		return err
	}

	return nil
}

func (s *Server) prepareImageDockerCompose(ctx context.Context, devcontainerDir, filePath, serviceName string) error {
	zap.L().Debug("Building from docker-compose",
		zap.String("filePath", filePath), zap.String("service", serviceName))

	/*
		fileContent, err := os.ReadFile(path.Join(devcontainerDir, filePath))
		if err != nil {
			return err
		}
	*/
	// var dockerCompose DockerCompose
	dir, filename := path.Split(path.Join(devcontainerDir, filePath))

	prj, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: path.Clean(dir),
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filename,
			},
		},
	})
	if err != nil {
		return errors.Errorf("Could not load dockerCompose file: %+v", err)
	}

	/*
		if err := yaml.Unmarshal(fileContent, &dockerCompose); err != nil {
			return err
		}
	*/

	if len(prj.Services) == 0 {
		return errors.Errorf("No Services found in this dockerCompose file")
	}

	svc, ok := prj.Services[serviceName]
	if !ok {
		return errors.Errorf("Could not find dockerCompose service: %s", serviceName)
	}

	zap.L().Debug("Got docker-composer service", zap.Any("service", svc))

	if svc.Init != nil && *svc.Init {
		s.containerInitProcess = true
	}

	if svc.Image != "" {
		return s.pullImageFromExternal(ctx, svc.Image, nil)
	}

	if svc.Build == nil {
		zap.L().Warn("Neither an image nor a build argument is provided in the dockerCompose file. Defaulting to base image")
		return s.pullImageFromExternal(ctx, "", nil)
	}

	dirPath := path.Dir(path.Join(devcontainerDir, filePath))

	args := make(map[string]string)

	if len(svc.Build.Args) > 0 {
		for k, v := range svc.Build.Args {
			if k != "" && v != nil && *v != "" {
				args[k] = *v
			}
		}
	}

	return s.buildImage(ctx, dirPath, svc.Build.Dockerfile, svc.Build.Context, args)
}

func (s *Server) prepareImageFromRepository(ctx context.Context) error {

	if err := s.shallowCloneMainRepository(ctx); err != nil {
		return errors.Errorf("Could not shallow clone main repo: %+v", err)
	}

	s.buildDir = "/octelium/workspace/repo"

	spec := func() *cordiumv1.Workspace_Spec_Image_Repository {

		if s.spec.Image != nil {
			return s.spec.Image.GetRepository()
		}
		return nil
	}()

	if spec != nil {
		switch spec.Type.(type) {
		case *cordiumv1.Workspace_Spec_Image_Repository_Devcontainer_:
			if spec.GetDevcontainer().DirPath != "" {
				var dirPath string
				var filePath string

				if spec.GetDevcontainer().DirPath == "." {
					dirPath = s.buildDir
					filePath = path.Join(dirPath, ".devcontainer.json")
				} else {
					dirPath = path.Join(s.buildDir, dirPath)
					filePath = path.Join(dirPath, "devcontainer.json")
				}

				zap.L().Debug("Building image from Template's devcontainer",
					zap.String("dir", dirPath), zap.String("devcontainer.json", filePath))

				return s.prepareImageFromDevcontainer(ctx, dirPath, filePath)
			}

		case *cordiumv1.Workspace_Spec_Image_Repository_Dockerfile_:
			contextPath := "."
			filePath := spec.GetDockerfile().Path
			if spec.GetDockerfile().Context != "" {
				contextPath = spec.GetDockerfile().Context
			}

			zap.L().Debug("Building image from Template's Dockerfile")

			return s.buildImage(ctx, s.buildDir, filePath, contextPath, nil)
		}
	}

	if ws, err := wsutils.LoadWorkspaceFile(ctx, &wsutils.LoadWorkspaceFileRequest{
		Parent:         s.initReq.Workspace,
		BaseDir:        s.buildDir,
		Space:          s.initReq.Space,
		SecretList:     s.initReq.SecretList,
		UserSecretList: s.initReq.UserSecretList,
	}); err == nil {
		zap.L().Debug("Found Workspace yaml file", zap.Any("ws", ws))
		s.spec, err = wsutils.MergeSpec(&wsutils.MergeSpecReq{
			Workspace:      s.initReq.Workspace,
			Template:       s.initReq.Template,
			ChildWorkspace: ws,
		})
		if err != nil {
			zap.L().Warn("Could not merge spec. Pulling base image", zap.Error(err))
			return s.pullImageFromExternal(ctx, "", nil)
		}

		zap.L().Debug("Spec has now been merged", zap.Any("spec", s.spec))

		if s.spec != nil && s.spec.Image != nil {

			switch s.spec.Image.Type.(type) {
			case *cordiumv1.Workspace_Spec_Image_Registry_:
				if s.spec.Image.GetRegistry().Url != "" {
					return s.pullImageFromExternal(ctx,
						s.spec.Image.GetRegistry().Url, s.spec.Image.GetRegistry().Authentication)
				}

			case *cordiumv1.Workspace_Spec_Image_Dockerfile_:
				return s.prepareImageContainerfile(ctx)
			case *cordiumv1.Workspace_Spec_Image_Git_:
				return s.prepareImageFromGit(ctx)
			case *cordiumv1.Workspace_Spec_Image_Repository_:
				if s.spec.Repository != nil {
					return s.prepareImageFromRepository(ctx)
				}

			}
		}
	}

	zap.L().Debug("Checking Template's devcontainer")

	if vutils.FSPathExists(path.Join(s.buildDir, ".devcontainer/devcontainer.json")) {
		dirPath := path.Join(s.buildDir, ".devcontainer")
		filePath := path.Join(s.buildDir, ".devcontainer/devcontainer.json")
		return s.prepareImageFromDevcontainer(ctx, dirPath, filePath)
	} else if vutils.FSPathExists(path.Join(s.buildDir, ".devcontainer.json")) {
		dirPath := s.buildContextDir
		filePath := path.Join(s.buildDir, ".devcontainer.json")
		return s.prepareImageFromDevcontainer(ctx, dirPath, filePath)
	}

	return s.pullImageFromExternal(ctx, "", nil)
}

func (s *Server) prepareImageFromGit(ctx context.Context) error {
	spec := s.spec.GetImage().GetGit()

	if err := gitutils.Clone(ctx, &gitutils.CloneOpts{
		Dir: s.buildDir,
		Repo: &cordiumv1.Workspace_Spec_Repository{
			Url: spec.Url,
			CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
				Checkout: spec.Checkout,
			},
		},
		UserUID: s.octeliumUID,
	}); err != nil {
		return err
	}

	if err := s.chownDirOctelium(ctx, s.buildDir); err != nil {
		return err
	}

	contextDir := "."

	if spec.Dockerfile != "" {
		fPath := path.Join(s.buildDir, spec.Dockerfile)
		if spec.Context != "" {
			contextDir = spec.Context
		}

		zap.L().Debug("Building image from Dockerfile in repo")

		return s.buildImage(ctx, s.buildDir, fPath, contextDir, nil)
	}

	var devcontainerDir string
	var devcontainerFilePath string

	zap.L().Debug("Checking for devcontainer.json")

	if vutils.FSPathExists(path.Join(s.buildDir, ".devcontainer/devcontainer.json")) {
		devcontainerDir = path.Join(s.buildDir, ".devcontainer")
		devcontainerFilePath = path.Join(s.buildDir, ".devcontainer/devcontainer.json")
	} else if vutils.FSPathExists(path.Join(s.buildDir, ".devcontainer.json")) {
		devcontainerDir = s.buildContextDir
		devcontainerFilePath = path.Join(s.buildDir, ".devcontainer.json")
	} else {
		zap.L().Warn("Could not find devcontainer.json. Defaulting to base image")
		return s.pullImageFromExternal(ctx, "", nil)
	}

	return s.prepareImageFromDevcontainer(ctx, devcontainerDir, devcontainerFilePath)
}

func (s *Server) prepareImageFromDevcontainer(ctx context.Context, devcontainerDir, devcontainerFilePath string) error {

	contextDir := "."

	zap.L().Debug("Found devcontainer file", zap.String("path", devcontainerFilePath))

	devcontainerBytes, err := os.ReadFile(devcontainerFilePath)
	if err != nil {
		return err
	}

	dc, err := devcontainers.LoadSpec(&devcontainers.LoadSpecOpts{
		Data:      devcontainerBytes,
		Workspace: s.initReq.Workspace,
	})
	if err != nil {
		return err
	}

	zap.L().Debug("Dev container content", zap.Any("devcontainer", dc))

	if dc.DockerComposeFile != "" && dc.Service != "" {
		return s.prepareImageDockerCompose(ctx, devcontainerDir, dc.DockerComposeFile, dc.Service)
	}

	if dc.Image != "" {
		zap.L().Debug("Found image in devcontainer.json. No need to build",
			zap.String("image", dc.Image))
		return s.pullImageFromExternal(ctx, dc.Image, nil)
	}

	var dockerfilePath string
	var args map[string]string
	if dc.Build == nil {
		if dc.Dockerfile == "" {
			zap.L().Warn("Either image or build must be set in devcontainer.json. Defaulting to base image")
			return s.pullImageFromExternal(ctx, "", nil)
		}
		dockerfilePath = path.Join(devcontainerDir, dc.Dockerfile)

	} else {
		if dc.Build.Dockerfile == "" {
			zap.L().Warn("Dockerfile path is not supplied. Defaulting to base image")
			return s.pullImageFromExternal(ctx, "", nil)
		}
		dockerfilePath = path.Join(devcontainerDir, dc.Build.Dockerfile)
		if dc.Build.Context != "" {
			contextDir = path.Join(devcontainerDir, dc.Build.Context)
		}

		args = dc.Build.Args
	}

	if dc.OverrideCommand != nil && !*dc.OverrideCommand {
		s.doNotOverrideCmd = true
	}

	s.containerInitProcess = dc.Init

	return s.buildImage(ctx, devcontainerDir, dockerfilePath, contextDir, args)
}
