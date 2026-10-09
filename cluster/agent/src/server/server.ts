import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as http from "node:http";
import type { AddressInfo } from "node:net";
import type { AgentBackend, AuthController } from "../agent/types.ts";
import type { Identity } from "../agent/pi/backend.ts";
import type { Config } from "../config.ts";
import {
  badRequest,
  forbidden,
  HTTPError,
  notFound,
  payloadTooLarge,
  unauthorized,
} from "../errors.ts";
import type { Logger } from "../log.ts";
import type { APIClient } from "../api/client.ts";
import {
  API_PREFIX,
  BLOCK_TYPES,
  PROTOCOL_VERSION,
  TERMINAL_RUN_EVENT_TYPES,
  type AgentEvent,
  type AgentInfo,
  type ApprovalDecisionRequest,
  type ConversationDetail,
  type CreateConversationRequest,
  type CreateConversationResponse,
  type CreateRunRequest,
  type ListAuthProvidersResponse,
  type ListConversationsResponse,
  type LoginPromptAnswer,
  type SetModelRequest,
  type StartLoginRequest,
  type UpdateConversationRequest,
  type UpdateSettingsRequest,
} from "../protocol/index.ts";
import type { RunManager } from "../runs/manager.ts";
import { isValidID, type ConversationStore } from "../store/conversations.ts";
import { FileTooLargeError, type FileStore } from "../store/files.ts";
import {
  contentDisposition,
  originMatches,
  readJSON,
  requestOrigin,
  sendError,
  sendJSON,
  sendNoContent,
} from "./http.ts";

export interface ServerDeps {
  config: Config;
  store: ConversationStore;
  files: FileStore;
  runs: RunManager;
  backend: AgentBackend & { issues?: string[] };
  client: APIClient;
  identity?: Identity;
  issues?: string[];
  version: string;
  logger: Logger;
}

type Params = Record<string, string>;

type Handler = (
  req: http.IncomingMessage,
  res: http.ServerResponse,
  params: Params,
  url: URL,
) => Promise<void> | void;

interface Route {
  method: string;
  pattern: RegExp;
  keys: string[];
  handler: Handler;
}

const inlineMimeTypes = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
  "application/pdf",
  "text/plain",
  "text/csv",
  "text/markdown",
  "application/json",
]);

const heartbeatMs = 15000;

export class AgentServer {
  private deps: ServerDeps;
  private routes: Route[] = [];
  readonly server: http.Server;
  private sseResponses = new Set<http.ServerResponse>();

  constructor(deps: ServerDeps) {
    this.deps = deps;
    this.registerRoutes();
    this.server = http.createServer((req, res) => {
      void this.handle(req, res);
    });
    this.server.requestTimeout = 0;
    this.server.headersTimeout = 60000;
    this.server.keepAliveTimeout = 65000;
  }

  private route(method: string, path: string, handler: Handler) {
    const keys: string[] = [];
    const pattern = new RegExp(
      `^${path.replace(/:(\w+)/g, (_m, key: string) => {
        keys.push(key);
        return "([^/]+)";
      })}$`,
    );
    this.routes.push({ method, pattern, keys, handler });
  }

