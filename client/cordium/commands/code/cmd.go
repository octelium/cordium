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

package code

import (
	"fmt"
	"os/exec"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type args struct {
	Dir string
}

var cmdArgs args

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Dir, "dir", "", "/workspace/repo",
		`Code Environment directory. By default it is set to Cordium default Repository directory`)
}

var Cmd = &cobra.Command{
	Use:   "code",
	Short: "Open a VScode instance for the Workspace",
	Example: `
cordium code abc
cordium code abc
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

	if _, err := exec.LookPath("code"); err != nil {
		return errors.Errorf(`"code" binary does not exist. You can download and install VScode via https://code.visualstudio.com/download`)
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	arg := i.FirstArg()
	ws, err := c.GetWorkspace(ctx, &metav1.GetOptions{
		Name: arg,
	})
	if err != nil {
		return err
	}
	if !ucordiumv1.ToWorkspace(ws).IsPreparingOrRunning() {
		return errors.Errorf("Workspace is not running")
	}

	cmdI := exec.CommandContext(ctx,
		"code",
		"--remote",
		fmt.Sprintf("ssh-remote+%s@%s-ssh.cordium.local.%s", ws.Metadata.Name, ws.Status.RegionRef.Name, i.Domain),
		cmdArgs.Dir,
	)

	cmdI.Stdin = cmd.InOrStdin()
	cmdI.Stdout = cmd.OutOrStdout()
	cmdI.Stderr = cmd.ErrOrStderr()

	return cmdI.Run()
}
