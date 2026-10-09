import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import type {
  AuthEvent,
  AuthInteraction,
  AuthPrompt,
} from "@earendil-works/pi-ai";
import type { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { badRequest, conflict, notFound } from "../../errors.ts";
import type { Logger } from "../../log.ts";
import type {
  AuthProvider,
  LoginEvent,
  LoginSession,
  ModelInfo,
  StartLoginRequest,
} from "../../protocol/index.ts";
import type { AuthController } from "../types.ts";

const preferredSelections: Record<string, string> = {
  anthropic: "copy_code",
};

const loginTTLMs = 15 * 60 * 1000;
const maxEvents = 50;
const maxAnswerLength = 8192;

const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

interface PendingPrompt {
  id: string;
  resolve(value: string): void;
  reject(err: Error): void;
}

interface LoginEntry {
  session: LoginSession;
  controller: AbortController;
  prompt?: PendingPrompt;
  timer: NodeJS.Timeout;
}

export interface AuthManagerOptions {
  modelRuntime: ModelRuntime;
  dataDir: string;
  logger: Logger;
  providers: string[];
  onLogin?(
    provider: string,
    selectModel: boolean,
  ): Promise<ModelInfo | undefined>;
  onLogout?(provider: string): Promise<void>;
}

export const toLoginEvent = (event: AuthEvent): LoginEvent => {
  switch (event.type) {
    case "info":
      return {
        type: "info",
        message: event.message,
        links: event.links?.map((l) => ({ url: l.url, label: l.label })),
      };
    case "auth_url":
      return {
        type: "auth_url",
        url: event.url,
        instructions: event.instructions,
      };
    case "device_code":
      return {
        type: "device_code",
        userCode: event.userCode,
        verificationUri: event.verificationUri,
        intervalSeconds: event.intervalSeconds,
        expiresInSeconds: event.expiresInSeconds,
      };
    case "progress":
      return { type: "progress", message: event.message };
  }
};

export class AuthManager implements AuthController {
  private opts: AuthManagerOptions;
  private providers: string[];
  private logins = new Map<string, LoginEntry>();

  constructor(opts: AuthManagerOptions) {
    this.opts = opts;
    this.providers = opts.providers;
  }

  getDeviceId(): string {
    const filePath = path.join(this.opts.dataDir, "device-id");
    try {
      const value = fs.readFileSync(filePath, "utf8").trim();
      if (uuidPattern.test(value)) {
        return value;
      }
    } catch {}

    const id = crypto.randomUUID();
    fs.mkdirSync(this.opts.dataDir, { recursive: true, mode: 0o700 });
    fs.writeFileSync(filePath, id, { mode: 0o600 });
    return id;
  }

  async listProviders(): Promise<AuthProvider[]> {
    const rt = this.opts.modelRuntime;
    const ret: AuthProvider[] = [];
    for (const id of this.providers) {
      const provider = rt.getProvider(id);
      const oauth = provider?.auth.oauth;
      if (!provider || !oauth) {
        continue;
      }
      ret.push({
        id,
        name: oauth.name,
        loginLabel: oauth.loginLabel,
        subscription: !!oauth.isSubscription,
        configured: rt.hasConfiguredAuth(id),
        oauth: rt.isUsingOAuth(id),
      });
    }
    return ret;
  }

  startLogin(req: StartLoginRequest): LoginSession {
    const provider = this.opts.modelRuntime.getProvider(req.provider);
    if (!this.providers.includes(req.provider) || !provider?.auth.oauth) {
      throw badRequest(
        `Signing in is not supported for the provider "${req.provider}"`,
      );
    }

    for (const entry of this.logins.values()) {
      if (
        entry.session.provider === req.provider &&
        entry.session.status === "pending"
      ) {
        this.cancelLogin(entry.session.id);
      }
    }

    const now = new Date().toISOString();
    const id = crypto.randomUUID();
    const entry: LoginEntry = {
      session: {
        id,
        provider: req.provider,
        status: "pending",
        events: [],
        createdAt: now,
        updatedAt: now,
      },
      controller: new AbortController(),
      timer: setTimeout(() => this.expire(id), loginTTLMs),
    };
    entry.timer.unref();
    this.logins.set(id, entry);

    void this.runLogin(entry, req.selectModel ?? true);
    return structuredClone(entry.session);
  }

  getLogin(id: string): LoginSession | undefined {
    const entry = this.logins.get(id);
    return entry ? structuredClone(entry.session) : undefined;
  }

  answerLogin(id: string, promptId: string, value: string): LoginSession {
    const entry = this.logins.get(id);
    if (!entry) {
      throw notFound(`Login not found: ${id}`);
    }
    if (typeof value !== "string" || value.length > maxAnswerLength) {
      throw badRequest('The "value" field must be a string');
    }
    if (
      entry.session.status !== "pending" ||
      !entry.prompt ||
      entry.prompt.id !== promptId
    ) {
      throw conflict("The prompt is not pending anymore");
    }
    entry.prompt.resolve(value);
    return structuredClone(entry.session);
  }

  cancelLogin(id: string): LoginSession {
    const entry = this.logins.get(id);
    if (!entry) {
      throw notFound(`Login not found: ${id}`);
    }
    if (entry.session.status === "pending") {
      entry.session.status = "cancelled";
      this.touch(entry);
      entry.controller.abort();
      entry.prompt?.reject(new Error("The login was cancelled"));
    }
    return structuredClone(entry.session);
  }

  async logout(provider: string): Promise<void> {
    if (!this.providers.includes(provider)) {
      throw badRequest(
        `Signing out is not supported for the provider "${provider}"`,
      );
    }
    await this.opts.modelRuntime.logout(provider);
    await this.opts.onLogout?.(provider);
  }

  dispose() {
    for (const entry of this.logins.values()) {
      clearTimeout(entry.timer);
      if (entry.session.status === "pending") {
        this.cancelLogin(entry.session.id);
      }
    }
    this.logins.clear();
  }

  private touch(entry: LoginEntry) {
    entry.session.updatedAt = new Date().toISOString();
  }

  private expire(id: string) {
    const entry = this.logins.get(id);
    if (!entry) {
      return;
    }
    if (entry.session.status === "pending") {
      this.cancelLogin(id);
    }
    this.logins.delete(id);
  }

  private async runLogin(entry: LoginEntry, selectModel: boolean) {
    const { provider } = entry.session;
    const interaction: AuthInteraction = {
      signal: entry.controller.signal,
      notify: (event) => {
        entry.session.events.push(toLoginEvent(event));
        if (entry.session.events.length > maxEvents) {
          entry.session.events.splice(
            0,
            entry.session.events.length - maxEvents,
          );
        }
        this.touch(entry);
      },
      prompt: (prompt) => this.prompt(entry, prompt),
    };

    try {
      await this.opts.modelRuntime.login(provider, "oauth", interaction, {
        getDeviceId: () => this.getDeviceId(),
      });
      if (entry.session.status !== "pending") {
        return;
      }
      if (this.opts.onLogin) {
        try {
          entry.session.model = await this.opts.onLogin(provider, selectModel);
        } catch (err) {
          this.opts.logger.warn("Could not select a model after signing in", {
            provider,
            error: err as Error,
          });
        }
      }
      entry.session.status = "completed";
      this.opts.logger.info("Signed in to the LLM provider", { provider });
    } catch (err) {
      if (entry.session.status === "pending") {
        entry.session.status = "failed";
        entry.session.error = {
          code: "login_failed",
          message: (err as Error).message ?? String(err),
        };
        this.opts.logger.warn("Could not sign in to the LLM provider", {
          provider,
          error: err as Error,
        });
      }
    } finally {
      entry.prompt = undefined;
      entry.session.prompt = undefined;
      this.touch(entry);
    }
  }

  private prompt(entry: LoginEntry, prompt: AuthPrompt): Promise<string> {
    if (prompt.type === "select") {
      const preferred = preferredSelections[entry.session.provider];
      if (preferred && prompt.options.some((o) => o.id === preferred)) {
        return Promise.resolve(preferred);
      }
    }

    return new Promise<string>((resolve, reject) => {
      if (prompt.signal?.aborted || entry.controller.signal.aborted) {
        reject(new Error("The prompt was cancelled"));
        return;
      }

      const id = crypto.randomUUID();
      const cleanup = () => {
        prompt.signal?.removeEventListener("abort", onAbort);
        if (entry.prompt?.id === id) {
          entry.prompt = undefined;
          entry.session.prompt = undefined;
          this.touch(entry);
        }
      };
      const onAbort = () => {
        cleanup();
        reject(new Error("The prompt was cancelled"));
      };
      prompt.signal?.addEventListener("abort", onAbort, { once: true });

      entry.prompt = {
        id,
        resolve: (value) => {
          cleanup();
          resolve(value);
        },
        reject: (err) => {
          cleanup();
          reject(err);
        },
      };
      entry.session.prompt = {
        id,
        type: prompt.type,
        message: prompt.message,
        placeholder: prompt.type === "select" ? undefined : prompt.placeholder,
        options:
          prompt.type === "select"
            ? prompt.options.map((o) => ({
                id: o.id,
                label: o.label,
                description: o.description,
              }))
            : undefined,
      };
      this.touch(entry);
    });
  }
}
