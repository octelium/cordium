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

	"github.com/octelium/cordium/client/cordium/commands/create/workspace"
	"github.com/octelium/cordium/client/cordium/commands/terminal"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type args struct {
	workspace.CreateWorkspaceArgs
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Space, "space", "", "", "Parent Space")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Template, "template", "", "", "Parent Template")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.File, "file", "", "", "Spec file path")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.Start, "start", "", false, "Start the Workspace after creation")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Repo, "repository", "", "", "Repository URL")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Image, "image", "", "", `Container image URL (e.g. "ubuntu:latest")`)
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Dockerfile, "dockerfile", "", "",
		`Provide a Dockerfile file path to build the Container image from it`)
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.Ephemeral, "ephemeral", "", false, "Set the Workspace storage to be ephemeral")
}

var Cmd = &cobra.Command{
	Use:   "run",
	Short: "Run a Workspace",
	Example: `
cordium run abc
	`,
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
			Space:      cmdArgs.Space,
			Template:   cmdArgs.Template,
			File:       cmdArgs.File,
			Start:      cmdArgs.Start,
			Repo:       cmdArgs.Repo,
			Image:      cmdArgs.Image,
			Dockerfile: cmdArgs.Dockerfile,
			Ephemeral:  cmdArgs.Ephemeral,
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

	return doRun(ctx, conn, ws)
}

func doRun(ctx context.Context, conn *grpc.ClientConn, ws *pb.Workspace) error {
	c := pb.NewMainServiceClient(conn)

	var doWaitStarting bool
	switch {
	case ucordiumv1.ToWorkspace(ws).IsRunning():
	case ucordiumv1.ToWorkspace(ws).IsStopping():
		return errors.Errorf("Workspace is stopping")
	case ucordiumv1.ToWorkspace(ws).IsStopped():
		if _, err := c.StartWorkspace(ctx, &pb.StartWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		}); err != nil {
			return err
		}

		doWaitStarting = true

	}

	if doWaitStarting {

		zap.L().Debug("Starting watchWorkspace")
		strm, err := c.WatchWorkspace(ctx, &pb.WatchWorkspaceRequest{})
		if err != nil {
			return err
		}

		isSameWorkspace := func(itm *pb.Workspace) bool {
			return itm.Metadata.Uid == ws.Metadata.Uid
		}

		if err := func() error {
			for {
				msg, err := strm.Recv()
				if err != nil {
					return err
				}

				switch msg.Type.(type) {
				case *pb.WatchWorkspaceResponse_Update_:
					cur := msg.GetUpdate().NewItem
					if !isSameWorkspace(cur) {
						continue
					}

					zap.L().Debug("Got Workspace update", zap.String("state", cur.Status.State.String()))

					switch {
					case ucordiumv1.ToWorkspace(msg.GetUpdate().NewItem).IsPreparingOrRunning():
						return nil
					}
				}
			}
		}(); err != nil {
			return err
		}
	}

	return terminal.DoCmdTerminal(ctx, conn, ws.Metadata.Name)
}
