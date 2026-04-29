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

package template

import (
	"fmt"
	"os"
	"strings"

	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type args struct {
	File string

	Image      string
	Dockerfile string

	Repo     string
	Branch   string
	Depth    uint32
	Checkout string

	EnvVars           []string
	EnvVarFromSecrets []string

	AdditionalRepos []string

	CPUMillicores uint32
	MemoryMB      uint32
	StorageMB     uint32

	GitProvider string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.File, "file", "f", "", "Path to a Template YAML spec file")

	Cmd.PersistentFlags().StringVarP(&cmdArgs.Image, "image", "", "", `Container image URL (e.g. "ubuntu:24.04", "python:3.11-slim")`)
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Dockerfile, "dockerfile", "", "",
		"Path to a local Dockerfile. The file is read and embedded inline. COPY/ADD with local context paths are not supported.")

	Cmd.PersistentFlags().StringVarP(&cmdArgs.Repo, "repository", "", "", "Primary repository URL to clone into /workspace/repo")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Branch, "branch", "b", "", "Branch to clone when using --repository (default: repository default branch)")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.Depth, "depth", 0, "Shallow clone depth when using --repository (0 = full clone)")
	Cmd.PersistentFlags().StringVar(&cmdArgs.Checkout, "checkout", "", "Specific commit, tag, or ref to checkout when using --repository")

	Cmd.PersistentFlags().StringArrayVarP(&cmdArgs.EnvVars, "env", "e", nil,
		"Set an environment variable in Workspaces from this Template (KEY=VALUE). Repeatable: -e FOO=bar -e BAZ=qux")
	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.EnvVarFromSecrets, "env-from-secret", nil,
		"Set an environment variable from a Space Secret (KEY=SECRET_NAME). Repeatable: --env-from-secret DB_URL=my-db-secret")

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.AdditionalRepos, "additional-repo", nil,
		"Clone an additional repository into Workspaces from this Template (NAME=URL). Repeatable: --additional-repo shared=https://github.com/org/shared")

	Cmd.PersistentFlags().Uint32Var(&cmdArgs.CPUMillicores, "cpu", 0, "Default CPU limit for Workspaces from this Template in millicores (e.g. 2000 = 2 cores)")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.MemoryMB, "memory", 0, "Default memory limit for Workspaces from this Template in megabytes (e.g. 4096 = 4 GB)")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.StorageMB, "storage", 0, "Default storage limit for Workspaces from this Template in megabytes (e.g. 20000 = 20 GB)")

	Cmd.PersistentFlags().StringVar(&cmdArgs.GitProvider, "git-provider", "",
		"Name of a GitProvider in the same Space for automatic OAuth2 token injection (e.g. github.my-project)")

	Cmd.MarkFlagsMutuallyExclusive("image", "dockerfile")
}

