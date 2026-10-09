import * as fs from "node:fs";
import * as path from "node:path";
import type {
  Api,
  ImageContent,
  Model,
  ModelThinkingLevel,
} from "@earendil-works/pi-ai";
import {
  createAgentSession,
  DefaultResourceLoader,
  ModelRuntime,
  SessionManager,
  SettingsManager,
  type AgentSession,
  type ToolDefinition,
} from "@earendil-works/pi-coding-agent";
import { APPROVAL_MODES, THINKING_LEVELS, type Config } from "../../config.ts";
import { badRequest } from "../../errors.ts";
import type { Logger } from "../../log.ts";
import type { APICatalog } from "../../api/catalog.ts";
import type { APIClient } from "../../api/client.ts";
import type {
  AgentSettings,
  ModelInfo,
  ModelsResponse,
  SetModelRequest,
  UpdateSettingsRequest,
} from "../../protocol/index.ts";
import type { ConversationStore } from "../../store/conversations.ts";
import type { FileStore } from "../../store/files.ts";
import { SettingsStore } from "../../store/settings.ts";
import { buildSystemPrompt, type PromptContext } from "../prompt.ts";
import { createAPITools } from "../tools/api.ts";
import { createBuiltinTools, filterAvailableTools } from "../tools/builtin.ts";
import {
  createPresentationTools,
  PRESENTATION_TOOL_NAMES,
} from "../tools/present.ts";
import { createWebTools } from "../tools/web.ts";
import { createWorkspaceTools } from "../tools/workspaces.ts";
import type {
  AgentBackend,
  BackendRunContext,
  BackendRunInput,
  BackendRunResult,
  SettingsProvider,
  ToolContextProvider,
} from "../types.ts";
import { AuthManager } from "./auth.ts";
import { PiEventMapper } from "./mapper.ts";
import { resolveModel, toModelInfo, type ResolvedModel } from "./model.ts";

const maxImageBytes = 5 * 1024 * 1024;

export const PREFERRED_MODELS: Record<string, string> = {
  anthropic: "claude-opus-4-8",
  openai: "gpt-5.5",
};

const imageMimeTypes = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
]);

export interface Identity {
  user?: {
    name?: string;
    uid?: string;
    displayName?: string;
  };
  workspace?: {
    name?: string;
    hostname?: string;
  };
}

export interface PiBackendOptions {
  config: Config;
  catalog: APICatalog;
  client: APIClient;
  apiUnavailableReason?: string;
  runtime: ToolContextProvider;
  store: ConversationStore;
  files: FileStore;
  logger: Logger;
  skillPaths?: string[];
  identity?: Identity;
  modelRuntime?: ModelRuntime;
  model?: Model<Api>;
}

interface SessionEntry {
  session: AgentSession;
  busy: boolean;
  idleTimer?: NodeJS.Timeout;
}

export const formatBytes = (n: number): string => {
  if (n < 1024) {
    return `${n} B`;
  }
  if (n < 1024 * 1024) {
    return `${(n / 1024).toFixed(1)} KB`;
  }
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
};

export class PiBackend implements AgentBackend, SettingsProvider {
  model?: ModelInfo;
  thinkingLevel?: string;
  auth?: AuthManager;
  private opts: PiBackendOptions;
  private settings: SettingsStore;
  private agentDir: string;
  private modelRuntime?: ModelRuntime;
  private resolvedModel?: Model<Api>;
  private modelError?: string;
  private resolving?: Promise<Model<Api>>;
  private sessions = new Map<string, SessionEntry>();
  private pendingSessions = new Map<string, Promise<SessionEntry>>();
  private systemPrompt = "";
  private toolNames: string[] = [];
  private builtinTools: string[] = [];

  constructor(opts: PiBackendOptions) {
    this.opts = opts;
    this.agentDir = path.join(opts.config.dataDir, "pi");
    this.settings = new SettingsStore(opts.config.dataDir);
    this.thinkingLevel =
      this.settings.read().thinkingLevel ?? opts.config.llm.thinkingLevel;
  }

  get issues(): string[] {
    return this.modelError ? [this.modelError] : [];
  }

