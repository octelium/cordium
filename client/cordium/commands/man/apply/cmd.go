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

package apply

import (
	"context"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/common/resources"
	"github.com/octelium/octelium/client/common/rscdiff"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type args struct {
	FilePath         string
	DoDelete         bool
	ResourceIncludes []string
	ResourceExcludes []string
}

var examples = `
cordium man apply -f /path/to/file.yaml
cordium man apply -f /path/to/directory
cat /path/to/file.yaml | cordium man apply -f -
`

var Cmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply the desired state to the Cluster",
	Long: `
Apply the desired state to the Cluster in a declarative way that is very similar to kubectl and helm. This command
accepts both single yaml files and directories. For the case of directories, all yaml files will be recursively searched for resources.
`,

	Example: examples,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.FilePath, "file", "f", "",
		"File/Directory path that contains the desired resources. If it is a directory all files including files in sub-directories are going to be included")
	Cmd.PersistentFlags().BoolVar(&cmdArgs.DoDelete, "prune", false,
		"Delete all objects that do not exist in the current desired resources as described in file/directory path but do exist in the Cluster. In other words, this synchronizes the current described state in the file/directory path and prunes all additional resources that exist on the Cluster but not in the current desired configuration. Disabled by default.")
	Cmd.PersistentFlags().StringSliceVar(&cmdArgs.ResourceIncludes, "include-kind", nil,
		"Only include this resource kind")
	Cmd.PersistentFlags().StringSliceVar(&cmdArgs.ResourceExcludes, "exclude-kind", nil,
		"Exclude this resource kind")
}

func diffResource(ctx context.Context,
	kind string, conn *grpc.ClientConn, desiredItems []umetav1.ResourceObjectI, doDelete bool) error {
	ctl, err := rscdiff.NewDiffCtl(ucordiumv1.API, kind, cordiumv1.NewMainServiceClient(conn),
		func() (umetav1.ResourceObjectI, error) {
			return ucordiumv1.NewObject(kind)
		}, func() (protoreflect.ProtoMessage, error) {
			return ucordiumv1.NewObjectListOptions(kind)
		}, desiredItems, doDelete)
	if err != nil {
		return err
	}
	_, err = ctl.Run(ctx)

	return err
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

	resources, err := resources.LoadResources(cmdArgs.FilePath, ucordiumv1.NewObject)
	if err != nil {
		return err
	}

	cc := func() *cordiumv1.ClusterConfig {
		for _, itm := range resources {
			if itm.GetKind() == ucordiumv1.KindClusterConfig {
				return itm.(*cordiumv1.ClusterConfig)
			}
		}
		return nil
	}()

	c := cordiumv1.NewManagementServiceClient(conn)

	if cc != nil {
		if _, err := c.UpdateClusterConfig(ctx, cc); err != nil {
			return err
		}
		cliutils.LineNotify("\n Cluster Configuration updated\n")
	}

	cliutils.LineNotify("Cluster resources successfully applied\n")

	return nil
}

func isInList(lst []string, arg string) bool {
	for _, itm := range lst {
		if itm == arg {
			return true
		}
	}
	return false
}

func deleteItem(lst []string, arg string) []string {
	for i, itm := range lst {
		if itm == arg {
			ret := append(lst[:i], lst[i+1:]...)
			return ret
		}
	}
	return lst
}

func deduplicateItems(lst []string) []string {
	var ret []string
	for _, itm := range lst {
		if !isInList(ret, itm) {
			ret = append(ret, itm)
		}
	}
	return ret
}
