import * as fs from "node:fs";
import { credentials } from "@grpc/grpc-js";
import { Cordium, NodeGrpcTransport } from "@octelium/cordium";
import type { JsonValue } from "@protobuf-ts/runtime";
import type { RpcTransport } from "@protobuf-ts/runtime-rpc";
import type { Config } from "../config.ts";
import type { APICatalog, APIMethod } from "./catalog.ts";

export type APIMode = "proxy" | "direct" | "disabled";

export class APIError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
  }
}

export class InvalidRequestError extends Error {}

export interface InvokeOptions {
  signal?: AbortSignal;
  timeoutMs?: number;
  maxMessages?: number;
  streamTimeoutMs?: number;
}

export interface InvokeResult {
  response: JsonValue;
  messageCount?: number;
}

export interface APIClientOptions {
  mode: APIMode;
  domain?: string;
  authProxySocket?: string;
  accessToken?: string;
  timeoutSeconds?: number;
  userAgent?: string;
}

export const resolveAPIMode = (
  cfg: Config["octelium"],
): { mode: APIMode; reason?: string } => {
  const proxyAvailable =
    !!cfg.authProxySocket && fs.existsSync(cfg.authProxySocket);

  switch (cfg.mode) {
    case "disabled":
      return {
        mode: "disabled",
        reason: "The Cordium API access is disabled",
      };
    case "proxy":
      return proxyAvailable
        ? { mode: "proxy" }
        : {
            mode: "disabled",
            reason: `The Octelium auth proxy socket is not available: ${cfg.authProxySocket ?? "(unset)"}`,
          };
    case "direct":
      return cfg.domain && cfg.accessToken
        ? { mode: "direct" }
        : {
            mode: "disabled",
            reason:
              "The direct mode requires both the Octelium domain and an access token",
          };
    default:
      if (proxyAvailable) {
        return { mode: "proxy" };
      }
      if (cfg.domain && cfg.accessToken) {
        return { mode: "direct" };
      }
      return {
        mode: "disabled",
        reason:
          "Could not find neither OCTELIUM_AUTH_PROXY_SOCKET nor OCTELIUM_DOMAIN and OCTELIUM_ACCESS_TOKEN",
      };
  }
};

export const errorCodeName = (code: string): string =>
  code.replace(/([a-z])([A-Z])/g, "$1_$2").toLowerCase();

export const toAPIError = (err: unknown): APIError => {
  if (err instanceof APIError) {
    return err;
  }
  const e = err as { code?: unknown; message?: string } | undefined;
  return new APIError(
    typeof e?.code === "string" ? errorCodeName(e.code) : "unknown",
    e?.message || String(err),
  );
};

const isRecord = (arg: unknown): arg is Record<string, unknown> =>
  typeof arg === "object" && arg !== null && !Array.isArray(arg);

export class APIClient {
  readonly mode: APIMode;
  readonly domain?: string;
  readonly catalog: APICatalog;
  readonly cordium?: Cordium;
  readonly transport?: RpcTransport;
  private timeoutMs: number;
  private ownedTransport?: NodeGrpcTransport;

  constructor(catalog: APICatalog, opts: APIClientOptions) {
    this.catalog = catalog;
    this.mode = opts.mode;
    this.domain = opts.domain;
    this.timeoutMs = (opts.timeoutSeconds ?? 30) * 1000;
    const userAgent = opts.userAgent ?? "cordium-agent";

    switch (opts.mode) {
      case "proxy": {
        this.ownedTransport = new NodeGrpcTransport({
          host: `unix:${opts.authProxySocket!}`,
          channelCredentials: credentials.createInsecure(),
          clientOptions: {
            "grpc.primary_user_agent": userAgent,
            "grpc.default_authority": "localhost",
          },
        });
        this.transport = this.ownedTransport;
        this.cordium = new Cordium({
          transport: this.transport,
          domain: opts.domain,
          timeoutMs: this.timeoutMs,
        });
        break;
      }
      case "direct": {
        this.cordium = new Cordium({
          domain: opts.domain,
          auth: { type: "accessToken", accessToken: opts.accessToken! },
          timeoutMs: this.timeoutMs,
          channelOptions: { "grpc.primary_user_agent": userAgent },
        });
        this.transport = this.cordium.octelium?.transport;
        break;
      }
    }
  }

