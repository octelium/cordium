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

package start

import (
	"strings"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/pkg/errors"
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
		vars, err := parseVars(cmdArgs.Vars)
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

func parseVars(raw []string) ([]*pb.Workspace_Spec_Var, error) {
	vars := make([]*pb.Workspace_Spec_Var, 0, len(raw))
	for _, s := range raw {
		name, value, ok := strings.Cut(s, "=")
		if !ok || name == "" {
			return nil, errors.Errorf("invalid --var value %q: expected NAME=VALUE", s)
		}
		vars = append(vars, &pb.Workspace_Spec_Var{
			Name:  name,
			Value: value,
		})
	}
	return vars, nil
}
