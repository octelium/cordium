export const PROTOCOL_VERSION = 1;

export const API_PREFIX = "/v1";

export const RESOURCE_URI_PREFIX = "octelium://resource/";

export interface ResourceRef {
  apiVersion: string;
  kind: string;
  name?: string;
  uid?: string;
}

export interface ErrorInfo {
  code?: string;
  message: string;
}

export interface Usage {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  cost: number;
}

export interface ModelInfo {
  provider: string;
  id: string;
  name?: string;
  api?: string;
  reasoning?: boolean;
  input?: ("text" | "image")[];
  contextWindow?: number;
}

export interface Conversation {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
  activeRunId?: string;
}

export type MessageRole = "user" | "assistant";

export type MessageStatus =
  "streaming" | "completed" | "failed" | "cancelled" | "interrupted";

export interface Message {
  id: string;
  conversationId: string;
  role: MessageRole;
  status: MessageStatus;
  createdAt: string;
  completedAt?: string;
  runId?: string;
  blocks: Block[];
  attachments?: FileInfo[];
  model?: ModelInfo;
  usage?: Usage;
  error?: ErrorInfo;
}

export type Block =
  | MarkdownBlock
  | ThinkingBlock
  | ToolBlock
  | ApprovalBlock
  | TableBlock
  | ChartBlock
  | ResourcesBlock
  | ArtifactBlock
  | NoticeBlock
  | ErrorBlock;

export type BlockType = Block["type"];

export const BLOCK_TYPES: BlockType[] = [
  "markdown",
  "thinking",
  "tool",
  "approval",
  "table",
  "chart",
  "resources",
  "artifact",
  "notice",
  "error",
];

export interface BlockBase {
  id: string;
  createdAt: string;
}

export interface MarkdownBlock extends BlockBase {
  type: "markdown";
  text: string;
}

export interface ThinkingBlock extends BlockBase {
  type: "thinking";
  text: string;
  redacted?: boolean;
}

export type ToolStatus =
  | "pending"
  | "running"
  | "awaiting_approval"
  | "completed"
  | "failed"
  | "rejected"
  | "cancelled";

export interface ToolBlock extends BlockBase {
  type: "tool";
  toolCallId: string;
  name: string;
  title: string;
  status: ToolStatus;
  input?: unknown;
  output?: string;
  outputTruncated?: boolean;
  details?: ToolDetails;
  startedAt?: string;
  completedAt?: string;
}

export type ToolDetails =
  | APICallDetails
  | CommandDetails
  | FileDetails
  | WorkspacesDetails
  | TransferDetails
  | GenericToolDetails;

export type APIRisk = "read" | "write" | "destructive" | "sensitive";

export interface APICallDetails {
  kind: "api";
  method: string;
  grpcMethod: string;
  risk: APIRisk;
  request?: unknown;
  response?: unknown;
  responseTruncated?: boolean;
  resultPath?: string;
  resources?: ResourceRef[];
  error?: ErrorInfo;
}

export interface CommandDetails {
  kind: "command";
  command: string;
  workspace?: string;
  cwd?: string;
  exitCode?: number;
  durationMs?: number;
  timedOut?: boolean;
  logPath?: string;
}

export interface FileDetails {
  kind: "file";
  path: string;
  operation: "read" | "write" | "edit";
  diff?: string;
}

export interface WorkspaceSummary {
  name: string;
  uid?: string;
  displayName?: string;
  state: string;
  space?: string;
  template?: string;
  region?: string;
  hostname?: string;
  image?: string;
  repository?: string;
  isEphemeral?: boolean;
  isAgent?: boolean;
  failure?: string;
  createdAt?: string;
}

export interface WorkspacesDetails {
  kind: "workspaces";
  action?: "list" | "create" | "start" | "stop" | "restart" | "delete";
  items: WorkspaceSummary[];
  errors?: { workspace: string; message: string }[];
}

export interface TransferDetails {
  kind: "transfer";
  source: string;
  destination: string;
  type: "file" | "directory";
  bytes: number;
}

export interface GenericToolDetails {
  kind: "generic";
  data: unknown;
}

export type ApprovalStatus = "pending" | "approved" | "rejected" | "cancelled";

