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

package volume

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
	Size   uint32
	Shared bool
	Region string
	Out    string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().Uint32VarP(&cmdArgs.Size, "size", "", 0, "The size of the Volume in megabytes")
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.Shared, "shared", "", false,
		"Allow the Volume to be mounted by several Workspaces at the same time")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Region, "region", "", "", "The Region that hosts the Volume")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")
}

var Cmd = &cobra.Command{
	Use:   "volume <name> [flags]",
	Short: "Create a Volume inside a Space",
	Long: `Create a Volume inside a Space.

A Volume is a persistent storage device that has a lifecycle of its own and
that is mounted by the Workspaces of the Space at arbitrary paths. Unlike a
Workspace's own storage, it outlives the Workspaces that mount it and it is
never included in their WorkspaceSnapshots.

An EXCLUSIVE Volume, which is the default, is mounted by a single running
Workspace at a time. A SHARED Volume can be mounted by several running
Workspaces at the same time and it requires the Cluster to be configured with a
shared filesystem storage backend.`,
	Example: `
  # Create a 50 GB Volume in the default Space
  cordium create volume datasets --size 50000

  # Create a Volume that several Workspaces can mount at the same time
  cordium create volume build-cache --size 20000 --shared

  # Create a Volume in a specific Space
  cordium create volume datasets.my-project --size 50000

  # Mount it in a new Workspace
  cordium create workspace --volume datasets:/data`,
	Aliases: []string{"volumes", "vol"},
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

	vol, err := c.CreateVolume(ctx, &pb.Volume{
		Metadata: &metav1.Metadata{
			Name: i.FirstArg(),
		},
		Spec: &pb.Volume_Spec{
			Size: &pb.Volume_Spec_Size{
				Megabytes: cmdArgs.Size,
			},
			AccessMode: func() pb.Volume_AccessMode {
				if cmdArgs.Shared {
					return pb.Volume_ACCESS_MODE_SHARED
				}
				return pb.Volume_ACCESS_MODE_EXCLUSIVE
			}(),
		},
		Status: &pb.Volume_Status{
			RegionRef: func() *metav1.ObjectReference {
				if cmdArgs.Region == "" {
					return nil
				}
				return &metav1.ObjectReference{
					Name: cmdArgs.Region,
				}
			}(),
		},
	})
	if err != nil {
		return err
	}

	if cmdArgs.Out != "" {
		out, err := cliutils.OutFormatPrint(cmdArgs.Out, vol)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(out))
		return nil
	}

	cliutils.LineNotify("Successfully created the Volume: %s of %d MB\n",
		ccommon.GetResourceShortName(vol), vol.Spec.Size.Megabytes)

	return nil
}
