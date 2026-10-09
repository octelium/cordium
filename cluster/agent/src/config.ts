import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { Type, type Static } from "typebox";
import Value from "typebox/value";

export const THINKING_LEVELS = [
  "off",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
] as const;

export type ThinkingLevel = (typeof THINKING_LEVELS)[number];

export const BUILTIN_TOOLS = [
  "read",
  "bash",
  "edit",
  "write",
  "grep",
  "find",
  "ls",
] as const;

export const DEFAULT_BUILTIN_TOOLS: string[] = [...BUILTIN_TOOLS];

export const APPROVAL_MODES = ["never", "destructive", "write"] as const;

export type ApprovalMode = (typeof APPROVAL_MODES)[number];

export const DEFAULT_SKILLS_REPOSITORY =
  "https://github.com/octelium/octelium-skills";

export const DEFAULT_LOGIN_PROVIDERS = ["anthropic", "openai"];

const ConfigInputSchema = Type.Object(
  {
    dataDir: Type.Optional(Type.String()),
    workDir: Type.Optional(Type.String()),
    logLevel: Type.Optional(Type.Enum(["debug", "info", "warn", "error"])),
    server: Type.Optional(
      Type.Object(
        {
          host: Type.Optional(Type.String()),
          port: Type.Optional(Type.Integer({ minimum: 0, maximum: 65535 })),
          allowedOrigins: Type.Optional(Type.Array(Type.String())),
          cors: Type.Optional(Type.Boolean()),
          authToken: Type.Optional(Type.String()),
          maxUploadBytes: Type.Optional(Type.Integer({ minimum: 1 })),
        },
        { additionalProperties: false },
      ),
    ),
    octelium: Type.Optional(
      Type.Object(
        {
          mode: Type.Optional(
            Type.Enum(["auto", "proxy", "direct", "disabled"]),
          ),
          domain: Type.Optional(Type.String()),
          authProxySocket: Type.Optional(Type.String()),
          accessToken: Type.Optional(Type.String()),
          timeoutSeconds: Type.Optional(Type.Integer({ minimum: 1 })),
        },
        { additionalProperties: false },
      ),
    ),
    llm: Type.Optional(
      Type.Object(
        {
          provider: Type.Optional(Type.String({ minLength: 1 })),
          service: Type.Optional(Type.String()),
          baseUrl: Type.Optional(Type.String()),
          api: Type.Optional(Type.String()),
          model: Type.Optional(Type.String()),
          apiKey: Type.Optional(Type.String()),
          apiKeyEnv: Type.Optional(Type.String()),
          headers: Type.Optional(Type.Record(Type.String(), Type.String())),
          thinkingLevel: Type.Optional(Type.Enum([...THINKING_LEVELS])),
          reasoning: Type.Optional(Type.Boolean()),
          contextWindow: Type.Optional(Type.Integer({ minimum: 1 })),
          maxTokens: Type.Optional(Type.Integer({ minimum: 1 })),
          input: Type.Optional(Type.Array(Type.Enum(["text", "image"]))),
          loginProviders: Type.Optional(Type.Array(Type.String())),
        },
        { additionalProperties: false },
      ),
    ),
    agent: Type.Optional(
      Type.Object(
        {
          tools: Type.Optional(Type.Array(Type.Enum([...BUILTIN_TOOLS]))),
          systemPromptAppend: Type.Optional(Type.String()),
          loadContextFiles: Type.Optional(Type.Boolean()),
          maxConcurrentRuns: Type.Optional(Type.Integer({ minimum: 1 })),
          sessionIdleTimeoutSeconds: Type.Optional(
            Type.Integer({ minimum: 1 }),
          ),
          compaction: Type.Optional(Type.Boolean()),
          maxRetries: Type.Optional(Type.Integer({ minimum: 0 })),
          generateTitles: Type.Optional(Type.Boolean()),
        },
        { additionalProperties: false },
      ),
    ),
    approvals: Type.Optional(
      Type.Object(
        {
          api: Type.Optional(Type.Enum([...APPROVAL_MODES])),
          commands: Type.Optional(Type.Boolean()),
        },
        { additionalProperties: false },
      ),
    ),
    skills: Type.Optional(
      Type.Object(
        {
          enabled: Type.Optional(Type.Boolean()),
          paths: Type.Optional(Type.Array(Type.String())),
          repositories: Type.Optional(
            Type.Array(
              Type.Object(
                {
                  url: Type.String({ minLength: 1 }),
                  ref: Type.Optional(Type.String()),
                  path: Type.Optional(Type.String()),
                },
                { additionalProperties: false },
              ),
            ),
          ),
        },
        { additionalProperties: false },
      ),
    ),
    webSearch: Type.Optional(
      Type.Object(
        {
          provider: Type.Enum(["brave", "tavily", "searxng"]),
          baseUrl: Type.Optional(Type.String()),
          apiKey: Type.Optional(Type.String()),
          apiKeyEnv: Type.Optional(Type.String()),
        },
        { additionalProperties: false },
      ),
    ),
  },
  { additionalProperties: false },
);

