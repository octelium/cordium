# Cordium

Cordium is a free and open source, self-hosted, identity-based, horizontally scalable sandbox platform built on [Kubernetes](https://kubernetes.io) and [Octelium](https://github.com/octelium/octelium) for both humans and machines, including AI agents. Cordium is designed as a unified platform that serves two primary purposes:

- A sandbox platform for running both long-lived workloads (such as remote development environments, persistent servers, and interactive coding sessions) and short-lived tasks (such as AI agent execution, CI/CD jobs, and automated scripts) inside reproducible rootless container-based sandboxes accessible via web, SSH, CLI, and gRPC-based SDKs.
- An identity-based zero-trust remote access platform that leverages [Octelium](https://octelium.com) ZTNA capabilities to provide secretless, policy-driven access to infrastructure resources from within Workspaces, without exposing, distributing, or managing upstream resource credentials.

## Table of Contents

- [Main Features](#main-features)
- [Core Concepts](#core-concepts)
- [Workspace Configuration](#workspace-configuration)
- [Access Methods](#access-methods)
  - [CLI](#cli)
- [Comparison with Other Platforms](#comparison-with-other-platforms)
- [License](#license)


## Main Features

- **Unified access for humans and machines.** The same Workspace can be accessed interactively through a browser-based terminal, via SSH, through the CLI, or programmatically via gRPC-based SDKs. This makes Cordium equally suitable as a remote development environment for engineers (comparable to GitHub Codespaces or Coder) and as an execution sandbox for AI agents, CI/CD pipelines, and automated workloads. Workspaces support both long-lived runs (remote development, persistent servers) and short-lived runs (AI agent tasks, build jobs, scripted automation).

- **Highly customizable sandbox environments.** Workspace filesystems can be built from OCI/Docker images, Dockerfiles, git repositories, and devcontainers. Multi-repository cloning, including private repositories with authentication. Workspace and Template configurations can be managed declaratively via YAML files and can be instantiated through the `cordium` CLI or managed programmatically via the gRPC API. Each running Workspace supports full root access within the sandbox, allowing users to run containers, install system packages, and run privileged services. Templates support pre-building for fast Workspace instantiation. Spaces provide namespacing for Workspaces, Templates, Secrets, and GitProviders. Secrets can be referenced in environment variables and repository authentication configurations. Workspace storage can be persistent or ephemeral. Resource limits (memory, CPU, and storage) can be defined at the Workspace, Space, and Cluster level.

- **Rootless container-based sandboxing on standard Kubernetes.** No bare-metal nodes or specialized hardware are needed. Workspaces run efficiently on any standard Kubernetes cluster.

- **Zero-trust platform on Octelium.** Cordium is built on [Octelium](https://octelium.com), inheriting its zero-trust infrastructure as a foundational layer and using its resource types (Users, Sessions, Devices, Services, Namespaces, Policies, and others) to provide the following capabilities:

    - **Dynamic secretless access.** Octelium's layer-7 awareness enables Users to seamlessly access resources protected by application-layer credentials without exposing, managing, or distributing such secrets (read more [here](https://octelium.com/docs/octelium/latest/management/core/service/secretless)). This works for HTTP APIs without sharing API keys and access tokens, SSH servers without sharing passwords and private keys, Kubernetes clusters, PostgreSQL/MySQL databases, and any L7 protocol protected by mTLS.

    - **Modern, dynamic, fine-grained access control.** Octelium provides a centralized, scalable, fine-grained, dynamic, context-aware, layer-7-aware, attribute-based access control system (ABAC) evaluated on a per-request basis (read more [here](https://octelium.com/docs/octelium/latest/management/core/policy)) with policy-as-code using [CEL](https://cel.dev/) and [OPA](https://www.openpolicyagent.org/) (Open Policy Agent). Octelium has no notion of an "admin" user, enforcing zero standing privileges by default.

    - **Continuous strong authentication.** A unified authentication system for both human and workload Users, supporting any web identity provider (IdP) that uses OpenID Connect or SAML 2.0, as well as GitHub OAuth2 (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers#web-identity-providers)). It also supports secretless authentication for workloads via OIDC-based assertions (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers#workload-identity-providers)). Built-in support for MFA, re-authentication, and login via FIDO2/WebAuthn/Passkey, TOTP, and TPM 2.0 Authenticators.

    - **OpenTelemetry-native auditing and visibility.** Real-time, identity-based, L7-aware visibility and access logging. Every request is logged and exported to your OpenTelemetry OTLP receivers for integration with log management and SIEM providers.

- **Kubernetes-native with pluggable storage.** Cordium leverages Kubernetes-native storage for Workspace persistence and integrates with any Kubernetes CSI driver and VolumeSnapshot provider. This includes Longhorn, AWS EBS, GCP Persistent Disk, Azure Disk, Ceph/Rook, OpenEBS, and any other CSI-compliant storage solution. Storage class and volume snapshot class selection is policy-driven via CEL expressions, allowing operators to route different Workspace types to different storage backends.

- **Ready for agentic AI.** Cordium is not only a sandbox for isolated long-lived and short-lived process execution by sandboxed AI agents. It leverages Octelium's zero-trust infrastructure to provide identity-based, fine-grained, L7-aware, context-aware, ABAC-based access to resources of any type from within Workspaces. This includes secretless access for resources that require application-layer credentials (API keys, access tokens, SSH passwords and private keys, database passwords, and mTLS private keys) without exposing, distributing, or sharing such credentials with the sandboxed AI agent. Credential mappings and privilege scopes can be dynamically assigned to specific agents based on identity and context.

- **Open source and designed for self-hosting.** Cordium, like Octelium itself, is fully open source and designed for single-tenant self-hosting. There is no proprietary cloud-based control plane, and this is not a limited open source version of a separate fully functional paid SaaS product. Cordium can be deployed on a single-node Kubernetes cluster running on a low-cost cloud VM/VPS, or on production-grade multi-node Kubernetes installations, cloud-based or on-premises, with no vendor lock-in.


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

Here are some Workspace/Template configurations that can be used to create and run Workspaces:


```yaml
spec:
  image:
    registry:
      url: python:3.12-bookworm

  runtime:
    tasks:
      - name: hello
        type: ON_CREATE
        run: |
          python3 --version
          echo "Cordium Workspace is ready"

  limit:
    cpu:
      millicores: 1000
    memory:
      megabytes: 2048
    storage:
      megabytes: 10000
```

```yaml
spec:
  image:
    dockerfile:
      inline: |
        FROM ubuntu:24.04

        ENV DEBIAN_FRONTEND=noninteractive

        RUN apt-get update && apt-get install -y \
            ca-certificates \
            curl \
            wget \
            git \
            jq \
            vim \
            nano \
            htop

  repository:
    url: https://github.com/example/payment-service
    cloneOptions:
      branch: main
      depth: 1
      singleBranch: true

  vars:
    name: CODEX_PROMPT
    value: |
      The test suite is failing.
      Analyze the repository, fix the failing tests,
      run the tests again, and create a git commit
      describing the fix.

  runtime:
    envVars:
      - key: OPENAI_API_KEY
        fromSecret: openai-api-key

      - key: OPENAI_MODEL
        value: o4-mini

      - key: GIT_AUTHOR_NAME
        value: cordium-codex

      - key: GIT_AUTHOR_EMAIL
        value: codex@example.com

    tasks:
      - name: setup
        type: ON_CREATE
        run: |
          apt-get update
          apt-get install -y git curl nodejs npm podman

          npm install -g @openai/codex

          npm ci

      - name: start-postgres
        type: POST_START
        run: |
          sudo podman run \
            --net host \
            -e POSTGRES_PASSWORD=password \
            -e POSTGRES_DB=app \
            -d docker.io/postgres:16

      - name: run-tests
        type: POST_START
        run: |
          npm test
        onFailure: ON_FAILURE_CONTINUE

      - name: start-service
        type: POST_START
        workingDir: /workspace/repo
        isBackground: true
        run: |
          if [ -f package.json ]; then npm run dev; fi

      - name: codex-remediation
        type: POST_START
        run: |
          codex exec "${{ vars.CODEX_PROMPT}}"

      - name: push-branch
        type: POST_START
        run: |
          BRANCH=codex-fix-$(date +%s)

          git checkout -b $BRANCH
          git push origin $BRANCH

  limit:
    cpu:
      millicores: 4000
    memory:
      megabytes: 8192
    storage:
      megabytes: 30000
```

## Access Methods

### Web Portal

The Cordium web portal is a browser-based interface for managing and interacting with Workspaces without installing any software. It is the primary interface for users and teams who want clientless access to their Workspaces. The Octelium web portal authenticates users through Octelium's IdentityProviders, including GitHub OAuth2 or any OpenID Connect or SAML 2.0 IdP (read more here) or directly via Passkeys (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers)).

### CLI

The `cordium` CLI provides full command-line access to Workspace management. Here are some examples:

```sh
# Create from the default Template and attach a terminal
cordium run

# Attach to an existing Workspace (starts it if stopped)
cordium run abc

# Create from a specific Template
cordium run --template ml-env.my-project

# Create from a YAML configuration file
cordium run --file workspace.yaml

# Create from a container image
cordium run --image python:3.11-slim

# Create from a Dockerfile
cordium run --dockerfile ./Dockerfile

# Create from a git repository, cloning a specific branch
cordium run --repository https://github.com/myorg/my-project --branch develop

# Create in a specific Space using that Space's default Template
cordium run --space my-project

# Create an ephemeral Workspace
cordium run --ephemeral

# Create an ephemeral Workspace that is deleted when the terminal session ends
cordium run --ephemeral --rm

# Ephemeral AI agent sandbox with resource limits and a secret
cordium run --ephemeral --rm \
  --image python:3.11-slim \
  --env-from-secret ANTHROPIC_API_KEY=anthropic-key \
  --cpu 2000 --memory 4096

# Set environment variables
cordium run --image node:20 -e NODE_ENV=development -e PORT=3000

# Source a variable from a Space Secret
cordium run --template backend.my-project \
  --env-from-secret DATABASE_URL=staging-db-url

# Clone the primary repository and an additional repository with vars
cordium run --repository https://github.com/myorg/api-service \
  --additional-repo shared-lib=https://github.com/myorg/shared-lib \
  --var SERVICE=services/payments \
  --var BRANCH=main


# Open a Workspace terminal
cordium terminal abc
cordium term abc          # short alias

# Run a command and propagate exit code
cordium exec abc -- make test

# Run in a specific working directory
cordium exec abc -w /workspace/repo -- go build ./...

# Run as root
cordium exec abc --root -- apt-get install -y ripgrep

# Set per-command environment variables
cordium exec abc -e GOOS=linux -e GOARCH=amd64 -- go build ./...

# Capture remote output locally
cordium exec abc -- cat /workspace/repo/output.json > local.json

# Interactive SSH session via an embedded SSH client in the cordium CLI
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

# Copy a local file to a Workspace
cordium cp ./config.json abc:/workspace/repo/config.json

# Copy a file from a Workspace to local
cordium cp abc:/workspace/repo/output.csv ./output.csv

# Copy directories recursively
cordium cp -r ./src/ abc:/workspace/repo/src/

# Copy between two Workspaces
cordium cp abc:/workspace/repo/model.pt def:/workspace/repo/model.pt


# Stream Workspace logs
cordium logs abc

# Start a Workspace
cordium start abc

# Stop a Workspace
cordium stop abc
cordium delete workspace abc

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


## Comparison with Other Platforms

| Capability / Property | Cordium | Daytona | E2B | GitHub Codespaces | Coder | DevPod |
|---|---|---|---|---|---|---|
| **Primary workloads** | Developers, AI agents, automation workloads | Developers, AI agents | AI agents | Developers | Developers, AI-assisted development | Developers |
| **License** | AGPLv3 | AGPLv3 | Mixed / managed-first | Proprietary | AGPLv3 | MPL 2.0 |
| **Self-hosted** | Yes | Yes | Limited | No | Yes | Local/client-side |
| **Managed SaaS offering** | No | Yes | Yes | Yes | Yes | No |
| **Kubernetes-native architecture** | Yes | Partial | No | No | Yes | No |
| **Horizontal scalability** | Yes, over Kubernetes | Distributed sandbox runtime | Managed proprietary infrastructure | GitHub-managed | Kubernetes/provider-dependent | Local/provider-dependent |
| **Isolation model** | Rootless nested containers | Containers / gVisor-style isolation | Firecracker microVMs | VMs/containers | Containers/VMs | Provider-dependent |
| **Root-equivalent access inside sandbox** | Yes | Yes | Limited | Yes | Configurable | Provider-dependent |
| **Nested container support** | Supported | Docker/container support | Limited | Supported | Supported | Provider-dependent |
| **Persistent stateful environments** | Yes | Yes | Limited/session-oriented | Yes | Yes | Provider-dependent |
| **Ephemeral execution support** | Yes | Yes | Yes | Limited | Limited | Limited |
| **Volume snapshot support** | CSI VolumeSnapshots | Limited | Platform-managed | Managed internally | Provider-dependent | No |
| **Devcontainer support** | Yes | Yes | Partial | Yes | Yes | Yes |
| **Template-based provisioning** | Yes | Yes | Limited | Limited | Yes | Yes |
| **Secretless infrastructure access** | Yes | No | No | No | No | No |
| **Built-in infrastructure access proxying** | SSH, Kubernetes, databases, HTTP APIs | No | No | No | Limited/external | No |
| **OpenTelemetry-native auditing** | Yes | Partial | Limited | Platform-managed | External integrations | No |
| **CLI-first workflows** | Yes | Yes | Yes | Partial | Yes | Yes |
| **Multi-user platform** | Yes | Yes | Limited | Yes | Yes | No |
| **SSH access** | Yes | Yes | Limited | Yes | Yes | Provider-dependent |
| **Public API / SDK access** | Yes | Yes | Yes | Yes | Yes | Limited |
| **Web terminal support** | Yes | Yes | Limited | Yes | Yes | IDE/provider-dependent |
| **GitOps declarative management** | Yes | Partial | No | No | Partial | No |
| **AI-agent-oriented SDK usage** | Yes | Yes | First-class | Limited | Emerging | Limited |
| **Long-running background workloads** | Yes | Yes | Session-oriented | Limited | Yes | Provider-dependent |
| **CI/CD-oriented execution** | Yes | Possible | Possible | Limited | Possible | No |
| **Primary differentiation** | Identity-centric sandbox platform with integrated secure infrastructure access | Fast stateful AI/dev sandboxes | Ephemeral AI code execution | Managed GitHub-native development | Enterprise remote development | Portable devcontainer orchestration |

### Notes

- Cordium focuses on combining sandboxed execution with identity-aware infrastructure access, policy enforcement, and Kubernetes-native orchestration.
- Daytona has evolved beyond traditional developer workspaces into a broader AI-agent and sandbox platform with strong emphasis on fast startup times and stateful environments.
- E2B focuses primarily on ephemeral AI code execution environments with strong isolation using Firecracker microVMs.
- GitHub Codespaces is a managed cloud development environment tightly integrated with GitHub and Visual Studio Code.
- Coder primarily focuses on self-hosted remote development infrastructure, though recent positioning increasingly includes AI-assisted and agent-driven workflows.
- DevPod focuses primarily on portable devcontainer orchestration across different infrastructure providers rather than centralized multi-tenant sandbox management.


## License

Cordium is licensed under the [GNU Affero General Public License v3.0](LICENSE).

Copyright © Octelium Labs, LLC. All rights reserved.