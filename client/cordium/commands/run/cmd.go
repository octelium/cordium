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

package run

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/octelium/cordium/client/cordium/commands/create/workspace"
	"github.com/octelium/cordium/client/cordium/commands/terminal"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type args struct {
	workspace.CreateWorkspaceArgs
	DoRemove bool
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Space, "space", "", "", "Parent Space name (e.g. my-project)")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Template, "template", "", "", "Parent Template name (e.g. ml-env.my-project)")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.File, "file", "", "", "Path to a Workspace YAML spec file")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Repo, "repository", "", "", "Primary repository URL to clone into /workspace/repo")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Image, "image", "", "", `Container image URL (e.g. "ubuntu:24.04", "python:3.11-slim")`)
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Dockerfile, "dockerfile", "", "",
		"Path to a local Dockerfile. The file is read and embedded inline. COPY/ADD with local context paths are not supported.")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.Ephemeral, "ephemeral", "", false, "Create an ephemeral Workspace whose storage is discarded on stop")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.DoRemove, "rm", "", false, "Delete the Workspace after the terminal session ends")

	Cmd.PersistentFlags().StringVarP(&cmdArgs.Branch, "branch", "b", "", "Branch to clone when using --repository (default: repository default branch)")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.Depth, "depth", 0, "Shallow clone depth when using --repository (0 = full clone)")
	Cmd.PersistentFlags().StringVar(&cmdArgs.Checkout, "checkout", "", "Specific commit, tag, or ref to checkout when using --repository")

	Cmd.PersistentFlags().StringArrayVarP(&cmdArgs.EnvVars, "env", "e", nil,
		"Set an environment variable in the Workspace (KEY=VALUE). Repeatable: -e FOO=bar -e BAZ=qux")
	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.EnvVarFromSecrets, "env-from-secret", nil,
		"Set an environment variable from a Space Secret (KEY=SECRET_NAME). Repeatable: --env-from-secret DB_URL=my-db-secret")

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.AdditionalRepos, "additional-repo", nil,
		"Clone an additional repository into the Workspace (NAME=URL). Repeatable: --additional-repo shared=https://github.com/org/shared")

	Cmd.PersistentFlags().Uint32Var(&cmdArgs.CPUMillicores, "cpu", 0, "CPU limit in millicores (e.g. 2000 = 2 cores). Uses Space/Cluster default if unset.")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.MemoryMB, "memory", 0, "Memory limit in megabytes (e.g. 4096 = 4 GB). Uses Space/Cluster default if unset.")
	Cmd.PersistentFlags().Uint32Var(&cmdArgs.StorageMB, "storage", 0, "Storage limit in megabytes (e.g. 20000 = 20 GB). Uses Space/Cluster default if unset.")

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.AppPorts, "port", nil,
		"Expose a named application port (NAME:PORT or PORT for unnamed). Repeatable. Append :default to mark as default app: --port web:3000:default")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.AutoStop, "auto-stop", "", false, "Automatically stop the Workspace after running all POST_START tasks")

	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.Vars, "var", nil,
		`Set a variable (NAME=VALUE). Repeatable: --var BRANCH=main --var SERVICE=payments`)

	Cmd.PersistentFlags().BoolVar(&cmdArgs.ServeAll, "serve-all", false,
		"Serve all Octelium services assigned to the User")
	Cmd.PersistentFlags().StringSliceVar(&cmdArgs.ServeServices, "serve", nil,
		"Select the Octelium Service names assigned to this User to be served")

	Cmd.MarkFlagsMutuallyExclusive("space", "template")
	Cmd.MarkFlagsMutuallyExclusive("image", "dockerfile")
}