  private registerRoutes() {
    this.route("GET", "/healthz", (_req, res) => {
      sendJSON(res, 200, { status: "ok" });
    });
    this.route("GET", `${API_PREFIX}/info`, (_req, res) => {
      sendJSON(res, 200, this.info());
    });

    this.route("GET", `${API_PREFIX}/conversations`, (_req, res) => {
      const body: ListConversationsResponse = {
        items: this.deps.store.list(),
      };
      sendJSON(res, 200, body);
    });
    this.route("POST", `${API_PREFIX}/conversations`, (req, res) =>
      this.createConversation(req, res),
    );
    this.route("GET", `${API_PREFIX}/conversations/:id`, (_req, res, params) =>
      this.getConversation(res, params.id),
    );
    this.route("PATCH", `${API_PREFIX}/conversations/:id`, (req, res, params) =>
      this.updateConversation(req, res, params.id),
    );
    this.route(
      "DELETE",
      `${API_PREFIX}/conversations/:id`,
      (_req, res, params) => this.deleteConversation(res, params.id),
    );
    this.route(
      "POST",
      `${API_PREFIX}/conversations/:id/runs`,
      async (req, res, params) => {
        const body = await readJSON<CreateRunRequest>(req);
        if (!body.input || typeof body.input !== "object") {
          throw badRequest('The "input" field is required');
        }
        sendJSON(res, 201, this.deps.runs.startRun(params.id, body.input));
      },
    );

    this.route("GET", `${API_PREFIX}/runs/:id`, (_req, res, params) => {
      const snapshot = this.deps.runs.getSnapshot(params.id);
      if (!snapshot) {
        throw notFound(`Run not found: ${params.id}`);
      }
      sendJSON(res, 200, snapshot);
    });
    this.route(
      "GET",
      `${API_PREFIX}/runs/:id/events`,
      (req, res, params, url) => this.streamEvents(req, res, params.id, url),
    );
    this.route("POST", `${API_PREFIX}/runs/:id/cancel`, (_req, res, params) => {
      sendJSON(res, 200, this.deps.runs.cancel(params.id));
    });
    this.route(
      "POST",
      `${API_PREFIX}/runs/:id/approvals/:approvalId`,
      async (req, res, params) => {
        const body = await readJSON<ApprovalDecisionRequest>(req);
        if (body.reason !== undefined && typeof body.reason !== "string") {
          throw badRequest('The "reason" field must be a string');
        }
        sendJSON(
          res,
          200,
          this.deps.runs.decideApproval(
            params.id,
            params.approvalId,
            body.decision,
            body.reason?.slice(0, 2000),
          ),
        );
      },
    );

    this.route("POST", `${API_PREFIX}/files`, (req, res, _params, url) =>
      this.uploadFile(req, res, url),
    );
    this.route("GET", `${API_PREFIX}/files/:id`, (_req, res, params) => {
      const file = this.deps.files.getUpload(params.id);
      if (!file) {
        throw notFound(`File not found: ${params.id}`);
      }
      sendJSON(res, 200, file);
    });

    this.route("GET", `${API_PREFIX}/artifacts/:id`, (_req, res, params) => {
      const artifact = this.deps.files.getArtifact(params.id);
      if (!artifact) {
        throw notFound(`Artifact not found: ${params.id}`);
      }
      sendJSON(res, 200, artifact.info);
    });
    this.route(
      "GET",
      `${API_PREFIX}/artifacts/:id/content`,
      (_req, res, params, url) => this.downloadArtifact(res, params.id, url),
    );

    this.route("GET", `${API_PREFIX}/models`, async (_req, res) => {
      const { backend } = this.deps;
      if (!backend.listModels) {
        throw notFound("Model selection is not supported");
      }
      sendJSON(res, 200, await backend.listModels());
    });
    this.route("PUT", `${API_PREFIX}/model`, async (req, res) => {
      const { backend } = this.deps;
      if (!backend.setModel) {
        throw notFound("Model selection is not supported");
      }
      const body = await readJSON<SetModelRequest>(req);
      sendJSON(res, 200, await backend.setModel(body));
    });

    this.route("GET", `${API_PREFIX}/settings`, (_req, res) => {
      const { backend } = this.deps;
      if (!backend.getSettings) {
        throw notFound("Settings are not supported");
      }
      sendJSON(res, 200, backend.getSettings());
    });
    this.route("PATCH", `${API_PREFIX}/settings`, async (req, res) => {
      const { backend } = this.deps;
      if (!backend.updateSettings) {
        throw notFound("Settings are not supported");
      }
      const body = await readJSON<UpdateSettingsRequest>(req);
      sendJSON(res, 200, backend.updateSettings(body));
    });

    this.route("GET", `${API_PREFIX}/auth/providers`, async (_req, res) => {
      const body: ListAuthProvidersResponse = {
        items: await this.auth().listProviders(),
      };
      sendJSON(res, 200, body);
    });
    this.route(
      "DELETE",
      `${API_PREFIX}/auth/providers/:id`,
      async (_req, res, params) => {
        await this.auth().logout(params.id);
        sendNoContent(res);
      },
    );
    this.route("POST", `${API_PREFIX}/auth/logins`, async (req, res) => {
      const body = await readJSON<StartLoginRequest>(req);
      if (typeof body.provider !== "string" || body.provider === "") {
        throw badRequest('The "provider" field is required');
      }
      if (
        body.selectModel !== undefined &&
        typeof body.selectModel !== "boolean"
      ) {
        throw badRequest('The "selectModel" field must be a boolean');
      }
      sendJSON(res, 201, this.auth().startLogin(body));
    });
    this.route("GET", `${API_PREFIX}/auth/logins/:id`, (_req, res, params) => {
      const login = this.auth().getLogin(params.id);
      if (!login) {
        throw notFound(`Login not found: ${params.id}`);
      }
      sendJSON(res, 200, login);
    });
    this.route(
      "POST",
      `${API_PREFIX}/auth/logins/:id/prompts/:promptId`,
      async (req, res, params) => {
        const body = await readJSON<LoginPromptAnswer>(req);
        sendJSON(
          res,
          200,
          this.auth().answerLogin(params.id, params.promptId, body.value),
        );
      },
    );
    this.route(
      "DELETE",
      `${API_PREFIX}/auth/logins/:id`,
      (_req, res, params) => {
        sendJSON(res, 200, this.auth().cancelLogin(params.id));
      },
    );
  }

