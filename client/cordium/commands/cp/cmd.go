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

package cp

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/main/userv1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/client/octelium/commands/cp"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type args struct {
	Recursive bool
}

var cmdArgs args

func init() {
	Cmd.Flags().BoolVarP(&cmdArgs.Recursive, "recursive", "r", false,
		"Recursively copy directories")
}

var Cmd = &cobra.Command{
	Use:   "cp <source> <destination>",
	Short: "Copy files between local filesystem and Workspaces",
	Long: `Copy files or directories between the local filesystem and a Workspace,
or between two Workspaces.

Workspace paths are specified as <workspace>:<path>.
Local paths are specified as plain filesystem paths.

Use -r to copy directories recursively.`,
	Example: `
  # Copy a local file to a Workspace
  cordium cp ./config.json abc:/workspace/repo/config.json

  # Copy a file from a Workspace to local
  cordium cp abc:/workspace/repo/output.csv ./output.csv

  # Copy a local directory to a Workspace recursively
  cordium cp -r ./src/ abc:/workspace/repo/src/

  # Copy a directory from a Workspace to local
  cordium cp -r abc:/workspace/repo/dist/ ./dist/

  # Copy a file from one Workspace to another
  cordium cp abc:/workspace/repo/model.pt def:/workspace/repo/model.pt

  # Copy a directory from one Workspace to another
  cordium cp -r abc:/workspace/data/ def:/workspace/data/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.ExactArgs(2),
}

func doCmd(cmd *cobra.Command, args []string) error {

	ctx := cmd.Context()
	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	srcI := parseEndpoint(args[0])
	dstI := parseEndpoint(args[1])

	if srcI.isLocal && dstI.isLocal {
		return errors.Errorf("at least one of source or destination must be a Workspace path (workspace:/path)")
	}

	src := &cp.Endpoint{
		User:    srcI.workspace,
		Path:    srcI.path,
		IsLocal: srcI.isLocal,
	}

	dst := &cp.Endpoint{
		User:    dstI.workspace,
		Path:    dstI.path,
		IsLocal: dstI.isLocal,
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewMainServiceClient(conn)

	userC := userv1.NewMainServiceClient(conn)

	setupWS := func(e *cp.Endpoint) error {
		name := e.User

		ws, err := c.GetWorkspace(ctx, &metav1.GetOptions{Name: name})
		if err != nil {
			return errors.Errorf("could not get Workspace %q: %+v", name, err)
		}
		if !ucordiumv1.ToWorkspace(ws).IsPreparingOrRunning() {
			return errors.Errorf("Workspace %q is not running (state: %s)", name, ws.Status.State.String())
		}

		cfg, err := userC.SetServiceConfigs(ctx, &userv1.SetServiceConfigsRequest{
			Name: fmt.Sprintf("%s-ssh.cordium", ws.Status.RegionRef.Name),
		})
		if err != nil {
			return err
		}

		hostKeyCallback, err := func() (ssh.HostKeyCallback, error) {
			if len(cfg.Configs) < 1 ||
				cfg.Configs[0].GetSsh() == nil ||
				len(cfg.Configs[0].GetSsh().KnownHosts) == 0 {
				return nil, errors.Errorf("Could not get Service knownHosts")
			}

			f, err := os.CreateTemp("", "known_hosts_*")
			if err != nil {
				return nil, err
			}
			defer os.Remove(f.Name())

			if _, err := f.WriteString(strings.Join(cfg.Configs[0].GetSsh().KnownHosts, "\n")); err != nil {
				f.Close()
				return nil, err
			}
			f.Close()

			return knownhosts.New(f.Name())
		}()
		if err != nil {
			return err
		}

		e.Network = func() string {
			switch cfg.L3Mode {
			case userv1.SetServiceConfigsResponse_V6:
				return "tcp6"
			default:
				return "tcp"
			}
		}()

		e.HostKeyCallback = hostKeyCallback
		e.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))

		return nil
	}

	if !src.IsLocal {
		if err := setupWS(src); err != nil {
			return err
		}
	}

	if !dst.IsLocal {
		if err := setupWS(dst); err != nil {
			return err
		}
	}

	return cp.DoCommand(ctx, &cp.DoCommandOpts{
		Recursive: cmdArgs.Recursive,
		Src:       src,
		Dst:       dst,
	})
}

type endpoint struct {
	workspace string
	path      string
	isLocal   bool
}

func parseEndpoint(arg string) *endpoint {

	idx := strings.Index(arg, ":")
	if idx > 1 {
		return &endpoint{
			workspace: arg[:idx],
			path:      arg[idx+1:],
			isLocal:   false,
		}
	}
	return &endpoint{
		path:    arg,
		isLocal: true,
	}
}
