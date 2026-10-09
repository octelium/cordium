import {
  parseResourceURI,
  RESOURCE_URI_PREFIX,
  type APIRisk,
  type ApprovalMode,
  type Block,
  type ModelInfo,
  type ResourceRef,
  type ToolBlock,
} from "@/apis/agent/protocol";
import { getPathSpaceRef } from "@/utils/octelium";
import { getShortNameFromStr } from "@/utils/pb";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { ObjectReference } from "@octelium/apis/main/metav1";
import { defaultUrlTransform } from "react-markdown";

export const formatSize = (n: number): string => {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`;
};

export const formatDuration = (ms: number): string => {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
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
      return `/workspaces/${encodeURIComponent(name)}`;
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

export const toWorkspaceState = (state: string): WsPB.Workspace_Status_State =>
  WsPB.Workspace_Status_State[
    state as keyof typeof WsPB.Workspace_Status_State
  ] ?? WsPB.Workspace_Status_State.UNKNOWN;

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

export const copyText = async (text: string): Promise<boolean> => {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
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

export const groupBlocks = (
  blocks: Block[],
): (Block | { type: "tools"; id: string; blocks: ToolBlock[] })[] => {
  const ret: (Block | { type: "tools"; id: string; blocks: ToolBlock[] })[] =
    [];
  for (const block of blocks) {
    const last = ret.at(-1);
    if (block.type === "tool") {
      if (last && last.type === "tools") {
        last.blocks.push(block);
      } else {
        ret.push({ type: "tools", id: block.id, blocks: [block] });
      }
    } else {
      ret.push(block);
    }
  }
  return ret;
};