  async init() {
    process.env.PI_OFFLINE ??= "1";
    process.env.PI_TELEMETRY ??= "0";
    process.env.PI_SKIP_VERSION_CHECK ??= "1";

    fs.mkdirSync(this.agentDir, { recursive: true, mode: 0o700 });

    this.modelRuntime =
      this.opts.modelRuntime ??
      (await ModelRuntime.create({
        authPath: path.join(this.agentDir, "auth.json"),
        modelsPath: null,
        allowModelNetwork: false,
        refreshOnCreate: false,
      }));

    this.auth = new AuthManager({
      modelRuntime: this.modelRuntime,
      dataDir: this.opts.config.dataDir,
      logger: this.opts.logger,
      providers: this.opts.config.llm.loginProviders,
      onLogin: (provider, selectModel) => this.onLogin(provider, selectModel),
      onLogout: (provider) => this.onLogout(provider),
    });

    const { available, unavailable } = filterAvailableTools(
      this.opts.config.agent.tools,
    );
    this.builtinTools = available;
    if (unavailable.length > 0) {
      this.opts.logger.info(
        "Some built-in tools are disabled since their binaries are not installed",
        { tools: unavailable.join(",") },
      );
    }

    this.toolNames = this.createTools("init").map((t) => t.name);
    this.systemPrompt = buildSystemPrompt(this.promptContext());

    try {
      await this.ensureModel();
    } catch (err) {
      this.opts.logger.warn("Could not resolve the LLM model", {
        error: err as Error,
      });
    }
  }

  private promptContext(): PromptContext {
    const { config, identity, client, catalog } = this.opts;
    return {
      domain: config.octelium.domain,
      user: identity?.user,
      workspace: identity?.workspace,
      workDir: config.workDir,
      apiMode: client.mode,
      apiUnavailableReason: this.opts.apiUnavailableReason,
      apiIndex: catalog.formatIndex(),
      tools: this.toolNames,
      append: config.agent.systemPromptAppend,
    };
  }

  get prompt(): string {
    return this.systemPrompt;
  }

  get(): AgentSettings {
    const stored = this.settings.read().approvals ?? {};
    return {
      approvals: {
        api: stored.api ?? this.opts.config.approvals.api,
        commands: stored.commands ?? this.opts.config.approvals.commands,
      },
    };
  }

  getSettings(): AgentSettings {
    return this.get();
  }

  updateSettings(req: UpdateSettingsRequest): AgentSettings {
    const approvals = req?.approvals;
    if (approvals !== undefined) {
      if (typeof approvals !== "object" || approvals === null) {
        throw badRequest('The "approvals" field must be an object');
      }
      if (
        approvals.api !== undefined &&
        !(APPROVAL_MODES as readonly string[]).includes(approvals.api)
      ) {
        throw badRequest(`Invalid approval mode: ${String(approvals.api)}`);
      }
      if (
        approvals.commands !== undefined &&
        typeof approvals.commands !== "boolean"
      ) {
        throw badRequest('The "approvals.commands" field must be a boolean');
      }
      const current = this.get().approvals;
      this.settings.update({
        approvals: {
          api: approvals.api ?? current.api,
          commands: approvals.commands ?? current.commands,
        },
      });
    }
    return this.get();
  }

  private async ensureModel(): Promise<Model<Api>> {
    if (this.resolvedModel) {
      return this.resolvedModel;
    }
    if (this.opts.model) {
      this.resolvedModel = this.opts.model;
      this.model = toModelInfo(this.opts.model);
      return this.resolvedModel;
    }
    this.resolving ??= this.resolveInitialModel()
      .then((resolved) => {
        this.resolvedModel = resolved.model;
        this.model = resolved.info;
        this.modelError = undefined;
        this.opts.logger.info("Using the LLM model", {
          provider: resolved.model.provider,
          model: resolved.model.id,
          api: resolved.model.api,
          baseUrl: resolved.baseUrl,
        });
        return resolved.model;
      })
      .catch((err: Error) => {
        this.modelError = err.message;
        throw err;
      })
      .finally(() => {
        this.resolving = undefined;
      });
    return this.resolving;
  }