export interface ApprovalBlock extends BlockBase {
  type: "approval";
  approvalId: string;
  toolCallId: string;
  title: string;
  description?: string;
  risk: APIRisk;
  status: ApprovalStatus;
  preview?: ApprovalPreview;
  reason?: string;
  decidedAt?: string;
}

export interface ApprovalPreview {
  method?: string;
  request?: unknown;
  command?: string;
  workspace?: string;
  workspaces?: string[];
}

export type TableColumnType =
  "string" | "number" | "boolean" | "datetime" | "json";

export interface TableColumn {
  key: string;
  label?: string;
  type?: TableColumnType;
  unit?: string;
}

export interface TableBlock extends BlockBase {
  type: "table";
  title?: string;
  caption?: string;
  columns: TableColumn[];
  rows: Record<string, unknown>[];
  totalRows: number;
  truncated?: boolean;
}

export type ChartType = "line" | "area" | "bar" | "pie" | "scatter";

export type ChartAxisType = "time" | "category" | "number";

export interface ChartSeries {
  key: string;
  label?: string;
  unit?: string;
}

export interface ChartSpec {
  chartType: ChartType;
  title?: string;
  description?: string;
  x: {
    key: string;
    label?: string;
    type?: ChartAxisType;
  };
  y?: {
    label?: string;
    unit?: string;
  };
  series: ChartSeries[];
  rows: Record<string, string | number | null>[];
  stacked?: boolean;
}

export interface ChartBlock extends BlockBase {
  type: "chart";
  chart: ChartSpec;
}

export interface ResourceItem {
  ref: ResourceRef;
  uri: string;
  snapshot?: Record<string, unknown>;
}

export interface ResourcesBlock extends BlockBase {
  type: "resources";
  title?: string;
  resources: ResourceItem[];
}

export interface ArtifactInfo {
  id: string;
  name: string;
  mimeType: string;
  size: number;
  title?: string;
  description?: string;
  createdAt: string;
  url: string;
}

export interface ArtifactBlock extends BlockBase {
  type: "artifact";
  artifact: ArtifactInfo;
}

export interface NoticeBlock extends BlockBase {
  type: "notice";
  level: "info" | "warning";
  text: string;
}

export interface ErrorBlock extends BlockBase {
  type: "error";
  message: string;
  code?: string;
}

export type RunStatus =
  "running" | "awaiting_approval" | "completed" | "failed" | "cancelled";

export interface RunActivity {
  kind: "retrying" | "compacting";
  message: string;
}

export interface Run {
  id: string;
  conversationId: string;
  status: RunStatus;
  createdAt: string;
  completedAt?: string;
  userMessageId: string;
  assistantMessageId: string;
  lastSeq: number;
  activity?: RunActivity;
  usage?: Usage;
  error?: ErrorInfo;
}

export type AgentEventPayload =
  | { type: "run.started"; run: Run }
  | { type: "run.updated"; run: Run }
  | { type: "run.completed"; run: Run }
  | { type: "run.failed"; run: Run }
  | { type: "run.cancelled"; run: Run }
  | { type: "message.created"; message: Message }
  | { type: "message.completed"; message: Message }
  | { type: "block.created"; messageId: string; block: Block }
  | { type: "block.delta"; messageId: string; blockId: string; text: string }
  | { type: "block.updated"; messageId: string; block: Block }
  | { type: "conversation.updated"; conversation: Conversation };

export type AgentEventType = AgentEventPayload["type"];

export type AgentEvent = {
  seq: number;
  ts: string;
  conversationId: string;
  runId: string;
} & AgentEventPayload;

export const TERMINAL_RUN_EVENT_TYPES: AgentEventType[] = [
  "run.completed",
  "run.failed",
  "run.cancelled",
];

export const isTerminalRunStatus = (status: RunStatus): boolean =>
  status === "completed" || status === "failed" || status === "cancelled";

export interface FileInfo {
  id: string;
  name: string;
  mimeType: string;
  size: number;
  path: string;
  createdAt: string;
}

export interface RunInput {
  text: string;
  attachments?: string[];
}

export interface CreateConversationRequest {
  title?: string;
  input?: RunInput;
}

export interface CreateConversationResponse {
  conversation: Conversation;
  run?: Run;
}

export interface UpdateConversationRequest {
  title?: string;
}

