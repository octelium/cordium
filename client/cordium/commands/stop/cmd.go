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

package stop

import (
	"os"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type args struct {
}

var cmdArgs args

func init() {
}

var Cmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a Workspace",
	Example: `
cordium stop abc
# Stop the Workspace from within the Workspace
cordium stop
# With an environment variable
CORDIUM_NAME=abc cordium stop
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

	name := i.FirstArg()
	if name == "" {
		name = os.Getenv("CORDIUM_NAME")
	}
	if name == "" {
		return errors.Errorf("You need to provide the Workspace name")
	}

	if _, err := c.StopWorkspace(ctx, &pb.StopWorkspaceRequest{
		WorkspaceRef: &metav1.ObjectReference{
			Name: name,
		},
	}); err != nil {
		return err
	}

	cliutils.LineNotify("Successfully stopped the Workspace: %s\n", name)

	return nil
}
