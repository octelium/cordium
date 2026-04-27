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

package build

import (
	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
	DoCancel bool
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().BoolVarP(&cmdArgs.DoCancel, "cancel", "", false, "Cancel Build")
}

var Cmd = &cobra.Command{
	Use:   "build",
	Short: "Build a Template",
	Example: `
cordium build my-template.my-space
cordium build template01
cordium build my-template --cancel
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.ExactArgs(1),
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

	tmpl, err := c.GetTemplate(ctx, &metav1.GetOptions{
		Name: i.FirstArg(),
	})
	if err != nil {
		return err
	}

	if cmdArgs.DoCancel {
		if _, err := c.CancelBuildTemplate(ctx, &pb.CancelBuildTemplateRequest{
			TemplateRef: &metav1.ObjectReference{
				Name: i.FirstArg(),
			},
		}); err != nil {
			return err
		}
		cliutils.LineNotify("Canceled a build for Template: %s\n", ccommon.GetResourceShortName(tmpl))
	} else {
		if _, err := c.BuildTemplate(ctx, &pb.BuildTemplateRequest{
			TemplateRef: &metav1.ObjectReference{
				Name: i.FirstArg(),
			},
		}); err != nil {
			return err
		}

		cliutils.LineNotify("Started a build for Template: %s\n", ccommon.GetResourceShortName(tmpl))
	}

	return nil
}
