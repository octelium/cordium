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

package exec

import (
	"context"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"al.essio.dev/pkg/shellescape"
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
	WorkingDir string
	EnvVars    []string
	RunAsRoot  bool
	NoStdin    bool
}

var cmdArgs args

func init() {
	Cmd.Flags().StringVarP(&cmdArgs.WorkingDir, "workdir", "w", "",
		"Working directory inside the Workspace for the command (e.g. /workspace/repo)")
	Cmd.Flags().StringArrayVarP(&cmdArgs.EnvVars, "env", "e", nil,
		"Set an environment variable for this command (KEY=VALUE). Repeatable: -e FOO=bar -e BAZ=qux")
	Cmd.Flags().BoolVar(&cmdArgs.RunAsRoot, "root", false,
		"Run the command as root inside the Workspace")
	Cmd.Flags().BoolVar(&cmdArgs.NoStdin, "no-stdin", false,
		"Disable stdin. Use when piping output from another command to avoid blocking on stdin reads.")
}

var Cmd = &cobra.Command{
	Use:   "exec <workspace> -- <command> [args...]",
	Short: "Run a command in a Workspace",
	Long: `Run a command in a running Workspace and stream its output.

The command and its arguments must follow a double-dash (--) separator.
stdin, stdout, and stderr are all connected, making exec suitable for
interactive commands, pipelines, and scripted automation.

The exit code of the remote command is propagated if the command exits
with a non-zero code, cordium exec exits with the same code.

Sending Ctrl+C terminates the remote command.`,
	Example: `
  # Run a single command
  cordium exec abc -- ls -lah /workspace/repo

  # Run in a specific working directory
  cordium exec abc -w /workspace/repo -- make build

  # Run as root
  cordium exec abc --root -- apt-get install -y ripgrep

  # Set environment variables for the command
  cordium exec abc -e GOOS=linux -e GOARCH=amd64 -- go build ./...

  # Combine working directory and environment variables
  cordium exec abc -w /workspace/repo \
    -e NODE_ENV=test \
    -e CI=true \
    -- npm test

  # Run a shell pipeline (use sh -c for pipelines and redirects)
  cordium exec abc -- sh -c "cat /etc/os-release | grep VERSION_ID"

  # Pipe local input to a remote command
  echo "SELECT version();" | cordium exec abc -- psql mydb

  # Capture remote command output to a local file
  cordium exec abc -- cat /workspace/repo/output.json > local-output.json

  # Stream logs from a running process
  cordium exec abc -- tail -f /var/log/app.log

  # Run a build and check its exit code in a script
  cordium exec abc -w /workspace/repo -- make test
  if [ $? -ne 0 ]; then echo "Tests failed"; exit 1; fi

  # Run without reading stdin
  cordium exec abc --no-stdin -- go vet ./...

  # Run a root command with environment overrides
  cordium exec abc --root -e DEBIAN_FRONTEND=noninteractive \
    -- apt-get install -y --no-install-recommends build-essential

  # Run a database migration as part of a CI/CD pipeline
  cordium exec abc -w /workspace/repo \
    -e DATABASE_URL=postgres://staging-db.octelium/myapp \
    -- npm run db:migrate

  # Run an AI agent task inside an ephemeral Workspace
  cordium exec abc \
    -e ANTHROPIC_API_KEY_SECRET=anthropic-key \
    -w /workspace/repo \
    -- claude --print "Fix the failing tests in src/auth/ and commit the result."`,
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

	envVars, err := parseEnvVars(cmdArgs.EnvVars)
	if err != nil {
		return err
	}

	quoted := make([]string, len(args[1:]))
	for idx, arg := range args[1:] {
		quoted[idx] = shellescape.Quote(arg)
	}

	if err := strm.Send(&pb.ExecRequest{
		Type: &pb.ExecRequest_Request_{
			Request: &pb.ExecRequest_Request{
				WorkspaceRef: &metav1.ObjectReference{
					Name: i.FirstArg(),
				},
				Command:    strings.Join(quoted, " "),
				WorkingDir: cmdArgs.WorkingDir,
				EnvVars:    envVars,
				RunAsRoot:  cmdArgs.RunAsRoot,
				HasStdin:   !cmdArgs.NoStdin,
			},
		},
	}); err != nil {
		return err
	}

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
					zap.L().Debug("Got close msg. Exiting ListenTerminal loop",
						zap.Int32("code", msg.GetExit().Code))

					cancel()
					if msg.GetExit().Code != 0 {
						os.Exit(int(msg.GetExit().Code))
					}
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

		if cmdArgs.NoStdin {
			return
		}

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

func parseEnvVars(raw []string) ([]*pb.ExecRequest_Request_EnvVar, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]*pb.ExecRequest_Request_EnvVar, 0, len(raw))
	for _, s := range raw {
		key, val, ok := strings.Cut(s, "=")
		if !ok || key == "" {
			return nil, errors.Errorf("invalid --env value %q: expected KEY=VALUE", s)
		}
		out = append(out, &pb.ExecRequest_Request_EnvVar{
			Key:   key,
			Value: val,
		})
	}
	return out, nil
}
