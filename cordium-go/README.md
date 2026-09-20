# cordium-go

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Reference](https://pkg.go.dev/badge/github.com/octelium/cordium/cordium-go.svg)](https://pkg.go.dev/github.com/octelium/cordium/cordium-go)

The official Go SDK for [Cordium](https://github.com/octelium/cordium), the self-hosted, identity-based sandbox platform built on Kubernetes and [Octelium](https://github.com/octelium/octelium).

It is what a program uses to create, start, drive and destroy sandboxes (i.e. Workspaces) — a CI pipeline, a scheduler, an internal developer platform, or an AI agent that needs somewhere safe to run code.

```go
c, err := cordium.New(ctx, cordium.WithDomain("example.com"))
if err != nil {
	return err
}
defer c.Close()

ws, err := c.Workspaces().Run(ctx,
	cordium.WithImage("python:3.11-slim"),
	cordium.Ephemeral(),
)
if err != nil {
	return err
}
defer ws.Delete(context.WithoutCancel(ctx))

res, err := ws.Exec(ctx, "python -c 'print(6 * 7)'")
if err != nil {
	return err
}
fmt.Println(res.Output(), res.ExitCode) // 42 0
```

## Contents

- [Install](#install)
- [Authentication](#authentication)
- [Workspaces](#workspaces)
- [Executing commands](#executing-commands)
- [Files](#files)
- [Logs and terminals](#logs-and-terminals)
- [Waiting and watching](#waiting-and-watching)
- [Applications and sharing](#applications-and-sharing)
- [Spaces, Templates and Secrets](#spaces-templates-and-secrets)
- [Errors](#errors)
- [The lower level API](#the-lower-level-api)
- [Design notes](#design-notes)

## Install

```bash
go get github.com/octelium/cordium/cordium-go
```

```go
import cordium "github.com/octelium/cordium/cordium-go"
```

The SDK requires Go 1.26 or later.

## Authentication

Cordium runs on top of an Octelium Cluster and therefore uses Octelium Credentials. By default the Client reads its Cluster domain and its credentials from the environment, which is what makes the same binary work unchanged on a laptop, in a CI job, in a Kubernetes Pod and inside a Cordium Workspace:

| Variable | Meaning |
| --- | --- |
| `CORDIUM_DOMAIN` or `OCTELIUM_DOMAIN` | The Cluster domain |
| `OCTELIUM_ACCESS_TOKEN` | A ready to use access token |
| `OCTELIUM_ASSERTION_FILE` | A file holding an assertion, re-read on every authentication |
| `OCTELIUM_ASSERTION` | An assertion supplied directly |
| `OCTELIUM_AUTH_TOKEN` | A Credential authentication token |

```go
c, err := cordium.New(ctx) // everything from the environment
```

Credentials can also be supplied explicitly:

```go
c, err := cordium.New(ctx,
	cordium.WithDomain("example.com"),
	cordium.WithAuthenticationToken(os.Getenv("MY_TOKEN")),
)
```

| Option | Use |
| --- | --- |
| `WithAuthenticationToken` | An Octelium Credential authentication token. One-time: the Session is not silently recreated once it expires |
| `WithAccessToken`, `WithAccessTokenProvider` | An externally managed access token |
| `WithAssertion`, `WithAssertionFile` | Workload identity federation (e.g. GitHub Actions OIDC, Kubernetes projected ServiceAccount tokens). Re-read on every authentication, so the Client can replace an expired Session on its own |
| `WithOcteliumClient` | Reuse an `octelium-go` Client that the process already has, along with its Session |
| `WithOcteliumOptions` | Anything else the Octelium SDK supports, such as custom authenticators, gRPC keepalive or an alternative transport |

A `Client` is safe for concurrent use. Create one per Cluster and share it for the lifetime of the process.

## Workspaces

A Workspace is Cordium's sandbox. It is described with composable options that cover the whole spec — image, repository, lifecycle Tasks, environment, network, resources and exposed ports:

```go
ws, err := c.Workspaces().Run(ctx,
	cordium.WithImage("node:20"),
	cordium.WithRepo("https://github.com/myorg/web",
		cordium.WithBranch("main"),
		cordium.WithDepth(1),
	),
	cordium.WithEnv("NODE_ENV", "development"),
	cordium.WithEnvFromSecret("NPM_TOKEN", "npm-token"),
	cordium.WithTask("deps", "npm ci",
		cordium.TaskWorkingDir("/workspace/repo"),
		cordium.TaskAbortOnFailure(),
	),
	cordium.WithPostStartTask("dev", "npm run dev",
		cordium.TaskWorkingDir("/workspace/repo"),
		cordium.TaskInBackground(),
	),
	cordium.WithApp("web", 3000, cordium.AsDefaultApp()),
	cordium.WithResources(cordium.Resources{CPUMillicores: 4000, MemoryMB: 8192}),
)
```

`Run` creates the Workspace, starts it and waits for it to reach the `RUNNING` state. `Create` does only the first step, which is what a caller that wants to start it later, or with run-specific variables, needs:

```go
ws, err := c.Workspaces().Create(ctx, cordium.WithTemplate("ci-runner.my-project"))
if err != nil {
	return err
}
if err := ws.Start(ctx, cordium.WithRunVar("BRANCH", "main"), cordium.WithRegion("eu-west")); err != nil {
	return err
}
if err := ws.WaitUntilRunning(ctx); err != nil {
	return err
}
```

The full lifecycle lives on the handle: `Start`, `Stop`, `Delete`, `Refresh`, `Update` and the wait helpers. The accessors (`Name`, `State`, `Hostname`, `URL`, `IsRunning`, …) read the last state the Cluster reported, with no network round trip.

Workspaces are listed one page at a time, or iterated over:

```go
for ws, err := range c.Workspaces().All(ctx, cordium.InSpace("my-project")) {
	if err != nil {
		return err
	}
	fmt.Println(ws.Name(), ws.State())
}
```

### Where the options come from

| Area | Options |
| --- | --- |
| Image | `WithImage`, `WithPrivateImage`, `WithDockerfile`, `WithDockerfileFile`, `WithDockerfileURL`, `WithImageFromGit`, `WithImageFromRepoDockerfile`, `WithImageFromRepoDevcontainer` |
| Repositories | `WithRepo` (+ `WithBranch`, `WithDepth`, `WithCheckout`, `WithSingleBranch`, `WithShallowSubmodules`, `WithoutLazyUnshallow`, `WithRepoAuth`), `WithAdditionalRepo` |
| Runtime | `WithEnv`, `WithEnvs`, `WithEnvFromSecret`, `WithTask`, `WithPostStartTask`, `WithPreStopTask`, `WithCmd`, `WithEntrypoint`, `WithoutInit`, `WithDevcontainerFeature`, `WithAutoStop`, `WithoutTimeout` |
| Isolation | `WithReadOnlyRootFilesystem`, `WithAddedCapabilities`, `WithDroppedCapabilities`, `WithEgressDefaultDeny`, `WithEgressAllow`, `WithEgressDeny` |
| Octelium | `WithOcteliumServices`, `WithAllOcteliumServices` |
| Exposure | `WithApp`, `WithPort` |
| Sizing | `WithCPU`, `WithMemory`, `WithStorage`, `WithResources` |
| Placement and parameters | `WithTemplate`, `WithSpace`, `WithVar`, `WithVars`, `Ephemeral`, `Persistent`, `WithDisplayName` |
| Escape hatches | `FromSpec`, `FromSpecJSON`, `WithSpecFunc` |

`WithSpecFunc` and `FromSpec` mean that a spec field the SDK has no option for yet is never out of reach:

```go
ws, err := c.Workspaces().Create(ctx,
	cordium.WithImage("ubuntu:24.04"),
	cordium.WithSpecFunc(func(spec *cordiumv1.Workspace_Spec) error {
		spec.Vars = append(spec.Vars, &cordiumv1.Workspace_Spec_Var{
			Name:  "REGION",
			Value: region,
		})
		return nil
	}),
)
```

## Executing commands

`Exec` runs a command, waits for it and hands back its output and its exit status:

```go
res, err := ws.Exec(ctx, "go test ./...",
	cordium.WithWorkingDir("/workspace/repo"),
	cordium.WithExecEnv("CGO_ENABLED", "0"),
	cordium.WithExecTimeout(10*time.Minute),
)
if err != nil {
	return err // the command could not be run at all
}
fmt.Println(res.ExitCode, res.StdoutString(), res.StderrString())
```

A non-zero exit code is **not** an error: `err` reports only a failure to run the command. That keeps the captured output available on the normal path, which is what a caller that has to decide what to do about a failure actually needs. `res.Err()` turns an unsuccessful command into an `*ExitError` when that is what you want.

The command is interpreted by the Workspace shell, so pipes and redirections work. `Argv` builds a command out of arguments that must **not** be interpreted, which is the right default for anything that comes from a user, a config file or a model:

```go
res, err := ws.Exec(ctx, cordium.Argv("git", "clone", untrustedURL, "/workspace/repo"))
```

`ExecStream` returns as soon as the command has started, so its output can be consumed while it runs. The output arrives on a single channel, in the order in which it was produced, with each chunk tagged with the stream it came from:

```go
sess, err := ws.ExecStream(ctx, "npm run build")
if err != nil {
	return err
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
```

A session is also an `io.Writer` onto the command's standard input, and `Kill` terminates it.

Other output options: `WithStdout`, `WithStderr` and `WithCombinedOutput` stream into writers as the output arrives; `WithoutCapture` turns the in-memory capture off for very chatty commands; `WithMaxCaptureBytes` caps it (4 MiB by default) and marks the result as truncated.

> **Standard input has no half-close.** Cordium's execution protocol carries no way to close a command's standard input on its own, so a command that reads until end of file (`cat`, `sha256sum`, `psql`) never sees the input end. `WithTerminateOnStdinEOF` terminates the command once the input source is exhausted, which closes its standard input as a side effect and lets it flush and exit. A command that must do substantial work *after* consuming its input should not be run that way.

## Files

```go
err := ws.WriteFileString(ctx, "/workspace/repo/.env", "LOG_LEVEL=debug\n")

report, err := ws.ReadFile(ctx, "/workspace/repo/coverage.out")

err = ws.UploadFile(ctx, "./patch.diff", "/workspace/repo/patch.diff")
err = ws.DownloadFile(ctx, "/workspace/repo/dist/app.tar.gz", "./app.tar.gz")
```

The transfer moves data base64 encoded through the same execution stream as any other command, so it needs `base64` and `head` inside the Workspace — coreutils and busybox both provide them. The exec options apply, so `AsRoot()` writes as root and `WithExecTimeout` bounds the transfer.

`WriteFile` and `ReadFile` hold the content in memory (`ReadFile` caps it at 64 MiB; `WithMaxCaptureBytes` changes the cap). `UploadFile`, `DownloadFile` and `DownloadFileTo` stream instead. For bulk data, a repository, an object store or an SSH/SFTP transfer is still the right tool.

## Logs and terminals

The initialization logs are where a failed image build or a failed lifecycle Task explains itself:

```go
logs, err := ws.Logs(ctx)
if err != nil {
	return err
}
defer logs.Close()

for entry := range logs.Entries() {
	fmt.Printf("[%s] %s", entry.Stage, entry.Data)
}
return logs.Err()
```

`ws.StreamLogsTo(ctx, os.Stdout, os.Stderr)` is the one-line version.

A terminal is a PTY-backed shell that outlives the connection that created it — several clients can attach to the same one, and detaching does not end it:

```go
term, err := ws.NewTerminal(ctx, cordium.WithTerminalSize(120, 40))
if err != nil {
	return err
}
defer term.Remove(context.WithoutCancel(ctx))

return term.Pipe(ctx, os.Stdin, os.Stdout)
```

## Waiting and watching

| Helper | Waits for |
| --- | --- |
| `WaitUntilRunning` | `RUNNING`: fully initialized |
| `WaitUntilReady` | `PREPARING` or `RUNNING`: the earliest point at which exec and terminals are accepted |
| `WaitUntilStopped` | `STOPPED`: how a one-shot, `WithAutoStop` Workspace is awaited |
| `WaitForState` | Any of the given states |
| `WaitFor` | An arbitrary condition on the Workspace |

The wait helpers follow the Cluster's watch stream, reconcile against the Cluster so that a state reached before the stream opened is never missed, and re-establish the stream when an intermediary drops it. The caller's context bounds the wait — it normally carries a deadline.

`Watch` exposes the raw event stream, which is what a dashboard or a scheduler needs:

```go
watcher, err := c.Workspaces().Watch(ctx)
if err != nil {
	return err
}
defer watcher.Close()

for ev := range watcher.Events() {
	if ev.StateChanged() {
		fmt.Printf("%s %s -> %s\n", ev.Type, ev.Workspace.Name(), ev.Workspace.State())
	}
}
return watcher.Err()
```

## Applications and sharing

An Application is a named port that the Cordium portal serves over HTTPS:

```go
fmt.Println(ws.URL())           // the default Application
fmt.Println(ws.AppURL("api"))   // a named one
fmt.Println(ws.PortURL(8080))   // a port

if err := ws.SharePort(ctx, "api", cordium.ShareWithMembers); err != nil {
	return err
}
```

Reaching those URLs still requires authorization. `c.HTTPClient()` is an HTTP client that attaches the Cluster access token to them, and to nothing else:

```go
resp, err := c.HTTPClient().Get(ws.AppURL("api") + "/healthz")
```

`WithAuthorizedHTTPHosts` widens the set of destinations that may receive the token, and `WithHTTPAuthorizationPolicy` replaces the rule outright.

## Spaces, Templates and Secrets

```go
spc, err := c.Spaces().Create(ctx, "my-project",
	cordium.AsOrganizationSpace(),
	cordium.WithSpaceDefaultResources(cordium.Resources{CPUMillicores: 2000, MemoryMB: 4096}),
	cordium.WithSpaceMaxResources(cordium.Resources{CPUMillicores: 8000}),
)

_, err = c.Secrets().CreateString(ctx, "npm-token.my-project", os.Getenv("NPM_TOKEN"))

_, err = c.Templates().Create(ctx, "ci-runner.my-project",
	cordium.WithImage("golang:1.25"),
	cordium.WithRepo("https://github.com/myorg/api"),
	cordium.WithGitProvider("github"),
)

_, err = c.Memberships().Add(ctx, "my-project.cordium", "jane@example.com", cordium.RoleAdmin)
```

A Template is a reusable Workspace configuration, so it takes the **same** options as a Workspace, minus the few that only a single Workspace can carry (`WithApp` and `Ephemeral`). A pre-build snapshots a fully initialized filesystem, which is the single biggest lever on startup time:

```go
if _, err := c.Templates().Build(ctx, "ci-runner.my-project", "latest"); err != nil {
	return err
}
build, err := c.Templates().WaitForBuild(ctx, "ci-runner.my-project")
```

Names are resolved by the Cluster: a short name such as `npm-token` lands in the caller's default Space, while `npm-token.my-project` is explicit. `ShortName` strips the qualification back off for display.

The rest of the resource API: `c.UserSecrets()` (including Cluster-generated SSH keys), `c.GitProviders()` (GitHub, GitLab and generic OAuth2), `c.Regions()`, `c.UserConfig()` (dotfiles, personal environment and preferred Region) and `c.Management()` (the ClusterConfig, for Cluster administrators).

## Errors

Cluster errors keep their gRPC status, and the SDK classifies them:

```go
_, err := c.Workspaces().Get(ctx, name)
switch {
case cordium.IsNotFound(err):
	// ...
case cordium.IsResourceExhausted(err):
	// a Space or Cluster quota was reached
case cordium.IsUnavailable(err):
	// transient; worth retrying
}
```

A failed run is reported as a `*WorkspaceFailureError` carrying the Cluster's own structured failure, so a caller can branch on the exact reason instead of parsing a message:

```go
var failure *cordium.WorkspaceFailureError
if errors.As(err, &failure) {
	switch cordium.FailureReason(failure.Failure) {
	case "ImagePull", "ImageBuild":
		// the image is wrong
	case "Task":
		log.Println("failed task:", failure.Failure.GetTask().GetName())
	}
}
```

## The lower level API

Nothing the Cluster exposes is out of reach. The generated gRPC clients are always available:

```go
c.MainService()       // Workspaces, Spaces, Templates, Secrets, Memberships, ...
c.WorkspaceService()  // exec, terminals and logs inside a running Workspace
c.ManagementService() // the administrative API
c.Conn()              // the authenticated gRPC connection
c.HTTPClient()        // an HTTP client for the Workspace Applications
c.Octelium()          // the Octelium SDK Client: Session, access token, transports
```

```go
list, err := c.MainService().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{
	Filter: &cordiumv1.ListWorkspaceOptions_SpaceRef{
		SpaceRef: &metav1.ObjectReference{Name: "my-project"},
	},
})
```

## Design notes

- **A Workspace handle caches state.** The lifecycle methods and the wait helpers keep the cache current; the accessors never hit the network. `Refresh` forces a reload.
- **Streams are channels, and they must be drained.** `Output()`, `Entries()` and `Events()` apply backpressure, which is what keeps a slow consumer from silently losing output. Every stream also ends on context cancellation, so nothing leaks. `ExecSession.Wait` drains the output channel for you only when `Output()` was never called.
- **`Close` the things you own.** The `Client`, and every `ExecSession`, `LogStream`, `Terminal` and `WorkspaceWatcher`. Cleanup that must survive a canceled context — deleting a one-shot Workspace above all — belongs on `context.WithoutCancel(ctx)`.
- **Options validate eagerly.** An invalid option fails at the call that used it, wrapping `ErrInvalidArgument`, rather than producing a spec the Cluster will reject later.
- **Maps produce stable specs.** `WithEnvs` and `WithVars` sort their keys, so the same map always yields the same spec.

## License

Apache-2.0. See [LICENSE](./LICENSE).
