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

package delete

import (
	"github.com/octelium/cordium/client/cordium/commands/delete/gitprovider"
	"github.com/octelium/cordium/client/cordium/commands/delete/secret"
	"github.com/octelium/cordium/client/cordium/commands/delete/space"
	"github.com/octelium/cordium/client/cordium/commands/delete/template"
	"github.com/octelium/cordium/client/cordium/commands/delete/usersecret"
	"github.com/octelium/cordium/client/cordium/commands/delete/workspace"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a resource",
	Aliases: []string{"del", "rm"},
}

func AddSubcommands() {
	Cmd.AddCommand(workspace.Cmd)
	Cmd.AddCommand(usersecret.Cmd)
	Cmd.AddCommand(secret.Cmd)
	Cmd.AddCommand(space.Cmd)
	Cmd.AddCommand(template.Cmd)
	Cmd.AddCommand(gitprovider.Cmd)
}
