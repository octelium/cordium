import {
  parseResourceURI,
  RESOURCE_URI_PREFIX,
  type APIRisk,
  type ApprovalMode,
  type Conversation,
  type Message,
  type ModelInfo,
  type ResourceRef,
  type TableBlock,
  type ToolBlock,
} from "@/apis/agent/protocol";
import { getPathSpaceRef } from "@/utils/octelium";
import { getShortNameFromStr } from "@/utils/pb";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { ObjectReference } from "@octelium/apis/main/metav1";
import * as React from "react";
import { toast } from "react-hot-toast";
import { defaultUrlTransform } from "react-markdown";

export const isMac =
  typeof navigator !== "undefined" &&
  /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);

export const modKey = isMac ? "⌘" : "Ctrl";

export const formatBytes = (n: number): string => {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`;
};

const spaceOf = (name: string): string => name.split(".").slice(1).join(".");

export const getResourceRoute = (ref: ResourceRef): string | undefined => {
  const name = ref.name;
  if (!name) {
    return undefined;
  }

  if (ref.apiVersion === "user/v1") {
    return ref.kind === "Service" ? "/services" : undefined;
  }
  if (ref.apiVersion !== "cordium/v1") {
    return undefined;
  }

  const spacePath = (spaceName: string) =>
    spaceName.includes(".")
      ? getPathSpaceRef(ObjectReference.create({ name: spaceName }))
      : undefined;

  switch (ref.kind) {
    case "Workspace":
      return getWorkspacePath(name);
    case "Space":
      return spacePath(name);
    case "Template": {
      const space = spacePath(spaceOf(name));
      return space
        ? `${space}/templates/${encodeURIComponent(getShortNameFromStr(name))}`
        : undefined;
    }
    case "Volume":
    case "Secret":
    case "Membership": {
      const space = spacePath(spaceOf(name));
      return space ? `${space}/${ref.kind.toLowerCase()}s` : undefined;
    }
    case "GitProvider": {
      const space = spacePath(spaceOf(name));
      return space ? `${space}/gitproviders` : undefined;
    }
    case "WorkspaceSnapshot":
      return "/snapshots";
    case "UserSecret":
      return "/usersecrets";
    case "UserConfig":
      return "/settings";
    default:
      return undefined;
  }
};

export const getResourceURIRoute = (uri: string): string | undefined => {
  const ref = parseResourceURI(uri);
  return ref ? getResourceRoute(ref) : undefined;
};

export const urlTransform = (url: string): string =>
  url.startsWith(RESOURCE_URI_PREFIX) ? url : defaultUrlTransform(url);

export const riskTone = (
  risk: APIRisk,
): "success" | "info" | "danger" | "warning" => {
  switch (risk) {
    case "read":
      return "success";
    case "write":
      return "info";
    case "destructive":
      return "danger";
    case "sensitive":
      return "warning";
  }
};

export const riskLabel: Record<APIRisk, string> = {
  read: "Read-only",
  write: "Change",
  destructive: "Destructive",
  sensitive: "Sensitive",
};

export const getWorkspacePath = (name: string, page = ""): string =>
  `/workspaces/${encodeURIComponent(name)}${page ? `/${page}` : ""}`;

export const getWorkspaceDisplayName = (ws?: WsPB.Workspace): string =>
  ws?.metadata?.displayName || ws?.metadata?.name || "Workspace";

export const toWorkspaceState = (state: string): WsPB.Workspace_Status_State =>
  WsPB.Workspace_Status_State[
    state as keyof typeof WsPB.Workspace_Status_State
  ] ?? WsPB.Workspace_Status_State.UNKNOWN;

export const workspaceStateLabel = (
  state: WsPB.Workspace_Status_State,
): string => {
  switch (state) {
    case WsPB.Workspace_Status_State.INIT_REQUEST:
      return "Requested";
    case WsPB.Workspace_Status_State.INITIALIZING:
      return "Initializing";
    case WsPB.Workspace_Status_State.PULLING_IMAGE:
      return "Pulling the image";
    case WsPB.Workspace_Status_State.BUILDING_IMAGE:
      return "Building the image";
    case WsPB.Workspace_Status_State.STARTING_RUNTIME:
      return "Starting the runtime";
    case WsPB.Workspace_Status_State.PREPARING:
      return "Running the setup tasks";
    case WsPB.Workspace_Status_State.RUNNING:
      return "Running";
    case WsPB.Workspace_Status_State.STOPPING_REQUEST:
    case WsPB.Workspace_Status_State.STOPPING:
      return "Stopping";
    case WsPB.Workspace_Status_State.STOPPED:
      return "Stopped";
    default:
      return "Unknown";
  }
};

export const isWorkspaceStarting = (ws?: WsPB.Workspace): boolean => {
  switch (ws?.status?.state) {
    case WsPB.Workspace_Status_State.INIT_REQUEST:
    case WsPB.Workspace_Status_State.INITIALIZING:
    case WsPB.Workspace_Status_State.PULLING_IMAGE:
    case WsPB.Workspace_Status_State.BUILDING_IMAGE:
    case WsPB.Workspace_Status_State.STARTING_RUNTIME:
    case WsPB.Workspace_Status_State.PREPARING:
      return true;
    default:
      return false;
  }
};

export const isWorkspaceStopping = (ws?: WsPB.Workspace): boolean =>
  ws?.status?.state === WsPB.Workspace_Status_State.STOPPING_REQUEST ||
  ws?.status?.state === WsPB.Workspace_Status_State.STOPPING;

export const getWorkspaceFailure = (
  ws?: WsPB.Workspace,
): string | undefined => {
  const failure = ws?.status?.run ? ws.status.run.failure : ws?.status?.failure;
  return failure
    ? failure.message || "The Workspace failed to start"
    : undefined;
};

export const formatValue = (value: unknown): string => {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
};

export const toJSON = (value: unknown): string => {
  if (value === undefined) return "";
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2);
};

export const saveBlob = (blob: Blob, name: string) => {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 10000);
};

export const toCSV = (
  columns: { key: string; label?: string }[],
  rows: Record<string, unknown>[],
): string => {
  const escape = (value: string) =>
    /[",\n\r]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;

  return [
    columns.map((c) => escape(c.label ?? c.key)).join(","),
    ...rows.map((row) =>
      columns.map((c) => escape(formatValue(row[c.key]))).join(","),
    ),
  ].join("\n");
};

export const toFileName = (value: string, ext: string): string =>
  `${
    value
      .trim()
      .replace(/[^\w.-]+/g, "_")
      .slice(0, 80) || "conversation"
  }.${ext}`;

export const copyText = async (value: string): Promise<boolean> => {
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    toast.error("Could not copy to the clipboard");
    return false;
  }
};

export const readStorage = (key: string): string | undefined => {
  try {
    return localStorage.getItem(key) ?? undefined;
  } catch {
    return undefined;
  }
};

export const writeStorage = (key: string, value: string) => {
  try {
    localStorage.setItem(key, value);
  } catch {
    return;
  }
};

export const searchTerms = (query: string): string[] => [
  ...new Set(
    query
      .toLowerCase()
      .split(/\s+/)
      .filter((term) => term !== ""),
  ),
];

export const matchesTerms = (text: string, terms: string[]): boolean => {
  const lower = text.toLowerCase();
  return terms.every((term) => lower.includes(term));
};

export interface ConversationGroup {
  label: string;
  items: Conversation[];
}

const dayMs = 24 * 60 * 60 * 1000;

export const conversationGroupLabel = (date: Date, now: Date): string => {
  const today = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
  ).getTime();
  const time = date.getTime();
  if (time >= today) return "Today";
  if (time >= today - dayMs) return "Yesterday";
  if (time >= today - 7 * dayMs) return "Previous 7 days";
  if (time >= today - 30 * dayMs) return "Previous 30 days";
  return date.toLocaleDateString(undefined, { month: "long", year: "numeric" });
};

export const groupConversations = (
  conversations: Conversation[],
  now = new Date(),
): ConversationGroup[] => {
  const ret: ConversationGroup[] = [];
  for (const conversation of conversations) {
    const label = conversationGroupLabel(new Date(conversation.updatedAt), now);
    const last = ret.at(-1);
    if (last?.label === label) {
      last.items.push(conversation);
    } else {
      ret.push({ label, items: [conversation] });
    }
  }
  return ret;
};

export const messageText = (message: Message): string =>
  message.blocks
    .map((block) => (block.type === "markdown" ? block.text.trim() : ""))
    .filter((text) => text !== "")
    .join("\n\n");

const tableMarkdown = (block: TableBlock): string => {
  const cell = (value: string) =>
    value.replace(/\|/g, "\\|").replace(/\s*\n\s*/g, " ");
  return [
    `| ${block.columns.map((c) => cell(c.label ?? c.key)).join(" | ")} |`,
    `| ${block.columns.map(() => "---").join(" | ")} |`,
    ...block.rows.map(
      (row) =>
        `| ${block.columns.map((c) => cell(formatValue(row[c.key]))).join(" | ")} |`,
    ),
  ].join("\n");
};

export const conversationMarkdown = (
  title: string,
  messages: Message[],
): string => {
  const ret: string[] = [`# ${title}`];

  for (const message of messages) {
    ret.push(message.role === "user" ? "## You" : "## Agent");
    if (message.attachments && message.attachments.length > 0) {
      ret.push(
        `Attachments: ${message.attachments.map((a) => a.name).join(", ")}`,
      );
    }
    for (const block of message.blocks) {
      switch (block.type) {
        case "markdown":
          if (block.text.trim() !== "") ret.push(block.text.trim());
          break;
        case "tool":
          ret.push(`- ${block.title || block.name} (${block.status})`);
          break;
        case "approval":
          ret.push(`> Approval ${block.status}: ${block.title}`);
          break;
        case "table":
          if (block.title) ret.push(`**${block.title}**`);
          ret.push(tableMarkdown(block));
          break;
        case "chart":
          ret.push(`_Chart: ${block.chart.title ?? block.chart.chartType}_`);
          break;
        case "resources":
          ret.push(
            block.resources
              .map((item) => `- ${item.ref.kind} ${item.ref.name ?? ""}`)
              .join("\n"),
          );
          break;
        case "artifact":
          ret.push(`_Artifact: ${block.artifact.name}_`);
          break;
        case "notice":
          ret.push(`> ${block.text}`);
          break;
        case "error":
          ret.push(`> Error: ${block.message}`);
          break;
      }
    }
  }

  return `${ret.join("\n\n")}\n`;
};

