import type { Api, Model } from "@earendil-works/pi-ai";
import type { ModelRuntime } from "@earendil-works/pi-coding-agent";
import type { Config } from "../../config.ts";
import type { Logger } from "../../log.ts";
import type { APIClient } from "../../api/client.ts";
import type { ModelInfo } from "../../protocol/index.ts";

export const OCTELIUM_PROVIDER = "octelium";
export const CUSTOM_PROVIDER = "custom";

export const DEFAULT_OCTELIUM_API = "openai-completions";

export class ModelResolutionError extends Error {}

export interface ResolvedModel {
  model: Model<Api>;
  info: ModelInfo;
  baseUrl?: string;
}

export interface ResolveModelDeps {
  modelRuntime: ModelRuntime;
  octelium?: APIClient;
  domain?: string;
  logger: Logger;
  fetchImpl?: typeof fetch;
}

export const toModelInfo = (model: Model<Api>): ModelInfo => ({
  provider: model.provider,
  id: model.id,
  name: model.name,
  api: model.api,
  reasoning: model.reasoning,
  input: model.input,
  contextWindow: model.contextWindow,
});

export const parseServiceName = (
  service: string,
): { name: string; namespace: string } => {
  const [name, namespace] = service.split(".");
  return { name, namespace: namespace || "default" };
};

export const defaultServiceURL = (service: string, domain?: string): string => {
  const { name, namespace } = parseServiceName(service);
  return domain
    ? `http://${name}.${namespace}.local.${domain}`
    : `http://${namespace === "default" ? name : `${name}.${namespace}`}`;
};

export const apiBasePath = (api: string): string => {
  if (api.startsWith("openai-") || api === "mistral-conversations") {
    return "/v1";
  }
  if (api === "google-generative-ai") {
    return "/v1beta";
  }
  return "";
};

export const withBasePath = (baseUrl: string, api: string): string => {
  const url = new URL(baseUrl);
  if (url.pathname === "/" || url.pathname === "") {
    url.pathname = apiBasePath(api);
  }
  return url.toString().replace(/\/$/, "");
};

interface UserService {
  metadata?: { name?: string };
  spec?: { port?: number; isTLS?: boolean };
  status?: { primaryHostname?: string; namespace?: string };
}

export const serviceURLFromUserService = (
  svc: UserService,
  service: string,
  domain?: string,
): string => {
  const { name, namespace } = parseServiceName(service);
  const isTLS = !!svc.spec?.isTLS;
  const primary = svc.status?.primaryHostname || `${name}.${namespace}`;
  const host =
    domain && !primary.endsWith(`.${domain}`)
      ? `${primary}.local.${domain}`
      : primary;
  const port = svc.spec?.port;
  const defaultPort = isTLS ? 443 : 80;
  return `${isTLS ? "https" : "http"}://${host}${port && port !== defaultPort ? `:${port}` : ""}`;
};

const resolveServiceURL = async (
  service: string,
  deps: ResolveModelDeps,
): Promise<string> => {
  const { name, namespace } = parseServiceName(service);
  if (deps.octelium?.enabled) {
    try {
      const svc = await deps.octelium.call<UserService>(
        "user.v1.MainService/GetService",
        { name: `${name}.${namespace}` },
        { timeoutMs: 10000 },
      );
      return serviceURLFromUserService(svc, service, deps.domain);
    } catch (err) {
      deps.logger.warn(
        "Could not get the LLM Service info. Using its default URL",
        {
          service,
          error: err as Error,
        },
      );
    }
  }
  return defaultServiceURL(service, deps.domain);
};

const findCatalogModel = (
  modelRuntime: ModelRuntime,
  id: string,
  api: string,
): Model<Api> | undefined => {
  const candidates = [
    id,
    id.includes("/") ? id.slice(id.lastIndexOf("/") + 1) : id,
  ];
  const all = modelRuntime.getModels();
  for (const candidate of candidates) {
    const matches = all.filter((m) => m.id === candidate);
    const match = matches.find((m) => m.api === api) ?? matches[0];
    if (match) {
      return match;
    }
  }
  return undefined;
};

