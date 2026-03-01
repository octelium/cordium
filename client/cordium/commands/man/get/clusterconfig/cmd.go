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

package clusterconfig

import (
	"fmt"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
	Out string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "", "Output format")
}

var Cmd = &cobra.Command{
	Use:   "clusterconfig",
	Short: "Get ClusterConfig",
	Example: `
cordium get cc
cordium get cc -o json
cordium get cc -o yaml
	`,
	Aliases: []string{"cc", "clustercfg"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
}

func doCmd(cmd *cobra.Command, args []string) error {
	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	conn, err := client.GetGRPCClientConn(cmd.Context(), i.Domain)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewManagementServiceClient(conn)

	res, err := c.GetClusterConfig(cmd.Context(), &pb.GetClusterConfigRequest{})
	if err != nil {
		return err
	}
	out, err := cliutils.OutFormatPrint(cmdArgs.Out, res)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(out))
	return nil
}