export type ConfigInput = Static<typeof ConfigInputSchema>;

export interface SkillsRepository {
  url: string;
  ref?: string;
  path?: string;
}

export interface WebSearchConfig {
  provider: "brave" | "tavily" | "searxng";
  baseUrl?: string;
  apiKey?: string;
  apiKeyEnv?: string;
}

export interface ApprovalsConfig {
  api: ApprovalMode;
  commands: boolean;
}

export interface Config {
  dataDir: string;
  workDir: string;
  logLevel: "debug" | "info" | "warn" | "error";
  server: {
    host: string;
    port: number;
    allowedOrigins: string[];
    cors: boolean;
    authToken?: string;
    maxUploadBytes: number;
  };
  octelium: {
    mode: "auto" | "proxy" | "direct" | "disabled";
    domain?: string;
    authProxySocket?: string;
    accessToken?: string;
    timeoutSeconds: number;
  };
  llm: {
    provider: string;
    service?: string;
    baseUrl?: string;
    api?: string;
    model?: string;
    apiKey?: string;
    apiKeyEnv?: string;
    headers?: Record<string, string>;
    thinkingLevel: ThinkingLevel;
    reasoning?: boolean;
    contextWindow?: number;
    maxTokens?: number;
    input?: ("text" | "image")[];
    loginProviders: string[];
  };
  agent: {
    tools: string[];
    systemPromptAppend?: string;
    loadContextFiles: boolean;
    maxConcurrentRuns: number;
    sessionIdleTimeoutSeconds: number;
    compaction: boolean;
    maxRetries: number;
    generateTitles: boolean;
  };
  approvals: ApprovalsConfig;
  skills: {
    enabled: boolean;
    paths: string[];
    repositories: SkillsRepository[];
  };
  webSearch?: WebSearchConfig;
}

export class ConfigError extends Error {}

type Env = Record<string, string | undefined>;

export const expandHome = (p: string): string => {
  if (p === "~") {
    return os.homedir();
  }
  if (p.startsWith("~/")) {
    return path.join(os.homedir(), p.slice(2));
  }
  return p;
};

const isObject = (arg: unknown): arg is Record<string, unknown> =>
  typeof arg === "object" && arg !== null && !Array.isArray(arg);

export const mergeConfigInputs = (
  ...inputs: (ConfigInput | undefined)[]
): ConfigInput => {
  const merge = (
    target: Record<string, unknown>,
    source: Record<string, unknown>,
  ) => {
    for (const [key, value] of Object.entries(source)) {
      if (value === undefined) {
        continue;
      }
      const current = target[key];
      if (isObject(current) && isObject(value) && key !== "headers") {
        target[key] = merge({ ...current }, value);
      } else {
        target[key] = value;
      }
    }
    return target;
  };

  return inputs.reduce<Record<string, unknown>>(
    (acc, input) => (input ? merge(acc, input) : acc),
    {},
  ) as ConfigInput;
};

export const validateConfigInput = (
  input: unknown,
  source: string,
): ConfigInput => {
  if (Value.Check(ConfigInputSchema, input)) {
    return input;
  }

  const errors = [...Value.Errors(ConfigInputSchema, input)]
    .filter((err) => err.keyword !== "boolean")
    .slice(0, 8)
    .map((err) => {
      const location = err.instancePath === "" ? "/" : err.instancePath;
      const extra =
        err.keyword === "additionalProperties"
          ? ` (${(err.params as { additionalProperties?: string[] }).additionalProperties?.join(", ")})`
          : "";
      return `${location}: ${err.message}${extra}`;
    });

  throw new ConfigError(`Invalid config in ${source}: ${errors.join("; ")}`);
};

