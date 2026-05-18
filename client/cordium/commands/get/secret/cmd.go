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

package secret

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
	Use:   "secret [name] [flags]",
	Short: "Get or list Secrets",
	Example: `
  # List all Secrets
  cordium get secrets

  # Get a specific Secret
  cordium get sec db-password.my-project

  # List Secrets in a Space
  cordium get sec --space my-project

  # Output a specific Secret as JSON
  cordium get sec db-password.my-project -o json

  # Output all Secrets as YAML
  cordium get secrets -o yaml`,
	Aliases: []string{"secrets", "sec"},
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
		res, err := c.GetSecret(cmd.Context(), &metav1.GetOptions{
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

	listOpts := &pb.ListSecretOptions{}

	if cmdArgs.Space != "" {
		listOpts.SpaceRef = &metav1.ObjectReference{
			Name: cmdArgs.Space,
		}
	} else {
		listOpts.SpaceRef = &metav1.ObjectReference{
			Name: "default",
		}
	}

	itmList, err := c.ListSecret(cmd.Context(), listOpts)
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
		cliutils.LineInfo("No Secrets Found\n")
		return nil
	}

	p := printer.NewPrinter("Name", "Created", "Space")
	for _, itm := range itmList.Items {
		p.AppendRow(
			ccommon.GetResourceShortName(itm),
			cliutils.GetResourceAge(itm),
			ccommon.GetResourceRefShortName(itm.Status.SpaceRef),
		)
	}

	p.Render()

	return nil
}
