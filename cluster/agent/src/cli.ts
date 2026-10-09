#!/usr/bin/env node
import { parseArgs } from "node:util";
import { createApp } from "./app.ts";
import { resolveModel } from "./agent/pi/model.ts";
import { APICatalog } from "./api/catalog.ts";
import { APIClient } from "./api/client.ts";
import {
  ConfigError,
  loadConfig,
  type Config,
  type ConfigInput,
} from "./config.ts";
import { logger, setLogLevel } from "./log.ts";
import { syncSkills } from "./skills.ts";
import { getVersion } from "./version.ts";

const usage = `Usage: cordium-agent <command> [options]

Commands:
  serve                      Run the agent HTTP server (default)
  doctor                     Check the configuration, the Cordium API and the LLM access
  api search <query...>      Search the API catalog
  api describe <method|type> Describe an API method or type
  api call <method> [json]   Invoke an API method
  config                     Print the effective configuration
  version                    Print the version

Options:
  -c, --config <path>        Path to the JSON config file
  -l, --listen <host:port>   The HTTP listen address (default 0.0.0.0:8080)
      --data-dir <path>      The data directory
      --work-dir <path>      The agent's working directory
      --log-level <level>    debug, info, warn or error
  -h, --help                 Show this help
`;

const parseListen = (value: string): ConfigInput["server"] => {
  const idx = value.lastIndexOf(":");
  if (idx < 0) {
    return { port: Number(value) };
  }
  const host = value.slice(0, idx).replace(/^\[|\]$/g, "");
  return { host: host || undefined, port: Number(value.slice(idx + 1)) };
};

const redactConfig = (config: Config) => ({
  ...config,
  server: {
    ...config.server,
    authToken: config.server.authToken ? "<redacted>" : undefined,
  },
  octelium: {
    ...config.octelium,
    accessToken: config.octelium.accessToken ? "<redacted>" : undefined,
  },
  llm: { ...config.llm, apiKey: config.llm.apiKey ? "<redacted>" : undefined },
  webSearch: config.webSearch
    ? {
        ...config.webSearch,
        apiKey: config.webSearch.apiKey ? "<redacted>" : undefined,
      }
    : undefined,
});

const serve = async (config: Config) => {
  const app = await createApp({ config });
  const addr = await app.start();
  logger.info("The Cordium agent is listening", {
    address: `${addr.address}:${addr.port}`,
    version: getVersion(),
  });

  let closing = false;
  const shutdown = async (signal: string) => {
    if (closing) {
      return;
    }
    closing = true;
    logger.info("Shutting down", { signal });
    try {
      await app.close();
    } finally {
      process.exit(0);
    }
  };
  process.on("SIGINT", () => void shutdown("SIGINT"));
  process.on("SIGTERM", () => void shutdown("SIGTERM"));
};

const doctor = async (config: Config): Promise<boolean> => {
  let ok = true;
  const report = (passed: boolean, msg: string) => {
    if (!passed) {
      ok = false;
    }
    process.stdout.write(`${passed ? "✓" : "✗"} ${msg}\n`);
  };

  let catalog: APICatalog;
  try {
    catalog = APICatalog.load();
    report(true, `Loaded the API catalog (${catalog.methods.length} methods)`);
  } catch (err) {
    report(false, `Could not load the API catalog: ${(err as Error).message}`);
    return false;
  }

  const { client, reason } = APIClient.fromConfig(catalog, config.octelium);
  try {
    if (reason) {
      report(false, `Cordium API: ${reason}`);
    } else {
      try {
        const status = await client.call<{
          user?: { metadata?: { name?: string } };
        }>("user.v1.MainService/GetStatus", {}, { timeoutMs: 15000 });
        report(
          true,
          `Cordium API (${client.mode} mode): authenticated as ${status.user?.metadata?.name ?? "<unknown>"}`,
        );
        const page = await client
          .requireCordium()
          .workspaces.list({ pageSize: 1 }, { timeoutMs: 15000 });
        report(
          true,
          `Cordium Workspaces: ${page.page.totalCount ?? page.items.length} owned by the User`,
        );
      } catch (err) {
        report(
          false,
          `Cordium API (${client.mode} mode): ${(err as Error).message}`,
        );
      }
    }

    const { ModelRuntime } = await import("@earendil-works/pi-coding-agent");
    process.env.PI_OFFLINE ??= "1";
    process.env.PI_TELEMETRY ??= "0";
    try {
      const modelRuntime = await ModelRuntime.create({
        modelsPath: null,
        allowModelNetwork: false,
        refreshOnCreate: false,
      });
      const resolved = await resolveModel(config.llm, {
        modelRuntime,
        octelium: client,
        domain: config.octelium.domain,
        logger,
      });
      report(
        true,
        `LLM model: ${resolved.model.provider}/${resolved.model.id} (${resolved.model.api}${resolved.baseUrl ? ` at ${resolved.baseUrl}` : ""})`,
      );
      const res = await modelRuntime.completeSimple(
        resolved.model,
        {
          messages: [
            {
              role: "user",
              content: "Reply with the single word OK.",
              timestamp: Date.now(),
            },
          ],
        },
        { maxTokens: 256, signal: AbortSignal.timeout(60000) },
      );
      if (res.stopReason === "error" || res.stopReason === "aborted") {
        report(
          false,
          `LLM request failed: ${res.errorMessage ?? res.stopReason}`,
        );
      } else {
        report(true, "LLM request succeeded");
      }
    } catch (err) {
      report(
        false,
        `LLM: ${(err as Error).message}. The Users can still sign in with their own subscriptions from the web portal`,
      );
    }

    const skills = await syncSkills(config, logger);
    report(
      true,
      `Skills paths: ${skills.length > 0 ? skills.join(", ") : "(none)"}`,
    );
  } finally {
    await client.close();
  }

  return ok;
};

