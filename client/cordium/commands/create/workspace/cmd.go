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

	Vars []string

	ServeServices []string
	ServeAll      bool

	ReadOnlyRootFilesystem bool

	AddCaps  []string
	DropCaps []string

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

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.Vars, "var", nil,
		`Set a variable (NAME=VALUE). Repeatable: --var BRANCH=main --var SERVICE=payments`)

	Cmd.PersistentFlags().BoolVar(&cmdArgs.ServeAll, "serve-all", false,
		"Serve all Octelium services assigned to the User")
	Cmd.PersistentFlags().StringSliceVar(&cmdArgs.ServeServices, "serve", nil,
		"Select the Octelium Service names assigned to this User to be served")

	Cmd.PersistentFlags().BoolVar(&cmdArgs.ReadOnlyRootFilesystem, "read-only", false,
		"Use read-only root filesystem")

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.AddCaps, "cap-add", nil,
		"Add a Linux capability to the Workspace container (repeatable: --cap-add NET_ADMIN --cap-add SYS_PTRACE)")
	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.DropCaps, "cap-drop", nil,
		"Drop a Linux capability from the Workspace container (repeatable: --cap-drop NET_RAW)")

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
--storage, --port, and --var can be combined with --file to override or extend
values from the YAML spec.`,
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

  # Create an ephemeral Workspace from a container image
  cordium create ws --ephemeral --image python:3.11-slim

  # Create a Workspace from a git repository
  cordium create ws --repository https://github.com/myorg/my-project --branch develop --depth 1

  # Set environment variables
  cordium create ws --image node:20 -e NODE_ENV=development -e LOG_LEVEL=debug

  # Set an environment variable from a Space Secret
  cordium create ws --image python:3.11 --env-from-secret DATABASE_URL=my-db-secret

  # Override Template variables
  cordium create ws --template go-build.my-project \
    --var BRANCH=feat/new-auth \
    --var SERVICE=services/payments

  # Ephemeral CI run from a parameterized Template
  cordium create ws --template ci-runner.my-project \
    --ephemeral \
    --var BRANCH=main \
    --var RUN_TESTS=true \
    --auto-stop \
    --start

  # Combine a YAML file with variable and environment overrides
  cordium create ws --file base.yaml \
    --var BRANCH=staging \
    -e ENVIRONMENT=staging \
    --env-from-secret DB_PASSWORD=staging-db-password

  # Override resource limits
  cordium create ws --template ml-env.research --cpu 8000 --memory 16384 --storage 50000

  # Expose named application ports
  cordium create ws --image node:20 \
    --port web:3000:default \
    --port api:8080 \
    --port storybook:6006

  # Use the "ws" alias
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
		Vars:              cmdArgs.Vars,

		ServeServices: cmdArgs.ServeServices,
		ServeAll:      cmdArgs.ServeAll,

		ReadOnlyRootFilesystem: cmdArgs.ReadOnlyRootFilesystem,

		AddCaps:  cmdArgs.AddCaps,
		DropCaps: cmdArgs.DropCaps,
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

	ServeServices []string
	ServeAll      bool

	AppPorts []string

	ReadOnlyRootFilesystem bool

	AddCaps  []string
	DropCaps []string

	Vars []string
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

	if o.ServeAll {
		if ws.Spec.Runtime == nil {
			ws.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}

		if ws.Spec.Runtime.Octelium == nil {
			ws.Spec.Runtime.Octelium = &pb.Workspace_Spec_Runtime_Octelium{}
		}

		ws.Spec.Runtime.Octelium.ServeAll = o.ServeAll
	}

	if len(o.ServeServices) > 0 {
		if ws.Spec.Runtime == nil {
			ws.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}

		if ws.Spec.Runtime.Octelium == nil {
			ws.Spec.Runtime.Octelium = &pb.Workspace_Spec_Runtime_Octelium{}
		}

		ws.Spec.Runtime.Octelium.ServeServices = o.ServeServices
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

	if len(o.AddCaps) > 0 || len(o.DropCaps) > 0 {
		if ws.Spec.Runtime == nil {
			ws.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}
		if ws.Spec.Runtime.Capabilities == nil {
			ws.Spec.Runtime.Capabilities = &pb.Workspace_Spec_Runtime_Capabilities{}
		}
		ws.Spec.Runtime.Capabilities.Add = append(
			ws.Spec.Runtime.Capabilities.Add, o.AddCaps...)
		ws.Spec.Runtime.Capabilities.Drop = append(
			ws.Spec.Runtime.Capabilities.Drop, o.DropCaps...)
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

	for _, raw := range o.Vars {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || name == "" {
			return nil, errors.Errorf("invalid --var value %q: expected NAME=VALUE", raw)
		}
		ws.Spec.Vars = append(ws.Spec.Vars, &pb.Workspace_Spec_Var{
			Name:  name,
			Value: value,
		})
	}

	if o.ReadOnlyRootFilesystem {
		if ws.Spec.Runtime == nil {
			ws.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}

		if ws.Spec.Runtime.Filesystem == nil {
			ws.Spec.Runtime.Filesystem = &pb.Workspace_Spec_Runtime_Filesystem{}
		}

		ws.Spec.Runtime.Filesystem.ReadOnly = o.ReadOnlyRootFilesystem
	}

	if o.AutoStop {
		if ws.Spec.Runtime == nil {
			ws.Spec.Runtime = &pb.Workspace_Spec_Runtime{}
		}

		ws.Spec.Runtime.AutoStop = o.AutoStop
	}

	ws.Spec.IsEphemeral = o.Ephemeral

	ws.Metadata = &metav1.Metadata{}

	if o.Template != "" {
		ws.Status.TemplateRef = &metav1.ObjectReference{
			Name: o.Template,
		}
	} else if o.Space != "" {
		ws.Status.TemplateRef = &metav1.ObjectReference{
			Name: fmt.Sprintf("default.%s", o.Space),
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