export interface ListConversationsResponse {
  items: Conversation[];
}

export interface RunSnapshot {
  run: Run;
  message?: Message;
}

export interface ConversationDetail {
  conversation: Conversation;
  messages: Message[];
  activeRun?: RunSnapshot;
}

export interface CreateRunRequest {
  input: RunInput;
}

export type ApprovalDecision = "approve" | "reject";

export interface ApprovalDecisionRequest {
  decision: ApprovalDecision;
  reason?: string;
}

export interface ErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

export interface ModelsResponse {
  current?: ModelInfo;
  thinkingLevel?: string;
  models: ModelInfo[];
  error?: ErrorInfo;
}

export interface SetModelRequest {
  provider: string;
  id: string;
  thinkingLevel?: string;
}

export type ApprovalMode = "never" | "destructive" | "write";

export const APPROVAL_MODES: ApprovalMode[] = ["never", "destructive", "write"];

export interface ApprovalSettings {
  api: ApprovalMode;
  commands: boolean;
}

export interface AgentSettings {
  approvals: ApprovalSettings;
}

export interface UpdateSettingsRequest {
  approvals?: Partial<ApprovalSettings>;
}

export interface AuthProvider {
  id: string;
  name: string;
  loginLabel?: string;
  subscription: boolean;
  configured: boolean;
  oauth: boolean;
}

export interface ListAuthProvidersResponse {
  items: AuthProvider[];
}

export type LoginStatus = "pending" | "completed" | "failed" | "cancelled";

export type LoginEvent =
  | {
      type: "info";
      message: string;
      links?: { url: string; label?: string }[];
    }
  | { type: "auth_url"; url: string; instructions?: string }
  | {
      type: "device_code";
      userCode: string;
      verificationUri: string;
      intervalSeconds?: number;
      expiresInSeconds?: number;
    }
  | { type: "progress"; message: string };

export interface LoginPrompt {
  id: string;
  type: "text" | "secret" | "select" | "manual_code";
  message: string;
  placeholder?: string;
  options?: { id: string; label: string; description?: string }[];
}

export interface LoginSession {
  id: string;
  provider: string;
  status: LoginStatus;
  events: LoginEvent[];
  prompt?: LoginPrompt;
  model?: ModelInfo;
  error?: ErrorInfo;
  createdAt: string;
  updatedAt: string;
}

export interface StartLoginRequest {
  provider: string;
  selectModel?: boolean;
}

export interface LoginPromptAnswer {
  value: string;
}

export const isTerminalLoginStatus = (status: LoginStatus): boolean =>
  status !== "pending";

export interface AgentInfo {
  name: string;
  version: string;
  protocolVersion: number;
  status: "ready" | "degraded";
  issues: string[];
  model?: ModelInfo;
  thinkingLevel?: string;
  cluster: {
    domain?: string;
    mode: "proxy" | "direct" | "disabled";
    user?: {
      name?: string;
      uid?: string;
      displayName?: string;
    };
  };
  workspace?: {
    name?: string;
    hostname?: string;
  };
  settings: AgentSettings;
  capabilities: {
    blockTypes: BlockType[];
    webSearch: boolean;
    uploads: {
      maxBytes: number;
    };
    models: boolean;
    login: boolean;
    settings: boolean;
  };
}

export const formatResourceURI = (ref: ResourceRef): string =>
  `${RESOURCE_URI_PREFIX}${ref.apiVersion}/${ref.kind}/${encodeURIComponent(ref.name ?? "")}`;

export const parseResourceURI = (uri: string): ResourceRef | undefined => {
  if (!uri.startsWith(RESOURCE_URI_PREFIX)) {
    return undefined;
  }

  const parts = uri.slice(RESOURCE_URI_PREFIX.length).split("/");
  if (parts.length !== 4 || parts.some((part) => part === "")) {
    return undefined;
  }

  try {
    return {
      apiVersion: `${parts[0]}/${parts[1]}`,
      kind: parts[2],
      name: decodeURIComponent(parts[3]),
    };
  } catch {
    return undefined;
  }
};

export const resourceKey = (ref: ResourceRef): string =>
  `${ref.apiVersion}/${ref.kind}/${ref.name ?? ref.uid ?? ""}`;
