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

package main

import (
	"os"

	"github.com/fatih/color"
	"github.com/octelium/cordium/client/cordium/commands"
	"github.com/spf13/cobra"
)

func init() {
	cobra.OnInitialize()
	commands.InitCmds()
}

func main() {
	if err := commands.Cmd.Execute(); err != nil {
		color.New(color.FgRed, color.Bold).Printf("%s\n", err)
		os.Exit(1)
	}
}
