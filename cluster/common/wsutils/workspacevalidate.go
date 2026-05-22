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

package wsutils

import (
	"context"
	"net/url"
	"regexp"

	"github.com/asaskevich/govalidator"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
)

type ValidateWorkspaceReq struct {
	Workspace      *cordiumv1.Workspace
	Space          *cordiumv1.Space
	SecretList     *cordiumv1.SecretList
	UserSecretList *cordiumv1.UserSecretList
}

func ValidateWorkspace(ctx context.Context, req *ValidateWorkspaceReq) error {

	if req == nil {
		return serr.InvalidArg("Nil req")
	}

	if req.Workspace == nil {
		return serr.InvalidArg("Nil req Workspace")
	}

	if req.Workspace.Spec == nil {
		return serr.InvalidArg("Nil req Workspace Spec")
	}

	if req.Space == nil {
		return serr.InvalidArg("Nil req Space")
	}

	spec := req.Workspace.Spec

	if req.Workspace.Status == nil || req.Workspace.Status.SpaceRef == nil {
		return serr.InvalidArg("Could not validate Workspace. No spaceRef")
	}

	space := req.Space

	if spec.Image != nil {
		specImage := spec.Image
		switch specImage.Type.(type) {
		case *cordiumv1.Workspace_Spec_Image_Dockerfile_:
			switch specImage.GetDockerfile().Type.(type) {
			case *cordiumv1.Workspace_Spec_Image_Dockerfile_Inline:
				inlineLen := len(specImage.GetDockerfile().GetInline())
				if inlineLen == 0 {
					return serr.InvalidArg("Empty Dockerfile")
				}
				if inlineLen > 5000 {
					return serr.InvalidArg("Too large Dockerfile")
				}
			case *cordiumv1.Workspace_Spec_Image_Dockerfile_Url:
				if !isValidURL(specImage.GetDockerfile().GetUrl()) {
					return serr.InvalidArg("Dockerfile URL is not valid")
				}

			}
		case *cordiumv1.Workspace_Spec_Image_Git_:
			if !isValidURL(specImage.GetGit().GetUrl()) {
				return serr.InvalidArg("Image git URL is not valid")
			}

			if specImage.GetGit().GetDockerfile() != "" &&
				!govalidator.IsUnixFilePath(specImage.GetGit().GetDockerfile()) {
				return serr.InvalidArg("Invalid git Dockerfile path")
			}

		case *cordiumv1.Workspace_Spec_Image_Registry_:
			url := specImage.GetRegistry().GetUrl()
			if url == "" {
				return serr.InvalidArg("Image URL is empty")
			}

			switch {
			case govalidator.IsUnixFilePath(url):
			case isValidURL(url):
			default:
				return serr.InvalidArg("Invalid Image URL: %s", url)
			}

			auth := specImage.GetRegistry().Authentication
			if auth != nil {
				if auth.Username == "" {
					return serr.InvalidArg("Empty registry authentication username")
				}
				if len(auth.Username) > 150 {
					return serr.InvalidArg("Registry authentication username is too long")
				}
				if !govalidator.IsASCII(auth.Username) {
					return serr.InvalidArg("Invalid registry authentication username")
				}

				if auth.Password == nil {
					return serr.InvalidArg("Basic authentication password must be set")
				}

				switch auth.Password.Type.(type) {
				case *cordiumv1.Workspace_Spec_Image_Registry_Authentication_Password_FromSecret:

					if auth.Password.GetFromSecret() == "" {
						return serr.InvalidArg("Empty Secret name")
					}
					if req.SecretList == nil {
						return serr.InvalidArg("No SecretList provided")
					}

					sec, err := ucordiumv1.ToSecretList(req.SecretList).GetByName(auth.Password.GetFromSecret())
					if err != nil {
						return serr.NotFound("The Secret: %s does not exist", auth.Password.GetFromSecret())
					}
					if sec.Status.SpaceRef.Uid != space.Metadata.Uid {
						return grpcutils.InvalidArg("The Secret does not exist: %s", auth.Password.GetFromSecret())
					}

				default:

				}
			}
		case *cordiumv1.Workspace_Spec_Image_Repository_:
			spec := specImage.GetRepository()
			switch spec.Type.(type) {
			case *cordiumv1.Workspace_Spec_Image_Repository_Devcontainer_:
				if !govalidator.IsUnixFilePath(spec.GetDevcontainer().DirPath) {
					return serr.InvalidArg("Invalid devcontainers dir path")
				}
			case *cordiumv1.Workspace_Spec_Image_Repository_Dockerfile_:
				if !govalidator.IsUnixFilePath(spec.GetDockerfile().Path) {
					return serr.InvalidArg("Invalid Dockerfile path")
				}
				if len(spec.GetDockerfile().Path) > 256 {
					return serr.InvalidArg("Invalid Dockerfile path")
				}
				if spec.GetDockerfile().Context != "" {
					if !govalidator.IsUnixFilePath(spec.GetDockerfile().Context) {
						return serr.InvalidArg("Invalid Dockerfile context")
					}
					if len(spec.GetDockerfile().Context) > 256 {
						return serr.InvalidArg("Invalid Dockerfile path")
					}
				}
			}
		}
	}

	if spec.Runtime != nil {
		specContainer := spec.Runtime

		if specContainer.EnvVars != nil {
			if len(specContainer.EnvVars) > 128 {
				return serr.InvalidArg("Too many container env vars")
			}

			for _, envVar := range specContainer.EnvVars {
				if envVar.Key == "" {
					return serr.InvalidArg("Env variable cannot have an empty key")
				}
				if !govalidator.IsASCII(envVar.Key) {
					return serr.InvalidArg("Invalid env var key")
				}

				if len(envVar.Key) > 64 {
					return serr.InvalidArg("Too long env var key")
				}

				switch envVar.Type.(type) {
				case *cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret:
					if envVar.GetFromSecret() == "" {
						return serr.InvalidArg("Empty Secret name for the env variable with key: %s", envVar.Key)
					}

					sec, err := ucordiumv1.ToSecretList(req.SecretList).GetByName(envVar.GetFromSecret())
					if err != nil {
						return serr.NotFound("The Secret: %s does not exist", envVar.GetFromSecret())
					}

					if sec.Status.SpaceRef.Uid != space.Metadata.Uid {
						return grpcutils.InvalidArg("The Secret does not exist: %s", envVar.GetFromSecret())
					}

				case *cordiumv1.Workspace_Spec_Runtime_EnvVar_Value:
					if len(envVar.GetValue()) == 0 {
						return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
					}

					if len(envVar.GetValue()) > 1024 {
						return serr.InvalidArg("Empty value for env var: %s", envVar.GetValue())
					}
				default:
					return serr.InvalidArg("No env variable value for the key: %s", envVar.Key)
				}
			}
		}

		if specContainer.Cmd != "" {
			if len(specContainer.Cmd) > 1024 {
				return serr.InvalidArg("Too long cmd: %s", specContainer.Cmd)
			}
		}

		if specContainer.Entrypoint != "" {
			if len(specContainer.Entrypoint) > 1024 {
				return serr.InvalidArg("Too long cmd: %s", specContainer.Entrypoint)
			}
		}

		if len(specContainer.Tasks) > 128 {
			return serr.InvalidArg("Too many tasks")
		}

		for _, cmd := range specContainer.Tasks {
			if len(cmd.EnvVars) > 256 {
				return serr.InvalidArg("Too large container env var list")
			}

			if cmd.Run == "" {
				return serr.InvalidArg("Empty container command")
			}

			if len(cmd.Run) > 5000 {
				return serr.InvalidArg("Command is too large")
			}

			if cmd.Type == cordiumv1.Workspace_Spec_Runtime_Task_UNKNOWN {
				return serr.InvalidArg("The type of the container command: %s must be set", cmd.Run)
			}

			if cmd.WorkingDir != "" {
				if !govalidator.IsUnixFilePath(cmd.WorkingDir) {
					return serr.InvalidArg("Invalid working dir path: %s", cmd.WorkingDir)
				}
				if len(cmd.WorkingDir) > 256 {
					return serr.InvalidArg("Too long working dir path: %s", cmd.WorkingDir)
				}
			}

			if len(cmd.EnvVars) > 128 {
				return serr.InvalidArg("Too many env vars")
			}

			for _, envVar := range cmd.EnvVars {
				if envVar.Key == "" {
					return serr.InvalidArg("Env variable cannot have an empty key")
				}
				if !govalidator.IsASCII(envVar.Key) {
					return serr.InvalidArg("Invalid env var key")
				}

				if len(envVar.Key) > 64 {
					return serr.InvalidArg("Too long env var key")
				}

				if len(envVar.Value) == 0 {
					return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
				}

				if len(envVar.Value) > 1024 {
					return serr.InvalidArg("Empty value for env var: %s", envVar.Key)
				}

			}
		}

		if specContainer.Capabilities != nil {
			if len(specContainer.Capabilities.Add) > 100 {
				return grpcutils.InvalidArg("Too many capabilities")
			}

			if len(specContainer.Capabilities.Drop) > 100 {
				return grpcutils.InvalidArg("Too many capabilities")
			}

			for _, cap := range specContainer.Capabilities.Add {
				if !k8sCapabilityRegex.MatchString(cap) {
					return grpcutils.InvalidArg("Invalid capability: %s", cap)
				}
			}

			for _, cap := range specContainer.Capabilities.Drop {
				if !k8sCapabilityRegex.MatchString(cap) {
					return grpcutils.InvalidArg("Invalid capability: %s", cap)
				}
			}
		}

		if spec.Runtime.Devcontainers != nil {
			if spec.Runtime.Devcontainers.Features != nil {
				if len(spec.Runtime.Devcontainers.Features) > 100 {
					return serr.InvalidArg("Too many features")
				}

				for _, ftr := range spec.Runtime.Devcontainers.Features {
					if ftr.Reference == "" {
						return serr.InvalidArg("Empty devcontainer feature reference")
					}

					if !govalidator.IsASCII(ftr.Reference) {
						return serr.InvalidArg("Invalid env var key")
					}

					if len(ftr.Reference) > 256 {
						return serr.InvalidArg("Too long env var key")
					}

					if len(ftr.Options) > 32 {
						return serr.InvalidArg("Too many feature options")
					}

					for _, opt := range ftr.Options {
						if opt.Key == "" {
							return serr.InvalidArg("Empty feature option key")
						}

						if !govalidator.IsASCII(opt.Key) {
							return serr.InvalidArg("Invalid feature option key")
						}

						if len(opt.Key) > 256 {
							return serr.InvalidArg("Too long feature option key")
						}

						if opt.Value == "" {
							return serr.InvalidArg("Empty feature option value")
						}

						if !govalidator.IsASCII(opt.Value) {
							return serr.InvalidArg("Invalid feature option value")
						}

						if len(opt.Value) > 256 {
							return serr.InvalidArg("Too long feature option value")
						}
					}
				}
			}
		}

		if spec.Runtime.Octelium != nil {
			if len(spec.Runtime.Octelium.ServeServices) > 128 {
				return serr.InvalidArg("Too many serveServices")
			}

			for _, svc := range spec.Runtime.Octelium.ServeServices {
				if err := apivalidation.ValidateName(svc, 0, 2); err != nil {
					return serr.InvalidArg("Invalid serveService: %s", err.Error())
				}
			}
		}
	}

	checkRepo := func(repo *cordiumv1.Workspace_Spec_Repository, mustHaveURL bool) error {
		if repo.Url == "" && mustHaveURL {
			return serr.InvalidArg("Repository URL is empty")
		}

		if repo.Url != "" {

			if !isValidURL(repo.GetUrl()) {
				return serr.InvalidArg("Invalid repository URL: %s", repo.GetUrl())
			}

			u, err := url.Parse(repo.GetUrl())
			if err != nil {
				return serr.InvalidArg("Invalid repo URL: %s", repo.GetUrl())
			}
			if u.Scheme != "https" {
				return serr.InvalidArg(`Repo URL must have https scheme (e.g. "https://github.com/org/repo")`)
			}
		}

		if repo.Authentication != nil {
			switch repo.Authentication.Type.(type) {
			case *cordiumv1.Workspace_Spec_Repository_Authentication_Http:

				basic := repo.Authentication.GetHttp()

				if basic.Username == "" {
					return serr.InvalidArg("Empty basic authentication username")
				}
				if len(basic.Username) > 150 {
					return serr.InvalidArg("Basic authentication username is too long")
				}
				if !govalidator.IsASCII(basic.Username) {
					return serr.InvalidArg("Invalid basic authentication username")
				}

				if basic.Password == nil {
					return serr.InvalidArg("Basic authentication password must be set")
				}

				switch basic.Password.Type.(type) {
				case *cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password_FromSecret:
					/*
						if space.Status.Type != cordiumv1.Space_Status_ORGANIZATION {
							return serr.InvalidArg("Secrets can only be used in ORGANIZATION Spaces")
						}
					*/
					if basic.Password.GetFromSecret() == "" {
						return serr.InvalidArg("Empty Secret name")
					}
					if req.SecretList == nil {
						return serr.InvalidArg("No SecretList provided")
					}

					sec, err := ucordiumv1.ToSecretList(req.SecretList).GetByName(basic.Password.GetFromSecret())
					if err != nil {
						return serr.NotFound("The Secret: %s does not exist", basic.Password.GetFromSecret())
					}
					if sec.Status.SpaceRef.Uid != space.Metadata.Uid {
						return grpcutils.InvalidArg("The Secret does not exist: %s", basic.Password.GetFromSecret())
					}

				default:

				}

			}
		}

		return nil
	}

	if spec.Repository != nil {
		if err := checkRepo(spec.Repository, false); err != nil {
			return err
		}
	}

	if spec.AdditionalRepositories != nil {
		if len(spec.AdditionalRepositories) > 32 {
			return serr.InvalidArg("Too many additional repositories")
		}
		var names []string

		for _, project := range spec.AdditionalRepositories {
			if project.Name == "" {
				return serr.InvalidArg("Empty additional repository name")
			}
			if !common.IsNameValid(project.Name) {
				return serr.InvalidArg("Invalid additional repository name: %s", project.Name)
			}
			if isInList(names, project.Name) {
				return serr.InvalidArg("The additional repository name: %s already exists", project.Name)
			}
			names = append(names, project.Name)

			if project.Repository == nil {
				return serr.InvalidArg("Repository details are not set for the additional Repository: %s", project.Name)
			}

			if err := checkRepo(project.Repository, true); err != nil {
				return err
			}
		}
	}

	if spec.Applications != nil {
		if len(spec.Applications) > 128 {
			return serr.InvalidArg("Too many applications")
		}
		var names []string

		hasDefault := false

		for _, app := range spec.Applications {
			if app.Name == "" {
				return serr.InvalidArg("Application name cannot be empty")
			}
			if !common.IsNameValid(app.Name) {
				return serr.InvalidArg("Invalid Application name: %s", app.Name)
			}

			if isInList(names, app.Name) {
				return serr.InvalidArg("The Application name: %s already exists", app.Name)
			}
			names = append(names, app.Name)

			if app.Port == 0 {
				return serr.InvalidArg("Port number must be set for Application: %s", app.Name)
			}

			if app.Port <= 0 || app.Port > 65535 {
				return serr.InvalidArg("Invalid Application port: %d", app.Port)
			}

			if len(app.DisplayName) > 128 {
				return serr.InvalidArg("Description is too long: %s", app.DisplayName)
			}

			if app.IsDefault && hasDefault {
				return serr.InvalidArg("Can only have one default Application")
			}

			if app.IsDefault {
				hasDefault = true
			}

		}
	}

	if spec.Limit != nil {
		limit := spec.Limit

		if limit.Cpu != nil {
			if limit.Cpu.Millicores > 1000_000 {
				return serr.InvalidArg("CPU are too large: %d", limit.Cpu.Millicores)
			}
		}

		if limit.Memory != nil {
			if limit.Memory.Megabytes > 10_000_000 {
				return serr.InvalidArg("Memory is too large: %d", limit.Memory.Megabytes)
			}
		}

		if limit.Storage != nil {
			if limit.Storage.Megabytes > 10_000_000 {
				return serr.InvalidArg("Storage is too large: %d", limit.Storage.Megabytes)
			}
		}
	}

	return nil
}

var k8sCapabilityRegex = regexp.MustCompile(`^(ALL|[A-Z][A-Z0-9_]{0,29})$`)
