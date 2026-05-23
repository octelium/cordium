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

package start

import (
	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
	Vars []string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.Vars, "var", nil,
		"Override a Template variable for this run (NAME=VALUE). "+
			"Takes precedence over Workspace and Template spec vars. "+
			"Repeatable: --var BRANCH=main --var TASK='fix the tests'")
}

var Cmd = &cobra.Command{
	Use:   "start <workspace> [flags]",
	Short: "Start a Workspace",
	Long: `Start a stopped Workspace.

Use --var to supply run-specific variable overrides. These take precedence
over variables defined in the Workspace and Template specs for this run only
and are not persisted back to the Workspace definition.`,
	Example: `
  # Start a Workspace
  cordium start abc

  # Start with run-specific variable overrides
  cordium start abc \
    --var BRANCH=feat/new-auth \
    --var PROMPT="Fix the authentication bug and run the tests."`,
	Args: cobra.ExactArgs(1),
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

	req := &pb.StartWorkspaceRequest{
		WorkspaceRef: &metav1.ObjectReference{
			Name: i.FirstArg(),
		},
	}

	if len(cmdArgs.Vars) > 0 {
		vars, err := ccommon.ParseVars(cmdArgs.Vars)
		if err != nil {
			return err
		}
		req.Config = &pb.StartWorkspaceRequest_Config{
			Vars: vars,
		}
	}

	if _, err := c.StartWorkspace(ctx, req); err != nil {
		return err
	}

	cliutils.LineNotify("Successfully started Workspace: %s\n", i.FirstArg())

	return nil
}