  private async resolveInitialModel(): Promise<ResolvedModel> {
    let configured: ResolvedModel | undefined;
    let configErr: Error | undefined;
    try {
      configured = await resolveModel(this.opts.config.llm, {
        modelRuntime: this.modelRuntime!,
        octelium: this.opts.client,
        domain: this.opts.config.octelium.domain,
        logger: this.opts.logger,
      });
    } catch (err) {
      configErr = err as Error;
    }

    const saved = this.settings.read().model;
    if (saved) {
      const model = this.modelRuntime!.getModel(saved.provider, saved.id);
      if (model && this.modelRuntime!.hasConfiguredAuth(model.provider)) {
        return { model, info: toModelInfo(model) };
      }
      this.opts.logger.warn("The selected model is not available anymore", {
        provider: saved.provider,
        model: saved.id,
      });
    }

    if (configured) {
      return configured;
    }

    for (const provider of this.opts.config.llm.loginProviders) {
      if (!this.modelRuntime!.hasConfiguredAuth(provider)) {
        continue;
      }
      const available = await this.modelRuntime!.getAvailable(provider);
      const model =
        available.find((m) => m.id === PREFERRED_MODELS[provider]) ??
        available[0];
      if (model) {
        return { model, info: toModelInfo(model) };
      }
    }

    throw configErr;
  }

  async listModels(): Promise<ModelsResponse> {
    let error: Error | undefined;
    try {
      await this.ensureModel();
    } catch (err) {
      error = err as Error;
    }
    const available = await this.modelRuntime!.getAvailable();
    return {
      current: this.model,
      thinkingLevel: this.thinkingLevel,
      models: available.map(toModelInfo),
      error:
        !this.model && error
          ? { code: "model_unavailable", message: error.message }
          : undefined,
    };
  }

  async setModel(req: SetModelRequest): Promise<ModelsResponse> {
    if (
      typeof req?.provider !== "string" ||
      typeof req.id !== "string" ||
      req.provider === "" ||
      req.id === ""
    ) {
      throw badRequest('The "provider" and "id" fields are required');
    }
    if (
      req.thinkingLevel !== undefined &&
      !(THINKING_LEVELS as readonly string[]).includes(req.thinkingLevel)
    ) {
      throw badRequest(`Invalid thinking level: ${req.thinkingLevel}`);
    }

    const model = this.modelRuntime!.getModel(req.provider, req.id);
    if (!model) {
      throw badRequest(`Unknown model ${req.provider}/${req.id}`);
    }
    if (!this.modelRuntime!.hasConfiguredAuth(model.provider)) {
      throw badRequest(
        `The provider "${model.provider}" is not configured. Sign in to it first`,
      );
    }

    this.applyModel(model, req.thinkingLevel);
    return this.listModels();
  }

  private applyModel(model: Model<Api>, thinkingLevel?: string) {
    this.resolvedModel = model;
    this.model = toModelInfo(model);
    this.modelError = undefined;
    if (thinkingLevel) {
      this.thinkingLevel = thinkingLevel;
    }
    this.settings.update({
      model: { provider: model.provider, id: model.id },
      thinkingLevel: this.thinkingLevel,
    });
    this.opts.logger.info("Using the LLM model", {
      provider: model.provider,
      model: model.id,
    });

    for (const entry of this.sessions.values()) {
      if (!entry.busy) {
        this.syncSession(entry).catch((err) =>
          this.opts.logger.warn("Could not update the session model", {
            error: err as Error,
          }),
        );
      }
    }
  }

  private async syncSession(entry: SessionEntry) {
    const model = this.resolvedModel;
    if (!model) {
      return;
    }
    if (
      entry.session.model?.provider !== model.provider ||
      entry.session.model?.id !== model.id
    ) {
      await entry.session.setModel(model);
    }
    if (
      this.thinkingLevel &&
      entry.session.thinkingLevel !== this.thinkingLevel
    ) {
      entry.session.setThinkingLevel(this.thinkingLevel as ModelThinkingLevel);
    }
  }

  private async onLogin(
    provider: string,
    selectModel: boolean,
  ): Promise<ModelInfo | undefined> {
    if (!selectModel && this.resolvedModel) {
      return undefined;
    }
    const available = await this.modelRuntime!.getAvailable(provider);
    const model =
      available.find((m) => m.id === PREFERRED_MODELS[provider]) ??
      available[0];
    if (!model) {
      return undefined;
    }
    this.applyModel(model);
    return toModelInfo(model);
  }

  private async onLogout(provider: string) {
    if (this.settings.read().model?.provider === provider) {
      this.settings.update({ model: undefined });
    }
    if (this.resolvedModel?.provider === provider) {
      this.resolvedModel = undefined;
      this.model = undefined;
      this.modelError = `Signed out of the LLM provider "${provider}"`;
    }
  }

