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

package snapshot

import (
	"fmt"

	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
	Workspace string
	Out       string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Workspace, "workspace", "", "", "The Workspace to be snapshotted")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")

	Cmd.MarkPersistentFlagRequired("workspace")
}

var Cmd = &cobra.Command{
	Use:   "snapshot <name> [flags]",
	Short: "Create a WorkspaceSnapshot of the storage of a Workspace",
	Long: `Create a point-in-time snapshot of the persistent storage of a Workspace.

The Workspace is neither stopped nor restarted and it remains fully usable while
the snapshot is being taken. Snapshotting a running Workspace produces a
crash-consistent snapshot while snapshotting a stopped one produces a clean
snapshot. The snapshot is taken asynchronously and it can only be restored from
once it reaches the READY state.

A new Workspace is restored from the snapshot with:
"cordium create workspace --snapshot <name>"`,
	Example: `
  # Snapshot a Workspace
  cordium create snapshot before-upgrade --workspace abc

  # Create a new Workspace out of the snapshot
  cordium create workspace --snapshot before-upgrade --start`,
	Aliases: []string{"snapshots", "snap"},
	Args:    cobra.ExactArgs(1),
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

	snapshot, err := c.CreateWorkspaceSnapshot(ctx, &pb.WorkspaceSnapshot{
		Metadata: &metav1.Metadata{
			Name: i.FirstArg(),
		},
		Spec: &pb.WorkspaceSnapshot_Spec{},
		Status: &pb.WorkspaceSnapshot_Status{
			WorkspaceRef: &metav1.ObjectReference{
				Name: cmdArgs.Workspace,
			},
		},
	})
	if err != nil {
		return err
	}

	if cmdArgs.Out != "" {
		out, err := cliutils.OutFormatPrint(cmdArgs.Out, snapshot)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(out))
		return nil
	}

	cliutils.LineNotify("Started taking the WorkspaceSnapshot: %s of the Workspace: %s\n",
		ccommon.GetResourceShortName(snapshot), cmdArgs.Workspace)

	return nil
}
