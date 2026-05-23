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

package terminal

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"time"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/term"
	"google.golang.org/grpc"
)

type args struct {
}

var cmdArgs args

func init() {
}

var Cmd = &cobra.Command{
	Use:     "terminal",
	Aliases: []string{"term"},
	Short:   "Run a terminal in a Workspace",
	Example: `
cordium terminal abc
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.ExactArgs(1),
}

func doCmd(cmd *cobra.Command, args []string) error {

	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := DoCmdTerminal(ctx, conn, i.FirstArg()); err != nil {
		return err
	}

	return nil
}

func DoCmdTerminal(ctx context.Context, conn *grpc.ClientConn, wsName string) error {
	ctx, cancel := notifyTerminalExitContext(ctx)
	defer cancel()

	c := pb.NewWorkspaceServiceClient(conn)

	fd := int(os.Stdin.Fd())
	st, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, st)

	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}

	zap.L().Debug("init window size", zap.Int("cols", cols), zap.Int("rows", rows))

	t, err := c.CreateTerminal(ctx, &pb.CreateTerminalRequest{
		WorkspaceRef: &metav1.ObjectReference{
			Name: wsName,
		},
		Cols: uint32(cols),
		Rows: uint32(rows),
	})
	if err != nil {
		return err
	}

	strm, err := c.ListenTerminal(ctx, &pb.ListenTerminalRequest{
		Id: t.Id,
	})
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	notifyWindowSizeChange(sigCh)
	defer signal.Stop(sigCh)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigCh:
				cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
				if err != nil {
					zap.L().Debug("Could not getSize", zap.Error(err))
					continue
				}

				zap.L().Debug("New window size", zap.Int("cols", cols), zap.Int("rows", rows))

				if _, err := c.SetTerminalWindowSize(ctx, &pb.SetTerminalWindowSizeRequest{
					Id:   t.Id,
					Cols: uint32(cols),
					Rows: uint32(rows),
				}); err != nil {
					zap.L().Debug("Could not SetTerminalWindowSize", zap.Error(err))
				}
			}
		}
	}(ctx)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				zap.L().Debug("ctx done. Exiting ListenTerminal loop")
				return
			default:
				msg, err := strm.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) || grpcerr.IsCanceled(err) {
						cancel()
						return
					}
					time.Sleep(200 * time.Millisecond)
					continue
				}

				switch msg.Type.(type) {
				case *pb.ListenTerminalResponse_Close_:
					zap.L().Debug("Got close msg. Exiting ListenTerminal loop")
					cancel()
					return
				case *pb.ListenTerminalResponse_Stdout_:
					if _, err := os.Stdout.Write(msg.GetStdout().Data); err != nil {
						zap.L().Debug("Could not write to stdout", zap.Error(err))
					}
				}

			}
		}
	}(ctx)

	go func(ctx context.Context) {
		buf := make([]byte, 3*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := os.Stdin.Read(buf)
				if err != nil || ctx.Err() != nil {
					return
				}
				if _, err := c.WriteTerminalData(ctx, &pb.WriteTerminalDataRequest{
					Id:   t.Id,
					Data: buf[:n],
				}); err != nil {
					zap.L().Debug("Could not WriteTerminalData")
				}
			}
		}
	}(ctx)

	zap.L().Debug("Waiting for exit")
	<-ctx.Done()

	{
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if _, err := c.RemoveTerminal(ctx, &pb.RemoveTerminalRequest{
			Id: t.Id,
		}); err != nil {
			zap.L().Debug("Could not RemoveTerminal", zap.Error(err))
		}
	}

	zap.L().Debug("Exiting...")

	return nil
}
