# Cordium

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Discord](https://img.shields.io/badge/chat-on%20discord-7289da.svg)](https://octelium.com/external/discord)
[![Slack](https://img.shields.io/badge/Slack-purple?logo=slack&logoColor=white)](https://octelium.com/external/slack)

Cordium is a free and open source, self-hosted platform that combines isolated, reproducible sandboxes with identity-based, secretless access to infrastructure. Built on Kubernetes and [Octelium](https://github.com/octelium/octelium), it supports developers, AI agents and automated workloads, from persistent remote development environments to ephemeral CI/CD jobs.

Every sandbox (i.e. _Workspace_) has its own Octelium _Session_ identity. Through Octelium's identity-aware proxies, it can access authorized databases, SSH servers, internal APIs, LLM providers and Kubernetes clusters without receiving their upstream credentials (e.g. API keys, database passwords and SSH private keys). Access is policy-driven and evaluated per request using identity and context.

[Documentation](https://octelium.com/docs/cordium/latest/overview/intro) · [Quick Installation Guide](https://octelium.com/docs/cordium/latest/overview/quick-install) · [Examples](https://octelium.com/docs/cordium/latest/examples)

## Tours

| [Web Portal Tour](https://octelium.com/assets/cordium-portal-tour-dark-B9SmYIOW.webm) | [CLI Tour](https://octelium.com/assets/cordium-cli-tour-k46x0Bs8.webm) |
|---|---|
| Create and manage _Workspaces_, open terminals and chat with the Cordium Agent. | Run _Workspaces_, execute commands, copy files and access infrastructure without credentials. |

## Main Features

- **Unified platform for humans and AI agents.** _Workspaces_ are isolated, rootless container sandboxes running on standard Kubernetes. The same _Workspace_ can be used for long-lived coding sessions and short-lived automated workloads, through a [web browser](https://octelium.com/docs/cordium/latest/use/web-portal), [SSH and editors](https://octelium.com/docs/cordium/latest/use/ssh) (e.g. VS Code, Zed and JetBrains), the CLI or the SDKs.

- **Declarative, reproducible environments.** Define images, Dockerfiles, devcontainers, repositories, lifecycle tasks, environment variables and resource limits in YAML. _Workspaces_ support root access inside the sandbox for installing system packages and running nested containers. Reusable [Templates](https://octelium.com/docs/cordium/latest/workspaces/templates) support pre-builds that capture a fully initialized filesystem for fast startup.

- **Persistent, ephemeral and forkable storage.** Persistent _Workspaces_ preserve their filesystem across stops and restarts, while ephemeral ones start fresh on every run. [WorkspaceSnapshots](https://octelium.com/docs/cordium/latest/workspaces/snapshots) checkpoint storage and restore it into new _Workspaces_ for rollback or parallel runs. [Volumes](https://octelium.com/docs/cordium/latest/workspaces/volumes) outlive individual _Workspaces_ and provide persistent or shared storage for datasets, caches and artifacts.

- **Network isolation and egress control.** Restrict outbound traffic by CIDR and port, including a default-deny posture, with [network policies](https://octelium.com/docs/cordium/latest/workspaces/network) enforced outside the sandbox. Read-only root filesystems and configurable Linux capabilities provide further controls for untrusted and AI-generated code.

- **A built-in AI agent.** Chat with the [Cordium Agent](https://octelium.com/docs/cordium/latest/use/agent) from the web portal to create and manage _Workspaces_, run commands and test suites, copy files and use authorized Octelium _Services_. The agent runs inside your own _Workspace_ with your own identity, with configurable approvals for sensitive operations.

- **Secretless access to infrastructure.** Standard tools (e.g. `psql`, `ssh`, `curl` and `kubectl`) access authorized [Octelium Services](https://octelium.com/docs/cordium/latest/workspaces/secretless) without holding upstream API keys, database passwords, SSH private keys or kubeconfigs. Credentials remain at Octelium's identity-aware proxy and are applied at the protocol layer.

- **Identity-based access control.** Octelium provides per-request, L7-aware attribute-based access control (ABAC), with policy-as-code using CEL and OPA and zero standing privileges by default. [Authentication](https://octelium.com/docs/octelium/latest/management/core/identity-providers) supports OIDC and SAML 2.0 identity providers, GitHub OAuth2, passkeys and workload identity federation.

- **OpenTelemetry-native auditing and visibility.** Infrastructure requests through Octelium are authorized and logged with the _Workspace_'s identity. Export L7-aware [access logs](https://octelium.com/docs/octelium/latest/management/core/visibility) to your OpenTelemetry OTLP receivers for integration with log management and SIEM providers.

- **APIs and SDKs.** The complete gRPC API used by the CLI and web portal is available through official [SDKs for Go, Python, Rust and TypeScript/JavaScript](https://octelium.com/docs/cordium/latest/use/api). Create and manage sandboxes, execute commands, transfer files and manage snapshots and _Volumes_ from your applications, AI agents and CI pipelines.

- **Open source and designed for self-hosting.** Run Cordium on a single-node VM or a production multi-node Kubernetes cluster, cloud-based or on-premises. There is no proprietary control plane, no tiered feature set and no vendor lock-in. [Spaces](https://octelium.com/docs/cordium/latest/workspaces/spaces) organize _Workspaces_, _Templates_, _Secrets_ and _Volumes_ for individuals and teams.

## Getting Started

1. **Install a Cluster.** Follow the [quick installation guide](https://octelium.com/docs/cordium/latest/overview/quick-install) for a single-node Linux VM/VPS, or the [Cluster installation guide](https://octelium.com/docs/cordium/latest/install/cluster) for an existing Octelium _Cluster_ or a multi-node Kubernetes installation.
2. **Install the CLIs.** Follow the [CLI installation guide](https://octelium.com/docs/cordium/latest/install/cli) for Linux, macOS and Windows.
3. **Run your first Workspace.** Follow the [first steps guide](https://octelium.com/docs/cordium/latest/overview/first-steps) to run a _Workspace_, create a _Template_, access infrastructure without credentials and take a snapshot. You can also use the web portal at `https://cordium.<DOMAIN>`.

## CLI Usage

Set your _Cluster_ domain, log in and create a _Workspace_ from your default _Template_ with an interactive terminal:

```bash
export OCTELIUM_DOMAIN=example.com
octelium login
cordium run
```

Create a disposable _Workspace_ from an image, or use your own YAML configuration:

```bash
cordium run --image python:3.13-slim --ephemeral --rm
cordium run --file workspace.yaml
```

For an existing _Workspace_ named `abc`, open a terminal (starting it first if stopped), run a test suite or follow its startup logs:

```bash
cordium run abc
cordium exec abc -w /workspace/repo -- make test
cordium logs abc
```

Read the [CLI guide](https://octelium.com/docs/cordium/latest/use/cli) for _Templates_, _Volumes_, snapshots, file transfers and automation, and the [SSH and IDE guide](https://octelium.com/docs/cordium/latest/use/ssh) for editor integration and port forwarding.

## Workspace Configuration

_Workspaces_ and _Templates_ share the same declarative spec structure. For example, the following `workspace.yaml` clones a Python project and installs its dependencies on the first run. Replace the repository URL with your own project:

```yaml
spec:
  image:
    registry:
      url: python:3.13-slim
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

Run it via `cordium run --file workspace.yaml`, or save it as a reusable _Template_ via `cordium create template python-dev --file workspace.yaml`.

Read the [configuration guide](https://octelium.com/docs/cordium/latest/workspaces/overview) for spec inheritance, repository configuration and variables, and the dedicated guides for [images and devcontainers](https://octelium.com/docs/cordium/latest/workspaces/image), [environment and tasks](https://octelium.com/docs/cordium/latest/workspaces/runtime), [resources and timeouts](https://octelium.com/docs/cordium/latest/workspaces/resources) and [secretless access](https://octelium.com/docs/cordium/latest/workspaces/secretless).

## FAQ

### How does Cordium differ from sandbox platforms such as Daytona and E2B?

Cordium combines sandboxed execution with Octelium's identity-based access to infrastructure. Every _Workspace_ has its own _Session_ identity, and the upstream credentials used to access an Octelium _Service_ remain outside the sandbox. Access can be scoped to the _Workspace_, its _Template_ or its _Space_ and is evaluated per request using identity and context. This applies to developers and AI agents alike, including access to private resources behind NAT and public APIs such as LLM providers. Read more about the architecture [here](https://octelium.com/docs/cordium/latest/overview/how).

### How are Workspaces sandboxed?

Each _Workspace_ runs in its own Kubernetes pod, where a supervisor uses Podman to launch the sandbox as a nested, rootless container. The sandbox has its own filesystem, processes and network, and its root user maps to an unprivileged user outside it. This lets you install system packages and run containers inside the _Workspace_ without granting it root access to the Kubernetes node.

The supervisor enforces resource limits and network egress policies outside the sandbox. Cordium runs on standard Kubernetes nodes without requiring a separate VM per _Workspace_ or specialized hardware. Read more about the isolation model [here](https://octelium.com/docs/cordium/latest/overview/how#workspace-isolation-model).

### What storage is needed for snapshots and shared Volumes?

_Template_ pre-builds and _WorkspaceSnapshots_ require a Kubernetes CSI storage backend with volume snapshot support. Shared _Volumes_ require a multi-writer filesystem backend (e.g. NFS, CephFS or EFS). Persistent _Workspaces_ and exclusive _Volumes_ can use regular single-writer storage. Read the [storage guide](https://octelium.com/docs/cordium/latest/management/storage) for configuration.

## License

Cordium-owned source code is licensed under the [Apache License 2.0](LICENSE).

Cordium is built on top of Octelium. Some Cordium components or complete deployments may include Octelium Cluster components, which are publicly licensed under AGPLv3. Octelium Labs, LLC owns the relevant copyrights for both projects.

Copyright © Octelium Labs, LLC. All rights reserved.
