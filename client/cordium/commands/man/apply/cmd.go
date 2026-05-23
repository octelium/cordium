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

package apply

import (
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/common/resources"
	"github.com/spf13/cobra"
)

var examples = `
cordium man apply /path/to/file.yaml
cordium man apply /path/to/directory

# Apply from stdin
cat /path/to/file.yaml | cordium man apply -
`

var Cmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply the desired state to the Cluster",
	Long: `
Apply the desired state to the Cluster in a declarative way that is very similar to kubectl and helm. This command
accepts both single yaml files and directories. For the case of directories, all yaml files will be recursively searched for resources.
`,

	Example: examples,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
}

func init() {

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

	resources, err := resources.LoadResources(i.FirstArg(), ucordiumv1.NewObject)
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

	return nil
}