  private createTools(conversationId: string): ToolDefinition[] {
    const { config, catalog, client, files, runtime, identity } = this.opts;
    const base = { conversationId, workDir: config.workDir, runtime };
    return [
      ...createBuiltinTools({
        ...base,
        names: this.builtinTools,
        settings: this,
      }),
      ...createWorkspaceTools({
        ...base,
        client,
        settings: this,
        agentWorkspace: identity?.workspace?.name,
        tmpDir: path.join(config.dataDir, "tmp"),
      }),
      ...createAPITools({
        ...base,
        catalog,
        client,
        settings: this,
      }),
      ...createPresentationTools({ ...base, files }),
      ...createWebTools({ ...base, webSearch: config.webSearch }),
    ] as unknown as ToolDefinition[];
  }

  private async getSession(conversationId: string): Promise<SessionEntry> {
    const existing = this.sessions.get(conversationId);
    if (existing) {
      if (existing.idleTimer) {
        clearTimeout(existing.idleTimer);
        existing.idleTimer = undefined;
      }
      return existing;
    }

    let pending = this.pendingSessions.get(conversationId);
    if (!pending) {
      pending = this.createSession(conversationId).finally(() =>
        this.pendingSessions.delete(conversationId),
      );
      this.pendingSessions.set(conversationId, pending);
    }
    return pending;
  }

  private async createSession(conversationId: string): Promise<SessionEntry> {
    const { config, store } = this.opts;
    const model = await this.ensureModel();

    const sessionDir = store.sessionDir(conversationId);
    fs.mkdirSync(sessionDir, { recursive: true, mode: 0o700 });
    const sessionFile = store.getSessionFile(conversationId);
    const sessionManager =
      sessionFile && fs.existsSync(sessionFile)
        ? SessionManager.open(sessionFile, sessionDir, config.workDir)
        : SessionManager.create(config.workDir, sessionDir, {
            id: conversationId,
          });

    const loader = new DefaultResourceLoader({
      cwd: config.workDir,
      agentDir: this.agentDir,
      noExtensions: true,
      noPromptTemplates: true,
      noThemes: true,
      noContextFiles: !config.agent.loadContextFiles,
      additionalSkillPaths: this.opts.skillPaths ?? [],
      systemPrompt: buildSystemPrompt(this.promptContext()),
    });
    await loader.reload();

    const { session } = await createAgentSession({
      cwd: config.workDir,
      agentDir: this.agentDir,
      modelRuntime: this.modelRuntime,
      model,
      thinkingLevel: this.thinkingLevel as ModelThinkingLevel,
      noTools: "builtin",
      customTools: this.createTools(conversationId),
      resourceLoader: loader,
      sessionManager,
      settingsManager: SettingsManager.inMemory({
        compaction: { enabled: config.agent.compaction },
        retry: {
          enabled: config.agent.maxRetries > 0,
          maxRetries: config.agent.maxRetries,
        },
        enableInstallTelemetry: false,
        enableAnalytics: false,
        cacheWarming: "off",
        defaultThinkingLevel: this.thinkingLevel as ModelThinkingLevel,
      }),
    });

    if (
      session.model?.provider !== model.provider ||
      session.model?.id !== model.id
    ) {
      await session.setModel(model);
    }

    const entry: SessionEntry = { session, busy: false };
    this.sessions.set(conversationId, entry);
    return entry;
  }

  private scheduleIdle(conversationId: string, entry: SessionEntry) {
    if (entry.idleTimer) {
      clearTimeout(entry.idleTimer);
    }
    entry.idleTimer = setTimeout(() => {
      if (entry.busy || this.sessions.get(conversationId) !== entry) {
        return;
      }
      this.sessions.delete(conversationId);
      entry.session.dispose();
    }, this.opts.config.agent.sessionIdleTimeoutSeconds * 1000);
    entry.idleTimer.unref();
  }

