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

package gitprovider

import (
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "gitprovider <name> [flags]",
	Short: "Delete a GitProvider",
	Example: `
  # Delete a GitProvider by qualified name
  cordium delete gitprovider my-github.my-project

  # Delete using a short name (Space inferred from context)
  cordium del gp my-github`,
	Aliases: []string{"gitproviders", "gp"},
	Args:    cobra.ExactArgs(1),
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

	_, err = c.DeleteGitProvider(ctx, &metav1.DeleteOptions{
		Name: i.FirstArg(),
	})
	if err != nil {
		return err
	}

	cliutils.LineNotify("Successfully deleted GitProvider: %s\n", i.FirstArg())

	return nil
}