const api = async (config: Config, args: string[]): Promise<boolean> => {
  const [sub, ...rest] = args;
  const catalog = APICatalog.load();

  switch (sub) {
    case "search": {
      process.stdout.write(
        `${catalog.formatSearchResults(catalog.search(rest.join(" "), 10))}\n`,
      );
      return true;
    }
    case "describe": {
      const name = rest[0] ?? "";
      const method = catalog.resolveMethod(name);
      if (method) {
        process.stdout.write(`${catalog.describeMethod(method)}\n`);
        return true;
      }
      const type = catalog.resolveType(name);
      if (type) {
        process.stdout.write(`${catalog.describeType(type)}\n`);
        return true;
      }
      process.stderr.write(`Unknown method or type: ${name}\n`);
      return false;
    }
    case "call": {
      const method = catalog.resolveMethod(rest[0] ?? "");
      if (!method) {
        process.stderr.write(`Unknown method: ${rest[0] ?? ""}\n`);
        return false;
      }
      const { client, reason } = APIClient.fromConfig(catalog, config.octelium);
      if (reason) {
        process.stderr.write(`${reason}\n`);
        return false;
      }
      try {
        const res = await client.invoke(
          method,
          rest[1] ? JSON.parse(rest[1]) : {},
        );
        process.stdout.write(`${JSON.stringify(res.response, null, 2)}\n`);
      } finally {
        await client.close();
      }
      return true;
    }
    default:
      process.stderr.write(usage);
      return false;
  }
};

const main = async () => {
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options: {
      config: { type: "string", short: "c" },
      listen: { type: "string", short: "l" },
      "data-dir": { type: "string" },
      "work-dir": { type: "string" },
      "log-level": { type: "string" },
      help: { type: "boolean", short: "h" },
    },
  });

  if (values.help) {
    process.stdout.write(usage);
    return;
  }

  const [command = "serve", ...args] = positionals;
  if (command === "version") {
    process.stdout.write(`${getVersion()}\n`);
    return;
  }

  const overrides: ConfigInput = {
    dataDir: values["data-dir"],
    workDir: values["work-dir"],
    logLevel: values["log-level"] as ConfigInput["logLevel"],
    server: values.listen ? parseListen(values.listen) : undefined,
  };

  const config = loadConfig({ configPath: values.config, overrides });
  setLogLevel(config.logLevel);

  switch (command) {
    case "serve":
      await serve(config);
      return;
    case "doctor":
      process.exitCode = (await doctor(config)) ? 0 : 1;
      return;
    case "api":
      process.exitCode = (await api(config, args)) ? 0 : 1;
      return;
    case "config":
      process.stdout.write(
        `${JSON.stringify(redactConfig(config), null, 2)}\n`,
      );
      return;
    default:
      process.stderr.write(usage);
      process.exitCode = 1;
  }
};

main().catch((err) => {
  if (err instanceof ConfigError) {
    process.stderr.write(`${err.message}\n`);
  } else {
    logger.error("Fatal error", { error: err as Error });
    process.stderr.write(`${(err as Error).stack ?? String(err)}\n`);
  }
  process.exit(1);
});
