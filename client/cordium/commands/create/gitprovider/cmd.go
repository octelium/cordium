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

package gitprovider

import (
	"fmt"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type args struct {
	Space                  string
	ProviderType           string
	ClientID               string
	ClientSecretFromSecret string
	Scopes                 []string
	AuthURL                string
	TokenURL               string
	Out                    string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVar(&cmdArgs.Space, "space", "",
		"Parent Space name (e.g. my-project)")
	Cmd.PersistentFlags().StringVar(&cmdArgs.ProviderType, "type", "",
		`Provider type: "github", "gitlab", or "oauth2"`)
	Cmd.PersistentFlags().StringVar(&cmdArgs.ClientID, "client-id", "",
		"OAuth2 application client ID")
	Cmd.PersistentFlags().StringVar(&cmdArgs.ClientSecretFromSecret, "client-secret-from-secret", "",
		"Name of the Space Secret containing the OAuth2 client secret")
	Cmd.PersistentFlags().StringArrayVar(&cmdArgs.Scopes, "scope", nil,
		"OAuth2 scope (repeatable: --scope repo --scope read:user). "+
			"Server-side defaults are applied if omitted.")
	Cmd.PersistentFlags().StringVar(&cmdArgs.AuthURL, "auth-url", "",
		"Authorization endpoint URL (required for --type oauth2)")
	Cmd.PersistentFlags().StringVar(&cmdArgs.TokenURL, "token-url", "",
		"Token endpoint URL (required for --type oauth2)")
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "",
		`Output format: "yaml" or "json"`)
}

var Cmd = &cobra.Command{
	Use:   "gitprovider <name> [flags]",
	Short: "Create a GitProvider within a Space",
	Long: `Create a GitProvider that configures OAuth2 authentication against a git
hosting service. The GitProvider can then be referenced by a Template to
enable automatic OAuth2 token injection into Workspaces, giving the Workspace
authenticated git access without credential management.

Supported provider types: github, gitlab, oauth2 (generic).

The client secret must be stored as a Space Secret and referenced by name
via --client-secret-from-secret. The secret value itself is never passed
on the command line.`,
	Example: `
  # Create a GitHub OAuth2 provider
  cordium create gitprovider my-github.my-project \
    --type github \
    --client-id abc123 \
    --client-secret-from-secret github-oauth-secret \
    --scope repo \
    --scope read:user

  # Create a GitLab provider (scopes optional — server defaults are applied)
  cordium create gitprovider my-gitlab.my-project \
    --type gitlab \
    --client-id def456 \
    --client-secret-from-secret gitlab-oauth-secret

  # Create a generic OAuth2 provider (e.g. Gitea, Forgejo)
  cordium create gitprovider my-gitea.my-project \
    --type oauth2 \
    --client-id ghi789 \
    --client-secret-from-secret gitea-oauth-secret \
    --auth-url https://gitea.example.com/login/oauth/authorize \
    --token-url https://gitea.example.com/login/oauth/access_token \
    --scope read:user

  # Using --space instead of a qualified name
  cordium create gp my-github --space my-project \
    --type github \
    --client-id abc123 \
    --client-secret-from-secret github-oauth-secret

  # Output the created resource as YAML
  cordium create gp my-github.my-project \
    --type github \
    --client-id abc123 \
    --client-secret-from-secret github-oauth-secret \
    -o yaml`,
	Aliases: []string{"gitproviders", "gp"},
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

	if cmdArgs.ProviderType == "" {
		return errors.New("--type is required: github, gitlab, or oauth2")
	}
	if cmdArgs.ClientID == "" {
		return errors.New("--client-id is required")
	}
	if cmdArgs.ClientSecretFromSecret == "" {
		return errors.New("--client-secret-from-secret is required")
	}
	if cmdArgs.ProviderType == "oauth2" {
		if cmdArgs.AuthURL == "" {
			return errors.New("--auth-url is required for --type oauth2")
		}
		if cmdArgs.TokenURL == "" {
			return errors.New("--token-url is required for --type oauth2")
		}
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	gp := &pb.GitProvider{
		Metadata: &metav1.Metadata{
			Name: args[0],
		},
		Spec:   &pb.GitProvider_Spec{},
		Status: &pb.GitProvider_Status{},
	}

	if cmdArgs.Space != "" {
		gp.Status.SpaceRef = &metav1.ObjectReference{
			Name: cmdArgs.Space,
		}
	}

	switch cmdArgs.ProviderType {
	case "github":
		gp.Spec.Type = &pb.GitProvider_Spec_Github_{
			Github: &pb.GitProvider_Spec_Github{
				ClientID: cmdArgs.ClientID,
				ClientSecret: &pb.GitProvider_Spec_Github_ClientSecret{
					Type: &pb.GitProvider_Spec_Github_ClientSecret_FromSecret{
						FromSecret: cmdArgs.ClientSecretFromSecret,
					},
				},
				Scopes: cmdArgs.Scopes,
			},
		}
	case "gitlab":
		gp.Spec.Type = &pb.GitProvider_Spec_Gitlab_{
			Gitlab: &pb.GitProvider_Spec_Gitlab{
				ClientID: cmdArgs.ClientID,
				ClientSecret: &pb.GitProvider_Spec_Gitlab_ClientSecret{
					Type: &pb.GitProvider_Spec_Gitlab_ClientSecret_FromSecret{
						FromSecret: cmdArgs.ClientSecretFromSecret,
					},
				},
				Scopes: cmdArgs.Scopes,
			},
		}
	case "oauth2":
		gp.Spec.Type = &pb.GitProvider_Spec_Oauth2{
			Oauth2: &pb.GitProvider_Spec_OAuth2{
				ClientID: cmdArgs.ClientID,
				ClientSecret: &pb.GitProvider_Spec_OAuth2_ClientSecret{
					Type: &pb.GitProvider_Spec_OAuth2_ClientSecret_FromSecret{
						FromSecret: cmdArgs.ClientSecretFromSecret,
					},
				},
				AuthURL:  cmdArgs.AuthURL,
				TokenURL: cmdArgs.TokenURL,
				Scopes:   cmdArgs.Scopes,
			},
		}
	default:
		return errors.Errorf("unknown --type %q: must be github, gitlab, or oauth2",
			cmdArgs.ProviderType)
	}

	gp, err = c.CreateGitProvider(ctx, gp)
	if err != nil {
		return err
	}

	if cmdArgs.Out != "" {
		out, err := cliutils.OutFormatPrint(cmdArgs.Out, gp)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(out))
	} else {
		cliutils.LineNotify("Successfully created GitProvider: %s\n", gp.Metadata.Name)
	}

	return nil
}
