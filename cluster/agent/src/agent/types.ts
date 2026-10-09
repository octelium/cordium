import type {
  AgentSettings,
  APIRisk,
  ApprovalPreview,
  AuthProvider,
  Conversation,
  ErrorInfo,
  FileInfo,
  LoginSession,
  ModelInfo,
  ModelsResponse,
  RunActivity,
  SetModelRequest,
  StartLoginRequest,
  UpdateSettingsRequest,
  Usage,
} from "../protocol/index.ts";
import type { MessageBuilder } from "../runs/builder.ts";
import type { ResourceCache } from "./resources.ts";

export interface ApprovalRequest {
  toolCallId: string;
  title: string;
  description?: string;
  risk: APIRisk;
  preview?: ApprovalPreview;
}

export interface ApprovalResult {
  approved: boolean;
  reason?: string;
}

export interface ToolRunContext {
  runId: string;
  conversationId: string;
  builder: MessageBuilder;
  resources: ResourceCache;
  resultsDir: string;
  requestApproval(
    req: ApprovalRequest,
    signal?: AbortSignal,
  ): Promise<ApprovalResult>;
}

export interface ToolContextProvider {
  current(conversationId: string): ToolRunContext | undefined;
}

export interface SettingsProvider {
  get(): AgentSettings;
}

export interface BackendRunInput {
  text: string;
  attachments: FileInfo[];
}

export interface BackendRunContext {
  runId: string;
  conversation: Conversation;
  builder: MessageBuilder;
  signal: AbortSignal;
  setActivity(activity: RunActivity | undefined): void;
}

export interface BackendRunResult {
  status: "completed" | "failed";
  error?: ErrorInfo;
  usage?: Usage;
}

export interface AuthController {
  listProviders(): Promise<AuthProvider[]>;
  startLogin(req: StartLoginRequest): LoginSession;
  getLogin(id: string): LoginSession | undefined;
  answerLogin(id: string, promptId: string, value: string): LoginSession;
  cancelLogin(id: string): LoginSession;
  logout(provider: string): Promise<void>;
}

export interface AgentBackend {
  readonly model?: ModelInfo;
  readonly thinkingLevel?: string;
  readonly auth?: AuthController;
  listModels?(): Promise<ModelsResponse>;
  setModel?(req: SetModelRequest): Promise<ModelsResponse>;
  getSettings?(): AgentSettings;
  updateSettings?(req: UpdateSettingsRequest): AgentSettings;
  run(
    input: BackendRunInput,
    ctx: BackendRunContext,
  ): Promise<BackendRunResult>;
  generateTitle(
    text: string,
    signal?: AbortSignal,
  ): Promise<string | undefined>;
  forgetConversation(conversationId: string): void;
  dispose(): Promise<void>;
}
