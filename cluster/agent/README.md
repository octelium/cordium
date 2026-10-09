# @octelium/cordium-agent

The Cordium AI agent behind the chat of the Cordium web portal (`https://cordium.<DOMAIN>/agent`). It runs as an ordinary process inside a Cordium Workspace of the User, uses that Workspace's own Octelium identity and exposes an HTTP/JSON + SSE API that is served as the Workspace's default Application. It manages the User's Workspaces: creating, starting, stopping and deleting them, running commands and test suites inside them, copying files between them, reaching the Octelium Services that the User can access, and everything else exposed by the Cordium API.

```text
Cordium web portal (chat UI)
   │  HTTP/JSON + SSE (https://<workspace>.cordium.<domain>, authenticated as the Workspace owner)
   ▼
cordium-agent ───────────────────── the User's agent Workspace (octelium.<user> Space) ─────────
   ├─ Pi (agent loop, sessions, compaction, retries, skills)
   ├─ tools
   │   ├─ workspace_list / workspace_create / workspace_control / workspace_exec /
   │   │  workspace_copy / workspace_logs        └─ @octelium/cordium SDK ─┐
   │   ├─ cordium_api_search / cordium_api_describe / cordium_api_call     │
   │   │     └─ @octelium/apis + apis/cordium.json (JSON Schemas) ─────────┤
   │   │                    gRPC over OCTELIUM_AUTH_PROXY_SOCKET ◄─────────┘──► Cordium, user and core APIs
   │   ├─ present_table / present_chart / present_resources / publish_artifact
   │   ├─ read / bash / edit / write / grep / find / ls  (the Workspace is the agent's computer)
   │   └─ web_fetch / web_search
   └─ LLM ──► the User's Claude/ChatGPT subscription, an Octelium LLM Service or any Pi provider
```

## Design

