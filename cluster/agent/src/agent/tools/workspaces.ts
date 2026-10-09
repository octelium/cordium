import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import {
  Workspace_Spec,
  Workspace_Status_State,
  type Workspace as WorkspaceResource,
  type Workspace_Spec_Image,
} from "@octelium/apis/main/cordiumv1";
import {
  shellQuote,
  WorkspaceFailureError,
  type Cordium,
  type Workspace,
  type WorkspaceOptions,
} from "@octelium/cordium";
import { defineTool } from "@earendil-works/pi-coding-agent";
import { Type } from "@earendil-works/pi-ai";
import type { AgentToolResult } from "@earendil-works/pi-agent-core";
import type { JsonValue } from "@protobuf-ts/runtime";
import type {
  APIRisk,
  CommandDetails,
  ToolDetails,
  WorkspaceSummary,
  WorkspacesDetails,
} from "../../protocol/index.ts";
import { toAPIError, type APIClient } from "../../api/client.ts";
import { splitCamelCase } from "../../api/search.ts";
import type { SettingsProvider } from "../types.ts";
import { needsApproval } from "./api.ts";
import { createTransferTool } from "./transfer.ts";
import {
  errorResult,
  requireRunContext,
  textResult,
  type ToolDeps,
} from "./util.ts";

export const WORKSPACE_TOOL_NAMES = [
  "workspace_list",
  "workspace_create",
  "workspace_control",
  "workspace_exec",
  "workspace_copy",
  "workspace_logs",
];

export interface WorkspaceToolDeps extends ToolDeps {
  client: APIClient;
  settings: SettingsProvider;
  agentWorkspace?: string;
  tmpDir: string;
  pollIntervalMs?: number;
}

type OnUpdate = (
  partial: AgentToolResult<ToolDetails | undefined>,
) => void | undefined;

const S = Workspace_Status_State;

export const stateName = (state?: Workspace_Status_State): string =>
  S[state ?? S.UNKNOWN] ?? "UNKNOWN";

export const isStable = (state: Workspace_Status_State): boolean =>
  state === S.RUNNING || state === S.STOPPED;

export const isStopping = (state: Workspace_Status_State): boolean =>
  state === S.STOPPING_REQUEST || state === S.STOPPING;

export const imageSummary = (
  image?: Workspace_Spec_Image,
): string | undefined => {
  const type = image?.type;
  switch (type?.oneofKind) {
    case "registry":
      return type.registry.url || undefined;
    case "dockerfile":
      return type.dockerfile.type.oneofKind === "url"
        ? `Dockerfile ${type.dockerfile.type.url}`
        : "inline Dockerfile";
    case "git":
      return `image Git repository ${type.git.url}`;
    case "repository":
      return "the repository's devcontainer/Dockerfile";
    default:
      return undefined;
  }
};

export const failureMessage = (ws: WorkspaceResource): string | undefined => {
  const failure = ws.status?.run ? ws.status.run.failure : ws.status?.failure;
  if (!failure) {
    return undefined;
  }
  return (
    failure.message ||
    (failure.type.oneofKind
      ? `${splitCamelCase(failure.type.oneofKind).toLowerCase()} failure`
      : "The Workspace run failed")
  );
};

export const summarizeWorkspace = (
  ws: WorkspaceResource,
  agentWorkspace?: string,
): WorkspaceSummary => {
  const name = ws.metadata?.name ?? "";
  const createdAt = ws.metadata?.createdAt;
  return {
    name,
    uid: ws.metadata?.uid || undefined,
    displayName: ws.metadata?.displayName || undefined,
    state: stateName(ws.status?.state),
    space: ws.status?.spaceRef?.name || undefined,
    template: ws.status?.templateRef?.name || undefined,
    region: ws.status?.regionRef?.name || undefined,
    hostname: ws.status?.hostname || undefined,
    image: imageSummary(ws.spec?.image),
    repository: ws.spec?.repository?.url || undefined,
    isEphemeral: ws.spec?.isEphemeral || undefined,
    isAgent: agentWorkspace && name === agentWorkspace ? true : undefined,
    failure: ws.status?.state === S.STOPPED ? failureMessage(ws) : undefined,
    createdAt: createdAt
      ? new Date(Number(createdAt.seconds) * 1000).toISOString()
      : undefined,
  };
};

