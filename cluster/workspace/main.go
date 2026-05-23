/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
