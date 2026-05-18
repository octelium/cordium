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

package commands

import (
	"github.com/octelium/cordium/client/cordium/commands/build"
	"github.com/octelium/cordium/client/cordium/commands/code"
	"github.com/octelium/cordium/client/cordium/commands/cp"
	"github.com/octelium/cordium/client/cordium/commands/create"
	"github.com/octelium/cordium/client/cordium/commands/delete"
	"github.com/octelium/cordium/client/cordium/commands/exec"
	"github.com/octelium/cordium/client/cordium/commands/get"
	"github.com/octelium/cordium/client/cordium/commands/logs"
	"github.com/octelium/cordium/client/cordium/commands/man"
	"github.com/octelium/cordium/client/cordium/commands/run"
	"github.com/octelium/cordium/client/cordium/commands/ssh"
	"github.com/octelium/cordium/client/cordium/commands/start"
	"github.com/octelium/cordium/client/cordium/commands/status"
	"github.com/octelium/cordium/client/cordium/commands/stop"
	"github.com/octelium/cordium/client/cordium/commands/terminal"
	"github.com/octelium/cordium/client/cordium/commands/version"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/common/commands/auth"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "cordium",
	Short: "Cordium enables you to manage and use containerized development environments to develop and access your Octelium Services",

	// SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return cliutils.PreRun(cmd, args)
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return cliutils.PostRun(cmd, args)
	},
}

func InitCmds() {

	Cmd.AddCommand(get.Cmd)
	Cmd.AddCommand(version.Cmd)
	Cmd.AddCommand(status.Cmd)
	Cmd.AddCommand(create.Cmd)
	Cmd.AddCommand(delete.Cmd)

	Cmd.AddCommand(start.Cmd)
	Cmd.AddCommand(stop.Cmd)
	Cmd.AddCommand(build.Cmd)

	Cmd.AddCommand(ssh.Cmd)
	Cmd.AddCommand(cp.Cmd)
	Cmd.AddCommand(code.Cmd)
	Cmd.AddCommand(auth.Cmd)
	Cmd.AddCommand(terminal.Cmd)
	Cmd.AddCommand(exec.Cmd)
	Cmd.AddCommand(run.Cmd)
	Cmd.AddCommand(man.Cmd)
	Cmd.AddCommand(logs.Cmd)

	get.AddSubcommands()
	create.AddSubcommands()
	delete.AddSubcommands()
	auth.AddSubcommands()
	man.AddSubcommands()
}

func init() {
	Cmd.PersistentFlags().String("domain", "", "The Cluster Domain")
	Cmd.PersistentFlags().String("homedir", "", "Override Octelium home directory")
	Cmd.PersistentFlags().Bool("logout", false, `Log out after executing the command. This is useful when using commands such as "octelium connect" inside ephemeral environments such as containers`)
}
