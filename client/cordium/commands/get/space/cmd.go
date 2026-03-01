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

package space

import (
	"fmt"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/common/printer"
	"github.com/spf13/cobra"
)

type args struct {
	Out string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")
}

var Cmd = &cobra.Command{
	Use:   "space",
	Short: "List Spaces",
	Example: `
cordium get space
cordium get spc -o json
cordium get spaces -o yaml
	`,
	Aliases: []string{"spaces", "spc"},
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
		res, err := c.GetSpace(cmd.Context(), &metav1.GetOptions{
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

	itmList, err := c.ListSpace(cmd.Context(), &pb.ListSpaceOptions{})
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
		cliutils.LineInfo("No Spaces Found\n")
		return nil
	}

	p := printer.NewPrinter("Name", "Description")
	for _, net := range itmList.Items {
		p.AppendRow(net.Metadata.Name, net.Metadata.Description)
	}

	p.Render()

	return nil
}
