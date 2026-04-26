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

package ssh

import (
	"fmt"
	"os/exec"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/main/userv1"
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
	Use:   "ssh",
	Short: "SSH into a Workspace.",
	Example: `
cordium ssh abc
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.ExactArgs(1),
}

func doCmd(cmd *cobra.Command, args []string) error {

	ctx := cmd.Context()
	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	if _, err := exec.LookPath("ssh"); err != nil {
		return errors.Errorf("ssh binary does not exist. Please install it.")
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	{
		userC := userv1.NewMainServiceClient(conn)
		r, err := userC.GetStatus(ctx, &userv1.GetStatusRequest{})
		if err != nil {
			return err
		}

		if !r.Session.Status.IsConnected {
			return errors.Errorf(
				`Currently not connected to the Octelium Cluster. Please run "octelium connect" command first`)
		}
	}

	arg := i.FirstArg()
	ws, err := c.GetWorkspace(ctx, &metav1.GetOptions{
		Name: arg,
	})
	if err != nil {
		return err
	}
	if !ucordiumv1.ToWorkspace(ws).IsPreparingOrRunning() {
		return errors.Errorf("Workspace is not running")
	}

	cmdI := exec.CommandContext(ctx, "ssh",
		fmt.Sprintf("%s@%s-ssh.cordium.local.%s", ws.Metadata.Name, ws.Status.RegionRef.Name, i.Domain))

	cmdI.Stdin = cmd.InOrStdin()
	cmdI.Stdout = cmd.OutOrStdout()
	cmdI.Stderr = cmd.ErrOrStderr()

	return cmdI.Run()
}
