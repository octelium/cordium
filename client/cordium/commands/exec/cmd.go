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

package exec

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type args struct {
}

var cmdArgs args

func init() {
}

var Cmd = &cobra.Command{
	Use:   "exec",
	Short: "Run a command in a Workspace",
	Example: `
cordium exec abc -- ls -lah /
cordium exec def -- sudo apt-get update
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.MinimumNArgs(2),
}

func doCmd(cmd *cobra.Command, args []string) error {

	ctx, doCancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	cancel := sync.OnceFunc(func() {
		doCancel()
	})

	defer cancel()

	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	zap.L().Debug("args", zap.Strings("args", args), zap.String("cmd", strings.Join(args[1:], " ")))

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := pb.NewWorkspaceServiceClient(conn)

	strm, err := c.Exec(ctx, grpc_retry.Disable())
	if err != nil {
		return err
	}

	strm.Send(&pb.ExecRequest{
		Type: &pb.ExecRequest_Request_{

			Request: &pb.ExecRequest_Request{
				WorkspaceRef: &metav1.ObjectReference{
					Name: i.FirstArg(),
				},
				Command: strings.Join(args[1:], " "),
			},
		},
	})

	go func(ctx context.Context) {
		defer zap.L().Debug("Exiting stdout loop")
		for {
			select {
			case <-ctx.Done():
				zap.L().Debug("ctx done. Exiting stdout loop")
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
				case *pb.ExecResponse_Exit_:
					zap.L().Debug("Got close msg. Exiting ListenTerminal loop")
					cancel()
					return
				case *pb.ExecResponse_Stdout_:
					if _, err := os.Stdout.Write(msg.GetStdout().Data); err != nil {
						zap.L().Debug("Could not write to stdout", zap.Error(err))
					} else {
						os.Stdout.Write([]byte("\n"))
					}
				case *pb.ExecResponse_Stderr_:
					if _, err := os.Stderr.Write(msg.GetStderr().Data); err != nil {
						zap.L().Debug("Could not write to stderr", zap.Error(err))
					} else {
						os.Stderr.Write([]byte("\n"))
					}
				}

			}
		}
	}(ctx)

	go func(ctx context.Context) {
		buf := make([]byte, 3*1024)
		defer zap.L().Debug("Exiting stdin loop")
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := os.Stdin.Read(buf)
				if err != nil {
					if errors.Is(err, io.EOF) {
						if err := strm.Send(&pb.ExecRequest{
							Type: &pb.ExecRequest_Kill_{
								Kill: &pb.ExecRequest_Kill{},
							},
						}); err != nil {
							if errors.Is(err, io.EOF) {
								cancel()
								return
							}
							zap.L().Debug("Could not send stdin", zap.Error(err))
						}
						return
					}

					return
				}

				if err := strm.Send(&pb.ExecRequest{
					Type: &pb.ExecRequest_WriteData_{
						WriteData: &pb.ExecRequest_WriteData{
							Data: buf[:n],
						},
					},
				}); err != nil {
					if errors.Is(err, io.EOF) {
						cancel()
						return
					}
					zap.L().Debug("Could not send stdin", zap.Error(err))
				}
			}
		}
	}(ctx)

	zap.L().Debug("Waiting for exit")
	<-ctx.Done()

	zap.L().Debug("Exiting...")

	return nil
}
