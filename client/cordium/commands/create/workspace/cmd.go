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

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
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
}
type args struct {
	Out string
	CreateWorkspaceArgs
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")
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
	Use:   "workspace",
	Short: "Create a Workspace",
	Example: `
cordium create workspace
cordium create ws -o json
cordium create workspaces
	`,
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
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	ws, err := DoCreateWorkspace(ctx, c, &DoCreateWorkspaceOpts{
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

	if cmdArgs.Start {
		cliutils.LineNotify("Successfully created and started Workspace: %s\n", ws.Metadata.Name)
	} else {
		cliutils.LineNotify("Successfully created Workspace: %s\n", ws.Metadata.Name)
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
	} else {
		if o.Repo != "" {
			ws.Spec.Repository = &pb.Workspace_Spec_Repository{
				Url: o.Repo,
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
			dockerFile, err := os.ReadFile(o.Dockerfile)
			if err != nil {
				return nil, err
			}
			ws.Spec.Image = &pb.Workspace_Spec_Image{
				Type: &pb.Workspace_Spec_Image_Dockerfile_{
					Dockerfile: &pb.Workspace_Spec_Image_Dockerfile{
						Type: &pb.Workspace_Spec_Image_Dockerfile_Inline{
							Inline: string(dockerFile),
						},
					},
				},
			}
		}
	}

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
