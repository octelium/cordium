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

package usersecret

import (
	"fmt"

	"github.com/octelium/cordium/client/cordium/commands/ccommon"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type args struct {
	Type     string
	Value    string
	FromFile string
	FromEnv  string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVar(&cmdArgs.Type, "type", "",
		`UserSecret type. Use "ssh-key" to generate a server-side ECDSA key pair.
Omit for a standard secret value (token, password, certificate, etc.).`)
	Cmd.PersistentFlags().StringVar(&cmdArgs.Value, "value", "",
		"UserSecret value (inline). Cannot be used with --file, --from-env or --type ssh-key.")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.FromFile, "file", "f", "",
		"Read the UserSecret value from a file. Set it to `-` to read from stdin. Cannot be used with --value, --from-env or --type ssh-key.")
	Cmd.PersistentFlags().StringVar(&cmdArgs.FromEnv, "from-env", "",
		"Read the UserSecret value from an environment variable. Cannot be used with --value, --file or --type ssh-key.")

	Cmd.MarkFlagsMutuallyExclusive("value", "file", "from-env")
}

var Cmd = &cobra.Command{
	Use:   "usersecret <name> [flags]",
	Short: "Create a UserSecret",
	Long: `Create a UserSecret. UserSecrets are per-user sensitive values used as
environment variable sources in UserConfig or as credentials for dotfile
repository access.

For standard secrets (tokens, passwords, certificates), provide the value
via --value, --file, --from-env, or an interactive prompt.

For SSH key UserSecrets (--type ssh-key), the server generates an ECDSA key
pair. The private key is stored and automatically loaded into every Workspace
SSH agent. The public key is printed after creation for external registration
(GitHub, GitLab, servers, etc.).`,
	Example: `
  # Create a UserSecret and enter the value interactively (no echo)
  cordium create usersecret my-github-token

  # Create a UserSecret with an inline value
  cordium create usersecret my-github-token --value "ghp_..."

  # Create a UserSecret from a file (content stored as binary)
  cordium create usec my-tls-cert --file ./cert.pem

  # Create a UserSecret from an environment variable
  cordium create usec my-github-token --from-env GITHUB_TOKEN

  # Generate a server-side ECDSA SSH key pair
  cordium create usec my-deploy-key --type ssh-key`,
	Aliases: []string{"usersecrets", "usec"},
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

	isSshKey := cmdArgs.Type == "ssh-key"

	if cmdArgs.Type != "" && !isSshKey {
		return errors.Errorf("unknown --type %q: the only supported type is ssh-key", cmdArgs.Type)
	}
	if isSshKey && (cmd.Flags().Changed("value") || cmd.Flags().Changed("file") || cmd.Flags().Changed("from-env")) {
		return errors.New("--value, --file and --from-env cannot be used with --type ssh-key: the key pair is generated server-side")
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	sec := &pb.UserSecret{
		Metadata: &metav1.Metadata{
			Name: i.FirstArg(),
		},
		Spec:   &pb.UserSecret_Spec{},
		Status: &pb.UserSecret_Status{},
	}

	if isSshKey {
		sec.Spec.Type = pb.UserSecret_Spec_SSH_KEY
	} else {
		sec.Spec.Type = pb.UserSecret_Spec_DEFAULT
		sec.Data = &pb.UserSecret_Data{}

		val, err := getValue()
		if err != nil {
			return err
		}

		if cmdArgs.FromFile != "" {
			sec.Data.Type = &pb.UserSecret_Data_ValueBytes{
				ValueBytes: val,
			}
		} else {
			sec.Data.Type = &pb.UserSecret_Data_Value{
				Value: string(val),
			}
		}
	}

	sec, err = c.CreateUserSecret(ctx, sec)
	if err != nil {
		return err
	}

	cliutils.LineNotify("Successfully created UserSecret: %s\n", ccommon.GetResourceShortName(sec))

	if isSshKey {
		if details, ok := sec.Status.Details.(*pb.UserSecret_Status_SshKey); ok &&
			details.SshKey != nil && details.SshKey.PublicKey != "" {
			fmt.Printf("\nPublic key — add this to GitHub, GitLab, or ~/.ssh/authorized_keys:\n\n%s\n",
				details.SshKey.PublicKey)
		}
	}

	return nil
}

func getValue() ([]byte, error) {
	return cliutils.GetDataValue(&cliutils.GetDataValueOpts{
		Value:    cmdArgs.Value,
		FromFile: cmdArgs.FromFile,
		FromEnv:  cmdArgs.FromEnv,
		Prompt:   "Enter the UserSecret value",
	})
}
