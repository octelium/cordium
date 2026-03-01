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

package code

import (
	"fmt"
	"os/exec"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type args struct {
	Dir string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Dir, "dir", "", "/workspace/repo",
		`Code Environment directory. By default it is set to Cordium default Repository directory`)
}

var Cmd = &cobra.Command{
	Use:   "code",
	Short: "Open a VScode instance for the Workspace",
	Example: `
cordium code abc
cordium code abc
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

	if _, err := exec.LookPath("code"); err != nil {
		return errors.Errorf(`"code" binary does not exist. You can download and install VScode via https://code.visualstudio.com/download`)
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

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

	cmdI := exec.CommandContext(ctx,
		"code",
		"--remote",
		fmt.Sprintf("ssh-remote+%s@%s-ssh.cordium.local.%s", ws.Metadata.Name, ws.Status.RegionRef.Name, i.Domain),
		cmdArgs.Dir,
	)

	cmdI.Stdin = cmd.InOrStdin()
	cmdI.Stdout = cmd.OutOrStdout()
	cmdI.Stderr = cmd.ErrOrStderr()

	return cmdI.Run()
}
