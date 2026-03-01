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

package secret

import (
	"os"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/spf13/cobra"
)

type args struct {
	Value    string
	FromFile string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVar(&cmdArgs.Value, "value", "", "Secret value")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.FromFile, "file", "f", "", "Get Secret value from file path")
}

var Cmd = &cobra.Command{
	Use:   "secret",
	Short: "Create a Secret",
	Example: `
cordium create secret my-secret
cordium create secret topsec01.space01
	`,
	Aliases: []string{"secrets", "sec"},
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

	sec := &pb.Secret{
		Spec: &pb.Secret_Spec{},
		Data: &pb.Secret_Data{},
	}

	val, err := getValue()
	if err != nil {
		return err
	}

	sec.Data = &pb.Secret_Data{
		Type: &pb.Secret_Data_ValueBytes{
			ValueBytes: val,
		},
	}

	sec.Metadata = &metav1.Metadata{
		Name: i.FirstArg(),
	}
	sec.Status = &pb.Secret_Status{}

	sec, err = c.CreateSecret(ctx, sec)
	if err != nil {
		return err
	}

	return nil
}

func getValue() ([]byte, error) {
	if cmdArgs.FromFile != "" {
		return os.ReadFile(cmdArgs.FromFile)
	}

	if cmdArgs.Value != "" {
		return []byte(cmdArgs.Value), nil
	}

	return cliutils.GetSecretPrompt()
}
