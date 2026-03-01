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

package terminal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
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
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
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
		Name:   wsName,
		Width:  uint32(cols),
		Height: uint32(rows),
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
	signal.Notify(sigCh, syscall.SIGWINCH)

	go func(ctx context.Context) {
		for range sigCh {
			cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				zap.L().Debug("Could not getSize", zap.Error(err))
				continue
			}

			zap.L().Debug("New window size", zap.Int("cols", cols), zap.Int("rows", rows))

			if _, err := c.SetTerminalWindowSize(ctx, &pb.SetTerminalWindowSizeRequest{
				Id:     t.Id,
				Width:  uint32(cols),
				Height: uint32(rows),
			}); err != nil {
				zap.L().Debug("Could not SetTerminalWindowSize", zap.Error(err))
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
					break
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