  static fromConfig(
    catalog: APICatalog,
    cfg: Config["octelium"],
    userAgent?: string,
  ): { client: APIClient; reason?: string } {
    const { mode, reason } = resolveAPIMode(cfg);
    return {
      client: new APIClient(catalog, {
        mode,
        domain: cfg.domain,
        authProxySocket: cfg.authProxySocket,
        accessToken: cfg.accessToken,
        timeoutSeconds: cfg.timeoutSeconds,
        userAgent,
      }),
      reason,
    };
  }

  get enabled(): boolean {
    return this.transport !== undefined;
  }

  requireCordium(): Cordium {
    if (!this.cordium) {
      throw new APIError(
        "unavailable",
        "The Cordium API is not available to this agent",
      );
    }
    return this.cordium;
  }

  parseRequest(method: APIMethod, request: unknown): object {
    const input = request === undefined || request === null ? {} : request;
    if (!isRecord(input)) {
      throw new InvalidRequestError(
        `The request of ${method.id} must be a JSON object of type ${method.requestType}`,
      );
    }
    try {
      return method.info.I.fromJson(input as JsonValue, {
        ignoreUnknownFields: false,
      }) as object;
    } catch (err) {
      throw new InvalidRequestError(
        `Invalid request for ${method.id} (type ${method.requestType}): ${(err as Error).message}`,
      );
    }
  }

  async invoke(
    method: APIMethod,
    request: unknown,
    opts: InvokeOptions = {},
  ): Promise<InvokeResult> {
    if (!this.transport) {
      throw new APIError(
        "unavailable",
        "The Cordium API is not available to this agent",
      );
    }
    if (!method.invocable) {
      throw new InvalidRequestError(
        `${method.id} is a ${method.kind} method which cannot be invoked by this agent`,
      );
    }

    const input = this.parseRequest(method, request);
    const { info } = method;

    try {
      if (method.kind === "unary") {
        const call = this.transport.unary(
          info,
          input,
          this.transport.mergeOptions({
            abort: opts.signal,
            timeout: opts.timeoutMs ?? this.timeoutMs,
          }),
        );
        const res = await call.response;
        return { response: info.O.toJson(res as never) };
      }

      const maxMessages = opts.maxMessages ?? 50;
      const controller = new AbortController();
      const onAbort = () => controller.abort();
      opts.signal?.addEventListener("abort", onAbort, { once: true });
      const timer = setTimeout(
        () => controller.abort(),
        opts.streamTimeoutMs ?? 10000,
      );

      const messages: JsonValue[] = [];
      try {
        const call = this.transport.serverStreaming(
          info,
          input,
          this.transport.mergeOptions({ abort: controller.signal }),
        );
        void call.status.catch(() => {});
        for await (const message of call.responses) {
          messages.push(info.O.toJson(message as never));
          if (messages.length >= maxMessages) {
            controller.abort();
            break;
          }
        }
      } catch (err) {
        if (!controller.signal.aborted || opts.signal?.aborted) {
          throw err;
        }
      } finally {
        clearTimeout(timer);
        opts.signal?.removeEventListener("abort", onAbort);
      }

      return { response: messages, messageCount: messages.length };
    } catch (err) {
      throw toAPIError(err);
    }
  }

  async call<T = Record<string, unknown>>(
    methodId: string,
    request: unknown = {},
    opts: InvokeOptions = {},
  ): Promise<T> {
    const method = this.catalog.resolveMethod(methodId);
    if (!method) {
      throw new InvalidRequestError(`Unknown method: ${methodId}`);
    }
    const res = await this.invoke(method, request, opts);
    return res.response as T;
  }

  async close() {
    await this.cordium?.close();
    this.ownedTransport?.close();
  }
}