  private buildPrompt(
    input: BackendRunInput,
    model: Model<Api>,
  ): { text: string; images: ImageContent[] } {
    const images: ImageContent[] = [];
    const lines: string[] = [];

    for (const file of input.attachments) {
      lines.push(
        `- ${file.name} (${file.mimeType}, ${formatBytes(file.size)}): ${file.path}`,
      );
      if (
        model.input.includes("image") &&
        imageMimeTypes.has(file.mimeType) &&
        file.size <= maxImageBytes
      ) {
        try {
          images.push({
            type: "image",
            data: fs.readFileSync(file.path).toString("base64"),
            mimeType: file.mimeType,
          });
        } catch (err) {
          this.opts.logger.warn("Could not read an image attachment", {
            path: file.path,
            error: err as Error,
          });
        }
      }
    }

    const text =
      lines.length === 0
        ? input.text
        : `${input.text}\n\n[The User attached the following files]\n${lines.join("\n")}`.trim();
    return { text, images };
  }

  async run(
    input: BackendRunInput,
    ctx: BackendRunContext,
  ): Promise<BackendRunResult> {
    const conversationId = ctx.conversation.id;
    const model = await this.ensureModel();
    const entry = await this.getSession(conversationId);
    if (entry.busy) {
      return {
        status: "failed",
        error: { message: "The conversation is busy" },
      };
    }

    const mapper = new PiEventMapper(
      ctx.builder,
      { hiddenTools: new Set(PRESENTATION_TOOL_NAMES) },
      ctx.setActivity,
    );
    const unsubscribe = entry.session.subscribe((event) => {
      try {
        mapper.handle(event);
      } catch (err) {
        this.opts.logger.warn("Could not handle an agent event", {
          type: event.type,
          error: err as Error,
        });
      }
    });

    const onAbort = () => {
      void entry.session.abort();
    };
    ctx.signal.addEventListener("abort", onAbort, { once: true });
    entry.busy = true;

    try {
      if (ctx.signal.aborted) {
        return { status: "completed", usage: mapper.usage };
      }
      await this.syncSession(entry);
      const { text, images } = this.buildPrompt(input, model);
      await entry.session.prompt(text, {
        images: images.length > 0 ? images : undefined,
        expandPromptTemplates: false,
      });
    } catch (err) {
      if (ctx.signal.aborted) {
        return { status: "completed", usage: mapper.usage };
      }
      return {
        status: "failed",
        error: { message: (err as Error).message ?? String(err) },
        usage: mapper.usage,
      };
    } finally {
      unsubscribe();
      ctx.signal.removeEventListener("abort", onAbort);
      entry.busy = false;
      if (entry.session.sessionFile) {
        this.opts.store.setSessionFile(
          conversationId,
          entry.session.sessionFile,
        );
      }
      this.scheduleIdle(conversationId, entry);
    }

    const last = mapper.lastAssistant;
    if (last?.stopReason === "error") {
      return {
        status: "failed",
        error: {
          code: "model_error",
          message: last.errorMessage ?? "The model request failed",
        },
        usage: mapper.usage,
      };
    }
    return { status: "completed", usage: mapper.usage };
  }

  async generateTitle(
    text: string,
    signal?: AbortSignal,
  ): Promise<string | undefined> {
    const model = await this.ensureModel();
    const timeout = AbortSignal.timeout(30000);
    const res = await this.modelRuntime!.completeSimple(
      model,
      {
        systemPrompt:
          "You generate short titles for chat conversations. Reply with a title of at most 6 words that summarizes the User's request. Reply with the title only, without quotes or trailing punctuation.",
        messages: [
          { role: "user", content: text.slice(0, 4000), timestamp: Date.now() },
        ],
      },
      {
        maxTokens: 512,
        signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
      },
    );
    if (res.stopReason === "error" || res.stopReason === "aborted") {
      return undefined;
    }
    const title = res.content
      .filter((c) => c.type === "text")
      .map((c) => (c as { text: string }).text)
      .join(" ")
      .split("\n")
      .map((line) => line.trim())
      .find((line) => line !== "")
      ?.replace(/^["'`*#\s]+|["'`*.\s]+$/g, "")
      .slice(0, 80);
    return title || undefined;
  }

  forgetConversation(conversationId: string) {
    const entry = this.sessions.get(conversationId);
    if (!entry) {
      return;
    }
    if (entry.idleTimer) {
      clearTimeout(entry.idleTimer);
    }
    this.sessions.delete(conversationId);
    entry.session.dispose();
  }

  async dispose() {
    this.auth?.dispose();
    for (const [id] of this.sessions) {
      this.forgetConversation(id);
    }
  }
}
