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
	"fmt"
	"time"

	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/common/printer"
	"github.com/spf13/cobra"
)

type args struct {
	Out      string
	Space    string
	Template string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Space, "space", "", "", "Filter by Space")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Template, "template", "", "", "Filter by Template")
}

var Cmd = &cobra.Command{
	Use:   "workspace [name] [flags]",
	Short: "Get or list Workspaces",
	Example: `
  # List all Workspaces
  cordium get workspaces

  # Get a specific Workspace
  cordium get ws abc

  # List Workspaces in a Space
  cordium get ws --space my-project

  # List Workspaces from a specific Template
  cordium get ws --template ml-env.my-project

  # Output a specific Workspace as JSON
  cordium get ws abc -o json

  # Output all Workspaces as YAML
  cordium get workspaces -o yaml`,
	Aliases: []string{"workspaces", "ws"},
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
}

func doCmd(cmd *cobra.Command, args []string) error {
	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	conn, err := client.GetGRPCClientConn(cmd.Context(), i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	if i.FirstArg() != "" {
		res, err := c.GetWorkspace(cmd.Context(), &metav1.GetOptions{
			Name: i.FirstArg(),
		})
		if err != nil {
			return err
		}
		out, err := cliutils.OutFormatPrint(cmdArgs.Out, res)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(out))
		return nil
	}

	listOpts := &pb.ListWorkspaceOptions{}

	if cmdArgs.Space != "" {
		listOpts.Filter = &pb.ListWorkspaceOptions_SpaceRef{
			SpaceRef: &metav1.ObjectReference{
				Name: cmdArgs.Space,
			},
		}
	} else if cmdArgs.Template != "" {
		listOpts.Filter = &pb.ListWorkspaceOptions_TemplateRef{
			TemplateRef: &metav1.ObjectReference{
				Name: cmdArgs.Template,
			},
		}
	}

	itmList, err := c.ListWorkspace(cmd.Context(), listOpts)
	if err != nil {
		return err
	}

	if cmdArgs.Out != "" {
		out, err := cliutils.OutFormatPrint(cmdArgs.Out, itmList)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(out))
		return nil
	}

	if len(itmList.Items) == 0 {
		cliutils.LineInfo("No Workspaces Found\n")
		return nil
	}

	p := printer.NewPrinter("Name", "Created", "Template", "Space", "State", "Last Started")
	for _, itm := range itmList.Items {
		p.AppendRow(itm.Metadata.Name,
			cliutils.GetResourceAge(itm),
			ccommon.GetResourceRefShortName(itm.Status.TemplateRef),
			ccommon.GetResourceRefShortName(itm.Status.SpaceRef),
			itm.Status.State.String(),
			cliutils.GetAgeFromTimestampMust(itm.Status.LastInitializedAt.AsTime().Format(time.RFC3339Nano)))
	}

	p.Render()

	return nil
}