- **Execution.** The agent is a Node.js process inside the User's agent Workspace. Its own shell commands run in that Workspace, while the commands in the User's other Workspaces run through the Cordium `WorkspaceService.Exec` API (no SSH needed). Both the agent's Workspace and the other Workspaces reach the Octelium Services that the User can access by name through their `octelium connect`.
- **Identity.** The Cordium, user and core APIs are called through the supervisor's auth proxy socket (`OCTELIUM_AUTH_PROXY_SOCKET`), exactly like the `cordium` and `octelium` CLIs do inside a Workspace. Every call is authorized server-side for the Workspace owner. The model never sees a credential.
- **APIs.** The typed protobuf messages and service clients come from the [`@octelium/apis`](https://www.npmjs.com/package/@octelium/apis) package and the high-level Workspace operations (lifecycle, exec, file transfers) from [`@octelium/cordium`](https://www.npmjs.com/package/@octelium/cordium). The documentation of the APIs is shipped as one deduplicated bundle of JSON Schemas (`apis/cordium.json`, generated from the `.proto` files with their doc comments) which powers a BM25 search over the methods, the rendering of the request schemas with their field documentation, and the method index of the system prompt (Cordium and user APIs). The model sends proto3 JSON, which is validated with the message types before the call (unknown fields and wrong types are rejected with precise errors).
- **Tools.** Besides the generic API tools, the agent has dedicated Workspace tools that are optimized for its use cases: `workspace_create` creates, starts and waits for a Workspace while streaming its state, `workspace_control` starts, stops, restarts, deletes or waits for several Workspaces in parallel, `workspace_exec` streams the output of a command (or starts it in the background), `workspace_copy` copies files and directories between Workspaces and the agent's own filesystem through the Exec API, and `workspace_logs` reads the initialization logs. Tool calls of the same model turn run concurrently, so the same operation can run in several Workspaces at once. The agent refuses to stop, restart or delete its own Workspace.
- **Approvals.** Methods are classified by risk (`read`, `write`, `destructive`, `sensitive`). Depending on the approval mode (`never`, `destructive`, which also covers the sensitive calls such as Secrets, Memberships and shared ports, or `write`), calls pause until the User approves them in the UI, and so do the shell commands if `commands` is set. The User can switch the mode at runtime from the portal (`PATCH /v1/settings`). Approvals are a UX safety net, not the security boundary: authorization is always enforced by the Cluster.
- **Model.** The Users can sign in with their own Claude (Pro/Max) or ChatGPT subscriptions from the portal. The OAuth flows run inside the agent and the tokens never leave the Workspace. Alternatively the model is served by an Octelium LLM Service (`llm.provider: "octelium"`) reached through `octelium connect`, or by any provider supported by Pi.
- **Backend abstraction.** The HTTP protocol knows nothing about Pi: runs go through the `AgentBackend` interface (`src/agent/types.ts`).
- **Persistence.** Conversations, finalized messages, Pi sessions, uploads, artifacts and settings are stored in the data directory. An in-flight message is checkpointed and recovered as `interrupted` if the process dies. Restarting the agent resumes the conversations with their full context.

## Running inside a Cordium Workspace

The Cordium `MainService.InitializeAgent` method provisions the agent of the calling User on demand: it creates the User's personal `octelium` Space (`octelium.<user>`, a system Space that hosts the Workspaces of all the Octelium-managed agents of the User), the `cordium-agent.octelium.<user>` Template and a Workspace that serves the agent as its default Application on port `8080`, and references that Workspace in the User's `UserConfig` (`status.agentWorkspaceRef`). Subsequent calls reconcile the Template and recreate the Workspace if it was deleted. The Template installs Node.js upon the first start of a Workspace if the image lacks it (`ON_CREATE`) and starts `npx --yes --prefer-online @octelium/cordium-agent@<version> serve` on every start (`POST_START`). It is generated from the `spec.agent` section of the Cordium ClusterConfig (`cordium man`):

| Field | Description |
| --- | --- |
| `isDisabled` | Disables `InitializeAgent` |
| `llm.service`, `llm.model` | The default LLM Service and model (i.e. `llm.service` and `llm.model` below) |
| `version` | The `@octelium/cordium-agent` npm version or dist-tag. Development builds default to their Git branch (`dev` when unknown) and production builds to their semver (`latest` when unknown) |
| `image`, `limit` | The Workspace image and compute resources |
| `config` | Any additional configuration (same structure as `config.json`), passed via `CORDIUM_AGENT_CONFIG_JSON` |

The web portal calls `InitializeAgent`, starts the Workspace and talks to the agent at the Workspace's hostname. The agent can also run in any other Workspace whose image has Node.js 22.19+ and `git`:

```yaml
spec:
  runtime:
    envVars:
      - key: CORDIUM_AGENT_CONFIG_JSON
        value: '{"approvals":{"api":"write"}}'
    tasks:
      - name: cordium-agent
        type: POST_START
        isBackground: true
        run: npx --yes --prefer-online @octelium/cordium-agent@latest serve
  applications:
    - name: agent
      displayName: Cordium Agent
      port: 8080
      isDefault: true
```

`npx @octelium/cordium-agent doctor` checks the configuration, the Cordium API access and the LLM access.

## Configuration

The configuration is read, in increasing precedence, from the defaults, a JSON file (`--config`, `CORDIUM_AGENT_CONFIG` or `<dataDir>/config.json`), the `CORDIUM_AGENT_CONFIG_JSON` environment variable and the CLI flags. Unknown fields are rejected. See `config.example.json`.

| Field | Default | Description |
| --- | --- | --- |
| `dataDir` | `~/.cordium-agent` | Conversations, sessions, uploads, artifacts, skills, settings |
| `workDir` | `/workspace` if it exists, else `$HOME` | The agent's working directory |
| `logLevel` | `info` | `debug`, `info`, `warn`, `error` |
| `server.host`, `server.port` | `0.0.0.0`, `8080` | Listen address |
| `server.allowedOrigins` | `https://cordium.<domain>`, the portal of the Workspace's Region, `https://$CORDIUM_HOSTNAME` | Accepted `Origin` headers (plus the request's own origin) |
| `server.cors` | `false` | Emit CORS headers. Keep it off since the Cordium portal Service already handles CORS for `https://cordium.<domain>` |
| `server.authToken` | | Optional bearer token (or `?access_token=`) required for `/v1/*` |
| `server.maxUploadBytes` | 100 MiB | Upload limit |
| `octelium.mode` | `auto` | `proxy` (auth proxy socket), `direct` (`OCTELIUM_DOMAIN` + `OCTELIUM_ACCESS_TOKEN`, for development), `disabled` |
| `octelium.domain` | `OCTELIUM_DOMAIN` | Cluster domain |
| `octelium.timeoutSeconds` | `30` | Per-call deadline |
| `llm.provider` | `octelium` | `octelium`, `custom` or any Pi provider ID |
| `llm.service` | | The LLM Service (`name` or `name.namespace`). Its URL is resolved via the user API |
| `llm.baseUrl` | | Overrides the endpoint (required for `custom`) |
| `llm.api` | `openai-completions` | Pi API of the endpoint, e.g. `openai-responses`, `anthropic-messages`, `google-generative-ai` |
| `llm.model` | | Model ID. Discovered from the endpoint's model list if unset (`octelium`/`custom`) |
| `llm.apiKey`, `llm.apiKeyEnv` | | Provider credentials. Not needed with an Octelium LLM Service |
| `llm.thinkingLevel` | `medium` | `off` … `max` |
| `llm.contextWindow`, `llm.maxTokens`, `llm.reasoning`, `llm.input` | from Pi's catalog | Model metadata for unknown model IDs |
| `llm.loginProviders` | `anthropic`, `openai` | The Pi providers that the Users can sign in to with their own subscriptions (Claude Pro/Max, ChatGPT) |
| `agent.tools` | all | Pi built-in tools: `read`, `bash`, `edit`, `write`, `grep`, `find`, `ls` (`grep`/`find` are dropped if `rg`/`fd` are missing) |
| `agent.systemPromptAppend` | | Extra instructions |
| `agent.loadContextFiles` | `false` | Load `AGENTS.md`-style files from the working directory |
| `agent.maxConcurrentRuns` | `4` | Across conversations |
| `agent.sessionIdleTimeoutSeconds` | `1800` | Idle Pi sessions are disposed (and transparently reopened) |
| `agent.compaction`, `agent.maxRetries` | `true`, `3` | Context compaction and LLM retries |
| `agent.generateTitles` | `true` | Generate conversation titles with the LLM |
| `approvals.api` | `destructive` | The default approval mode: `never`, `destructive` (deletions and sensitive calls) or `write` (every call that is not read-only). The User can override it from the portal |
| `approvals.commands` | `false` | Require an approval for every shell command, in the agent's Workspace or in other Workspaces |
| `skills.repositories` | `octelium/octelium-skills@main` | Agent Skills repositories (including the `cordium` skill), cloned/updated at startup (best effort) |
| `skills.paths` | | Local skill directories |
| `webSearch` | | `{"provider": "brave" \| "tavily" \| "searxng", "baseUrl", "apiKey" \| "apiKeyEnv"}` |

