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

package usersecret

import (
	"fmt"
	"os"

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
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVar(&cmdArgs.Type, "type", "",
		`UserSecret type. Use "ssh-key" to generate a server-side ECDSA key pair.
Omit for a standard secret value (token, password, certificate, etc.).`)
	Cmd.PersistentFlags().StringVar(&cmdArgs.Value, "value", "",
		"UserSecret value (inline). Cannot be used with --file or --type ssh-key.")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.FromFile, "file", "f", "",
		"Read the UserSecret value from a file. Cannot be used with --value or --type ssh-key.")

	Cmd.MarkFlagsMutuallyExclusive("value", "file")
}

var Cmd = &cobra.Command{
	Use:   "usersecret <name> [flags]",
	Short: "Create a UserSecret",
	Long: `Create a UserSecret. UserSecrets are per-user sensitive values used as
environment variable sources in UserConfig or as credentials for dotfile
repository access.

For standard secrets (tokens, passwords, certificates), provide the value
via --value, --file, or an interactive prompt.

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
	if isSshKey && (cmd.Flags().Changed("value") || cmd.Flags().Changed("file")) {
		return errors.New("--value and --file cannot be used with --type ssh-key: the key pair is generated server-side")
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
	if cmdArgs.FromFile != "" {
		return os.ReadFile(cmdArgs.FromFile)
	}
	if cmdArgs.Value != "" {
		return []byte(cmdArgs.Value), nil
	}

	return cliutils.GetSecretPrompt()
}
