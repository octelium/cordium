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
}

var Cmd = &cobra.Command{
	Use:   "template",
	Short: "List Templates",
	Example: `
cordium get template
cordium get tmpl -o json
cordium get templates -o yaml
	`,
	Aliases: []string{"templates", "tmpl"},
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
		res, err := c.GetTemplate(cmd.Context(), &metav1.GetOptions{
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

	listOpts := &pb.ListTemplateOptions{}

	if cmdArgs.Space != "" {
		listOpts.SpaceRef = &metav1.ObjectReference{
			Name: cmdArgs.Space,
		}
	}

	itmList, err := c.ListTemplate(cmd.Context(), listOpts)
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
		cliutils.LineInfo("No Templates Found\n")
		return nil
	}

	p := printer.NewPrinter("Name", "Created", "Space")
	for _, itm := range itmList.Items {
		p.AppendRow(ccommon.GetResourceShortName(itm),
			cliutils.GetResourceAge(itm),
			ccommon.GetResourceRefShortName(itm.Status.SpaceRef))
	}

	p.Render()

	return nil
}
