# Cordium

Cordium is a free and open source, self-hosted, identity-based sandbox platform built on [Kubernetes](https://kubernetes.io) and [Octelium](https://octelium.com). It provides isolated, rootless container-based reproducible environments, called **Workspaces**, for both humans and machines, including AI agents.

Cordium is designed as a unified platform that serves two primary purposes:

- A sandbox platform for running both long-lived workloads (such as remote development environments, persistent servers, and interactive coding sessions) and short-lived tasks (such as AI agent execution, CI/CD jobs, and automated scripts) inside reproducible rootless container-based sandboxes accessible via web, SSH, CLI, and gRPC-based SDKs.
- An identity-based zero-trust remote access platform that leverages [Octelium](https://octelium.com) ZTNA capabilities to provide secretless, policy-driven access to infrastructure resources from within Workspaces, without exposing, distributing, or managing upstream resource credentials.

Cordium is 100% free and open source, self-hosted, and licensed under the [GNU Affero General Public License v3.0](LICENSE).


## Table of Contents

- [Main Features](#main-features)
- [Architecture](#architecture)
- [Core Concepts](#core-concepts)
  - [Spaces](#spaces)
  - [Workspaces](#workspaces)
  - [Templates](#templates)
  - [Secrets](#secrets)
  - [GitProviders](#gitproviders)
  - [User Configuration](#user-configuration)
- [Workspace Configuration](#workspace-configuration)
  - [Image Sources](#image-sources)
  - [Repository Cloning](#repository-cloning)
  - [Runtime Configuration](#runtime-configuration)
  - [Applications and Port Sharing](#applications-and-port-sharing)
  - [Devcontainer Support](#devcontainer-support)
  - [Variable Substitution](#variable-substitution)
  - [YAML Configuration Files](#yaml-configuration-files)
- [Resource Limits](#resource-limits)
- [Workspace Lifecycle](#workspace-lifecycle)
- [Access Methods](#access-methods)
  - [Web Portal](#web-portal)
  - [CLI](#cli)
  - [gRPC API](#grpc-api)
- [Comparison with Other Platforms](#comparison-with-other-platforms)
- [Self-Hosting](#self-hosting)
- [License](#license)


## Main Features

- **Unified access for humans and machines.** The same Workspace can be accessed interactively through a browser-based terminal, via SSH, through the CLI, or programmatically via gRPC-based SDKs. This makes Cordium equally suitable as a remote development environment for engineers (comparable to GitHub Codespaces or Coder) and as an execution sandbox for AI agents, CI/CD pipelines, and automated workloads. Workspaces support both long-lived runs (remote development, persistent servers) and short-lived runs (AI agent tasks, build jobs, scripted automation).

- **Highly customizable sandbox environments.** Workspace filesystems can be built from OCI/Docker images, Dockerfiles, git repositories, and devcontainers. Multi-repository cloning, including private repositories with authentication. Workspace and Template configurations are fully declarative via YAML files and can be instantiated through the `cordium` CLI or managed programmatically via the gRPC API. Each running Workspace supports full root access within the sandbox, allowing users to run containers, install system packages, and run privileged services. Templates support pre-building for fast Workspace instantiation. Spaces provide namespacing for Workspaces, Templates, Secrets, and GitProviders. Secrets can be referenced in environment variables and repository authentication configurations. Workspace storage can be persistent or ephemeral. Resource limits (memory, CPU, and storage) can be defined at the Workspace, Space, and Cluster level. Variable substitution (`${{ vars.NAME }}`) allows Templates to be parameterized at instantiation time.

- **Rootless container-based sandboxing on standard Kubernetes.** No bare-metal nodes or specialized hardware are needed. Workspaces run efficiently on any Kubernetes cluster while providing isolation through Linux namespaces, cgroups, seccomp, and capabilities.

- **Zero-trust platform on Octelium.** Cordium is built on [Octelium](https://octelium.com), inheriting its zero-trust infrastructure as a foundational layer and using its resource types (Users, Sessions, Devices, Services, Namespaces, Policies, and others) to provide the following capabilities:

    - **Dynamic secretless access.** Octelium's layer-7 awareness enables Users to seamlessly access resources protected by application-layer credentials without exposing, managing, or distributing such secrets (read more [here](https://octelium.com/docs/octelium/latest/management/core/service/secretless)). This works for HTTP APIs without sharing API keys and access tokens, SSH servers without sharing passwords and private keys, Kubernetes clusters, PostgreSQL/MySQL databases, and any L7 protocol protected by mTLS.

    - **Modern, dynamic, fine-grained access control.** Octelium provides a centralized, scalable, fine-grained, dynamic, context-aware, layer-7-aware, attribute-based access control system (ABAC) evaluated on a per-request basis (read more [here](https://octelium.com/docs/octelium/latest/management/core/policy)) with policy-as-code using [CEL](https://cel.dev/) and [OPA](https://www.openpolicyagent.org/) (Open Policy Agent). Octelium has no notion of an "admin" user, enforcing zero standing privileges by default.

    - **Continuous strong authentication.** A unified authentication system for both human and workload Users, supporting any web identity provider (IdP) that uses OpenID Connect or SAML 2.0, as well as GitHub OAuth2 (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers#web-identity-providers)). It also supports secretless authentication for workloads via OIDC-based assertions (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers#workload-identity-providers)). Built-in support for MFA, re-authentication, and login via FIDO2/WebAuthn/Passkey, TOTP, and TPM 2.0 Authenticators.

    - **OpenTelemetry-native auditing and visibility.** Real-time, identity-based, L7-aware visibility and access logging. Every request is logged and exported to your OpenTelemetry OTLP receivers for integration with log management and SIEM providers.

- **Kubernetes-native with pluggable storage.** Cordium leverages Kubernetes-native storage for Workspace persistence and integrates with any Kubernetes CSI driver and VolumeSnapshot provider. This includes Longhorn, AWS EBS, GCP Persistent Disk, Azure Disk, Ceph/Rook, OpenEBS, and any other CSI-compliant storage solution. Storage class and volume snapshot class selection is policy-driven via CEL expressions, allowing operators to route different Workspace types to different storage backends.

- **Ready for agentic AI.** Cordium is not only a sandbox for isolated long-lived and short-lived process execution by sandboxed AI agents. It leverages Octelium's zero-trust infrastructure to provide identity-based, fine-grained, L7-aware, context-aware, ABAC-based access to resources of any type from within Workspaces. This includes secretless access for resources that require application-layer credentials (API keys, access tokens, SSH passwords and private keys, database passwords, and mTLS private keys) without exposing, distributing, or sharing such credentials with the sandboxed AI agent. Credential mappings and privilege scopes can be dynamically assigned to specific agents based on identity and context.

- **Open source and designed for self-hosting.** Cordium, like Octelium itself, is fully open source and designed for single-tenant self-hosting. There is no proprietary cloud-based control plane, and this is not a limited open source version of a separate fully functional paid SaaS product. Cordium can be deployed on a single-node Kubernetes cluster running on a low-cost cloud VM/VPS, or on production-grade multi-node Kubernetes installations, cloud-based or on-premises, with no vendor lock-in.

## Architecture

Cordium runs entirely on Kubernetes. Each Workspace is a managed Kubernetes pod, provisioned and controlled by **Nocturne**, the Workspace controller. Each Workspace pod uses a three-layer isolation model:

1. **Outer supervisor container** (privileged, bootstrap only): sets up the cgroup hierarchy, configures user namespace mappings, creates device nodes, and launches the inner container via rootless Podman. Does not run user code.

2. **Hardened inner container**: runs with `--cap-drop ALL` and selective capability grants, a custom seccomp profile, `hidepid=2` on `/proc`, a mostly read-only filesystem, and a customized nested cgroup hierarchy. This is the actual security boundary.

3. **Workspace container** (rootless, user-facing): runs inside the rootless context with slirp4netns network isolation and cgroup-enforced resource limits. Despite being rootless from the host's perspective, the Workspace container provides full root capability within its user namespace. Users can install packages, run nested containers via rootless Podman, manage services, and execute arbitrary processes as root, all without affecting the host.

```
Kubernetes Node (dedicated Workspace pool)
└── Workspace Pod
    └── Outer Supervisor Container  (privileged, bootstrap only)
        └── Inner Container         (hardened: seccomp, cap-drop, hidepid)
            └── Workspace Container (rootless Podman, full root in user namespace)
```

Workspace storage is backed by Kubernetes PersistentVolumeClaims, compatible with any CSI driver. Template pre-builds use Kubernetes VolumeSnapshots to capture a fully initialized Workspace state, enabling subsequent Workspaces to restore from the snapshot and start in seconds.

When a Workspace starts, a dedicated Octelium Session is created for that Workspace run. An `octelium connect` process running inside the Workspace establishes an encrypted tunnel to the Octelium gateway using this Session as its identity. Processes inside the Workspace then access Octelium-managed resources through standard tools and the credential exchange happens entirely at the Octelium gateway. The Workspace never holds the actual credentials.


## Core Concepts

Cordium organizes resources in a hierarchical structure:

```
Cluster
└── Space (namespace for related resources)
    ├── Template (reusable Workspace configuration)
    │   └── Workspace (running sandbox instance)
    ├── Secret (sensitive values, referenced by Workspaces, Templates, and GitProviders)
    └── GitProvider (OAuth2 git authentication configuration)

User (Octelium-managed, global scope)
├── UserSecret (per-user sensitive values, including SSH key pairs)
└── UserConfig (per-user defaults: dotfiles, environment variables, tasks)
```

### Spaces

A **Space** is the top-level namespace that groups related Workspaces, Templates, Secrets, and GitProviders. All resources belong to exactly one Space. Each User can create one or more Spaces. Each Space has:

- A unique name scoped to the owning User
- One or more Templates (a `default` Template is created automatically)
- Space-scoped Secrets accessible to all Templates and Workspaces within the Space
- Optional GitProviders for OAuth2-based git authentication
- Optional runtime configuration (environment variables and lifecycle tasks) inherited by all Workspaces in the Space
- Optional resource limits (default and maximum) applied to Workspaces within the Space

Spaces provide a natural boundary for projects or use cases. A developer might have a Space for each major project; an AI agent platform might create a Space per tenant or per workflow.

### Workspaces

A **Workspace** (synonymous with **sandbox**) is the fundamental execution unit in Cordium. Each Workspace belongs to one Template and one Space, is owned by an Octelium User, and runs as an isolated, rootless container environment. Each running Workspace:

- Has full root access within the container (install packages, run nested containers, manage services)
- Can have persistent or ephemeral storage
- Is accessible via web-based terminals, SSH, `cordium` CLI commands, or programmatic API access
- Has a dedicated Octelium Session for secretless infrastructure access via `octelium connect`

Workspace spec includes: image configuration (public/private registry, Dockerfile, Git repo, devcontainer), primary and additional repository cloning with authentication, runtime tasks (`ON_CREATE`, `POST_START`, `PRE_STOP`), devcontainer features, environment variables with static values or Secret references, resource limits (CPU, memory, storage), and named applications to expose Workspace servers.

### Templates

A **Template** defines a reusable Workspace configuration within a Space. Templates encapsulate the full specification for a Workspace: image source, repository configuration, runtime tasks, environment variables, resource limits, and application definitions.

Every Space has a `default` Template created automatically. Additional Templates can be created for different project configurations, language environments, or use cases within the same Space.

Templates support **pre-building**: a build Workspace is created from the Template spec, runs to completion, and produces a VolumeSnapshot. Subsequent Workspaces instantiated from a pre-built Template restore from this snapshot, reducing startup time from minutes to seconds. Template spec shares most of Workspace spec configuration, and additionally supports a GitProvider association for automatic OAuth2 credential injection and a `vars` definition for parameterized instantiation.

### Secrets

**Secrets** are sensitive values (API keys, access tokens, SSH private keys, etc.) stored within a Space. They can be referenced by name in Template and Space runtime configurations, for example as environment variable values or repository authentication credentials. Secret values are never returned through the API after creation and can only be used by the Cluster or exposed to Workspaces at runtime.

### GitProviders

A **GitProvider** configures OAuth2 authentication against a git hosting service: GitHub, GitLab, or any generic OAuth2 provider. When a Template references a GitProvider, users authenticate once through the Cordium portal's OAuth2 flow, and their access tokens are automatically injected into Workspaces at startup, enabling authenticated `git clone`, `git push`, and other git operations without manual credential configuration.

### User Configuration

Per-user configuration applies across all of a user's Workspaces, regardless of Space or Template:

- **Dotfiles**: A git repository URL containing dotfiles. Cordium clones it into `~/dotfiles` at Workspace creation time and runs standard install scripts (`install.sh`, `bootstrap.sh`, `setup.sh`, or their equivalents).
- **User environment variables**: Injected into every Workspace. Values can be static or sourced from UserSecrets.
- **User tasks**: `POST_START` tasks that run in every Workspace after startup.
- **UserSecrets**: Per-user encrypted values (strings, byte arrays, SSH keys) stored independently of Space-scoped Secrets. SSH key UserSecrets are generated server-side as ECDSA key pairs. The public key is available for external registration; the private key is automatically loaded into the Workspace's SSH agent.


## Workspace Configuration

Workspaces can be configured through multiple mechanisms merged at initialization time in a defined precedence order: Workspace spec → Template spec → Space spec → Cluster defaults. Environment variables are merged across all levels; resource limits are resolved by precedence and then capped by Space and Cluster maximums.

### Image Sources

| Source | Description |
|---|---|
| **Container registry** | Pull a pre-built image from any OCI-compatible registry, with optional authentication |
| **Dockerfile** | Provide a Dockerfile inline or from a URL in the Template or Workspace spec |
| **Git repository** | Clone a repository and build from a Dockerfile or devcontainer spec found within it |
| **Repository devcontainer** | Detect and build from `.devcontainer/devcontainer.json` in the configured repository |

If no image source is specified, Cordium uses a default base image with common development tools pre-installed.

### Repository Cloning

- **Primary repository**: Cloned into `/workspace/repo` with configurable branch, depth, shallow submodule, and checkout options.
- **Additional repositories**: Cloned into `/workspace/additional-repos/<name>` with independent configuration.
- **Authentication**: HTTP basic auth with credentials sourced from Secrets, or automatic credential injection via GitProvider OAuth2 tokens.
- **Shallow cloning**: Initial clone is shallow for fast startup; full history is fetched asynchronously in the background.

### Runtime Configuration

**Environment variables** can be set at multiple levels with values either static or resolved from Secrets/UserSecrets:

```yaml
spec:
  runtime:
    envVars:
      - key: DATABASE_URL
        fromSecret: my-database-url
      - key: NODE_ENV
        value: development
```

**Lifecycle tasks** run at specific points in the Workspace lifecycle:

| Type | When it runs | Use case |
|---|---|---|
| `ON_CREATE` | First Workspace start only | Install dependencies, compile, seed databases |
| `POST_START` | Every Workspace start | Start background services, run dev servers |
| `PRE_STOP` | Before Workspace stops | Graceful shutdown, cleanup |

Tasks support per-task environment variables, working directories, `runAsRoot`, `isBackground`, and `onFailure` behavior:

```yaml
spec:
  runtime:
    tasks:
      - name: install-deps
        run: npm install
        type: ON_CREATE
        workingDir: /workspace/repo
        onFailure: ON_FAILURE_ABORT
      - name: dev-server
        run: npm run dev
        type: POST_START
        isBackground: true
        workingDir: /workspace/repo
      - name: setup-db
        run: service postgresql start && createdb myapp
        type: POST_START
        runAsRoot: true
```

### Applications and Port Sharing

Workspaces can define named applications mapped to ports, accessible through the portal's reverse proxy:

```yaml
spec:
  applications:
    - name: web
      port: 3000
      isDefault: true
    - name: api
      port: 8080
    - name: docs
      port: 4000
```

The default application is accessible at `<workspace>.cordium.<domain>`. Named applications are accessible at `<app>_<workspace>.cordium.<domain>`. Applications can be shared with Space members or all authenticated users via the `ShareWorkspacePort` API.

### Devcontainer Support

Cordium implements the [Development Container specification](https://containers.dev/). Workspaces built from repositories containing `.devcontainer/devcontainer.json` or `.devcontainer.json` automatically build or pull the specified image, apply `containerEnv` values, install devcontainer features from OCI registries, run all lifecycle hooks, and install VS Code extensions. Docker Compose-based devcontainer configurations are also supported.

### Variable Substitution

Templates support a `vars` definition for parameterized instantiation. Variables are referenced with `${{ vars.NAME }}` syntax inside string fields (task scripts, repository URLs, image URLs, environment variable values). Values are resolved at Workspace creation time from per-Workspace overrides, falling back to Template-defined defaults.

```yaml
spec:
  vars:
    - name: BRANCH
      value: main
    - name: SERVICE
      value: svc
  repository:
    url: https://github.com/myorg/monorepo
    cloneOptions:
      branch: ${{ vars.BRANCH }}
  runtime:
    tasks:
      - name: build
        run: cd ${{ vars.SERVICE }} && go build ./...
        type: ON_CREATE
        workingDir: /workspace/repo
```

```sh
# Override variables at run time
cordium run --template go-build.my-project \
  --var SERVICE=services/payments \
  --var BRANCH=feat/new-feature
```

### YAML Configuration Files

Workspace and Template configurations can be passed to `cordium` CLI commands via `--file config.yaml`. A `.octelium/workspace.yaml` file placed in a repository is automatically detected and merged with the Template spec at initialization time.

Example: a full-stack development Workspace.

```yaml
spec:
  image:
    registry:
      url: node:20-bookworm
  repository:
    url: https://github.com/myorg/my-fullstack-app
    cloneOptions:
      branch: main
  runtime:
    envVars:
      - key: NODE_ENV
        value: development
      - key: DATABASE_URL
        fromSecret: staging-db-url
    tasks:
      - name: install
        run: npm ci
        type: ON_CREATE
        workingDir: /workspace/repo
      - name: migrate
        run: npx prisma migrate dev
        type: ON_CREATE
        workingDir: /workspace/repo
      - name: dev-server
        run: npm run dev
        type: POST_START
        isBackground: true
        workingDir: /workspace/repo
  applications:
    - name: web
      port: 3000
      isDefault: true
    - name: storybook
      port: 6006
  limit:
    cpu:
      millicores: 4000
    memory:
      megabytes: 8192
    storage:
      megabytes: 30000
```

Example: an ephemeral AI agent sandbox.

```yaml
spec:
  image:
    registry:
      url: python:3.11-slim-bookworm
  runtime:
    envVars:
      - key: ANTHROPIC_API_KEY
        fromSecret: anthropic-api-key
      # Database, SSH, and API access provided secretlessly via octelium connect.
      # No credentials needed here.
    tasks:
      - name: install-agent
        run: pip install --no-cache-dir anthropic aider-chat
        type: ON_CREATE
        onFailure: ON_FAILURE_ABORT
  autoStop: true
  limit:
    cpu:
      millicores: 2000
    memory:
      megabytes: 4096
    storage:
      megabytes: 10000
```


## Resource Limits

Resource limits are enforced at the cgroup level through a hierarchical precedence system.

### Per-Workspace Limits

```yaml
spec:
  limit:
    cpu:
      millicores: 2000    # 2 CPU cores
    memory:
      megabytes: 4096     # 4 GB RAM
    storage:
      megabytes: 20000    # 20 GB disk
```

### Per-Space Limits

```yaml
spec:
  limit:
    defaultLimit:
      cpu:
        millicores: 2000
      memory:
        megabytes: 4096
      storage:
        megabytes: 20000
    maxLimit:
      cpu:
        millicores: 8000
      memory:
        megabytes: 16384
      storage:
        megabytes: 50000
```

### Per-Cluster Limits

```yaml
spec:
  workspace:
    limit:
      maxPerUser: 50
      maxActivePerUser: 5
      defaultUserSpaceLimit:
        cpu:
          millicores: 2000
        memory:
          megabytes: 4096
        storage:
          megabytes: 20000
      maxLimit:
        cpu:
          millicores: 16000
        memory:
          megabytes: 32768
        storage:
          megabytes: 100000
      buildLimit:
        cpu:
          millicores: 4000
        memory:
          megabytes: 8192
        storage:
          megabytes: 30000
```

The resolution order is: Workspace spec → Template spec → Space default → Cluster default, with Space maximum and Cluster maximum applied as hard caps.


## Workspace Lifecycle

Workspaces follow a defined state machine:

```
STOPPED ──► INIT_REQUEST ──► INITIALIZING ──► PULLING_IMAGE ──► BUILDING_IMAGE
                                                                      │
                                                                      ▼
STOPPED ◄── STOPPING ◄── STOPPING_REQUEST ◄── RUNNING ◄── PREPARING ◄── STARTING_RUNTIME
```

State transitions are observable via the `WatchWorkspace` streaming RPC. Failure at any stage (image pull, image build, repository clone, task execution, health check) is captured with structured failure information including failure type, message, and exit code.

Workspaces can be **ephemeral** (storage discarded on stop) or **persistent** (storage preserved via PVCs across restarts). Persistent Workspaces resume with full filesystem state intact; only `POST_START` tasks re-run on subsequent starts.

Setting `autoStop: true` in the Workspace spec causes the Workspace to stop automatically once all non-background lifecycle tasks complete, without any manual intervention. This is the recommended configuration for CI/CD jobs and AI agent tasks.


## Access Methods

### Web Portal

The Cordium portal provides a browser-based interface for Workspace management and interaction, including:

- Clientless web-based interactive terminal access to running Workspaces without requiring SSH client installation
- A reverse proxy for accessing Workspace applications through subdomain routing
- GitProvider OAuth2 authentication flows
- Real-time Workspace state updates and log streaming via WebSocket

### CLI

The `cordium` CLI provides full command-line access to Workspace management.

**Run and attach to a Workspace:**

```sh
# Create a new Workspace from the default Template and attach a terminal
cordium run

# Create from a YAML configuration file
cordium run --file workspace.yaml

# Create from a specific Template
cordium run --template ml-env.my-project

# Create from a git repository
cordium run --repository https://github.com/myorg/my-project --branch develop

# Create from a container image with environment variables
cordium run --image python:3.11 -e PYTHONUNBUFFERED=1

# Create from a Dockerfile
cordium run --dockerfile ./Dockerfile

# Create an ephemeral Workspace that is deleted after the session ends
cordium run --ephemeral --rm

# Attach to an existing Workspace (starts it if stopped)
cordium run abc
```

**Open a terminal:**

```sh
cordium terminal abc
cordium term abc          # short alias
```

**Execute remote commands:**

```sh
# Run a command and propagate exit code
cordium exec abc -- make test

# Run in a specific working directory
cordium exec abc -w /workspace/repo -- go build ./...

# Run as root
cordium exec abc --root -- apt-get install -y ripgrep

# Set per-command environment variables
cordium exec abc -e GOOS=linux -e GOARCH=amd64 -- go build ./...

# Pipe local input to a remote command
echo "SELECT version();" | cordium exec abc -- psql mydb

# Capture remote output locally
cordium exec abc -- cat /workspace/repo/output.json > local.json
```

**SSH access:**

```sh
# Interactive SSH session
cordium ssh abc

# Run a remote command via SSH
cordium ssh abc -- uptime

# Local port forwarding
cordium ssh abc -L 5432:localhost:5432

# Multiple port forwards without a shell
cordium ssh abc -N -L 5432:localhost:5432 -L 6379:localhost:6379

# Dynamic SOCKS5 proxy
cordium ssh abc -D 1080 -N

# Generate an SSH config block for use with VS Code, JetBrains, Zed, rsync
cordium ssh abc --print-config >> ~/.ssh/config
```

**File transfer:**

```sh
# Copy a local file to a Workspace
cordium cp ./config.json abc:/workspace/repo/config.json

# Copy a file from a Workspace to local
cordium cp abc:/workspace/repo/output.csv ./output.csv

# Copy directories recursively
cordium cp -r ./src/ abc:/workspace/repo/src/

# Copy between two Workspaces
cordium cp abc:/workspace/repo/model.pt def:/workspace/repo/model.pt
```

**Stream Workspace logs:**

```sh
# Stream build and task logs
cordium logs abc

# Stream logs with timestamps
cordium logs abc --timestamp

# Stream logs without color (useful for CI/CD capture)
cordium logs abc --no-color
```

**Workspace lifecycle management:**

```sh
cordium start abc
cordium stop abc
cordium delete workspace abc
cordium list workspaces
cordium list workspaces --space my-project
```

**Space and Template management:**

```sh
# Create a Space
cordium create space my-project

# Create a Template with inline configuration
cordium create template ml-env.my-project \
  --image python:3.11 \
  --repository https://github.com/myorg/ml-project \
  --env-from-secret WANDB_API_KEY=wandb-secret \
  --cpu 8000 --memory 16384

# Build a Template pre-build snapshot
cordium build ml-env.my-project

# Create a Workspace from a Template with variable overrides
cordium run --template go-build.my-project \
  --var SERVICE=services/payments \
  --var BRANCH=main \
  --ephemeral
```

### gRPC API

The complete Cordium API is exposed via gRPC across three services:

**`MainService`**: Resource management and Workspace lifecycle. CRUD operations for Spaces, Templates, Workspaces, Secrets, GitProviders, UserSecrets, and UserConfig; `StartWorkspace` / `StopWorkspace`; `WatchWorkspace` streaming RPC; `BuildTemplate` / `CancelBuildTemplate`; `ShareWorkspacePort` / `UnshareWorkspacePort`.

**`WorkspaceService`**: Terminal and command execution. `CreateTerminal` / `RemoveTerminal` / `ListTerminal`; `ListenTerminal` streaming RPC; `WriteTerminalData` / `SetTerminalWindowSize`; `Exec` bidirectional streaming RPC; `ListenLog` streaming RPC.

**`ManagementService`**: Cluster configuration. `GetClusterConfig` / `UpdateClusterConfig`.

All API operations are authenticated via Octelium session tokens. A minimal Go client:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/octelium/octelium/apis/main/cordiumv1"
    "github.com/octelium/octelium/octelium-go"
)

func doMain(ctx context.Context) error {
    octeliumC, err := octelium.NewClient(ctx, &octelium.ClientConfig{
        Domain:              "example.com",
        AuthenticationToken: os.Getenv("AUTH_TOKEN"),
    })
    if err != nil {
        return err
    }
    defer octeliumC.Close()

    grpcConn, err := octeliumC.GRPC().GetConn(ctx)
    if err != nil {
        return err
    }

    c := cordiumv1.NewMainServiceClient(grpcConn)

    ws, err := c.CreateWorkspace(ctx, &cordiumv1.Workspace{
        Spec: &cordiumv1.Workspace_Spec{},
        Status: &cordiumv1.Workspace_Status{
            IsEphemeral: true,
            TemplateRef: &metav1.ObjectReference{Name: "agent-sandbox.ai-ops"},
            VarOverrides: map[string]string{
                "REPO_URL": "https://github.com/myorg/target-repo",
                "TASK":     "Fix failing tests in src/auth/ and commit the result.",
            },
        },
    })
    if err != nil {
        return err
    }

    fmt.Printf("Created Workspace: %s\n", ws.Metadata.Name)
    return nil
}
```


## Comparison with Other Platforms

| Feature | Cordium | E2B | Daytona | Gitpod | GitHub Codespaces | Coder | Devpod |
|---|---|---|---|---|---|---|---|
| **License** | AGPLv3 | Apache 2.0 | Apache 2.0 | AGPL (deprecated) | Proprietary | AGPLv3 | MPL 2.0 |
| **Self-hosted** | Yes | Yes (limited) | Yes | No | No | Yes | Client-only |
| **Sandbox isolation** | Rootless containers (3-layer) | Firecracker microVMs | Containers | Containers | Hyper-V VMs | Containers | Via providers |
| **Infrastructure** | Kubernetes | Bare-metal (KVM) | Docker/K8s | Proprietary | Azure | K8s/Docker | Local/cloud VMs |
| **Root inside sandbox** | Yes | Yes (VM) | Varies | Limited | Yes (VM) | Config-dependent | Provider-dependent |
| **Nested containers** | Yes (rootless Podman) | Yes (VM) | Docker-in-Docker | Limited | Yes (VM) | Config-dependent | Provider-dependent |
| **Storage** | K8s PVCs (any CSI) | Ephemeral | Docker volumes | Proprietary | Azure disks | K8s PVCs | Provider-dependent |
| **Pre-builds / snapshots** | Yes (VolumeSnapshot) | No | No | Yes | Yes | No | No |
| **Identity system** | Octelium (OIDC, SAML, workload) | API keys | OAuth2 | GitHub/GitLab | GitHub | OIDC/OAuth2 | None |
| **Secretless resource access** | Yes (SSH, DB, HTTP, mTLS) | No | No | No | No | No | No |
| **Zero-trust architecture** | Yes (Octelium ZTNA) | No | No | No | No | No | No |
| **L7-aware access control** | Yes (per-request, ABAC) | No | No | No | No | No | No |
| **AI agent focus** | Yes | Yes (primary) | Limited | No | No | No | No |
| **OpenTelemetry native** | Yes | No | No | No | No | Prometheus only | No |
| **Declarative YAML config** | Yes (Workspace/Template spec) | No | Yes | `.gitpod.yml` | `devcontainer.json` | Terraform | `devcontainer.json` |
| **Variable substitution** | Yes (`${{ vars.NAME }}`) | No | No | Limited | No | Terraform vars | No |
| **Web terminal** | Yes (clientless) | No | Yes | Yes (VS Code) | Yes (VS Code) | Yes | No |
| **gRPC API** | Yes (full lifecycle + exec) | REST/SDK | REST | Limited | REST | REST | No |

**Cordium vs. E2B.** E2B uses Firecracker microVMs requiring bare-metal KVM infrastructure. Cordium runs on standard Kubernetes nodes using rootless containers with the same full-root-in-sandbox capability, adds identity-based secretless infrastructure access, and provides an interactive development experience alongside programmatic API access.

**Cordium vs. Daytona.** Daytona focuses on developer workspace management without an identity or access control layer. Cordium adds Octelium zero-trust, secretless access, hierarchical resource limits, and VolumeSnapshot-based pre-builds.

**Cordium vs. GitHub Codespaces.** Codespaces is a proprietary SaaS product tied to GitHub and Azure. Cordium is self-hosted, infrastructure-agnostic, and provides identity-based infrastructure access rather than relying on credential distribution.

**Cordium vs. Coder.** Coder and Cordium share similar Kubernetes infrastructure requirements. Cordium adds the Octelium zero-trust layer (secretless access, dynamic ABAC, L7-aware observability), variable substitution, and is designed for both human and machine users. Coder uses Terraform for workspace provisioning; Cordium uses a declarative YAML-based configuration with a hierarchical Space/Template/Workspace resource model.

**Cordium vs. Gitpod.** Gitpod's open source offering has been deprecated. Cordium is fully open source and maintained for self-hosting with no proprietary cloud dependency.

**Cordium vs. Devpod.** Devpod is a client-side tool with no server-side management, access control, or multi-user capabilities. Cordium is a server-side platform with centralized management and identity-based access control.


## License

Cordium is licensed under the [GNU Affero General Public License v3.0](LICENSE).

Copyright © Octelium Labs, LLC. All rights reserved.