export const formatDuration = (ms: number): string => {
  if (!Number.isFinite(ms) || ms < 0) return "";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 10000) return `${(ms / 1000).toFixed(1)}s`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
};

export const formatElapsed = (ms: number): string => {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  if (seconds < 60) return `${seconds}s`;
  return `${Math.floor(seconds / 60)}m ${String(seconds % 60).padStart(2, "0")}s`;
};

export const formatCount = (n: number): string => {
  if (n < 1000) return String(n);
  if (n < 1000000) return `${(n / 1000).toFixed(n < 10000 ? 1 : 0)}k`;
  return `${(n / 1000000).toFixed(1)}M`;
};

export const toolDuration = (block: ToolBlock): number | undefined =>
  block.startedAt && block.completedAt
    ? new Date(block.completedAt).getTime() -
      new Date(block.startedAt).getTime()
    : undefined;

export const greeting = (date = new Date()): string => {
  const hour = date.getHours();
  if (hour >= 5 && hour < 12) return "Good morning";
  if (hour >= 12 && hour < 18) return "Good afternoon";
  return "Good evening";
};

export const modelLabel = (model?: ModelInfo) =>
  model ? (model.name ?? model.id) : "No model";

export const approvalModes: {
  value: ApprovalMode;
  label: string;
  description: string;
}[] = [
  {
    value: "destructive",
    label: "Risky only",
    description:
      "Ask before deletions and sensitive changes (Secrets, Memberships, shared ports)",
  },
  {
    value: "write",
    label: "All changes",
    description: "Ask before every change, including creating and starting",
  },
  {
    value: "never",
    label: "Never",
    description: "Never ask. The agent acts on its own within your permissions",
  },
];

export type FileKind =
  | "image"
  | "spreadsheet"
  | "archive"
  | "code"
  | "text"
  | "file";

export const fileKind = (mimeType: string, name = ""): FileKind => {
  if (mimeType.startsWith("image/")) return "image";
  if (mimeType === "text/csv" || /\.(csv|tsv|xlsx?)$/i.test(name)) {
    return "spreadsheet";
  }
  if (/(zip|tar|gzip|compressed)/.test(mimeType)) return "archive";
  if (
    /(json|yaml|xml|javascript|typescript)/.test(mimeType) ||
    /\.(json|ya?ml|ts|js|go|py|sh|toml)$/i.test(name)
  ) {
    return "code";
  }
  if (mimeType.startsWith("text/") || mimeType === "application/pdf") {
    return "text";
  }
  return "file";
};

export const useNow = (active: boolean, intervalMs = 1000): number => {
  const [now, setNow] = React.useState(() => Date.now());

  React.useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(timer);
  }, [active, intervalMs]);

  return now;
};