const discoverModelID = async (
  baseUrl: string,
  api: string,
  apiKey: string | undefined,
  fetchImpl: typeof fetch,
): Promise<string | undefined> => {
  const headers: Record<string, string> = { accept: "application/json" };
  if (apiKey) {
    if (api === "anthropic-messages") {
      headers["x-api-key"] = apiKey;
      headers["anthropic-version"] = "2023-06-01";
    } else {
      headers.authorization = `Bearer ${apiKey}`;
    }
  }
  const url =
    api === "anthropic-messages" ? `${baseUrl}/v1/models` : `${baseUrl}/models`;
  const res = await fetchImpl(url, {
    headers,
    signal: AbortSignal.timeout(15000),
  });
  if (!res.ok) {
    return undefined;
  }
  const body = (await res.json()) as {
    data?: { id?: string }[];
    models?: { name?: string }[];
  };
  return (
    body.data?.find((m) => typeof m.id === "string")?.id ??
    body.models
      ?.find((m) => typeof m.name === "string")
      ?.name?.replace(/^models\//, "")
  );
};

const resolveAPIKey = (cfg: Config["llm"]): string | undefined =>
  cfg.apiKey ?? (cfg.apiKeyEnv ? process.env[cfg.apiKeyEnv] : undefined);

const registerGatewayModel = async (
  provider: string,
  cfg: Config["llm"],
  baseUrl: string,
  apiKey: string,
  deps: ResolveModelDeps,
): Promise<ResolvedModel> => {
  const api = cfg.api ?? DEFAULT_OCTELIUM_API;
  const fullBaseUrl = withBasePath(baseUrl, api);

  let modelID = cfg.model;
  if (!modelID) {
    try {
      modelID = await discoverModelID(
        fullBaseUrl,
        api,
        apiKey,
        deps.fetchImpl ?? fetch,
      );
    } catch (err) {
      deps.logger.warn("Could not discover the LLM models", {
        baseUrl: fullBaseUrl,
        error: err as Error,
      });
    }
  }
  if (!modelID) {
    throw new ModelResolutionError(
      `No model is set (llm.model) and it could not be discovered from ${fullBaseUrl}`,
    );
  }

  const known = findCatalogModel(deps.modelRuntime, modelID, api);

  deps.modelRuntime.registerProvider(provider, {
    name: provider === OCTELIUM_PROVIDER ? "Octelium" : provider,
    baseUrl: fullBaseUrl,
    api: api as Api,
    apiKey,
    headers: cfg.headers,
    models: [
      {
        id: modelID,
        name: known?.name ?? modelID,
        api: api as Api,
        reasoning: cfg.reasoning ?? known?.reasoning ?? false,
        thinkingLevelMap: known?.thinkingLevelMap,
        input: cfg.input ?? known?.input ?? ["text"],
        contextWindow: cfg.contextWindow ?? known?.contextWindow ?? 128000,
        maxTokens: cfg.maxTokens ?? known?.maxTokens ?? 16384,
        cost: known?.cost ?? {
          input: 0,
          output: 0,
          cacheRead: 0,
          cacheWrite: 0,
        },
      },
    ],
  });

  const model = deps.modelRuntime.getModel(provider, modelID);
  if (!model) {
    throw new ModelResolutionError(
      `Could not register the model ${provider}/${modelID}`,
    );
  }
  return { model, info: toModelInfo(model), baseUrl: fullBaseUrl };
};

export const resolveModel = async (
  cfg: Config["llm"],
  deps: ResolveModelDeps,
): Promise<ResolvedModel> => {
  const provider = cfg.provider;

  if (provider === OCTELIUM_PROVIDER) {
    if (!cfg.service && !cfg.baseUrl) {
      throw new ModelResolutionError(
        'The "octelium" LLM provider requires either llm.service (the name of the LLM Service) or llm.baseUrl',
      );
    }
    const baseUrl =
      cfg.baseUrl ?? (await resolveServiceURL(cfg.service!, deps));
    return registerGatewayModel(
      OCTELIUM_PROVIDER,
      cfg,
      baseUrl,
      resolveAPIKey(cfg) ?? "octelium",
      deps,
    );
  }

  if (provider === CUSTOM_PROVIDER) {
    if (!cfg.baseUrl) {
      throw new ModelResolutionError(
        'The "custom" LLM provider requires llm.baseUrl',
      );
    }
    return registerGatewayModel(
      CUSTOM_PROVIDER,
      cfg,
      cfg.baseUrl,
      resolveAPIKey(cfg) ?? "none",
      deps,
    );
  }

  if (!cfg.model) {
    throw new ModelResolutionError(
      `The llm.model field is required for the "${provider}" provider`,
    );
  }

  if (cfg.baseUrl || cfg.headers) {
    deps.modelRuntime.registerProvider(provider, {
      baseUrl: cfg.baseUrl,
      headers: cfg.headers,
    });
  }

  const apiKey = resolveAPIKey(cfg);
  if (apiKey) {
    await deps.modelRuntime.setRuntimeApiKey(provider, apiKey);
  }

  const model = deps.modelRuntime.getModel(provider, cfg.model);
  if (!model) {
    const available = deps.modelRuntime
      .getModels(provider)
      .map((m) => m.id)
      .slice(0, 30);
    throw new ModelResolutionError(
      available.length > 0
        ? `Unknown model ${provider}/${cfg.model}. Some available models: ${available.join(", ")}`
        : `Unknown LLM provider "${provider}"`,
    );
  }

  return { model, info: toModelInfo(model) };
};
