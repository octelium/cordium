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

package cordium_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	cordium "github.com/octelium/cordium/cordium-go"
	"github.com/octelium/octelium/apis/main/cordiumv1"
)

// Creating a sandbox, running a command in it and throwing it away is the
// shortest useful path through the SDK.
func Example() {
	ctx := context.Background()

	c, err := cordium.New(ctx, cordium.WithDomain("example.com"))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Run(ctx,
		cordium.WithImage("python:3.11-slim"),
		cordium.Ephemeral(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Delete(context.WithoutCancel(ctx))

	res, err := ws.Exec(ctx, "python -c 'print(6 * 7)'")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Output())
}

// A Workspace that clones a repository, exposes a dev server and installs its
// dependencies before anyone connects to it.
func ExampleWorkspaceClient_Run_devEnvironment() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Run(ctx,
		cordium.WithImage("node:20"),
		cordium.WithRepo("https://github.com/myorg/web",
			cordium.WithBranch("main"),
			cordium.WithDepth(1),
		),
		cordium.WithTask("deps", "npm ci",
			cordium.TaskWorkingDir("/workspace/repo"),
			cordium.TaskAbortOnFailure(),
		),
		cordium.WithPostStartTask("dev", "npm run dev",
			cordium.TaskWorkingDir("/workspace/repo"),
			cordium.TaskInBackground(),
		),
		cordium.WithApp("web", 3000, cordium.AsDefaultApp()),
		cordium.WithResources(cordium.Resources{
			CPUMillicores: 4000,
			MemoryMB:      8192,
			StorageMB:     20000,
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("dev server:", ws.AppURL("web"))
}

// An unattended job: the Workspace stops itself once its Tasks complete, and
// the caller waits for that rather than for a RUNNING state.
func ExampleWorkspaceClient_Run_batchJob() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Create(ctx,
		cordium.WithTemplate("ci-runner.my-project"),
		cordium.Ephemeral(),
		cordium.WithAutoStop(),
		cordium.WithVar("BRANCH", "main"),
		cordium.WithTask("test", "make test", cordium.TaskAbortOnFailure()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Delete(context.WithoutCancel(ctx))

	if err := ws.Start(ctx); err != nil {
		log.Fatal(err)
	}

	// Follow the initialization logs while the job runs.
	go ws.StreamLogsTo(ctx, os.Stdout, os.Stderr)

	if err := ws.WaitUntilStopped(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("the job finished")
}

// Streaming a command's output as it is produced, which is what a UI or an
// agent loop needs.
func ExampleWorkspace_ExecStream() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Get(ctx, "abc")
	if err != nil {
		log.Fatal(err)
	}

	sess, err := ws.ExecStream(ctx, "go test ./...",
		cordium.WithWorkingDir("/workspace/repo"))
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()

	for out := range sess.Output() {
		switch out.Stream {
		case cordium.StreamStdout:
			os.Stdout.Write(out.Data)
		case cordium.StreamStderr:
			os.Stderr.Write(out.Data)
		}
	}

	res, err := sess.Wait()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("exit code:", res.ExitCode)
}

// Arguments that come from a user, a config file or a model must not be
// interpreted by the Workspace shell.
func ExampleArgv() {
	userSuppliedPath := "/workspace/repo/my file.txt"

	command := cordium.Argv("wc", "-l", userSuppliedPath)
	fmt.Println(command)
	// Output: wc -l '/workspace/repo/my file.txt'
}

// Turning an unsuccessful command into an error, while keeping its output.
func ExampleExecResult_Err() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Get(ctx, "abc")
	if err != nil {
		log.Fatal(err)
	}

	res, err := ws.Exec(ctx, "make lint")
	if err != nil {
		log.Fatal(err) // The command could not be run at all.
	}
	if err := res.Err(); err != nil {
		log.Printf("lint failed: %v\n%s", err, res.CombinedString())
	}
}

// Piping data into a command. Cordium cannot half-close a command's standard
// input, so a command that reads until end of file needs
// WithTerminateOnStdinEOF.
func ExampleWithStdin() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Get(ctx, "abc")
	if err != nil {
		log.Fatal(err)
	}

	res, err := ws.Exec(ctx, "sha256sum",
		cordium.WithStdinString("the payload to hash"),
		cordium.WithTerminateOnStdinEOF(),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Output())
}

// Following every Workspace of the calling User, which is how a dashboard or a
// scheduler keeps its own view up to date.
func ExampleWorkspaceClient_Watch() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	watcher, err := c.Workspaces().Watch(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	for ev := range watcher.Events() {
		if ev.StateChanged() {
			fmt.Printf("%s %s -> %s\n", ev.Type, ev.Workspace.Name(), ev.Workspace.State())
		}
	}
	if err := watcher.Err(); err != nil {
		log.Fatal(err)
	}
}

// Iterating over every Workspace, one page at a time.
func ExampleWorkspaceClient_All() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	for ws, err := range c.Workspaces().All(ctx, cordium.InSpace("my-project")) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(ws.Name(), ws.State(), ws.URL())
	}
}

// Pre-building a Template so that its Workspaces start from a ready-made
// storage snapshot instead of being initialized from scratch.
func ExampleTemplateClient_Build() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if _, err := c.Templates().Create(ctx, "ml-env.research",
		cordium.WithImage("pytorch/pytorch:2.5.0-cuda12.4-cudnn9-devel"),
		cordium.WithRepo("https://github.com/myorg/research"),
		cordium.WithTask("deps", "pip install -r requirements.txt",
			cordium.TaskWorkingDir("/workspace/repo")),
	); err != nil && !cordium.IsAlreadyExists(err) {
		log.Fatal(err)
	}

	if _, err := c.Templates().Build(ctx, "ml-env.research", "latest"); err != nil {
		log.Fatal(err)
	}

	build, err := c.Templates().WaitForBuild(ctx, "ml-env.research")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("pre-build ready:", build.GetId())
}

// Dropping down to the generated gRPC API for anything the high level API does
// not cover.
func ExampleClient_MainService() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	list, err := c.MainService().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("total:", list.GetListResponseMeta().GetTotalCount())
}

// Interpreting the failure of a run.
func ExampleFailureReason() {
	ctx := context.Background()

	c, err := cordium.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ws, err := c.Workspaces().Run(ctx, cordium.WithImage("python:3.11-slim"))
	if err != nil {
		var failure *cordium.WorkspaceFailureError
		if errors.As(err, &failure) {
			switch cordium.FailureReason(failure.Failure) {
			case "ImagePull", "ImageBuild":
				log.Fatal("the image is wrong: ", failure.Failure.GetMessage())
			case "Task":
				log.Fatal("the setup task failed: ", failure.Failure.GetTask().GetName())
			default:
				log.Fatal(err)
			}
		}
		log.Fatal(err)
	}
	fmt.Println(ws.Name())
}
