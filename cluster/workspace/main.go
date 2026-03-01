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
	"context"

	wsc "github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/workspace/workspace"
	"github.com/octelium/octelium/cluster/common/components"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:  "workspace",
	Long: `workspace`,
}

var serveCmd = &cobra.Command{
	Use: "serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := workspace.Run(cmd.Context())
		if err != nil {
			zap.L().Fatal("main err", zap.Error(err))
		}

		return nil
	},
}

func init() {
	components.SetComponentNamespace(wsc.ComponentNamespaceCordium)
	components.SetComponentType(wsc.Workspace)
}

func main() {

	components.RunComponent(func(ctx context.Context) error {
		rootCmd.SetContext(ctx)
		rootCmd.AddCommand(serveCmd)

		if err := rootCmd.Execute(); err != nil {
			zap.L().Fatal("main err", zap.Error(err))
		}
		return nil
	}, nil)
}