## HTTP API

All the types are exported from `@octelium/cordium-agent/protocol` (`src/protocol/index.ts`), which has no runtime dependencies. `make gen-api` copies it to the web portal.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness |
| `GET` | `/v1/info` | Version, protocol version, model, identity, settings, capabilities, issues |
| `GET` | `/v1/conversations` | List the conversations |
| `POST` | `/v1/conversations` | Create a conversation, optionally starting a run: `{"title"?, "input"?: {"text", "attachments"?}}` |
| `GET` | `/v1/conversations/{id}` | The conversation, its messages and the snapshot of its active run |
| `PATCH` | `/v1/conversations/{id}` | Rename: `{"title"}` |
| `DELETE` | `/v1/conversations/{id}` | Cancel its run and delete it |
| `POST` | `/v1/conversations/{id}/runs` | Start a run: `{"input": {"text", "attachments"?}}` (`409` if a run is active) |
| `GET` | `/v1/runs/{id}` | Run snapshot (run + in-progress assistant message) |
| `GET` | `/v1/runs/{id}/events` | SSE stream of the run's events |
| `POST` | `/v1/runs/{id}/cancel` | Cancel the run |
| `POST` | `/v1/runs/{id}/approvals/{approvalId}` | `{"decision": "approve" \| "reject", "reason"?}` |
| `POST` | `/v1/files?name=<file name>` | Upload a file (raw body). Returns a `FileInfo` whose `id` can be attached to a run |
| `GET` | `/v1/artifacts/{id}` | Artifact metadata |
| `GET` | `/v1/artifacts/{id}/content` | Download an artifact (`attachment`, sandboxed CSP; `?inline=1` for safe types) |
| `GET` | `/v1/models` | The current model, the thinking level and the available models |
| `PUT` | `/v1/model` | Switch the model: `{"provider", "id", "thinkingLevel"?}` |
| `GET` | `/v1/settings` | The approval settings |
| `PATCH` | `/v1/settings` | Update them: `{"approvals": {"api"?, "commands"?}}` |
| `GET` | `/v1/auth/providers` | The subscription providers and whether they are signed in |
| `DELETE` | `/v1/auth/providers/{id}` | Sign out |
| `POST` | `/v1/auth/logins` | Start signing in: `{"provider", "selectModel"?}`. Returns a `LoginSession` |
| `GET` | `/v1/auth/logins/{id}` | Poll a `LoginSession` (`events` such as `auth_url`, the pending `prompt`, `status`) |
| `POST` | `/v1/auth/logins/{id}/prompts/{promptId}` | Answer the pending prompt: `{"value"}` |
| `DELETE` | `/v1/auth/logins/{id}` | Cancel signing in |