  private auth(): AuthController {
    if (!this.deps.backend.auth) {
      throw notFound("Signing in is not supported");
    }
    return this.deps.backend.auth;
  }

  info(): AgentInfo {
    const { config, backend, client, identity } = this.deps;
    const issues = [...(this.deps.issues ?? []), ...(backend.issues ?? [])];
    return {
      name: "cordium-agent",
      version: this.deps.version,
      protocolVersion: PROTOCOL_VERSION,
      status: issues.length === 0 ? "ready" : "degraded",
      issues,
      model: backend.model,
      thinkingLevel: backend.thinkingLevel,
      cluster: {
        domain: config.octelium.domain,
        mode: client.mode,
        user: identity?.user,
      },
      workspace: identity?.workspace,
      settings: backend.getSettings?.() ?? {
        approvals: config.approvals,
      },
      capabilities: {
        blockTypes: BLOCK_TYPES,
        webSearch: !!config.webSearch,
        uploads: { maxBytes: config.server.maxUploadBytes },
        models: !!backend.listModels,
        login: !!backend.auth,
        settings: !!backend.updateSettings,
      },
    };
  }

  private allowedOrigin(req: http.IncomingMessage, origin: string): boolean {
    if (origin === requestOrigin(req)) {
      return true;
    }
    return this.deps.config.server.allowedOrigins.some((pattern) =>
      originMatches(origin, pattern),
    );
  }

