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

package template

import (
	"os"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/spf13/cobra"
)

type args struct {
	File string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.File, "file", "", "", "Spec file path")
}

var Cmd = &cobra.Command{
	Use:   "template",
	Short: "Create a Template",
	Example: `
cordium create template template01 -f /PATH/TO/TEMPLATE.YAML
cordium create tmpl my-template.my-space -f /PATH/TO/TEMPLATE.YAML
	`,
	Aliases: []string{"templates", "tmpl"},
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

	prj := &pb.Template{
		Spec: &pb.Template_Spec{},
	}
	if cmdArgs.File != "" {
		yamlBytes, err := os.ReadFile(cmdArgs.File)
		if err != nil {
			return err
		}
		if err := pbutils.UnmarshalYAML(yamlBytes, prj); err != nil {
			return err
		}
	}

	prj.Metadata = &metav1.Metadata{
		Name: i.FirstArg(),
	}
	prj.Status = &pb.Template_Status{}

	prj, err = c.CreateTemplate(ctx, prj)
	if err != nil {
		return err
	}

	return nil
}
