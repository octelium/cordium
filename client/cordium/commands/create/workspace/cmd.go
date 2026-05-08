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
	"strings"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type CreateWorkspaceArgs struct {
	Space      string
	Template   string
	File       string
	Start      bool
	Repo       string
	Image      string
	Dockerfile string
	Ephemeral  bool

	Branch   string
	Depth    uint32
	Checkout string

	EnvVars           []string
	EnvVarFromSecrets []string

	AdditionalRepos []string

	CPUMillicores uint32
	MemoryMB      uint32
	StorageMB     uint32

	AppPorts []string

	AutoStop bool

	Out string
}

type args struct {
	CreateWorkspaceArgs
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Space, "space", "", "", "Parent Space name (e.g. my-project)")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Template, "template", "", "", "Parent Template name (e.g. ml-env.my-project)")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.File, "file", "", "", "Path to a Workspace YAML spec file")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.Start, "start", "", false, "Start the Workspace immediately after creation")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Repo, "repository", "", "", "Primary repository URL to clone into /workspace/repo")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Image, "image", "", "", `Container image URL (e.g. "ubuntu:24.04", "python:3.11-slim")`)
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Dockerfile, "dockerfile", "", "",
		"Path to a local Dockerfile. The file is read and embedded inline. COPY/ADD with local context paths are not supported; use --file for those cases.")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.Ephemeral, "ephemeral", "", false, "Create an ephemeral Workspace whose storage is discarded on stop")

	Cmd.PersistentFlags().StringVarP(&cmdArgs.Branch, "branch", "b", "", "Branch to clone when using --repository (default: repository default branch)")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.Depth, "depth", 0, "Shallow clone depth when using --repository (0 = full clone)")
	Cmd.PersistentFlags().StringVar(&cmdArgs.Checkout, "checkout", "", "Specific commit, tag, or ref to checkout when using --repository")

	Cmd.PersistentFlags().StringArrayVarP(&cmdArgs.EnvVars, "env", "e", nil,
		`Set an environment variable in the Workspace (KEY=VALUE). Repeatable: -e FOO=bar -e BAZ=qux`)
	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.EnvVarFromSecrets, "env-from-secret", nil,
		`Set an environment variable from a Space Secret (KEY=SECRET_NAME). Repeatable: --env-from-secret DB_URL=my-db-secret`)

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.AdditionalRepos, "additional-repo", nil,
		`Clone an additional repository into the Workspace (NAME=URL). Repeatable: --additional-repo shared=https://github.com/org/shared`)

	Cmd.PersistentFlags().Uint32Var(&cmdArgs.CPUMillicores, "cpu", 0, "CPU limit in millicores (e.g. 2000 = 2 cores). Uses Space/Cluster default if unset.")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.MemoryMB, "memory", 0, "Memory limit in megabytes (e.g. 4096 = 4 GB). Uses Space/Cluster default if unset.")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.StorageMB, "storage", 0, "Storage limit in megabytes (e.g. 20000 = 20 GB). Uses Space/Cluster default if unset.")

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.AppPorts, "port", nil,
		`Expose a named application port (NAME:PORT or PORT for unnamed). Repeatable: --port web:3000 --port api:8080. Append ":default" to mark as the default app: --port web:3000:default`)

	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", `Show the created Workspace. Current values are "yaml" or "json"`)

	Cmd.PersistentFlags().BoolVarP(&cmdArgs.AutoStop, "auto-stop", "", false, "Automatically stop the Workspace after running all POST_START tasks")

	Cmd.MarkFlagsMutuallyExclusive("space", "template")
	Cmd.MarkFlagsMutuallyExclusive("image", "dockerfile")
}

