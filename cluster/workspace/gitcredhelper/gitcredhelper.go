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