export const formatSummary = (ws: WorkspaceSummary): string => {
  const parts = [ws.state];
  if (ws.space) {
    parts.push(`Space ${ws.space}`);
  }
  if (ws.template) {
    parts.push(`Template ${ws.template}`);
  }
  if (ws.region) {
    parts.push(`Region ${ws.region}`);
  }
  if (ws.image) {
    parts.push(`image ${ws.image}`);
  }
  if (ws.repository) {
    parts.push(`repository ${ws.repository}`);
  }
  if (ws.hostname) {
    parts.push(`https://${ws.hostname}`);
  }
  if (ws.isEphemeral) {
    parts.push("ephemeral");
  }
  const flags = [
    ws.isAgent ? "this agent's own Workspace" : undefined,
    ws.failure ? `last run failed: ${ws.failure}` : undefined,
  ].filter(Boolean);
  return `- ${ws.name}${ws.displayName ? ` "${ws.displayName}"` : ""}: ${parts.join(" · ")}${flags.length > 0 ? ` (${flags.join("; ")})` : ""}`;
};

export const matchesName = (actual: string | undefined, query: string) =>
  !!actual && (actual === query || actual.startsWith(`${query}.`));

const workspaceName = Type.String({
  description: "The Workspace name (e.g. abc)",
});

const describeError = (err: unknown): string => {
  if (err instanceof WorkspaceFailureError) {
    return err.message;
  }
  const apiErr = toAPIError(err);
  return `${apiErr.message} (${apiErr.code})`;
};

export interface WaitResult {
  workspace: Workspace;
  reached: boolean;
  failure?: string;
}

export const waitForState = async (
  ws: Workspace,
  target: "running" | "stopped" | "stable",
  opts: {
    deadline: number;
    signal?: AbortSignal;
    pollIntervalMs?: number;
    onState?: (ws: Workspace) => void;
  },
): Promise<WaitResult> => {
  const interval = opts.pollIntervalMs ?? 2000;
  for (;;) {
    await ws.refresh({ signal: opts.signal });
    opts.onState?.(ws);
    const state = ws.state;
    if (
      (target === "running" && state === S.RUNNING) ||
      (target === "stopped" && state === S.STOPPED) ||
      (target === "stable" && isStable(state))
    ) {
      return { workspace: ws, reached: true };
    }
    if (target === "running" && (state === S.STOPPED || isStopping(state))) {
      return {
        workspace: ws,
        reached: false,
        failure:
          failureMessage(ws.toProto()) ??
          `The Workspace ${ws.name} stopped before it was running`,
      };
    }
    if (Date.now() + interval > opts.deadline) {
      return { workspace: ws, reached: false };
    }
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(resolve, interval);
      opts.signal?.addEventListener(
        "abort",
        () => {
          clearTimeout(timer);
          reject(new Error("The operation was cancelled"));
        },
        { once: true },
      );
    });
  }
};

class ProgressReporter {
  private states = new Map<string, string>();
  private started = Date.now();
  private lastSent = 0;
  private timer?: NodeJS.Timeout;
  private onUpdate?: OnUpdate;

  constructor(onUpdate?: OnUpdate) {
    this.onUpdate = onUpdate;
  }

  set(name: string, state: string) {
    if (this.states.get(name) === state) {
      return;
    }
    this.states.set(name, state);
    this.flush();
  }

  private flush() {
    if (!this.onUpdate || this.timer) {
      return;
    }
    const wait = this.lastSent + 250 - Date.now();
    if (wait > 0) {
      this.timer = setTimeout(() => {
        this.timer = undefined;
        this.flush();
      }, wait);
      this.timer.unref();
      return;
    }
    this.lastSent = Date.now();
    const elapsed = Math.round((this.lastSent - this.started) / 1000);
    this.onUpdate({
      content: [
        {
          type: "text",
          text: [...this.states.entries()]
            .map(([name, state]) => `${name}: ${state}`)
            .concat(`(${elapsed}s)`)
            .join("\n"),
        },
      ],
      details: undefined,
    });
  }

  close() {
    clearTimeout(this.timer);
    this.timer = undefined;
    this.onUpdate = undefined;
  }
}