var Cmd = &cobra.Command{
	Use:   "template <name> [flags]",
	Short: "Create a Template",
	Long: `Create a new Template within a Space.

The Template name must be fully qualified as <template-name>.<space-name> (e.g.
ml-env.my-project). A default Template named "default" is created automatically
for every new Space; additional Templates can be created for different project
configurations, language environments, or use cases within the same Space.

Template spec shares most of the Workspace spec: image, repository, runtime
environment variables, lifecycle tasks, additional repositories, and resource
limits. Workspaces created from this Template inherit all of these settings,
which can be further overridden at the Workspace level.

Templates additionally support a GitProvider association (--git-provider) for
automatic OAuth2 token injection into Workspaces, enabling authenticated git
operations without manual credential management.

Use --file to supply a full YAML spec. All other flags can be used standalone
or combined with --file to override specific fields.

To trigger a pre-build snapshot after creation, run:
  cordium build <template-name>`,
	Example: `
  # Create a minimal Template from a container image
  cordium create template python-base.my-project --image python:3.11-slim

  # Create a Template from a YAML spec file
  cordium create tmpl ml-env.research -f template.yaml

  # Create a Template from a git repository (auto-detects devcontainer or Dockerfile)
  cordium create template backend.my-project \
    --repository https://github.com/myorg/backend-service

  # Create a Template from a specific branch at shallow depth
  cordium create template backend.my-project \
    --repository https://github.com/myorg/backend-service \
    --branch main \
    --depth 1

  # Create a Template from a local Dockerfile
  cordium create template custom-env.my-project --dockerfile ./Dockerfile

  # Set environment variables for all Workspaces from this Template
  cordium create template node-app.my-project \
    --image node:20-bookworm \
    --repository https://github.com/myorg/node-app \
    -e NODE_ENV=development \
    -e LOG_LEVEL=debug

  # Source environment variables from Space Secrets
  cordium create template ml-env.research \
    --image python:3.11 \
    --env-from-secret WANDB_API_KEY=wandb-secret \
    --env-from-secret HF_TOKEN=huggingface-secret

  # Mix static and secret-sourced environment variables
  cordium create template data-pipeline.analytics \
    --image python:3.11-slim \
    -e PYTHONUNBUFFERED=1 \
    -e ENVIRONMENT=staging \
    --env-from-secret DATABASE_URL=staging-db-secret \
    --env-from-secret AWS_CREDENTIALS=aws-secret

  # Clone the primary repository and additional repositories
  cordium create template fullstack.my-project \
    --repository https://github.com/myorg/api-service \
    --additional-repo shared-lib=https://github.com/myorg/shared-lib \
    --additional-repo proto=https://github.com/myorg/proto-defs

  # Set default resource limits for Workspaces from this Template
  cordium create template ml-gpu.research \
    --image pytorch/pytorch:2.1.0-cuda11.8-cudnn8-runtime \
    --cpu 8000 \
    --memory 32768 \
    --storage 100000

  # Associate a GitProvider for automatic OAuth2 token injection
  cordium create template private-repo.my-project \
    --repository https://github.com/myorg/private-service \
    --git-provider github.my-project

  # Full example: image, repo, secrets, limits, and GitProvider
  cordium create template backend-service.my-project \
    --image golang:1.23-bookworm \
    --repository https://github.com/myorg/backend-service \
    --branch main \
    --env-from-secret DATABASE_DSN=dev-postgres-dsn \
    -e GOFLAGS="-mod=mod" \
    -e LOG_FORMAT=json \
    --additional-repo shared=https://github.com/myorg/go-shared \
    --cpu 4000 \
    --memory 8192 \
    --storage 20000 \
    --git-provider github.my-project

  # Combine a YAML file with inline overrides
  cordium create template ml-env.research \
    -f base-template.yaml \
    --env-from-secret WANDB_API_KEY=wandb-secret \
    --cpu 8000 \
    --memory 16384`,
	Aliases: []string{"templates", "tmpl"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
}

func doCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	tmpl := &pb.Template{
		Spec: &pb.Template_Spec{},
	}

	if cmdArgs.File != "" {
		yamlBytes, err := os.ReadFile(cmdArgs.File)
		if err != nil {
			return err
		}
		if err := pbutils.UnmarshalYAML(yamlBytes, tmpl); err != nil {
			return err
		}
	}

	if cmdArgs.Image != "" {
		tmpl.Spec.Image = &pb.Workspace_Spec_Image{
			Type: &pb.Workspace_Spec_Image_Registry_{
				Registry: &pb.Workspace_Spec_Image_Registry{
					Url: cmdArgs.Image,
				},
			},
		}
	} else if cmdArgs.Dockerfile != "" {
		dockerfileBytes, err := os.ReadFile(cmdArgs.Dockerfile)
		if err != nil {
			return err
		}
		tmpl.Spec.Image = &pb.Workspace_Spec_Image{
			Type: &pb.Workspace_Spec_Image_Dockerfile_{
				Dockerfile: &pb.Workspace_Spec_Image_Dockerfile{
					Type: &pb.Workspace_Spec_Image_Dockerfile_Inline{
						Inline: string(dockerfileBytes),
					},
				},
			},
		}
	}

	if cmdArgs.Repo != "" {
		if tmpl.Spec.Repository == nil {
			tmpl.Spec.Repository = &pb.Workspace_Spec_Repository{}
		}
		tmpl.Spec.Repository.Url = cmdArgs.Repo
	}

	if cmdArgs.Branch != "" || cmdArgs.Depth > 0 || cmdArgs.Checkout != "" {
		if tmpl.Spec.Repository == nil {
			tmpl.Spec.Repository = &pb.Workspace_Spec_Repository{}
		}
		if tmpl.Spec.Repository.CloneOptions == nil {
			tmpl.Spec.Repository.CloneOptions = &pb.Workspace_Spec_Repository_CloneOptions{}
		}
		if cmdArgs.Branch != "" {
			tmpl.Spec.Repository.CloneOptions.Branch = cmdArgs.Branch
		}
		if cmdArgs.Depth > 0 {
			tmpl.Spec.Repository.CloneOptions.Depth = cmdArgs.Depth
		}
		if cmdArgs.Checkout != "" {
			tmpl.Spec.Repository.CloneOptions.Checkout = cmdArgs.Checkout
		}
	}

	if len(cmdArgs.EnvVars) > 0 || len(cmdArgs.EnvVarFromSecrets) > 0 {
		if tmpl.Spec.Runtime == nil {
			tmpl.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}
		for _, raw := range cmdArgs.EnvVars {
			key, val, ok := strings.Cut(raw, "=")
			if !ok || key == "" {
				return errors.Errorf("invalid --env value %q: expected KEY=VALUE", raw)
			}
			tmpl.Spec.Runtime.EnvVars = append(tmpl.Spec.Runtime.EnvVars,
				&pb.Workspace_Spec_Runtime_EnvVar{
					Key:  key,
					Type: &pb.Workspace_Spec_Runtime_EnvVar_Value{Value: val},
				})
		}
		for _, raw := range cmdArgs.EnvVarFromSecrets {
			key, secretName, ok := strings.Cut(raw, "=")
			if !ok || key == "" || secretName == "" {
				return errors.Errorf("invalid --env-from-secret value %q: expected KEY=SECRET_NAME", raw)
			}
			tmpl.Spec.Runtime.EnvVars = append(tmpl.Spec.Runtime.EnvVars,
				&pb.Workspace_Spec_Runtime_EnvVar{
					Key:  key,
					Type: &pb.Workspace_Spec_Runtime_EnvVar_FromSecret{FromSecret: secretName},
				})
		}
	}

	for _, raw := range cmdArgs.AdditionalRepos {
		name, repoURL, ok := strings.Cut(raw, "=")
		if !ok || name == "" || repoURL == "" {
			return errors.Errorf("invalid --additional-repo value %q: expected NAME=URL", raw)
		}
		tmpl.Spec.AdditionalRepositories = append(tmpl.Spec.AdditionalRepositories,
			&pb.Workspace_Spec_AdditionalRepository{
				Name:      name,
				ClonePath: fmt.Sprintf("/workspace/additional-repos/%s", name),
				Repository: &pb.Workspace_Spec_Repository{
					Url: repoURL,
				},
			},
		)
	}

	if cmdArgs.CPUMillicores > 0 || cmdArgs.MemoryMB > 0 || cmdArgs.StorageMB > 0 {
		if tmpl.Spec.Limit == nil {
			tmpl.Spec.Limit = &pb.Workspace_Spec_Limit{}
		}
		if cmdArgs.CPUMillicores > 0 {
			tmpl.Spec.Limit.Cpu = &pb.Workspace_Spec_Limit_CPU{Millicores: cmdArgs.CPUMillicores}
		}
		if cmdArgs.MemoryMB > 0 {
			tmpl.Spec.Limit.Memory = &pb.Workspace_Spec_Limit_Memory{Megabytes: cmdArgs.MemoryMB}
		}
		if cmdArgs.StorageMB > 0 {
			tmpl.Spec.Limit.Storage = &pb.Workspace_Spec_Limit_Storage{Megabytes: cmdArgs.StorageMB}
		}
	}

	if cmdArgs.GitProvider != "" {
		tmpl.Spec.GitProvider = cmdArgs.GitProvider
	}

	tmpl.Metadata = &metav1.Metadata{
		Name: i.FirstArg(),
	}
	tmpl.Status = &pb.Template_Status{}

	tmpl, err = c.CreateTemplate(ctx, tmpl)
	if err != nil {
		return err
	}

	cliutils.LineNotify("Successfully created Template: %s\n", ccommon.GetResourceShortName(tmpl))

	return nil
}