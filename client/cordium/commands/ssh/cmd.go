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

	"github.com/juju/errors"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	sshc "github.com/octelium/octelium/client/octelium/commands/ssh"
	"github.com/spf13/cobra"
)

type args struct {
	LocalForwards   []string
	DynamicForwards []string
	NoCommand       bool
}

var cmdArgs args

func init() {
	Cmd.Flags().StringArrayVarP(&cmdArgs.LocalForwards, "local", "L", nil,
		"Local port forward: [bind_addr:]port:host:hostport")
	Cmd.Flags().StringArrayVarP(&cmdArgs.DynamicForwards, "dynamic", "D", nil,
		"Dynamic (SOCKS5) forward: [bind_addr:]port")
	Cmd.Flags().BoolVarP(&cmdArgs.NoCommand, "no-command", "N", false,
		"Do not execute a remote command (useful for port forwarding only)")
}

var Cmd = &cobra.Command{
	Use:   "ssh <workspace-name> [-- command [args...]]",
	Short: "Open an SSH session to a Workspace",
	Long: `Open an interactive SSH session or execute a remote command on a connected
a running Workspace using its name.

A remote command and its arguments can be passed after a double-dash (--).
If no command is given, an interactive shell is opened.`,
	Example: `
  # Open an interactive shell
  octelium ssh abc

  # Run a single remote command
  octelium ssh abc -- uptime

  # Run a shell pipeline
  octelium ssh abc -- sh -c "ps aux | grep python"

  # Local port forward: forward local :5432 to remote localhost:5432
  octelium ssh abc -L 5432:localhost:5432

  # Multiple port forwards, no interactive shell
  octelium ssh abc -N \
    -L 5432:localhost:5432 \
    -L 6379:localhost:6379

  # Dynamic SOCKS5 proxy on local port 1080
  octelium ssh abc -D 1080 -N`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.MinimumNArgs(1),
}

func doCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	wsName := args[0]

	var remoteCommand []string
	if len(args) > 1 {
		remoteCommand = args[1:]
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	ws, err := c.GetWorkspace(ctx, &metav1.GetOptions{Name: wsName})
	if err != nil {
		return errors.Errorf("could not get Workspace %q: %+v", wsName, err)
	}
	if !ucordiumv1.ToWorkspace(ws).IsPreparingOrRunning() {
		return errors.Errorf("Workspace %q is not running (state: %s)", wsName, ws.Status.State.String())
	}

	return sshc.DoCommand(ctx, &sshc.DoCommandOpts{
		Domain:          i.Domain,
		Service:         fmt.Sprintf("%s-ssh.cordium", ws.Status.RegionRef.Name),
		SSHUser:         wsName,
		Command:         remoteCommand,
		NoCommand:       cmdArgs.NoCommand,
		DynamicForwards: cmdArgs.DynamicForwards,
		LocalForwards:   cmdArgs.LocalForwards,
	})
}