var Cmd = &cobra.Command{
	Use:   "workspace [flags]",
	Short: "Create a new Workspace",
	Long: `Create a new Workspace from a Template, YAML file, container image, Dockerfile, or git repository.

If no Template or Space is specified, the default Template of the default Space is used.
If --start is given, the Workspace is started immediately after creation.
Use "cordium run" to create and attach an interactive terminal in a single step.

Flags such as --env, --env-from-secret, --additional-repo, --cpu, --memory,
--storage, and --port can be combined with --file to override or extend values
from the YAML spec.`,
	Example: `
  # Create a Workspace from the default Template
  cordium create workspace

  # Create a Workspace in a specific Space
  cordium create ws --space ml-research

  # Create a Workspace from a specific Template
  cordium create ws --template backend-service.my-project

  # Create a Workspace from a YAML spec file
  cordium create ws --file workspace.yaml

  # Create and show Workspace as YAML
  cordium create ws --out yaml

  # Create and show Workspace as JSON
  cordium create ws --out json

  # Create an ephemeral Workspace from a container image
  cordium create ws --ephemeral --image python:3.11-slim

  # Create a Workspace from a git repository
  cordium create ws --repository https://github.com/myorg/my-project

  # Clone a specific branch at a shallow depth
  cordium create ws --repository https://github.com/myorg/my-project --branch develop --depth 1

  # Clone a repository and check out a specific tag
  cordium create ws --repository https://github.com/myorg/my-project --checkout v2.3.0

  # Create a Workspace from a local Dockerfile
  cordium create ws --dockerfile ./Dockerfile

  # Set static environment variables
  cordium create ws --image node:20 -e NODE_ENV=development -e LOG_LEVEL=debug

  # Set an environment variable from a Space Secret
  cordium create ws --image python:3.11 --env-from-secret DATABASE_URL=my-db-secret

  # Mix static and secret-sourced environment variables
  cordium create ws --template ml-env.research \
    -e WANDB_PROJECT=my-exp \
    --env-from-secret WANDB_API_KEY=wandb-secret \
    --env-from-secret HF_TOKEN=huggingface-secret

  # Clone the primary repository and an additional shared library repository
  cordium create ws --repository https://github.com/myorg/api-service \
    --additional-repo shared-lib=https://github.com/myorg/shared-lib \
    --additional-repo proto=https://github.com/myorg/proto-defs

  # Override resource limits
  cordium create ws --template ml-env.research --cpu 8000 --memory 16384 --storage 50000

  # Expose named application ports
  cordium create ws --image node:20 \
    --repository https://github.com/myorg/fullstack \
    --port web:3000:default \
    --port api:8080 \
    --port storybook:6006

  # Ephemeral AI agent sandbox from an image with secrets and resource limits
  cordium create ws --ephemeral \
    --image python:3.11-slim \
    --env-from-secret ANTHROPIC_API_KEY=anthropic-key \
    --cpu 2000 --memory 4096 --storage 10000 \
    --start --out name

  # Combine a YAML file with inline overrides
  cordium create ws --file base.yaml \
    -e ENVIRONMENT=staging \
    --env-from-secret DB_PASSWORD=staging-db-password \
    --cpu 4000

  # Use the "ws" alias with a template
  cordium create ws --template ml-env.research --start`,
	Aliases: []string{"workspaces", "ws"},
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

	ws, err := DoCreateWorkspace(ctx, c, &DoCreateWorkspaceOpts{
		Space:             cmdArgs.Space,
		Template:          cmdArgs.Template,
		File:              cmdArgs.File,
		Start:             cmdArgs.Start,
		Repo:              cmdArgs.Repo,
		Image:             cmdArgs.Image,
		Dockerfile:        cmdArgs.Dockerfile,
		Ephemeral:         cmdArgs.Ephemeral,
		Branch:            cmdArgs.Branch,
		Depth:             cmdArgs.Depth,
		Checkout:          cmdArgs.Checkout,
		EnvVars:           cmdArgs.EnvVars,
		EnvVarFromSecrets: cmdArgs.EnvVarFromSecrets,
		AdditionalRepos:   cmdArgs.AdditionalRepos,
		CPUMillicores:     cmdArgs.CPUMillicores,
		MemoryMB:          cmdArgs.MemoryMB,
		StorageMB:         cmdArgs.StorageMB,
		AppPorts:          cmdArgs.AppPorts,
		AutoStop:          cmdArgs.AutoStop,
	})
	if err != nil {
		return err
	}

	if cmdArgs.Start {
		cliutils.LineNotify("Successfully created and started Workspace: %s\n", ws.Metadata.Name)
	} else {
		if cmdArgs.Out != "" {
			out, err := cliutils.OutFormatPrint(cmdArgs.Out, ws)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", string(out))
		} else {
			cliutils.LineNotify("Successfully created Workspace: %s\n", ws.Metadata.Name)
		}

	}

	return nil
}

