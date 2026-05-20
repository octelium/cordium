# Cordium

Cordium is a free and open source, self-hosted, identity-based sandbox platform built on Kubernetes and [Octelium](https://octelium.com). It provides isolated, reproducible general-purpose sandboxes for developers, AI agents, and automated workloads that are accessible through web terminals, SSH, CLI, and gRPC APIs.

What sets Cordium apart is how Workspaces access infrastructure (e.g. remote internal resources behind NAT, publicly protected SaaS resources, IoT, etc.). Instead of injecting credentials into the environment, every Workspace operates with a dedicated Octelium identity. Databases, SSH servers, HTTP APIs, and Kubernetes clusters are accessed through Octelium's identity-aware, secretless access ZTNA infrastructure without exposing long-lived credentials such as API tokens, passwords, SSH private keys, or kubeconfigs directly inside the Workspace.

## Table of Contents

- [Main Features](#main-features)
- [Core Concepts](#core-concepts)
- [Workspace Configuration](#workspace-configuration)
- [Access Methods](#access-methods)
  - [CLI](#cli)
- [Install CLI](#install-cli)
- [Install your First Cluster](#install-your-first-cluster)
- [Comparison with Other Platforms](#comparison-with-other-platforms)
- [License](#license)


## Main Features

- **Workspaces for humans and machines.** Every Workspace is an isolated, rootless container sandbox running on standard Kubernetes, accessible via browser terminal, SSH, CLI, and gRPC API. Workspaces can be persistent (filesystem preserved across restarts) or ephemeral and can stop automatically when their tasks complete. The same platform serves interactive long-lived coding sessions and short-lived automated workloads.

- **Declarative, reproducible environments.** Workspace environments are defined in YAML specs covering the container image, repository cloning, lifecycle tasks, environment variables, resource limits, variable substitution, and application ports. Templates allow a single configuration to be reused across many Workspaces. Pre-built Templates capture a fully initialized filesystem as a Kubernetes VolumeSnapshot, reducing cold startup from minutes to seconds.

- **Secretless access to infrastructure.** Workspaces access databases, SSH servers, HTTP APIs, Kubernetes clusters, and mTLS-protected services without credentials ever reaching the Workspace. API keys, passwords, SSH private keys, and kubeconfigs are held at the Octelium identity-aware proxy and injected at the protocol layer if the Workspace identity is authorized. The Workspace itself does not hold credentials, eliminating credential sprawl for both developers and AI agents.

- **Identity-based access control and observability.** Every Workspace has an Octelium Session that represents its identity. Infrastructure access is governed by per-request, L7-aware attribute-based access control (ABAC) with policy-as-code using CEL and OPA, enforcing zero standing privileges by default. Authentication supports any OIDC or SAML 2.0 identity provider (IdP), GitHub OAuth2, workload OIDC assertions, and native FIDO2, WebAuthn, and TOTP.

- **OpenTelemetry-native auditing and visibility.** Real-time, identity-based, L7-aware visibility and access logging. Every request is logged and exported to your OpenTelemetry OTLP receivers for integration with log management and SIEM providers.

- **Purpose-built for AI agents.** Every agent run gets a dedicated Octelium identity and a clean, isolated Workspace with enforced resource limits and no state bleed between runs. Agents access databases, APIs, and internal services through their Workspace identity with no credential injection, so a compromised or misbehaving agent cannot exfiltrate credentials that were never present. Ephemeral storage and auto-stop on task completion require no manual cleanup. Pre-built Templates with agent frameworks pre-installed start in seconds via snapshot restoration.

- **Open source and self-hosted.** Cordium is fully open source under AGPLv3. It runs on any Kubernetes cluster, from a single-node VM to production multi-node installations, cloud or on-premises. There is no proprietary control plane, no tiered feature set, and no vendor lock-in.

## Concepts

- **Space** is the top-level namespace in Cordium. It groups Templates, Workspaces, Secrets, and GitProviders under a single organizational unit.
- **Workspace** (synonymous with sandbox) is the fundamental execution unit in Cordium. It is an isolated, rootless container-based environment that can be used interactively or programmatically via web-based console, cordium CLI, standard SSH, and gRPC-based APIs.
- **Template** defines a reusable Workspace configuration within a Space. When a Workspace is created, it is initialized from the selected Template's spec. Every Space has a `default` Template created automatically. A Template's spec shares most of Workspace spec (image, runtime, etc.) as well as an optional GitProvider association. Templates support pre-builds via Kubernetes VolumeSnapshot to reduce startup time from minutes to seconds for dependency-heavy Templates.
- **Secrets** represents a sensitive value (API keys, tokens, passwords, certificates) stored within a Space. Secrets are referenced by name in Template specs.


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
curl -fsSL https://octelium.com/install.sh | bash
curl -fsSL https://octelium.com/install-cordium.sh | bash
```

For Windows in Powershell

```powershell
iwr https://octelium.com/install.ps1 -useb | iex
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