var Cmd = &cobra.Command{
	Use:   "run [workspace-name] [flags]",
	Short: "Run a Workspace",
	Long: `Create a new Workspace, or start it if it already exists, and then run a terminal.

After the Workspace reaches RUNNING state, an interactive terminal session is
opened. When the terminal session ends, the Workspace keeps running unless
--rm is given, in which case it is deleted automatically.`,
	Example: `
  # Create a new Workspace from the default Template and open a terminal
  cordium run

  # Run a terminal for an existing Workspace, start it if it is stopped
  cordium run abc

  # Create from a specific Template and open a terminal
  cordium run --template backend-service.my-project

  # Create from a YAML spec file and open a terminal
  cordium run --file workspace.yaml

  # Create from a container image
  cordium run --image python:3.11

  # Create from a local Dockerfile
  cordium run --dockerfile ./Dockerfile

  # Create from a git repository
  cordium run --repository https://github.com/myorg/my-project

  # Clone a specific branch at a shallow depth
  cordium run --repository https://github.com/myorg/my-project --branch develop --depth 1

  # Clone a repository and check out a specific tag
  cordium run --repository https://github.com/myorg/my-project --checkout v2.3.0

  # Run a Workspace that is deleted when the terminal exits
  cordium run --rm

  # Run a Workspace from an image with a terminal, then delete it
  cordium run --rm --image ubuntu:24.04

  # Set environment variables at creation time
  cordium run --image node:20 -e NODE_ENV=development -e PORT=3000

  # Source an environment variable from a Space Secret
  cordium run --image python:3.11 --env-from-secret DATABASE_URL=my-db-secret

  # Mix static and secret-sourced environment variables
  cordium run --template ml-env.research \
    -e WANDB_PROJECT=my-experiment \
    --env-from-secret WANDB_API_KEY=wandb-secret \
    --env-from-secret HF_TOKEN=huggingface-secret

  # Clone the primary repository and an additional shared library
  cordium run --repository https://github.com/myorg/api-service \
    --additional-repo shared-lib=https://github.com/myorg/shared-lib \
    --additional-repo proto=https://github.com/myorg/proto-defs

  # Override resource limits
  cordium run --template ml-env.research --cpu 8000 --memory 16384 --storage 50000

  # Expose named application ports
  cordium run --image node:20 \
    --repository https://github.com/myorg/fullstack \
    --port web:3000:default \
    --port api:8080 \
    --port storybook:6006

  # Ephemeral AI agent sandbox with secrets and resource bounds
  cordium run --ephemeral --rm \
    --image python:3.11-slim \
    --env-from-secret ANTHROPIC_API_KEY=anthropic-key \
    --cpu 2000 --memory 4096 --storage 10000

  # Combine a YAML file with inline overrides
  cordium run --file base.yaml \
    -e ENVIRONMENT=staging \
    --env-from-secret DB_PASSWORD=staging-db-password \
    --cpu 4000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.MaximumNArgs(1),
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

	var ws *pb.Workspace
	if i.FirstArg() == "" {
		ws, err = workspace.DoCreateWorkspace(ctx, c, &workspace.DoCreateWorkspaceOpts{
			Space:             cmdArgs.Space,
			Template:          cmdArgs.Template,
			File:              cmdArgs.File,
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
		})
		if err != nil {
			return err
		}
	} else {
		ws, err = c.GetWorkspace(ctx, &metav1.GetOptions{
			Name: i.FirstArg(),
		})
		if err != nil {
			return err
		}
	}

	if err := doRun(ctx, conn, ws); err != nil {
		zap.L().Debug("doRun exited with err", zap.Error(err))
	}

	if cmdArgs.DoRemove {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()

		if _, err := c.DeleteWorkspace(ctx, &metav1.DeleteOptions{
			Uid: ws.Metadata.Uid,
		}); err != nil {
			return err
		}
	}

	return nil
}

func doRun(ctx context.Context, conn *grpc.ClientConn, ws *pb.Workspace) error {
	c := pb.NewMainServiceClient(conn)

	var doWaitStarting bool
	switch {
	case ucordiumv1.ToWorkspace(ws).IsPreRunning() || ucordiumv1.ToWorkspace(ws).IsRunning():
	case ucordiumv1.ToWorkspace(ws).IsStopping():
		return errors.Errorf("Workspace is stopping")
	case ucordiumv1.ToWorkspace(ws).IsStopped():
		if _, err := c.StartWorkspace(ctx, &pb.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		}); err != nil {
			if !grpcerr.AlreadyExists(err) {
				return err
			}
		}
		doWaitStarting = true
	}

	if doWaitStarting {
		zap.L().Debug("Starting watchWorkspace")
		strm, err := c.WatchWorkspace(ctx, &pb.WatchWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		if err != nil {
			return err
		}

		if err := func() error {
			s := cliutils.NewSpinner(os.Stdout)
			s.SetSuffix("Waiting for the Workspace to run")
			s.Start()
			defer s.Stop()

			for {
				msg, err := strm.Recv()
				if err != nil {
					return err
				}

				switch msg.Type.(type) {
				case *pb.WatchWorkspaceResponse_Update_:
					cur := msg.GetUpdate().NewItem
					old := msg.GetUpdate().OldItem
					zap.L().Debug("Got Workspace update", zap.String("state", cur.Status.State.String()))

					if cur.Status.State != old.Status.State {
						s.SetSuffix(fmt.Sprintf("Workspace status: %s", cur.Status.State.String()))
					}

					switch {
					case ucordiumv1.ToWorkspace(cur).IsPreparingOrRunning():
						return nil
					case ucordiumv1.ToWorkspace(cur).IsStopped():
						if cur.Status.Failure != nil {
							return errors.Errorf("Workspace failed to start")
						}
						return errors.Errorf("Workspace stopped unexpectedly during startup")
					}
				}
			}
		}(); err != nil {
			return err
		}
	}

	return terminal.DoCmdTerminal(ctx, conn, ws.Metadata.Name)
}