  private checkAuth(req: http.IncomingMessage, url: URL) {
    const token = this.deps.config.server.authToken;
    if (!token) {
      return;
    }
    const header = req.headers.authorization ?? "";
    const provided = header.startsWith("Bearer ")
      ? header.slice(7)
      : (url.searchParams.get("access_token") ?? "");
    const a = Buffer.from(provided);
    const b = Buffer.from(token);
    if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) {
      throw unauthorized();
    }
  }

  private async handle(req: http.IncomingMessage, res: http.ServerResponse) {
    const started = Date.now();
    let url: URL;
    try {
      url = new URL(req.url ?? "/", "http://localhost");
    } catch {
      sendError(res, badRequest("Invalid URL"));
      return;
    }

    res.setHeader("x-content-type-options", "nosniff");
    res.setHeader("referrer-policy", "no-referrer");

    try {
      const origin = req.headers.origin;
      if (origin) {
        if (!this.allowedOrigin(req, origin)) {
          throw forbidden("Origin not allowed");
        }
        if (this.deps.config.server.cors) {
          res.setHeader("access-control-allow-origin", origin);
          res.setHeader("access-control-allow-credentials", "true");
          res.setHeader("vary", "Origin");
          if (req.method === "OPTIONS") {
            res.writeHead(204, {
              "access-control-allow-methods":
                "GET, POST, PUT, PATCH, DELETE, OPTIONS",
              "access-control-allow-headers":
                "authorization, content-type, last-event-id, x-file-name",
              "access-control-max-age": "600",
            });
            res.end();
            return;
          }
        }
      }

      const candidates = this.routes.filter((r) =>
        r.pattern.test(url.pathname),
      );
      if (candidates.length === 0) {
        throw notFound();
      }
      const route = candidates.find((r) => r.method === req.method);
      if (!route) {
        throw new HTTPError(405, "method_not_allowed", "Method not allowed");
      }

      if (url.pathname.startsWith(`${API_PREFIX}/`)) {
        this.checkAuth(req, url);
      }

      const match = route.pattern.exec(url.pathname)!;
      const params: Params = {};
      route.keys.forEach((key, i) => {
        try {
          params[key] = decodeURIComponent(match[i + 1]);
        } catch {
          throw notFound();
        }
      });
      for (const key of ["id", "approvalId", "promptId"]) {
        if (params[key] !== undefined && !isValidID(params[key])) {
          throw notFound();
        }
      }

      await route.handler(req, res, params, url);
    } catch (err) {
      if (!(err instanceof HTTPError)) {
        this.deps.logger.error("Request failed", {
          method: req.method,
          path: url.pathname,
          error: err as Error,
        });
      }
      sendError(res, err);
    } finally {
      this.deps.logger.debug("Request", {
        method: req.method,
        path: url.pathname,
        status: res.statusCode,
        durationMs: Date.now() - started,
      });
    }
  }

  private async createConversation(
    req: http.IncomingMessage,
    res: http.ServerResponse,
  ) {
    const body = await readJSON<CreateConversationRequest>(req);
    if (body.title !== undefined && typeof body.title !== "string") {
      throw badRequest('The "title" field must be a string');
    }
    const { store, runs } = this.deps;
    const conversation = store.create(body.title?.slice(0, 200));
    const ret: CreateConversationResponse = { conversation };
    if (body.input) {
      try {
        ret.run = runs.startRun(conversation.id, body.input);
      } catch (err) {
        store.delete(conversation.id);
        throw err;
      }
      ret.conversation = store.get(conversation.id) ?? conversation;
    }
    sendJSON(res, 201, ret);
  }

  private getConversation(res: http.ServerResponse, id: string) {
    const { store, runs } = this.deps;
    const conversation = store.get(id);
    if (!conversation) {
      throw notFound(`Conversation not found: ${id}`);
    }
    const body: ConversationDetail = {
      conversation,
      messages: store.getMessages(id),
      activeRun: runs.getActiveSnapshot(id),
    };
    sendJSON(res, 200, body);
  }

  private async updateConversation(
    req: http.IncomingMessage,
    res: http.ServerResponse,
    id: string,
  ) {
    const body = await readJSON<UpdateConversationRequest>(req);
    if (body.title !== undefined && typeof body.title !== "string") {
      throw badRequest('The "title" field must be a string');
    }
    const conversation = this.deps.store.update(id, {
      title: body.title?.slice(0, 200),
    });
    if (!conversation) {
      throw notFound(`Conversation not found: ${id}`);
    }
    sendJSON(res, 200, conversation);
  }

  private async deleteConversation(res: http.ServerResponse, id: string) {
    const { store, runs, backend } = this.deps;
    if (!store.get(id)) {
      throw notFound(`Conversation not found: ${id}`);
    }
    await runs.cancelConversation(id);
    backend.forgetConversation(id);
    runs.forgetConversation(id);
    store.delete(id);
    sendNoContent(res);
  }

  private streamEvents(
    req: http.IncomingMessage,
    res: http.ServerResponse,
    runId: string,
    url: URL,
  ) {
    const lastEventID =
      (req.headers["last-event-id"] as string | undefined) ??
      url.searchParams.get("after") ??
      "0";
    const afterSeq = Number.parseInt(lastEventID, 10) || 0;

    let closed = false;
    let heartbeat: NodeJS.Timeout | undefined;
    const write = (event: AgentEvent) => {
      if (closed) {
        return;
      }
      res.write(
        `id: ${event.seq}\nevent: ${event.type}\ndata: ${JSON.stringify(event)}\n\n`,
      );
      if (TERMINAL_RUN_EVENT_TYPES.includes(event.type)) {
        close();
      }
    };

    let unsubscribe = () => {};
    const close = () => {
      if (closed) {
        return;
      }
      closed = true;
      if (heartbeat) {
        clearInterval(heartbeat);
      }
      unsubscribe();
      this.sseResponses.delete(res);
      res.end();
    };

    const sub = this.deps.runs.subscribe(runId, afterSeq, write);
    if (!sub) {
      throw notFound(`Run not found: ${runId}`);
    }
    unsubscribe = sub.unsubscribe;

    if (sub.finished && sub.replay.length === 0) {
      sendNoContent(res);
      return;
    }

    res.writeHead(200, {
      "content-type": "text/event-stream; charset=utf-8",
      "cache-control": "no-cache, no-transform",
      connection: "keep-alive",
      "x-accel-buffering": "no",
    });
    res.flushHeaders();
    res.write("retry: 3000\n\n");
    this.sseResponses.add(res);

    req.on("close", close);
    res.on("error", close);

    for (const event of sub.replay) {
      write(event);
    }
    if (sub.finished) {
      close();
      return;
    }

    heartbeat = setInterval(() => {
      if (!closed) {
        res.write(": ping\n\n");
      }
    }, heartbeatMs);
    heartbeat.unref();
  }

  private async uploadFile(
    req: http.IncomingMessage,
    res: http.ServerResponse,
    url: URL,
  ) {
    const header = req.headers["x-file-name"] as string | undefined;
    let name = url.searchParams.get("name") ?? undefined;
    if (!name && header) {
      try {
        name = decodeURIComponent(header);
      } catch {
        name = header;
      }
    }
    if (!name) {
      throw badRequest('The "name" query parameter is required');
    }
    const maxBytes = this.deps.config.server.maxUploadBytes;
    if (Number(req.headers["content-length"] ?? "0") > maxBytes) {
      throw payloadTooLarge(`The file exceeds ${maxBytes} bytes`);
    }
    const contentType = (req.headers["content-type"] ?? "")
      .split(";")[0]
      .trim();
    try {
      const info = await this.deps.files.saveUpload(
        name,
        contentType || undefined,
        req,
        maxBytes,
      );
      sendJSON(res, 201, info);
    } catch (err) {
      if (err instanceof FileTooLargeError) {
        throw payloadTooLarge(err.message);
      }
      throw err;
    }
  }

  private downloadArtifact(res: http.ServerResponse, id: string, url: URL) {
    const artifact = this.deps.files.getArtifact(id);
    if (!artifact) {
      throw notFound(`Artifact not found: ${id}`);
    }
    const { info, filePath } = artifact;
    const wantsInline = url.searchParams.get("inline") === "1";
    const inline = wantsInline && inlineMimeTypes.has(info.mimeType);
    const contentType =
      info.mimeType.startsWith("text/") || info.mimeType === "application/json"
        ? `${info.mimeType}; charset=utf-8`
        : info.mimeType;
    const stat = fs.statSync(filePath);

    res.writeHead(200, {
      "content-type": contentType,
      "content-length": stat.size,
      "content-disposition": contentDisposition(
        inline ? "inline" : "attachment",
        info.name,
      ),
      "content-security-policy":
        "sandbox; default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'",
      "cache-control": "private, no-store",
    });
    fs.createReadStream(filePath)
      .on("error", () => res.destroy())
      .pipe(res);
  }

  async listen(host: string, port: number): Promise<AddressInfo> {
    await new Promise<void>((resolve, reject) => {
      this.server.once("error", reject);
      this.server.listen(port, host, () => {
        this.server.off("error", reject);
        resolve();
      });
    });
    return this.server.address() as AddressInfo;
  }

  async close() {
    for (const res of this.sseResponses) {
      res.end();
    }
    this.sseResponses.clear();
    await new Promise<void>((resolve) => {
      this.server.close(() => resolve());
      this.server.closeAllConnections?.();
    });
  }
}