const toWorkspaceOptions = (params: {
  displayName?: string;
  space?: string;
  template?: string;
  snapshot?: string;
  image?: string;
  dockerfile?: string;
  repository?: {
    url: string;
    branch?: string;
    depth?: number;
    checkout?: string;
  };
  env?: Record<string, string>;
  vars?: Record<string, string>;
  resources?: { cpu?: number; memory?: number; storage?: number };
  ephemeral?: boolean;
  autoStop?: boolean;
  applications?: { name: string; port: number; default?: boolean }[];
  spec?: object;
}): WorkspaceOptions => {
  let template = params.template;
  if (template && params.space && !template.includes(".")) {
    template = `${template}.${params.space}`;
  } else if (!template && params.space) {
    template = `default.${params.space}`;
  }

  return {
    displayName: params.displayName,
    template,
    snapshot: params.snapshot,
    image:
      params.image ??
      (params.dockerfile ? { dockerfile: params.dockerfile } : undefined),
    repository: params.repository
      ? {
          url: params.repository.url,
          cloneOptions:
            params.repository.branch ||
            params.repository.depth ||
            params.repository.checkout
              ? {
                  branch: params.repository.branch,
                  depth: params.repository.depth,
                  checkout: params.repository.checkout,
                }
              : undefined,
        }
      : undefined,
    env: params.env,
    vars: params.vars,
    resources: params.resources,
    ephemeral: params.ephemeral,
    autoStop: params.autoStop,
    applications: params.applications,
    spec: params.spec
      ? Workspace_Spec.fromJson(params.spec as unknown as JsonValue, {
          ignoreUnknownFields: false,
        })
      : undefined,
  };
};

class OutputCollector {
  readonly logPath: string;
  private fd?: number;
  private decoders = {
    stdout: new TextDecoder(),
    stderr: new TextDecoder(),
  };
  private head = "";
  private tail = "";
  private total = 0;
  private maxHead: number;
  private maxTail: number;

  constructor(logPath: string, maxHead = 4000, maxTail = 20000) {
    this.logPath = logPath;
    this.maxHead = maxHead;
    this.maxTail = maxTail;
  }

  write(stream: "stdout" | "stderr", data: Uint8Array) {
    if (this.fd === undefined) {
      fs.mkdirSync(path.dirname(this.logPath), { recursive: true });
      this.fd = fs.openSync(this.logPath, "w", 0o600);
    }
    fs.writeSync(this.fd, data);
    const text = this.decoders[stream].decode(data, { stream: true });
    this.total += text.length;
    if (this.head.length < this.maxHead) {
      const room = this.maxHead - this.head.length;
      this.head += text.slice(0, room);
    }
    this.tail = (this.tail + text).slice(-this.maxTail);
  }

  close() {
    if (this.fd !== undefined) {
      fs.closeSync(this.fd);
      this.fd = undefined;
    }
  }

  get written(): boolean {
    return this.total > 0;
  }

  preview(max = 4000): string {
    return this.tail.slice(-max);
  }

  text(): { text: string; truncated: boolean } {
    if (this.total <= this.maxHead + this.maxTail) {
      return {
        text: this.total <= this.maxHead ? this.head : this.headAndTail(),
        truncated: false,
      };
    }
    return {
      text: `${this.head}\n…[${this.total - this.head.length - this.tail.length} characters omitted, the full output is saved to ${this.logPath}]…\n${this.tail}`,
      truncated: true,
    };
  }

  private headAndTail(): string {
    const overlap = this.head.length + this.tail.length - this.total;
    return overlap >= 0
      ? this.head + this.tail.slice(overlap)
      : `${this.head}${this.tail}`;
  }
}

export const backgroundCommand = (command: string, id: string): string => {
  const log = `/tmp/cordium-agent/${id}.log`;
  const inner = `sh -c ${shellQuote(command)}; echo $? > ${log}.exit`;
  return [
    "mkdir -p /tmp/cordium-agent",
    `if command -v setsid >/dev/null 2>&1; then setsid nohup sh -c ${shellQuote(inner)} > ${log} 2>&1 < /dev/null & else nohup sh -c ${shellQuote(inner)} > ${log} 2>&1 < /dev/null & fi`,
    "echo $!",
  ].join("\n");
};