type DoCreateWorkspaceOpts struct {
	Space      string
	Template   string
	File       string
	Start      bool
	Repo       string
	Image      string
	Dockerfile string
	Ephemeral  bool

	Branch   string
	Depth    uint32
	Checkout string

	EnvVars           []string
	EnvVarFromSecrets []string

	AdditionalRepos []string

	CPUMillicores uint32
	MemoryMB      uint32
	StorageMB     uint32

	AutoStop bool

	AppPorts []string
}

func DoCreateWorkspace(ctx context.Context, c pb.MainServiceClient, o *DoCreateWorkspaceOpts) (*pb.Workspace, error) {
	var err error
	ws := &pb.Workspace{
		Spec: &pb.Workspace_Spec{},
	}

	if o.File != "" {
		yamlBytes, err := os.ReadFile(o.File)
		if err != nil {
			return nil, err
		}
		if err := pbutils.UnmarshalYAML(yamlBytes, ws); err != nil {
			return nil, err
		}
	}

	if o.Repo != "" {
		if ws.Spec.Repository == nil {
			ws.Spec.Repository = &pb.Workspace_Spec_Repository{}
		}
		ws.Spec.Repository.Url = o.Repo
	}

	if o.Branch != "" || o.Depth > 0 || o.Checkout != "" {
		if ws.Spec.Repository == nil {
			ws.Spec.Repository = &pb.Workspace_Spec_Repository{}
		}
		if ws.Spec.Repository.CloneOptions == nil {
			ws.Spec.Repository.CloneOptions = &pb.Workspace_Spec_Repository_CloneOptions{}
		}
		if o.Branch != "" {
			ws.Spec.Repository.CloneOptions.Branch = o.Branch
		}
		if o.Depth > 0 {
			ws.Spec.Repository.CloneOptions.Depth = o.Depth
		}
		if o.Checkout != "" {
			ws.Spec.Repository.CloneOptions.Checkout = o.Checkout
		}
	}

	if o.Image != "" {
		ws.Spec.Image = &pb.Workspace_Spec_Image{
			Type: &pb.Workspace_Spec_Image_Registry_{
				Registry: &pb.Workspace_Spec_Image_Registry{
					Url: o.Image,
				},
			},
		}
	} else if o.Dockerfile != "" {
		dockerfileBytes, err := os.ReadFile(o.Dockerfile)
		if err != nil {
			return nil, err
		}
		ws.Spec.Image = &pb.Workspace_Spec_Image{
			Type: &pb.Workspace_Spec_Image_Dockerfile_{
				Dockerfile: &pb.Workspace_Spec_Image_Dockerfile{
					Type: &pb.Workspace_Spec_Image_Dockerfile_Inline{
						Inline: string(dockerfileBytes),
					},
				},
			},
		}
	}

	if len(o.EnvVars) > 0 || len(o.EnvVarFromSecrets) > 0 {
		if ws.Spec.Runtime == nil {
			ws.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}
		for _, raw := range o.EnvVars {
			key, val, ok := strings.Cut(raw, "=")
			if !ok || key == "" {
				return nil, errors.Errorf("invalid --env value %q: expected KEY=VALUE", raw)
			}
			ws.Spec.Runtime.EnvVars = append(ws.Spec.Runtime.EnvVars, &pb.Workspace_Spec_Runtime_EnvVar{
				Key:  key,
				Type: &pb.Workspace_Spec_Runtime_EnvVar_Value{Value: val},
			})
		}
		for _, raw := range o.EnvVarFromSecrets {
			key, secretName, ok := strings.Cut(raw, "=")
			if !ok || key == "" || secretName == "" {
				return nil, errors.Errorf("invalid --env-from-secret value %q: expected KEY=SECRET_NAME", raw)
			}
			ws.Spec.Runtime.EnvVars = append(ws.Spec.Runtime.EnvVars, &pb.Workspace_Spec_Runtime_EnvVar{
				Key:  key,
				Type: &pb.Workspace_Spec_Runtime_EnvVar_FromSecret{FromSecret: secretName},
			})
		}
	}

	for _, raw := range o.AdditionalRepos {
		name, repoURL, ok := strings.Cut(raw, "=")
		if !ok || name == "" || repoURL == "" {
			return nil, errors.Errorf("invalid --additional-repo value %q: expected NAME=URL", raw)
		}
		ws.Spec.AdditionalRepositories = append(ws.Spec.AdditionalRepositories,
			&pb.Workspace_Spec_AdditionalRepository{
				Name: name,
				Repository: &pb.Workspace_Spec_Repository{
					Url: repoURL,
				},
			},
		)
	}

	if o.CPUMillicores > 0 || o.MemoryMB > 0 || o.StorageMB > 0 {
		if ws.Spec.Limit == nil {
			ws.Spec.Limit = &pb.Workspace_Spec_Limit{}
		}
		if o.CPUMillicores > 0 {
			ws.Spec.Limit.Cpu = &pb.Workspace_Spec_Limit_CPU{Millicores: o.CPUMillicores}
		}
		if o.MemoryMB > 0 {
			ws.Spec.Limit.Memory = &pb.Workspace_Spec_Limit_Memory{Megabytes: o.MemoryMB}
		}
		if o.StorageMB > 0 {
			ws.Spec.Limit.Storage = &pb.Workspace_Spec_Limit_Storage{Megabytes: o.StorageMB}
		}
	}

	for _, raw := range o.AppPorts {
		app, err := parseAppPort(raw)
		if err != nil {
			return nil, err
		}
		ws.Spec.Applications = append(ws.Spec.Applications, app)
	}

	ws.Spec.AutoStop = o.AutoStop

	ws.Metadata = &metav1.Metadata{}
	ws.Status = &pb.Workspace_Status{
		IsEphemeral: o.Ephemeral,
	}

	if o.Template != "" {
		ws.Status.TemplateRef = &metav1.ObjectReference{
			Name: o.Template,
		}
	} else if o.Space != "" {
		ws.Status.SpaceRef = &metav1.ObjectReference{
			Name: o.Space,
		}
	}

	ws, err = c.CreateWorkspace(ctx, ws)
	if err != nil {
		return nil, err
	}

	if o.Start {
		if _, err := c.StartWorkspace(ctx, &pb.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		}); err != nil {
			return nil, err
		}
	}

	return ws, nil
}

