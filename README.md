# Cordium

Cordium is a free and open source, self-hosted, horizontally scalable, identity-based sandbox platform built on [Kubernetes](https://kubernetes.io) and [Octelium](https://octelium.com). It provides isolated, rootless container-based reproducible environments, called **Workspaces**, for both humans and machines, including AI agents.

Cordium is designed as a unified platform that serves two primary purposes:
- A sandbox platform for running both long-lived workloads (such as remote development environments, persistent servers, and interactive coding sessions) and short-lived tasks (such as AI agent execution, CI/CD jobs, and automated scripts) inside reproducible rootless container-based sandboxes accessible via web, SSH, CLI, and gRPC-based SDKs.
- An identity-based zero-trust remote access that leverages [Octelium](https://octelium.com) ZTNA capabilities to provide secretless, policy-driven access to infrastructure resources from within Workspaces, without exposing, distributing, or managing upstream resource credentials.

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
  - [YAML Configuration Files](#yaml-configuration-files)
- [Resource Limits](#resource-limits)
- [Workspace Lifecycle](#workspace-lifecycle)
- [Access Methods](#access-methods)
  - [Web Portal](#web-portal)
  - [CLI](#cli)
  - [gRPC API](#grpc-api)
- [License](#license)

## Main Features

The following is a summary of Cordium's main features and design principles.

- **Unified access for humans and machines.** The same Workspace can be accessed interactively through a browser-based terminal, via SSH, through the CLI, or programmatically via gRPC-based SDKs. This makes Cordium equally suitable as a remote development environment for engineers (comparable to GitHub Codespaces or Coder) and as an execution sandbox for AI agents, CI/CD pipelines, and automated workloads. Workspaces support both long-lived runs (remote development, persistent servers) and short-lived runs (AI agent tasks, build jobs, scripted automation).

- **Highly customizable sandbox environments.** Workspace filesystems can be built from OCI/Docker images, Dockerfiles, git repositories, and devcontainers. Multi git-repository cloning, including private repos with authentication. Workspace and Template configurations are fully declarative via YAML files and can be instantiated through the `cordium` CLI or managed programmatically via the gRPC API. Each running Workspace supports full root access within the sandbox, allowing users to run containers, install system packages, and run privileged services. Templates support pre-building for fast Workspace instantiation. Spaces provide namespacing for Workspaces, Templates, Secrets, and GitProviders. Secrets can be referenced in environment variables and repository authentication configurations. Workspace storage can be persistent or ephemeral. Define resource limits (i.e. memory, CPU an storage) at the Workspace, Space and Cluster level.

- **Rootless container-based sandboxing on standard Kubernetes.** No bare-metal nodes or specialized hardware specialized hardware are needed (however an optional KVM-based sandboxing is planned). Workspaces run efficiently on any Kubernetes cluster while providing isolation through Linux namespaces, cgroups, seccomp and capabilities.

- **Zero-trust platform on Octelium.** Cordium is built on [Octelium](https://octelium.com), inheriting its zero-trust infrastructure as a foundational layer and using its resource types (Users, Sessions, Devices, Services, Namespaces, Policies, and others) to provide the following capabilities:

    - **Dynamic secretless access.** Octelium's layer-7 awareness enables Users to seamlessly access resources protected by application-layer credentials without exposing, managing, or distributing such secrets (read more [here](https://octelium.com/docs/octelium/latest/management/core/service/secretless)). This works for HTTP APIs without sharing API keys and access tokens, SSH servers without sharing passwords and private keys, Kubernetes clusters, PostgreSQL/MySQL databases, and any L7 protocol protected by mTLS.

    - **Modern, dynamic, fine-grained access control.** Octelium provides a centralized, scalable, fine-grained, dynamic, context-aware, layer-7-aware, attribute-based access control system (ABAC) evaluated on a per-request basis (read more [here](https://octelium.com/docs/octelium/latest/management/core/policy)) with policy-as-code using [CEL](https://cel.dev/) and [OPA](https://www.openpolicyagent.org/) (Open Policy Agent). Octelium has no notion of an "admin" user, enforcing zero standing privileges by default.

    - **Continuous strong authentication.** A unified authentication system for both human and workload Users, supporting any web identity provider (IdP) that uses OpenID Connect or SAML 2.0, as well as GitHub OAuth2 (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers#web-identity-providers)). It also supports secretless authentication for workloads via OIDC-based assertions (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers#workload-identity-providers)). Built-in support for MFA, re-authentication, and login via FIDO2/WebAuthn/Passkey, TOTP, and TPM 2.0 Authenticators.

    - **OpenTelemetry-native auditing and visibility.** Real-time, identity-based, L7-aware visibility and access logging. Every request is logged and exported to your OpenTelemetry OTLP receivers for integration with log management and SIEM providers.

- **Kubernetes-native with pluggable storage.** Cordium leverages Kubernetes-native storage for Workspace persistence and integrates with any Kubernetes CSI driver and VolumeSnapshot provider. This includes Longhorn, AWS EBS, GCP Persistent Disk, Azure Disk, Ceph/Rook, OpenEBS, and any other CSI-compliant storage solution. Storage class and volume snapshot class selection is policy-driven via CEL expressions, allowing operators to route different Workspace types to different storage backends.

- **Ready for agentic AI.** Cordium is not only a sandbox for isolated long-lived and short-lived process execution by sandboxed AI agents. It leverages Octelium's zero-trust infrastructure to provide identity-based, fine-grained, L7-aware, context-aware, ABAC-based access to resources of any type from within Workspaces. This includes secretless access for resources that require application-layer credentials (API keys, access tokens, SSH passwords and private keys, database passwords, and mTLS private keys) without exposing, distributing, or sharing such credentials with the sandboxed AI agent. Credential mappings and privilege scopes can be dynamically assigned to specific agents based on identity and context.

- **Open source and designed for self-hosting.** Cordium, like Octelium itself, is fully open source and designed for single-tenant self-hosting. There is no proprietary cloud-based control plane, and this is not a limited open source version of a separate fully functional paid SaaS product. Cordium can be deployed on a single-node Kubernetes cluster running on a low-cost cloud VM/VPS, or on production-grade multi-node Kubernetes installations, cloud-based or on-premises, with no vendor lock-in.

## Architecture


Cordium runs entirely on Kubernetes. Each Workspace is a managed Kubernetes pod, provisioned and controlled by _Nocturne_, the resource controller. Each Workspace pod uses a three-layer isolation model: An outer privileged Kubernetes pod container used only for bootstrapping a much more restricted container with a handful allowed capabilities, a seccomp profile, customized nested cgroup and mostly read-only filesystem. The inner, restricted, rootful container is used to run another rootless podman container that actually runs the Workspace.

## Core Concepts

Cordium organizes resources in a hierarchical structure:

```
Cluster
└── Space (namespace for related resources)
    ├── Template (reusable Workspace configuration)
    │   └── Workspace (running sandbox instance)
    ├── Secret (sensitive values, referenced by Workspaces, Templates, and GitProviders)
    └── GitProvider (OAuth2 git authentication configuration)
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

A **Workspace** is the sandbox unit in Cordium. Each Workspace belongs to one Template and one Space and it is owned by an Octelium User. It is an isolated, rootless container environment. Each running Workspace has full root access within the container (install packages, run containers, manage services), it can have either Persistent or ephemeral storage. A Workspace can be accessed via web-based terminals, SSH, `cordium` CLI commands (e.g. `cordium run`, `cordium terminal`) or via programmatic API access. Workspace spec contains the following:

- Image configuration (public/private registry, Dockerfile, Git repo, devcontainer)
- Primary and additional repository cloning with authentication for private repos
- Runtime tasks (`ON_CREATE`, `POST_START`, `PRE_STOP`) with per-task environment variables.
- Devcontainer features
- Environment variables with static values or Secret references
- Resource limits (CPU, memory, storage)
- Named applications to expose Workspace servers (e.g. web applications and APIs)

### Templates

A **Template** defines a reusable Workspace configuration within a Space. Templates encapsulate the full specification for a Workspace: image source, repository configuration, runtime tasks, environment variables, resource limits, and application definitions.

Every Space has a `default` Template created automatically. Additional Templates can be created for different project configurations, language environments, or use cases within the same Space.

Templates support **pre-building**: a build Workspace is created from the Template spec, runs to completion (image pull, repository clone, task execution). Subsequent Workspaces instantiated from a built Template restore from this snapshot, reducing startup time from minutes to seconds. Template spec shares most of Workspace spec configuration, and additionally has GitProvider association for automatic credential injection.

### Secrets

**Secrets** are sensitive values (e.g. API keys, access tokens, SSH private keys, etc.) stored within a Space. They can be referenced by name in Template and Space runtime configurations, for example, as environment variable values or repository authentication credentials. Secret values are never returned through the API after creation and can only be used by the Cluster or exposed to Workspaces at runtime.

### GitProviders

A **GitProvider** configures OAuth2 authentication against a git hosting service: GitHub, GitLab, or any generic OAuth2 provider. When a Template references a GitProvider, users authenticate once through the Cordium portal's OAuth2 flow, and their access tokens are automatically injected into Workspaces at startup.

This enables authenticated `git clone`, `git push`, and other git operations within Workspaces without manual credential configuration. Tokens are stored encrypted in per-user UserSecret objects and are scoped to the owning user.

### User Configuration

Per-user configuration applies across all of a user's Workspaces, regardless of Space or Template:

- **Dotfiles**: Configure a git repository URL containing your dotfiles. Cordium clones the repository into `~/dotfiles` at Workspace creation time and runs standard install scripts (`install.sh`, `bootstrap.sh`, `setup.sh`, or their equivalents).

- **User environment variables**: Define environment variables injected into every Workspace. Values can be static or sourced from UserSecrets.

- **User tasks**: Define lifecycle tasks (`POST_START`) that run in every Workspace after startup.

- **UserSecrets**: Per-user encrypted values (strings, byte arrays, SSH keys) stored independently of Space-scoped Secrets. SSH key UserSecrets are generated server-side as ECDSA key pairs. The public key is available for registration with external services, and the private key is automatically loaded into the Workspace's SSH agent.

- **Preferred region**: Set a default region for Workspace placement.

## Workspace Configuration

Workspaces can be configured through multiple mechanisms, which are merged at initialization time in a defined precedence order: Workspace spec → Template spec → Space spec → Cluster defaults. Environment variables, for example, are merged across all levels.

### Image Sources

The container image for a Workspace can come from several sources:

| Source | Description |
|---|---|
| **Container registry** | Pull a pre-built image from any OCI-compatible registry, with optional authentication |
| **Dockerfile** | Provide a Dockerfile inline or from a URL in the Template or Workspace spec |
| **Git repository** | Clone a repository and build from a Dockerfile or devcontainer spec found within it |
| **Repository devcontainer** | Detect and build from `.devcontainer/devcontainer.json` in the configured repository |

If no image source is specified, Cordium uses a default base image with common development tools pre-installed.

### Repository Cloning

Workspaces can clone a primary repository and zero or more additional repositories at creation time:

- **Primary repository**: Cloned into `/workspace/repo` with configurable branch, depth, shallow submodule, and checkout options.
- **Additional repositories**: Cloned into `/workspace/additional-repos/<name>` with independent configuration.
- **Authentication**: HTTP basic auth with credentials sourced from Secrets, or automatic credential injection via GitProvider OAuth2 tokens.
- **Shallow cloning**: Initial clone is shallow for fast startup; full history is fetched asynchronously in the background.

### Runtime Configuration

Runtime configuration controls what happens inside the Workspace after the container starts.

**Environment variables** can be set at multiple levels (Workspace, Template, Space, User) with values either static or resolved from Secrets/UserSecrets:

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

Tasks can run as the Workspace user or as root, support per-task environment variables and working directories, and can be configured to abort or continue on failure:

```yaml
spec:
  runtime:
    tasks:
      - name: install-deps
        run: npm install
        type: ON_CREATE
        workingDir: /workspace/repo
      - name: dev-server
        run: npm run dev
        type: POST_START
        isBackground: true
        workingDir: /workspace/repo
      - name: setup-db
        run: sudo service postgresql start && createdb myapp
        type: POST_START
        runAsRoot: true
```

### Applications and Port Sharing

Workspaces can define named applications mapped to ports. These applications are accessible through the portal's reverse proxy via subdomain routing:

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

The default application is accessible at `<workspace>.cordium.<domain>`. Named applications are accessible at `<app>_<workspace>.cordium.<domain>`. Specific ports can also be accessed directly via `port_<number>_<workspace>.cordium.<domain>`.

Applications can be shared with Space members or with all authenticated users via the `ShareWorkspacePort` API.

### Devcontainer Support

Cordium implements the [Development Container specification](https://containers.dev/). Workspaces built from repositories containing `.devcontainer/devcontainer.json` or `.devcontainer.json` automatically:

- Build or pull the specified container image
- Apply `containerEnv` values
- Install devcontainer features from OCI registries
- Run `onCreateCommand`, `updateContentCommand`, `postCreateCommand`, and `postStartCommand` lifecycle hooks
- Install VS Code / OpenVSCode Server extensions specified in `customizations.vscode.extensions`

Docker Compose-based devcontainer configurations are also supported, including multi-service setups.

### YAML Configuration Files

Workspaces can be configured via YAML files that map to the Workspace spec. Configurations can be passed to `cordium` CLI commands (such as `cordium run --file config.yaml` or `cordium create workspace --file config.yaml`). Additionally, a `.octelium/workspace.yaml` file placed in a repository is automatically detected and merged with the Template spec at initialization time.

Example: a full-stack development Workspace.

```yaml
apiVersion: v1
kind: Workspace
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

Example: a Python ML Workspace.

```yaml
apiVersion: v1
kind: Workspace
spec:
  image:
    dockerfile:
      inline: |
        FROM python:3.11-slim
        RUN apt-get update && apt-get install -y git curl build-essential
        RUN pip install poetry
  repository:
    url: https://github.com/myorg/ml-pipeline
    cloneOptions:
      branch: develop
      depth: 1
  runtime:
    envVars:
      - key: WANDB_API_KEY
        fromSecret: wandb-key
    tasks:
      - name: install
        run: poetry install --no-interaction
        type: ON_CREATE
        workingDir: /workspace/repo
      - name: jupyter
        run: poetry run jupyter lab --ip=0.0.0.0 --port=8888 --no-browser
        type: POST_START
        isBackground: true
        workingDir: /workspace/repo
  applications:
    - name: jupyter
      port: 8888
      isDefault: true
  limit:
    cpu:
      millicores: 8000
    memory:
      megabytes: 16384
    storage:
      megabytes: 50000
```

Example: a minimal ephemeral sandbox.

```yaml
apiVersion: v1
kind: Workspace
spec:
  image:
    registry:
      url: ubuntu:24.04
  runtime:
    tasks:
      - name: setup
        run: apt-get update && apt-get install -y curl git vim
        type: ON_CREATE
        runAsRoot: true
```

## Resource Limits

Resource limits are enforced at the cgroup level and applied through a hierarchical precedence system.

### Per-Workspace Limits

Individual Workspaces can specify CPU (millicores), memory (megabytes), and storage (megabytes) limits directly in their spec:

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

Spaces can define default limits (applied when a Workspace or Template does not specify its own) and maximum limits (hard caps that cannot be exceeded by any Workspace in the Space):

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

Cluster-level configuration defines global defaults and maximums, and controls per-user Workspace counts and active Workspace limits:

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

The resolution order is: Workspace spec → Template spec → Space default → Cluster default, with Space maximum and Cluster maximum applied as hard caps at the end.

## Workspace Lifecycle

Workspaces follow a defined state machine:

```
STOPPED ──► INIT_REQUEST ──► INITIALIZING ──► PULLING_IMAGE ──► BUILDING_IMAGE
                                                                      │
                                                                      ▼
STOPPED ◄── STOPPING ◄── STOPPING_REQUEST ◄── RUNNING ◄── PREPARING ◄── STARTING_RUNTIME
```

You can watch for Workspace changes via the `WatchWorkspace` streaming RPC. Failure at any stage (image pull, image build, repository clone, task execution, health check) is captured with structured failure information including failure type, message, and exit code where applicable.

Workspaces can be **ephemeral** (storage is discarded on stop) or **persistent** (storage is preserved across restarts via PVCs). Persistent Workspaces resume where they left off, including full filesystem state. Only `POST_START` tasks are re-run on subsequent starts.

## Access Methods

### Web Portal

The Cordium portal provides a browser-based interface for Workspace management and interaction, including clientless web-based interactive terminal access to running Workspaces without requiring SSH client installation, a reverse proxy for accessing Workspace applications through subdomain routing, GitProvider OAuth2 authentication flows, and real-time Workspace state updates via WebSocket.

### CLI

The `cordium` CLI provides full command-line access to Workspace management.

**Create and run a Workspace interactively:**

```sh
# Create a new Workspace and attach a terminal
cordium run

# Create from a YAML configuration file
cordium run --file workspace.yaml

# Create from a specific Template
cordium run --template my-template

# Create from a git repository
cordium run --repository https://github.com/myorg/my-project

# Create from a container image
cordium run --image python:3.11

# Create from a Dockerfile
cordium run --dockerfile ./Dockerfile

# Create an ephemeral Workspace
cordium run --ephemeral

# Run a terminal in an existing Workspace (starting it if stopped)
cordium run abc
```

**Open a terminal in a running Workspace:**

```sh
# Interactive terminal session
cordium terminal abc

# Short alias
cordium term abc
```

**Execute commands remotely:**

```sh
# Run a single command
cordium exec abc -- ls -la /workspace/repo

# Run a multi-word command
cordium exec abc -- make build

# Pipe input
echo "SELECT 1;" | cordium exec abc -- psql mydb

# Run with sudo
cordium exec abc -- sudo apt-get update
```

**SSH into a Workspace:**

```sh
# Standard SSH access
cordium ssh abc
```

**Manage Workspaces:**

```sh
# List Workspaces
cordium list workspaces

# Start a stopped Workspace
cordium start abc

# Stop a running Workspace
cordium stop abc

# Delete a Workspace
cordium delete workspace abc
```

**Manage Spaces and Templates:**

```sh
# Create a Space
cordium create space my-project

# Create a Template within a Space
cordium create template ml-env.my-project

# Build a Template
cordium build ml-env.my-project

# Create a Workspace from a specific Template
cordium run --template ml-env.my-project
```

### gRPC API

The complete Cordium API is exposed via gRPC, defined across three services:

**`MainService`**: Resource management and Workspace lifecycle.

- CRUD operations for Spaces, Templates, Workspaces, Secrets, GitProviders, UserSecrets, and UserConfig
- `StartWorkspace` / `StopWorkspace` for lifecycle control
- `WatchWorkspace` streaming RPC for real-time state observation
- `BuildTemplate` / `CancelBuildTemplate` for Template pre-build management
- `ShareWorkspacePort` / `UnshareWorkspacePort` for application sharing
- `ListRegion` for region discovery

**`WorkspaceService`**: Terminal and execution (region-scoped).

- `CreateTerminal` / `RemoveTerminal` / `ListTerminal` for terminal management
- `ListenTerminal` streaming RPC for terminal output
- `WriteTerminalData` / `SetTerminalWindowSize` for terminal input
- `Exec` bidirectional streaming RPC for remote command execution
- `ListenLog` streaming RPC for build and task log output

**`ManagementService`**: Cluster configuration.

- `GetClusterConfig` / `UpdateClusterConfig` for cluster-level settings

The gRPC API is suitable for building custom integrations, AI agent tooling, CI/CD plugins, and programmatic Workspace management. All operations are authenticated via Octelium session tokens.

## License

Cordium is licensed under the [GNU Affero General Public License v3.0](LICENSE).

Copyright © Octelium Labs, LLC. All rights reserved.