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

package secret

import (
	"os"

	"github.com/octelium/cordium/client/cordium/commands/ccommon"
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
	Use:   "secret <name> [flags]",
	Short: "Create a Secret within a Space",
	Example: `
  # Create a Secret and enter the value interactively
  cordium create secret db-password.my-project

  # Create a Secret with an inline value
  cordium create secret stripe-key.my-project --value "sk-live-..."

  # Create a Secret from a file
  cordium create secret tls-cert.my-project --file ./cert.pem

  # Create a Secret in the default Space
  cordium create secret db-password`,
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

	cliutils.LineNotify("Successfully created Secret: %s\n", ccommon.GetResourceShortName(sec))

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
