[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Discord](https://img.shields.io/badge/chat-on%20discord-7289da.svg)](https://octelium.com/external/discord)
[![Slack](https://img.shields.io/badge/Slack-purple?logo=slack&logoColor=white)](https://octelium.com/external/slack)

<div align="center">
    <br />
    <img src="./unsorted/logo/main.png" alt="Cordium Logo" width="350"/>
</div>


Cordium is a free and open source, self-hosted, identity-based sandbox platform built on Kubernetes and [Octelium](https://github.com/octelium/octelium). Cordium is a general-purpose platform that provides isolated, reproducible isolated sandboxes for developers, AI agents, and automated workloads. Cordium can be used for various use cases, including as a remote development environment for coding sessions (e.g. VSCode, Zed, etc.), sandboxes for AI agents and CI/CD workloads, and secretless secure remote access to infrastructure for developers and AI agents.


The main differentiator for Cordium, when compared to remote development environments (e.g. GitHub Codespaces, Coder, etc.) and sandbox platforms (e.g. E2B, Daytona, etc.), is that Cordium automatically provides identity-based, secretless access to resources and infrastructure from within the sandboxes. Cordium leverages Octelium ZTNA capabilities to provide secretless, policy-driven access to infrastructure resources (SSH servers, databases, internal HTTP APIs, mTLS services) from within the sandbox, without exposing, distributing, or managing upstream application-layer credentials (e.g. API keys and access tokens, SSH private keys, database passwords, etc.). In short, Cordium is not just a sandbox platform for isolated execution, but also a platform for secure access to infrastructure that eliminates credential injection at scale.

## Table of Contents

- [Main Features](#main-features)
- [Core Concepts](#core-concepts)
- [Workspace Configuration](#workspace-configuration)
- [Access Methods](#access-methods)
- [CLI Usage](#cli-usage)
- [Install CLI](#install-cli)
- [Install your First Cluster](#install-your-first-cluster)
- [Comparison with Other Platforms](#comparison-with-other-platforms)
- [License](#license)


## Main Features

- **Unified platform for humans and AI agents.** Every sandbox (i.e. Workspace) is an isolated, rootless container sandbox running on standard Kubernetes, accessible via browser terminal, SSH, CLI, and gRPC API. Workspaces can be persistent (filesystem preserved across restarts) or ephemeral and can stop automatically when their tasks complete. The same platform can be used for long-lived coding sessions (e.g. via VSCode, Zed, etc.) and short-lived automated workloads (e.g. AI agent tasks, CI/CD, etc.).

- **Declarative, reproducible environments.** Workspace environments are defined in YAML specs covering the container image, repository cloning, lifecycle tasks, environment variables, resource limits (i.e. CPU, memory, storage), variable substitution, and application ports. Templates allow a single configuration to be reused across many Workspaces. Pre-built Templates capture a fully initialized filesystem as a Kubernetes VolumeSnapshot, reducing cold startup from minutes to seconds.

- **Secretless access to infrastructure.** Workspaces access databases, SSH servers, HTTP APIs, Kubernetes clusters, and mTLS-protected services without credentials ever reaching the Workspace. API keys, passwords, SSH private keys, and kubeconfigs are held at the Octelium identity-aware proxy and injected at the protocol layer if the Workspace identity is authorized. The Workspace itself does not hold credentials, eliminating credential sprawl for both developers and AI agents.

- **Identity-based access control and observability.** Every Workspace has an Octelium Session that represents its identity. Infrastructure access is governed by per-request, L7-aware attribute-based access control (ABAC) with policy-as-code using CEL and OPA, enforcing zero standing privileges by default. Authentication supports any OIDC or SAML 2.0 identity provider (IdP), GitHub OAuth2, workload OIDC assertions, and native FIDO2, WebAuthn, and TOTP.

- **OpenTelemetry-native auditing and visibility.** Real-time, identity-based, L7-aware visibility and access logging. Every request is logged and exported to your OpenTelemetry OTLP receivers for integration with log management and SIEM providers.

- **Open source and self-hosted.** Cordium is fully open source under Apache-2.0. It runs on any Kubernetes cluster, from a single-node VM to production multi-node installations, cloud or on-premises. There is no proprietary control plane, no tiered feature set, and no vendor lock-in.

## Concepts


![Cordium](https://octelium.com/assets/cordium-hierarchy-k-2W_obH.webp)


- **Space** is the top-level namespace in Cordium. It groups Templates, Workspaces, Secrets, and GitProviders under a single organizational unit.
- **Workspace** (synonymous with sandbox) is the fundamental execution unit in Cordium. It is an isolated, rootless container-based environment that can be used interactively or programmatically via web-based console, cordium CLI, standard SSH, and gRPC-based APIs.
- **Template** defines a reusable Workspace configuration within a Space. When a Workspace is created, it is initialized from the selected Template's spec. Every Space has a `default` Template created automatically. A Template's spec shares most of Workspace spec (image, runtime, etc.) as well as an optional GitProvider association. Templates support pre-builds via Kubernetes VolumeSnapshot to reduce startup time from minutes to seconds for dependency-heavy Templates.
- **Secrets** represents a sensitive value (API keys, tokens, passwords, certificates) stored within a Space. Secrets are referenced by name in Template specs.


## Workspace Configuration

Workspace and Template specs are defined in YAML and applied via `cordium run --file spec.yaml` or `cordium create template --file spec.yaml`. Here is a minimal example:

```yaml
spec:
  image:
    registry:
      url: python:3.12-bookworm

  repository:
    url: https://github.com/myorg/my-project

  runtime:
    tasks:
      - name: install
        type: ON_CREATE
        workingDir: /workspace/repo
        run: pip install -r requirements.txt
        onFailure: ON_FAILURE_ABORT
```

Here is a more comprehensive example:

```yaml
spec:
  image:
    dockerfile:
      inline: |
        FROM node:22-bookworm

        ENV DEBIAN_FRONTEND=noninteractive

        RUN apt-get update -qq && \
            apt-get install -y --no-install-recommends \
              podman \
              postgresql-client \
              curl \
              git \
            && rm -rf /var/lib/apt/lists/*

        RUN npm install -g @anthropic-ai/claude-code

  repository:
    url: https://github.com/myorg/api-service
    cloneOptions:
      branch: ${{ vars.BRANCH }}
      depth: 1

  vars:
    - name: BRANCH
      value: main
    - name: TASK
      value: "Review the codebase and fix any failing tests."

  runtime:
    autoStop: true
    envVars:
      - key: NODE_ENV
        value: test
      - key: SENTRY_DSN
        fromSecret: sentry-dsn

    tasks:
      - name: install-dependencies
        type: ON_CREATE
        workingDir: /workspace/repo
        run: npm ci
        onFailure: ON_FAILURE_ABORT

      - name: start-postgres
        type: POST_START
        runAsRoot: true
        run: |
          podman rm -f workspace-postgres 2>/dev/null || true
          podman run \
            --name workspace-postgres \
            --net host \
            -e POSTGRES_PASSWORD=password \
            -e POSTGRES_DB=app \
            -d docker.io/postgres:16

          for i in $(seq 1 30); do
            pg_isready -h localhost -p 5432 -U postgres && break
            sleep 2
          done

      - name: run-tests
        type: POST_START
        workingDir: /workspace/repo
        run: npm test
        onFailure: ON_FAILURE_CONTINUE

      - name: run-agent
        type: POST_START
        workingDir: /workspace/repo
        run: claude "${{ vars.TASK }}"

  limit:
    cpu:
      millicores: 4000
    memory:
      megabytes: 8192
    storage:
      megabytes: 20480
```

## Access Methods

### Web Portal

The Cordium web portal is a browser-based interface for managing and interacting with Workspaces without installing any software. It is the primary interface for users and teams who want clientless access to their Workspaces. The Octelium web portal authenticates users through Octelium's IdentityProviders, including GitHub OAuth2 or any OpenID Connect or SAML 2.0 IdP (read more here) or directly via Passkeys (read more [here](https://octelium.com/docs/octelium/latest/management/core/identity-providers)).



> [!NOTE]
> You can a watch a short demo video for the Cordium web portal [here](https://octelium.com/assets/cordium-web-DDaI3_PJ.webm).


## CLI Usage

The `cordium` CLI provides full command-line access to Workspace management. Here are some examples:

```sh
export OCTELIUM_DOMAIN=example.com
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

# Connect to the Octelium Cluster to be able to use native SSH utils (e.g. OpenSSH, scp, rsync, etc.)
octelium connect -d

# Generate an SSH config block for use with VS Code, JetBrains, Zed, rsync
cordium ssh abc --print-config >> ~/.ssh/config

# Then use VS Code Remote SSH
code --remote ssh-remote+cordium-abc /workspace/repo

# You can also use other tools such as rsync
rsync -avz ./dist/ cordium-abc:/workspace/repo/dist/

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


## Install your First Cluster

Read this quick guide [here](https://octelium.com/docs/cordium/latest/overview/quick-install) to install a single-node Cordium _Cluster_ on top of any cheap cloud VM/VPS instance (e.g. DigitalOcean Droplet, Hetzner server, AWS EC2, Vultr, etc...) or a local Linux machine/Linux VM inside a MacOS/Windows machine with at least 4GB of RAM and 20GB of disk storage running a recent Linux distribution (Ubuntu 24.04 LTS or later, Debian 12+, etc...), which is good enough for most development, personal or undemanding production use cases that do not require highly available multi-node _Clusters_. Once you SSH into your VPS/VM as root, you can install the _Cluster_ as follows:

```bash
curl -o install-cluster.sh https://octelium.com/install-cluster.sh
chmod +x install-cluster.sh

# Replace <DOMAIN> with your actual domain
./install-cluster.sh --domain <DOMAIN> --cordium
```

Once the _Cluster_ is installed. You can run your first Workspace as follows:

```bash
cordium run
```


## Install CLI

Install the `cordium` CLI as follows:

For Linux and MacOS

```bash
# Install Octelium CLIs
curl -fsSL https://octelium.com/install.sh | bash
# Install Cordium CLI
curl -fsSL https://octelium.com/install-cordium.sh | bash
```

For Windows in Powershell

```powershell
# Install Octelium CLIs
iwr https://octelium.com/install.ps1 -useb | iex
# Install Cordium CLI
iwr https://octelium.com/install-cordium.ps1 -useb | iex
```

You can also install the CLIs via Homebrew as follows:

```bash
brew install octelium/tap/octelium
brew install octelium/tap/cordium
```


## Comparison with Other Platforms

| Capability / Property | Cordium | Daytona | E2B | GitHub Codespaces | Coder | DevPod |
|---|---|---|---|---|---|---|
| **Primary workloads** | Developers, AI agents, automation workloads | Developers, AI agents | AI agents | Developers | Developers, AI-assisted development | Developers |
| **License** | Apache 2.0 | AGPLv3 | Mixed / managed-first | Proprietary | AGPLv3 | MPL 2.0 |
| **Self-hosted** | Yes | Yes | Limited | No | Yes | Local/client-side |
| **Managed SaaS offering** | No | Yes | Yes | Yes | Yes | No |
| **Kubernetes-native architecture** | Yes | Partial | No | No | Yes | No |
| **Horizontal scalability** | Yes, over Kubernetes | Distributed sandbox runtime | Managed proprietary infrastructure | GitHub-managed | Kubernetes/provider-dependent | Local/provider-dependent |
| **Isolation model** | Rootless nested containers | Containers / gVisor-style isolation | Firecracker microVMs | VMs/containers | Containers/VMs | Provider-dependent |
| **Root-equivalent access inside sandbox** | Yes | Yes | Limited | Yes | Configurable | Provider-dependent |
| **Nested container support** | Supported | Docker/container support | Limited | Supported | Supported | Provider-dependent |
| **Persistent stateful environments** | Yes | Yes | Limited/session-oriented | Yes | Yes | Provider-dependent |
| **Ephemeral execution support** | Yes | Yes | Yes | Limited | Limited | Limited |
| **Devcontainer support** | Yes | Yes | Partial | Yes | Yes | Yes |
| **Template-based provisioning** | Yes | Yes | Limited | Limited | Yes | Yes |
| **Secretless infrastructure access** | Yes | No | No | No | No | No |
| **Built-in infrastructure access** | Yes | Limited / external VPN integration | No | No | Limited / external | No |
| **Credential-sprawl reduction for infrastructure access** | First-class design goal | No, typically relies on sandbox/app credentials or external network integration | No, API-key-driven sandbox access | Limited to GitHub ecosystem integrations | External secret management / platform integrations | No centralized model |
| **Workspace identity model** | Dedicated Octelium Session identity per Workspace | Sandbox/user/org identity model | API-key / sandbox identity model | GitHub user/org identity | Coder user/workspace identity | Local user/provider identity |
| **L7-aware access control to infrastructure** | Yes, via Octelium Policies | No | No | No | No / external only | No |
| **Policy-as-code access control** | Yes, CEL and OPA | Limited / platform policy model | Limited | GitHub/org policies | RBAC/templates/external policy patterns | No |
| **OIDC human authentication** | Yes | Yes / Auth0-backed | Not primary user model | Enterprise SSO via GitHub org/enterprise | Yes | Provider-dependent / external |
| **SAML human authentication** | Yes | Enterprise option / contact Daytona | Not primary user model | Enterprise SSO via GitHub org/enterprise | Yes / enterprise-oriented | Provider-dependent / external |
| **GitHub identity provider** | Yes | Yes | Not primary user model | Native | Yes | Provider-dependent / external |
| **Native Passkeys / WebAuthn / FIDO2** | Yes | Not primary documented OSS feature | Not primary model | Via GitHub account security | External IdP / deployment-dependent | External |
| **Workload OIDC assertion authentication** | Yes | No / not primary documented feature | No | GitHub Actions OIDC for GitHub workflows, not sandbox identity | External IdP / deployment-dependent | No |
| **OAuth2 client credentials support** | Yes, through Octelium workload / IdP patterns where configured | API/JWT/token model, not primary OAuth2 client-credentials infrastructure access model | No, API-key/access-token model | GitHub Apps/OAuth ecosystem | External IdP / deployment-dependent | No |
| **Bearer/API-token authentication for platform API** | Yes | Yes, API keys/JWT | Yes, `E2B_API_KEY` / access token | Yes, GitHub tokens/API | Yes | Limited/provider-dependent |
| **SSH access** | Yes | Yes, token-based SSH access | Limited | Yes | Yes | Provider-dependent |
| **OpenTelemetry-native auditing** | Yes | Partial / SDK tracing | Limited | Platform-managed | External integrations | No |
| **CLI-first workflows** | Yes | Yes | Yes | Partial | Yes | Yes |
| **Multi-user platform** | Yes | Yes | Limited | Yes | Yes | No |
| **Public API / SDK access** | Yes | Yes | Yes | Yes | Yes | Limited |
| **Web terminal support** | Yes | Yes | Limited | Yes | Yes | IDE/provider-dependent |
| **GitOps declarative management** | Yes | Partial | No | No | Partial | No |
| **AI-agent-oriented SDK usage** | Yes | Yes | First-class | Limited | Emerging | Limited |
| **Long-running background workloads** | Yes | Yes | Session-oriented | Limited | Yes | Provider-dependent |
| **CI/CD-oriented execution** | Yes | Possible | Possible | Limited | Possible | No |
| **No proprietary control plane required** | Yes | No for managed, yes for OSS deployment | No for managed E2B | No | Yes for self-hosted Coder | Yes |
| **Primary differentiation** | Identity-centric sandbox platform with integrated secure infrastructure access | Fast stateful AI/dev sandboxes | Ephemeral AI code execution | Managed GitHub-native development | Enterprise remote development | Portable devcontainer orchestration |

## License

Cordium-owned source code is licensed under the [Apache License 2.0](LICENSE).

Cordium is built on top of Octelium. Some Cordium components or complete deployments may include Octelium Cluster components, which are publicly licensed under AGPLv3. Octelium Labs, LLC owns the relevant copyrights for both projects.

Copyright © Octelium Labs, LLC. All rights reserved.