export const readConfigFile = (filePath: string): ConfigInput => {
  let raw: string;
  try {
    raw = fs.readFileSync(filePath, "utf8");
  } catch (err) {
    throw new ConfigError(
      `Could not read config file ${filePath}: ${(err as Error).message}`,
    );
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (err) {
    throw new ConfigError(
      `Could not parse config file ${filePath}: ${(err as Error).message}`,
    );
  }

  return validateConfigInput(parsed, filePath);
};

export const getDefaultDataDir = (env: Env = process.env): string =>
  env.CORDIUM_AGENT_DATA_DIR
    ? expandHome(env.CORDIUM_AGENT_DATA_DIR)
    : path.join(os.homedir(), ".cordium-agent");

const getDefaultWorkDir = (): string =>
  fs.existsSync("/workspace") ? "/workspace" : os.homedir();

export const getPortalOrigin = (domain: string): string =>
  `https://cordium.${domain}`;

export const getRegionPortalOrigin = (
  hostname: string,
  domain?: string,
): string | undefined => {
  const idx = hostname.indexOf(".");
  if (idx < 0) {
    return undefined;
  }
  const parent = hostname.slice(idx + 1);
  if (domain && !parent.endsWith(`cordium.${domain}`)) {
    return undefined;
  }
  return `https://${parent}`;
};

export const resolveConfig = (
  input: ConfigInput,
  env: Env = process.env,
): Config => {
  const domain = input.octelium?.domain ?? env.OCTELIUM_DOMAIN;

  const allowedOrigins = [...(input.server?.allowedOrigins ?? [])];
  if (domain) {
    allowedOrigins.push(getPortalOrigin(domain));
  }
  if (env.CORDIUM_HOSTNAME) {
    const regionOrigin = getRegionPortalOrigin(env.CORDIUM_HOSTNAME, domain);
    if (regionOrigin) {
      allowedOrigins.push(regionOrigin);
    }
    allowedOrigins.push(`https://${env.CORDIUM_HOSTNAME}`);
  }

  const llm = input.llm ?? {};

  return {
    dataDir: expandHome(input.dataDir ?? getDefaultDataDir(env)),
    workDir: expandHome(input.workDir ?? getDefaultWorkDir()),
    logLevel:
      input.logLevel ??
      (env.CORDIUM_AGENT_LOG_LEVEL as Config["logLevel"]) ??
      "info",
    server: {
      host: input.server?.host ?? "0.0.0.0",
      port: input.server?.port ?? 8080,
      allowedOrigins: [...new Set(allowedOrigins)],
      cors: input.server?.cors ?? false,
      authToken: input.server?.authToken,
      maxUploadBytes: input.server?.maxUploadBytes ?? 100 * 1024 * 1024,
    },
    octelium: {
      mode: input.octelium?.mode ?? "auto",
      domain,
      authProxySocket:
        input.octelium?.authProxySocket ?? env.OCTELIUM_AUTH_PROXY_SOCKET,
      accessToken: input.octelium?.accessToken ?? env.OCTELIUM_ACCESS_TOKEN,
      timeoutSeconds: input.octelium?.timeoutSeconds ?? 30,
    },
    llm: {
      ...llm,
      provider: llm.provider ?? "octelium",
      thinkingLevel: llm.thinkingLevel ?? "medium",
      loginProviders: llm.loginProviders ?? [...DEFAULT_LOGIN_PROVIDERS],
    },
    agent: {
      tools: input.agent?.tools ?? [...DEFAULT_BUILTIN_TOOLS],
      systemPromptAppend: input.agent?.systemPromptAppend,
      loadContextFiles: input.agent?.loadContextFiles ?? false,
      maxConcurrentRuns: input.agent?.maxConcurrentRuns ?? 4,
      sessionIdleTimeoutSeconds: input.agent?.sessionIdleTimeoutSeconds ?? 1800,
      compaction: input.agent?.compaction ?? true,
      maxRetries: input.agent?.maxRetries ?? 3,
      generateTitles: input.agent?.generateTitles ?? true,
    },
    approvals: {
      api: input.approvals?.api ?? "destructive",
      commands: input.approvals?.commands ?? false,
    },
    skills: {
      enabled: input.skills?.enabled ?? true,
      paths: (input.skills?.paths ?? []).map(expandHome),
      repositories: input.skills?.repositories ?? [
        { url: DEFAULT_SKILLS_REPOSITORY, ref: "main" },
      ],
    },
    webSearch: input.webSearch,
  };
};

export interface LoadConfigOptions {
  configPath?: string;
  overrides?: ConfigInput;
  env?: Env;
}

export const loadConfig = (opts: LoadConfigOptions = {}): Config => {
  const env = opts.env ?? process.env;

  let configPath = opts.configPath ?? env.CORDIUM_AGENT_CONFIG;
  if (!configPath) {
    const defaultPath = path.join(
      expandHome(opts.overrides?.dataDir ?? getDefaultDataDir(env)),
      "config.json",
    );
    if (fs.existsSync(defaultPath)) {
      configPath = defaultPath;
    }
  }

  const fileInput = configPath
    ? readConfigFile(expandHome(configPath))
    : undefined;

  let envInput: ConfigInput | undefined;
  if (env.CORDIUM_AGENT_CONFIG_JSON) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(env.CORDIUM_AGENT_CONFIG_JSON);
    } catch (err) {
      throw new ConfigError(
        `Could not parse CORDIUM_AGENT_CONFIG_JSON: ${(err as Error).message}`,
      );
    }
    envInput = validateConfigInput(parsed, "CORDIUM_AGENT_CONFIG_JSON");
  }

  const overrides = opts.overrides
    ? validateConfigInput(opts.overrides, "overrides")
    : undefined;

  return resolveConfig(mergeConfigInputs(fileInput, envInput, overrides), env);
};
