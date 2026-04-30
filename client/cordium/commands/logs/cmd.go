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

package logs

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/client/common/client"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type args struct {
	NoColor   bool
	Timestamp bool
}

var cmdArgs args

func init() {
	Cmd.Flags().BoolVar(&cmdArgs.NoColor, "no-color", false,
		"Disable color and formatting in output")
	Cmd.Flags().BoolVarP(&cmdArgs.Timestamp, "timestamp", "t", false,
		"Prefix each log line with a timestamp")
}

var Cmd = &cobra.Command{
	Use:   "logs <workspace>",
	Short: "Stream logs from a Workspace",
	Long: `Show logs of an initializing or running Workspace in real-time.

Logs cover image pull and build output, repository clone output, and the
stdout and stderr of all lifecycle tasks (ON_CREATE, POST_START, PRE_STOP).`,
	Example: `
  # Stream logs from a Workspace
  cordium logs abc

  # Stream logs with timestamps
  cordium logs abc --timestamp

  # Stream logs with color disabled
  cordium logs abc --no-color`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
	Args: cobra.ExactArgs(1),
}

var (
	stderrLine = color.New(color.FgRed)
	tsColor    = color.New(color.FgHiBlack)
)

func doCmd(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cmdArgs.NoColor {
		color.NoColor = true
	}

	i, err := cliutils.GetCLIInfo(cmd, args)
	if err != nil {
		return err
	}

	conn, err := client.GetGRPCClientConn(ctx, i.Domain)
	if err != nil {
		return err
	}
	defer conn.Close()

	mainC := pb.NewMainServiceClient(conn)
	wsC := pb.NewWorkspaceServiceClient(conn)

	ws, err := mainC.GetWorkspace(ctx, &metav1.GetOptions{
		Name: i.FirstArg(),
	})
	if err != nil {
		return err
	}

	switch {
	case ucordiumv1.ToWorkspace(ws).IsInitializing():
		wtch, err := mainC.WatchWorkspace(ctx, &pb.WatchWorkspaceRequest{
			WorkspaceRef: umetav1.GetObjectReference(ws),
		})
		if err != nil {
			return err
		}

		if err := func() error {
			for {
				msg, err := wtch.Recv()
				if err != nil {
					return err
				}

				switch msg.Type.(type) {
				case *pb.WatchWorkspaceResponse_Update_:
					cur := msg.GetUpdate().NewItem
					old := msg.GetUpdate().OldItem
					zap.L().Debug("Got Workspace update", zap.String("state", cur.Status.State.String()))

					if cur.Status.State == old.Status.State {
						continue
					}

					switch {
					case !ucordiumv1.ToWorkspace(cur).IsInitializing():
						return nil

					}
				}
			}
		}(); err != nil {
			return err
		}
	case ucordiumv1.ToWorkspace(ws).IsStopped():
		return errors.Errorf("Workspace is stopped")
	}

	logStrm, err := wsC.ListenLog(ctx, &pb.ListenLogRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := logStrm.Recv()
		if err != nil {
			if grpcerr.IsCanceled(err) || grpcerr.IsDeadlineExceeded(err) {
				return nil
			}
			if err == io.EOF {
				zap.L().Debug("log stream ended")
				<-ctx.Done()
				return nil
			}
			zap.L().Debug("log stream recv error", zap.Error(err))
			time.Sleep(200 * time.Millisecond)
			continue
		}

		writeLogLine(msg)
	}
}

func writeLogLine(msg *pb.ListenLogResponse) {
	if len(msg.Data) == 0 {
		return
	}

	line := strings.TrimRight(string(msg.Data), "\n")
	if line == "" {
		return
	}

	var ts string
	if msg.CreatedAt != nil && cmdArgs.Timestamp {
		ts = tsColor.Sprintf("[%s] ", msg.CreatedAt.AsTime().Format("15:04:05.000"))
	}

	switch msg.Mode {
	case pb.ListenLogResponse_MODE_STDERR:
		fmt.Fprintln(os.Stderr, ts+stderrLine.Sprint(line))
	default:
		fmt.Fprintln(os.Stdout, ts+line)
	}
}
