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

package space

import (
	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
}

var cmdArgs args

func init() {
}

var Cmd = &cobra.Command{
	Use:   "space",
	Short: "Create a Space",
	Example: `
cordium create space my-space
cordium create spc devops -o json
cordium create spaces spc01
	`,
	Aliases: []string{"spaces", "spc"},
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
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	spc := &pb.Space{
		Metadata: &metav1.Metadata{
			Name: i.FirstArg(),
		},
		Spec: &pb.Space_Spec{},
	}

	spc, err = c.CreateSpace(ctx, spc)
	if err != nil {
		return err
	}

	cliutils.LineNotify("Successfully created Space: %s\n", ccommon.GetResourceShortName(spc))

	return nil
}