func parseAppPort(raw string) (*pb.Workspace_Spec_Application, error) {
	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 1:
		port, err := parsePort(parts[0])
		if err != nil {
			return nil, errors.Errorf("invalid --port value %q: %v", raw, err)
		}
		return &pb.Workspace_Spec_Application{
			Name: fmt.Sprintf("port-%s", parts[0]),
			Port: port,
		}, nil

	case 2:
		port, err := parsePort(parts[1])
		if err != nil {
			return nil, errors.Errorf("invalid --port value %q: %v", raw, err)
		}
		return &pb.Workspace_Spec_Application{
			Name: parts[0],
			Port: port,
		}, nil

	case 3:
		port, err := parsePort(parts[1])
		if err != nil {
			return nil, errors.Errorf("invalid --port value %q: %v", raw, err)
		}
		isDefault := parts[2] == "default"
		return &pb.Workspace_Spec_Application{
			Name:      parts[0],
			Port:      port,
			IsDefault: isDefault,
		}, nil

	default:
		return nil, errors.Errorf("invalid --port value %q: expected PORT, NAME:PORT, or NAME:PORT:default", raw)
	}
}

func parsePort(s string) (int32, error) {
	var port int32
	if _, err := fmt.Sscanf(s, "%d", &port); err != nil || port < 1 || port > 65535 {
		return 0, errors.Errorf("port must be a number between 1 and 65535, got %q", s)
	}
	return port, nil
}
