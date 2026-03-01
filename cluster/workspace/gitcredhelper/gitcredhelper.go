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

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/octelium/cordium/cluster/common/wsclient"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func parseFromStdin() (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 {
			tuple := strings.Split(line, "=")
			if len(tuple) == 2 {
				result[tuple[0]] = strings.TrimSpace(tuple[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

var rootCmd = &cobra.Command{
	Use: "cordium-git-cred-helper",
}

var getCmd = &cobra.Command{
	Use: "get",
	RunE: func(cmd *cobra.Command, args []string) error {

		err := doGet()
		if err != nil {
			zap.L().Fatal("main err", zap.Error(err))
		}

		return nil
	},
}

var storeCmd = &cobra.Command{
	Use: "store",
	RunE: func(cmd *cobra.Command, args []string) error {

		err := doStore()
		if err != nil {
			zap.L().Fatal("main err", zap.Error(err))
		}

		return nil
	},
}

var eraseCmd = &cobra.Command{
	Use: "erase",
	RunE: func(cmd *cobra.Command, args []string) error {

		err := doErase()
		if err != nil {
			zap.L().Fatal("main err", zap.Error(err))
		}

		return nil
	},
}

func main() {
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(eraseCmd)

	if err := rootCmd.Execute(); err != nil {
		zap.L().Fatal("main err", zap.Error(err))
	}
}

func doGet() error {

	inMap, err := parseFromStdin()
	if err != nil {
		return err
	}

	if isInternal() {
		inMap["username"] = os.Getenv("CORDIUM_GIT_USERNAME")
		inMap["password"] = os.Getenv("CORDIUM_GIT_PASSWORD")
		for k, v := range inMap {
			fmt.Printf("%s=%s\n", k, v)
		}

		return nil
	}

	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{
		ClientInWorkspace: true,
	})
	if err != nil {
		return err
	}

	defer grpcConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := ccordiumv1.NewWorkspaceServiceClient(grpcConn)
	resp, err := c.GetGitCreds(ctx, &ccordiumv1.GetGitCredsRequest{
		Request: inMap,
		WorkDir: func() string {
			wd, err := os.Getwd()
			if err == nil {
				return wd
			}
			return ""
		}(),
	})
	if err != nil {
		return err
	}

	for k, v := range resp.Response {
		fmt.Printf("%s=%s\n", k, v)
	}

	return nil
}

func doStore() error {
	if isInternal() {
		return nil
	}

	inMap, err := parseFromStdin()
	if err != nil {
		return err
	}

	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{
		ClientInWorkspace: true,
	})
	if err != nil {
		return err
	}

	defer grpcConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := ccordiumv1.NewWorkspaceServiceClient(grpcConn)
	_, err = c.StoreGitCreds(ctx, &ccordiumv1.StoreGitCredsRequest{
		Request: inMap,
		WorkDir: func() string {
			wd, err := os.Getwd()
			if err == nil {
				return wd
			}
			return ""
		}(),
	})
	if err != nil {
		return err
	}

	return nil
}

func doErase() error {

	if isInternal() {
		return nil
	}
	inMap, err := parseFromStdin()
	if err != nil {
		return err
	}

	grpcConn, err := wsclient.GetWorkspaceGRPCClient(&wsclient.GetWorkspaceGRPCClientOpts{
		ClientInWorkspace: true,
	})
	if err != nil {
		return err
	}

	defer grpcConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := ccordiumv1.NewWorkspaceServiceClient(grpcConn)
	_, err = c.EraseGitCreds(ctx, &ccordiumv1.EraseGitCredsRequest{
		Request: inMap,
		WorkDir: func() string {
			wd, err := os.Getwd()
			if err == nil {
				return wd
			}
			return ""
		}(),
	})
	if err != nil {
		return err
	}

	return nil
}

func isInternal() bool {
	return os.Getenv("CORDIUM_GIT_INTERNAL") == "true"
}
