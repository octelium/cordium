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

package volume

import (
	"fmt"

	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/common/printer"
	"github.com/spf13/cobra"
)

type args struct {
	Out   string
	Space string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Space, "space", "", "", "Filter by Space")
}

var Cmd = &cobra.Command{
	Use:   "volume [name] [flags]",
	Short: "Get or list Volumes",
	Example: `
  # List all Volumes of the default Space
  cordium get volumes

  # Get a specific Volume
  cordium get vol datasets

  # List the Volumes of a Space
  cordium get volumes --space my-project

  # Output a specific Volume as JSON
  cordium get vol datasets -o json`,
	Aliases: []string{"volumes", "vol"},
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
		res, err := c.GetVolume(cmd.Context(), &metav1.GetOptions{
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

	listOpts := &pb.ListVolumeOptions{}

	if cmdArgs.Space != "" {
		listOpts.SpaceRef = &metav1.ObjectReference{
			Name: cmdArgs.Space,
		}
	} else {
		listOpts.SpaceRef = &metav1.ObjectReference{
			Name: "default",
		}
	}

	itmList, err := c.ListVolume(cmd.Context(), listOpts)
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
		cliutils.LineInfo("No Volumes Found\n")
		return nil
	}

	p := printer.NewPrinter("Name", "Created", "Space", "State", "Access", "Size")
	for _, itm := range itmList.Items {
		p.AppendRow(
			ccommon.GetResourceShortName(itm),
			cliutils.GetResourceAge(itm),
			ccommon.GetResourceRefShortName(itm.Status.SpaceRef),
			itm.Status.State.String(),
			itm.Spec.AccessMode.String(),
			fmt.Sprintf("%d MB", itm.Spec.Size.GetMegabytes()),
		)
	}

	p.Render()

	return nil
}
