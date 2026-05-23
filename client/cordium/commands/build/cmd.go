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
	Use:   "build <template> [flags]",
	Short: "Trigger or cancel a Template pre-build",
	Example: `
  # Trigger a pre-build for a Template
  cordium build ml-env.my-project

  # Trigger a pre-build in the default Space
  cordium build ml-env

  # Cancel a running pre-build
  cordium build ml-env.my-project --cancel`,
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