export const createWorkspaceTools = (deps: WorkspaceToolDeps) => {
  const { client } = deps;

  const getCordium = (): Cordium => client.requireCordium();

  const approve = async (
    toolCallId: string,
    risk: APIRisk,
    title: string,
    preview: { workspaces?: string[]; workspace?: string; command?: string },
    signal?: AbortSignal,
  ): Promise<string | undefined> => {
    if (!needsApproval(risk, deps.settings.get().approvals.api)) {
      return undefined;
    }
    const ctx = requireRunContext(deps);
    const decision = await ctx.requestApproval(
      { toolCallId, title, risk, preview },
      signal,
    );
    if (decision.approved) {
      return undefined;
    }
    return `The User did not approve this action${decision.reason ? `. Reason: ${decision.reason}` : ""}. Do not retry it unless the User asks you to.`;
  };

  const list = defineTool({
    name: "workspace_list",
    label: "List Workspaces",
    description:
      "List the User's Cordium Workspaces with their state, Space, Template, Region, image, repository and hostname. Filters are applied client-side.",
    parameters: Type.Object({
      space: Type.Optional(
        Type.String({
          description:
            'Only the Workspaces of this Space, e.g. "my-space" (a personal Space) or "team.cordium" (an organization Space)',
        }),
      ),
      template: Type.Optional(
        Type.String({
          description:
            'Only the Workspaces of this Template, e.g. "default.my-space"',
        }),
      ),
      state: Type.Optional(
        Type.Enum(["all", "running", "stopped", "transitioning"], {
          description: "Filter by state (default all)",
        }),
      ),
      query: Type.Optional(
        Type.String({
          description:
            "Only the Workspaces whose name or display name contains this text",
        }),
      ),
      limit: Type.Optional(
        Type.Integer({
          minimum: 1,
          maximum: 500,
          description: "Maximum number of Workspaces to return (default 100)",
        }),
      ),
    }),
    async execute(_toolCallId, params, signal) {
      const limit = params.limit ?? 100;
      const items: WorkspaceSummary[] = [];
      let total = 0;
      try {
        for await (const ws of getCordium().workspaces.all(
          { pageSize: 100, orderBy: "createdAt", order: "desc" },
          { signal },
        )) {
          const summary = summarizeWorkspace(ws.toProto(), deps.agentWorkspace);
          if (params.space && !matchesName(summary.space, params.space)) {
            continue;
          }
          if (
            params.template &&
            !matchesName(summary.template, params.template)
          ) {
            continue;
          }
          if (params.state && params.state !== "all") {
            const state = ws.state;
            if (params.state === "running" && state !== S.RUNNING) {
              continue;
            }
            if (params.state === "stopped" && state !== S.STOPPED) {
              continue;
            }
            if (params.state === "transitioning" && isStable(state)) {
              continue;
            }
          }
          if (params.query) {
            const q = params.query.toLowerCase();
            if (
              !summary.name.toLowerCase().includes(q) &&
              !(summary.displayName ?? "").toLowerCase().includes(q)
            ) {
              continue;
            }
          }
          total++;
          if (items.length < limit) {
            items.push(summary);
          }
          if (total >= 1000) {
            break;
          }
        }
      } catch (err) {
        return errorResult(
          `Could not list the Workspaces: ${describeError(err)}`,
        );
      }

      const details: WorkspacesDetails = {
        kind: "workspaces",
        action: "list",
        items,
      };
      if (items.length === 0) {
        return textResult("No matching Workspaces found.", details);
      }
      return textResult(
        [
          `${total} matching Workspace(s)${total > items.length ? `, ${items.length} shown` : ""}:`,
          ...items.map(formatSummary),
        ].join("\n"),
        details,
      );
    },
  });

  const createAndStart = async (
    options: WorkspaceOptions,
    params: { start?: boolean; wait?: boolean; timeoutSeconds?: number },
    progress: ProgressReporter,
    signal?: AbortSignal,
  ): Promise<AgentToolResult<ToolDetails | undefined>> => {
    let ws: Workspace;
    try {
      ws = await getCordium().workspaces.create(options, { signal });
    } catch (err) {
      return errorResult(
        `Could not create the Workspace: ${describeError(err)}`,
      );
    }
    progress.set(ws.name, "created");

    const details: WorkspacesDetails = {
      kind: "workspaces",
      action: "create",
      items: [summarizeWorkspace(ws.toProto(), deps.agentWorkspace)],
    };

    if (params.start === false) {
      return textResult(
        `Created the Workspace ${ws.name} (STOPPED).\n${formatSummary(details.items[0])}`,
        details,
      );
    }

    try {
      await ws.start({}, { signal });
    } catch (err) {
      return errorResult(
        `Created the Workspace ${ws.name} but could not start it: ${describeError(err)}`,
        details,
      );
    }
    progress.set(ws.name, stateName(ws.state));

    if (params.wait === false) {
      details.items = [summarizeWorkspace(ws.toProto(), deps.agentWorkspace)];
      return textResult(
        `Created and started the Workspace ${ws.name}. It is now ${stateName(ws.state)}; use workspace_control with action "wait" to wait until it is RUNNING.`,
        details,
      );
    }

    const result = await waitForState(ws, "running", {
      deadline: Date.now() + (params.timeoutSeconds ?? 900) * 1000,
      signal,
      onState: (w) => progress.set(w.name, stateName(w.state)),
      pollIntervalMs: deps.pollIntervalMs,
    });
    const summary = summarizeWorkspace(ws.toProto(), deps.agentWorkspace);
    details.items = [summary];

    if (result.failure) {
      return errorResult(
        `The Workspace ${ws.name} was created but its start failed: ${result.failure}\nIt is preserved for inspection; delete it with workspace_control if it is not needed anymore.`,
        details,
      );
    }
    if (!result.reached) {
      return textResult(
        `The Workspace ${ws.name} is still ${summary.state}. Use workspace_control with action "wait" to keep waiting.\n${formatSummary(summary)}`,
        details,
      );
    }
    return textResult(
      `The Workspace ${ws.name} is RUNNING.\n${formatSummary(summary)}${summary.repository ? "\nThe repository is cloned into /workspace/repo." : ""}`,
      details,
    );
  };

  const create = defineTool({
    name: "workspace_create",
    label: "Create Workspace",
    description:
      'Create a new Cordium Workspace (sandbox) owned by the User and, by default, start it and wait until it is RUNNING. The Workspace is created from a Template: either the given one, the default Template of the given Space, or the User\'s default Template. The Cluster assigns its name. The primary repository is cloned into /workspace/repo. Use the spec field (Workspace.Spec in proto3 JSON, see cordium_api_describe with {"type": "cordium.v1.Workspace.Spec"}) for advanced settings such as tasks, volumes, additional repositories, network rules or capabilities.',
    parameters: Type.Object({
      displayName: Type.Optional(Type.String()),
      space: Type.Optional(
        Type.String({
          description:
            'The Space, e.g. "my-space" (personal) or "team.cordium" (organization). Its default Template is used unless template is set',
        }),
      ),
      template: Type.Optional(
        Type.String({
          description:
            'The Template, e.g. "python.my-space", or just "python" together with space',
        }),
      ),
      snapshot: Type.Optional(
        Type.String({
          description: "A WorkspaceSnapshot whose storage is restored",
        }),
      ),
      image: Type.Optional(
        Type.String({
          description:
            'A container image, e.g. "debian:latest", "node:24" or "python:3.13"',
        }),
      ),
      dockerfile: Type.Optional(
        Type.String({
          description: "An inline Dockerfile to build the image from",
        }),
      ),
      repository: Type.Optional(
        Type.Object({
          url: Type.String({ description: "The HTTPS URL of the repository" }),
          branch: Type.Optional(Type.String()),
          depth: Type.Optional(Type.Integer({ minimum: 0 })),
          checkout: Type.Optional(
            Type.String({ description: "A commit, tag or ref to check out" }),
          ),
        }),
      ),
      env: Type.Optional(
        Type.Record(Type.String(), Type.String(), {
          description: "Environment variables",
        }),
      ),
      vars: Type.Optional(
        Type.Record(Type.String(), Type.String(), {
          description: "Variables substituted in the spec",
        }),
      ),
      resources: Type.Optional(
        Type.Object({
          cpu: Type.Optional(
            Type.Integer({ minimum: 1, description: "CPU millicores" }),
          ),
          memory: Type.Optional(
            Type.Integer({ minimum: 1, description: "Memory in megabytes" }),
          ),
          storage: Type.Optional(
            Type.Integer({ minimum: 1, description: "Storage in megabytes" }),
          ),
        }),
      ),
      ephemeral: Type.Optional(
        Type.Boolean({
          description: "Discard the storage every time the Workspace stops",
        }),
      ),
      autoStop: Type.Optional(
        Type.Boolean({
          description:
            "Stop the Workspace once its foreground lifecycle tasks are done",
        }),
      ),
      applications: Type.Optional(
        Type.Array(
          Type.Object({
            name: Type.String(),
            port: Type.Integer({ minimum: 1, maximum: 65535 }),
            default: Type.Optional(Type.Boolean()),
          }),
          {
            description:
              "Named ports served at https://<application>_<workspace>.cordium.<domain> (the default one at https://<workspace>.cordium.<domain>)",
          },
        ),
      ),
      spec: Type.Optional(
        Type.Object(
          {},
          {
            additionalProperties: true,
            description:
              "Advanced: a Workspace.Spec in proto3 JSON. The other fields override it",
          },
        ),
      ),
      start: Type.Optional(
        Type.Boolean({ description: "Start the Workspace (default true)" }),
      ),
      wait: Type.Optional(
        Type.Boolean({
          description: "Wait until the Workspace is RUNNING (default true)",
        }),
      ),
      timeoutSeconds: Type.Optional(
        Type.Integer({
          minimum: 10,
          maximum: 3600,
          description:
            "How long to wait for the Workspace to be RUNNING (default 900)",
        }),
      ),
    }),
    async execute(toolCallId, params, signal, onUpdate) {
      const denied = await approve(
        toolCallId,
        "write",
        "Create a Workspace",
        {},
        signal,
      );
      if (denied) {
        return errorResult(denied);
      }

      let options: WorkspaceOptions;
      try {
        options = toWorkspaceOptions(params);
      } catch (err) {
        return errorResult(
          `Invalid Workspace configuration: ${(err as Error).message}`,
        );
      }

      const progress = new ProgressReporter(onUpdate as OnUpdate);
      try {
        return await createAndStart(options, params, progress, signal);
      } finally {
        progress.close();
      }
    },
  });

  const control = defineTool({
    name: "workspace_control",
    label: "Control Workspaces",
    description:
      "Start, stop, restart or delete one or more Workspaces in parallel, or wait until they reach a stable state (RUNNING or STOPPED). Deleting a Workspace deletes its storage and cannot be undone. The agent's own Workspace cannot be stopped, restarted or deleted.",
    parameters: Type.Object({
      action: Type.Enum(["start", "stop", "restart", "delete", "wait"]),
      workspaces: Type.Array(workspaceName, { minItems: 1, maxItems: 20 }),
      wait: Type.Optional(
        Type.Boolean({
          description:
            "Wait until the action completes, i.e. RUNNING after start/restart and STOPPED after stop (default true)",
        }),
      ),
      timeoutSeconds: Type.Optional(
        Type.Integer({
          minimum: 10,
          maximum: 3600,
          description: "How long to wait (default 900)",
        }),
      ),
    }),
    async execute(toolCallId, params, signal, onUpdate) {
      const names = [...new Set(params.workspaces)];
      const action = params.action;
      if (
        action !== "start" &&
        action !== "wait" &&
        deps.agentWorkspace &&
        names.includes(deps.agentWorkspace)
      ) {
        return errorResult(
          `${deps.agentWorkspace} is the agent's own Workspace and it cannot be ${action === "stop" ? "stopped" : action === "restart" ? "restarted" : "deleted"} by the agent. Ask the User to do it from the web portal instead.`,
        );
      }

      if (action !== "wait") {
        const denied = await approve(
          toolCallId,
          action === "delete" ? "destructive" : "write",
          `${action[0].toUpperCase()}${action.slice(1)} ${names.length === 1 ? `the Workspace ${names[0]}` : `${names.length} Workspaces`}`,
          { workspaces: names },
          signal,
        );
        if (denied) {
          return errorResult(denied);
        }
      }

      const progress = new ProgressReporter(onUpdate as OnUpdate);
      const deadline = Date.now() + (params.timeoutSeconds ?? 900) * 1000;
      const shouldWait = params.wait !== false;
      const cordium = getCordium();

      const run = async (
        name: string,
      ): Promise<{ summary?: WorkspaceSummary; message: string }> => {
        const onState = (w: Workspace) =>
          progress.set(name, stateName(w.state));
        const ws = await cordium.workspaces.get(name, { signal });
        onState(ws);

        if (action === "delete") {
          await ws.delete({ signal });
          progress.set(name, "deleted");
          return {
            summary: {
              ...summarizeWorkspace(ws.toProto(), deps.agentWorkspace),
              state: "DELETED",
            },
            message: `${name}: deleted`,
          };
        }

        if (action === "stop" || action === "restart") {
          if (ws.state !== S.STOPPED) {
            if (!isStopping(ws.state)) {
              await ws.stop({ signal });
            }
            if (shouldWait || action === "restart") {
              const res = await waitForState(ws, "stopped", {
                deadline,
                signal,
                onState,
                pollIntervalMs: deps.pollIntervalMs,
              });
              if (!res.reached) {
                return {
                  summary: summarizeWorkspace(ws.toProto()),
                  message: `${name}: still ${stateName(ws.state)} (timed out)`,
                };
              }
            }
          }
          if (action === "stop") {
            return {
              summary: summarizeWorkspace(ws.toProto(), deps.agentWorkspace),
              message: `${name}: ${stateName(ws.state)}`,
            };
          }
        }

        if (action === "start" || action === "restart") {
          await ws.start({}, { signal });
          onState(ws);
          if (!shouldWait) {
            return {
              summary: summarizeWorkspace(ws.toProto(), deps.agentWorkspace),
              message: `${name}: ${stateName(ws.state)}`,
            };
          }
          const res = await waitForState(ws, "running", {
            deadline,
            signal,
            onState,
            pollIntervalMs: deps.pollIntervalMs,
          });
          const summary = summarizeWorkspace(ws.toProto(), deps.agentWorkspace);
          if (res.failure) {
            throw new Error(`the start failed: ${res.failure}`);
          }
          return {
            summary,
            message: `${name}: ${res.reached ? "RUNNING" : `still ${summary.state} (timed out)`}`,
          };
        }

        const res = await waitForState(ws, "stable", {
          deadline,
          signal,
          onState,
          pollIntervalMs: deps.pollIntervalMs,
        });
        const summary = summarizeWorkspace(ws.toProto(), deps.agentWorkspace);
        return {
          summary,
          message: `${name}: ${res.reached ? summary.state : `still ${summary.state} (timed out)`}${summary.failure ? ` (last run failed: ${summary.failure})` : ""}`,
        };
      };

      const results = await Promise.allSettled(names.map((n) => run(n)));
      progress.close();
      const details: WorkspacesDetails = {
        kind: "workspaces",
        action: action === "wait" ? undefined : action,
        items: [],
        errors: [],
      };
      const lines: string[] = [];
      results.forEach((res, i) => {
        if (res.status === "fulfilled") {
          lines.push(res.value.message);
          if (res.value.summary) {
            details.items.push(res.value.summary);
          }
        } else {
          const message = describeError(res.reason);
          lines.push(`${names[i]}: failed: ${message}`);
          details.errors!.push({ workspace: names[i], message });
        }
      });
      if (details.errors!.length === 0) {
        delete details.errors;
      }

      return textResult(
        lines.join("\n"),
        details,
        results.every((r) => r.status === "rejected"),
      );
    },
  });

  const exec = defineTool({
    name: "workspace_exec",
    label: "Run in Workspace",
    description:
      "Run a shell command inside another running Workspace (via the Cordium API, no SSH needed) and return its exit code and output. The command is interpreted by the remote POSIX shell (pipes and redirections work) and runs as the Workspace user unless root is set. Stdin is closed. Long outputs are truncated and saved to a log file in the agent's Workspace. For commands that run for a long time (e.g. full test suites), set background to true and poll the returned log file. Use the bash tool instead for the agent's own Workspace.",
    parameters: Type.Object({
      workspace: workspaceName,
      command: Type.String({ description: "The shell command to run" }),
      cwd: Type.Optional(
        Type.String({
          description:
            "The working directory, e.g. /workspace/repo (the default is the Workspace user's home)",
        }),
      ),
      env: Type.Optional(Type.Record(Type.String(), Type.String())),
      root: Type.Optional(Type.Boolean({ description: "Run as root" })),
      timeoutSeconds: Type.Optional(
        Type.Integer({
          minimum: 1,
          maximum: 14400,
          description:
            "Kill the command after this many seconds (default 1200)",
        }),
      ),
      background: Type.Optional(
        Type.Boolean({
          description:
            "Start the command detached and return immediately with its PID and log file (default false)",
        }),
      ),
    }),
    async execute(toolCallId, params, signal, onUpdate) {
      if (deps.settings.get().approvals.commands) {
        const ctx = requireRunContext(deps);
        const decision = await ctx.requestApproval(
          {
            toolCallId,
            title: `Run a command in the Workspace ${params.workspace}`,
            risk: "write",
            preview: { command: params.command, workspace: params.workspace },
          },
          signal,
        );
        if (!decision.approved) {
          return errorResult(
            `The User did not approve running this command${decision.reason ? `. Reason: ${decision.reason}` : ""}. Do not retry it unless the User asks you to.`,
          );
        }
      }

      const ctx = requireRunContext(deps);
      const details: CommandDetails = {
        kind: "command",
        command: params.command,
        workspace: params.workspace,
        cwd: params.cwd,
      };

      let ws: Workspace;
      try {
        ws = await getCordium().workspaces.get(params.workspace, { signal });
      } catch (err) {
        return errorResult(
          `Could not get the Workspace ${params.workspace}: ${describeError(err)}`,
          details,
        );
      }
      if (!ws.isReady) {
        return errorResult(
          `The Workspace ${ws.name} is ${stateName(ws.state)}. Start it first with workspace_control (action "start") and retry.`,
          details,
        );
      }

      const id = `${new Date().toISOString().replace(/[:.]/g, "-")}-${ws.name}-${crypto.randomBytes(3).toString("hex")}`;
      const background = params.background === true;
      const command = background
        ? backgroundCommand(params.command, id)
        : params.command;
      const timeoutMs =
        (background ? 60 : (params.timeoutSeconds ?? 1200)) * 1000;
      const collector = new OutputCollector(
        path.join(ctx.resultsDir, "exec", `${id}.log`),
      );
      const started = Date.now();
      let timedOut = false;
      let lastUpdate = 0;

      const session = ws.execStream(command, {
        cwd: params.cwd,
        env: params.env,
        root: params.root,
        interactive: false,
        maxCaptureBytes: 64 * 1024,
        maxBufferBytes: 32 * 1024 * 1024,
        timeoutMs: 0,
        signal,
      });
      const timer = setTimeout(() => {
        timedOut = true;
        void session.kill().catch(() => session.close());
      }, timeoutMs);

      let exitCode: number | undefined;
      try {
        for await (const chunk of session) {
          collector.write(chunk.stream, chunk.data);
          const now = Date.now();
          if (onUpdate && now - lastUpdate > 500) {
            lastUpdate = now;
            (onUpdate as OnUpdate)({
              content: [{ type: "text", text: collector.preview() }],
              details: undefined,
            });
          }
        }
        exitCode = (await session.wait()).exitCode;
      } catch (err) {
        if (!timedOut) {
          collector.close();
          return errorResult(
            `The command could not be run in ${ws.name}: ${describeError(err)}`,
            details,
          );
        }
      } finally {
        clearTimeout(timer);
        collector.close();
      }

      details.exitCode = exitCode;
      details.durationMs = Date.now() - started;
      if (timedOut) {
        details.timedOut = true;
      }
      const { text, truncated } = collector.text();
      if (truncated) {
        details.logPath = collector.logPath;
      }

      if (background) {
        const pid = text.trim().split("\n").at(-1) ?? "";
        const log = `/tmp/cordium-agent/${id}.log`;
        return textResult(
          `Started the command in the background in ${ws.name}${/^\d+$/.test(pid) ? ` (PID ${pid})` : ""}. Its output is written to ${log} and its exit code to ${log}.exit once it is done. Check its progress with workspace_exec, e.g. "tail -n 100 ${log}; cat ${log}.exit 2>/dev/null".`,
          details,
          exitCode !== 0,
        );
      }

      const header = timedOut
        ? `The command was killed after ${Math.round(timeoutMs / 1000)}s.`
        : `Exit code: ${exitCode}`;
      return textResult(
        `${header}\n${text === "" ? "(no output)" : text}`,
        details,
        timedOut || exitCode !== 0,
      );
    },
  });

  const logs = defineTool({
    name: "workspace_logs",
    label: "Workspace logs",
    description:
      "Read the initialization logs of a Workspace (repository cloning, image pulling/building and the output of its lifecycle tasks) and its last run failure, if any. Useful to troubleshoot a Workspace that failed to start or whose tasks misbehave. The logs are only available while the Workspace is PREPARING or RUNNING.",
    parameters: Type.Object({
      workspace: workspaceName,
      tail: Type.Optional(
        Type.Integer({
          minimum: 1,
          maximum: 2000,
          description: "Number of the last lines to return (default 200)",
        }),
      ),
      waitSeconds: Type.Optional(
        Type.Integer({
          minimum: 1,
          maximum: 60,
          description: "How long to collect the logs (default 3)",
        }),
      ),
    }),
    async execute(_toolCallId, params, signal) {
      let ws: Workspace;
      try {
        ws = await getCordium().workspaces.get(params.workspace, { signal });
      } catch (err) {
        return errorResult(
          `Could not get the Workspace ${params.workspace}: ${describeError(err)}`,
        );
      }
      const summary = summarizeWorkspace(ws.toProto(), deps.agentWorkspace);
      const failure = failureMessage(ws.toProto());
      const header = [
        formatSummary(summary),
        failure ? `Last run failure: ${failure}` : undefined,
      ]
        .filter(Boolean)
        .join("\n");

      if (!ws.isReady) {
        return textResult(
          `${header}\nThe initialization logs are not available since the Workspace is ${summary.state}.`,
          { kind: "workspaces", items: [summary] },
        );
      }

      const lines: string[] = [];
      const decoder = new TextDecoder();
      try {
        for await (const entry of ws.logs({
          signal,
          timeoutMs: (params.waitSeconds ?? 3) * 1000,
        })) {
          const text = decoder.decode(entry.data).replace(/\r/g, "");
          for (const line of text.split("\n")) {
            if (line.trim() !== "") {
              lines.push(`[${entry.stage}] ${line}`);
            }
          }
        }
      } catch (err) {
        const code = toAPIError(err).code;
        if (code !== "deadline_exceeded" && code !== "cancelled") {
          return errorResult(
            `${header}\nCould not read the logs: ${describeError(err)}`,
          );
        }
      }

      const tail = params.tail ?? 200;
      const shown = lines.slice(-tail);
      return textResult(
        `${header}\n${lines.length > shown.length ? `(last ${shown.length} of ${lines.length} lines)\n` : ""}${shown.length > 0 ? shown.join("\n") : "(no logs)"}`,
        { kind: "workspaces", items: [summary] },
      );
    },
  });

  const copy = createTransferTool({
    ...deps,
    approve: (toolCallId, title, signal) =>
      approve(toolCallId, "write", title, {}, signal),
  });

  return [list, create, control, exec, copy, logs];
};
