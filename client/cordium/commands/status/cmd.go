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

package status

import (
	"fmt"

	"github.com/octelium/octelium/apis/main/userv1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
	Out string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "yaml", "Output format")
}

var Cmd = &cobra.Command{
	Use:   "status",
	Short: "Show general information about User and Cluster",
	Example: `
cordium status
cordium status -o yaml
cordium status -o json
	`,

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

	c := userv1.NewMainServiceClient(conn)

	resp, err := c.GetStatus(cmd.Context(), &userv1.GetStatusRequest{})
	if err != nil {
		return err
	}

	if cmdArgs.Out != "" {
		out, err := cliutils.OutFormatPrint(cmdArgs.Out, resp)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(out))
		return nil
	}

	return nil
}