Errors are `{"error": {"code", "message"}}`. `POST`/`PUT`/`PATCH` bodies must be `application/json`.

### Messages, blocks and events

A run appends one user message and one assistant message to the conversation. A message is an ordered list of typed blocks: `markdown`, `thinking`, `tool` (with structured `details`: `api` calls with the method, risk, request, response and the referenced resources, `command` with the Workspace, exit code and duration, `file` with diffs, `workspaces` with Workspace summaries and `transfer`), `approval`, `table`, `chart`, `resources`, `artifact`, `notice` and `error`. Resources are linked as `octelium://resource/<apiVersion>/<Kind>/<name>`. Clients must ignore unknown block types and unknown fields; `PROTOCOL_VERSION` is bumped for incompatible changes only.

`GET /v1/runs/{id}/events` streams the `AgentEvent`s (`run.*`, `message.*`, `block.created`, `block.delta`, `block.updated`, `conversation.updated`) as SSE. The events of a run are replayed from the start, or after `Last-Event-ID` / `?after=<seq>`, so a client can reconnect or attach to a run in progress. The stream ends after a terminal run event; a finished run with nothing left to replay returns `204`.

## Requirements on the Cluster side

- The portal Service allows the CORS origin `https://cordium.<domain>` with credentials and the portal's CSP allows `https://*.cordium.<domain>`, so that the portal can reach the agent at its Workspace's hostname. Other Workspace Applications are deliberately not allowed since they would be able to drive the agents of other Users.
- To use an Octelium LLM Service, the User must be authorized to access it.
- Do not share the agent's Application with other Users: the agent acts with the Workspace owner's identity.

## Publishing

`.github/workflows/publish-agent.yaml` publishes the package using npm trusted publishing (GitHub OIDC). Configure the package's trusted publisher on npmjs.com with GitHub organization `octelium`, repository `cordium`, workflow filename `publish-agent.yaml`, no environment, and permission to run `npm publish`.

Pushing a `v*.*.*` Git tag or publishing a GitHub release publishes the tag's semver, removing the leading `v`, which is the version that the API server of that release runs. Stable releases use `latest`; semver prereleases and GitHub prereleases use `next`. Automatic runs skip versions already published by another event.

To publish manually, run the workflow from GitHub Actions, select the branch or Git tag and supply the npm `tag` (e.g. `dev` or `main` for the development builds of the API server). An optional `version` sets an explicit semver. Without it, a selected Git tag supplies the version, while a selected branch produces a unique prerelease containing the npm tag.

## Development

```bash
npm ci
npm test              # node:test, no network needed (faux LLM + mock gRPC API with a fake Cordium)
npm run typecheck
npm run build         # dist/
npm run dev           # node src/cli.ts serve
node src/cli.ts api search "share workspace port"
node src/cli.ts api describe cordium.v1.MainService/CreateVolume
```

Outside a Workspace, set `OCTELIUM_DOMAIN` and `OCTELIUM_ACCESS_TOKEN` to use the direct mode.

`apis/cordium.json` is regenerated by `make gen-api` (or `make gen-api-agent` after `make cp-pb`) from the `.proto` files via `scripts/gen-apis.mjs`.
