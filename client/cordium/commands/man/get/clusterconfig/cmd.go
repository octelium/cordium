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
