import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import {
  fauxProvider,
  type FauxProviderHandle,
  type FauxResponseStep,
} from "@earendil-works/pi-ai";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { createApp, type App } from "../app.ts";
import {
  mergeConfigInputs,
  resolveConfig,
  type ConfigInput,
} from "../config.ts";
import { setLogLevel } from "../log.ts";
import { APICatalog } from "../api/catalog.ts";
import type { AgentEvent } from "../protocol/index.ts";
import {
  startMockAPI,
  type MockAPI,
  type MockAPIOptions,
  type MockHandler,
} from "./mockapi.ts";

let catalog: APICatalog | undefined;

export const getCatalog = (): APICatalog => {
  catalog ??= APICatalog.load();
  return catalog;
};

export interface TestApp {
  app: App;
  mock: MockAPI;
  faux: FauxProviderHandle;
  modelRuntime: ModelRuntime;
  baseUrl: string;
  root: string;
  close(): Promise<void>;
}

export interface TestAppOptions {
  handlers?: Record<string, MockHandler>;
  responses?: FauxResponseStep[];
  config?: ConfigInput;
  root?: string;
  mock?: MockAPIOptions;
  env?: Record<string, string | undefined>;
}

export const defaultHandlers: Record<string, MockHandler> = {
  "user.v1.MainService/GetStatus": () => ({
    domain: "example.com",
    user: { metadata: { name: "alice", uid: "u-1" } },
  }),
};

export const createTestApp = async (
  opts: TestAppOptions = {},
): Promise<TestApp> => {
  setLogLevel("error");
  const root =
    opts.root ?? fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-test-"));
  const mock = await startMockAPI(
    {
      ...defaultHandlers,
      ...opts.handlers,
    },
    opts.mock,
  );

  const modelRuntime = await ModelRuntime.create({
    authPath: path.join(root, "auth.json"),
    modelsPath: null,
    allowModelNetwork: false,
    refreshOnCreate: false,
  });
  const faux = fauxProvider({
    provider: "faux",
    models: [{ id: "faux-1", input: ["text", "image"] }, { id: "faux-2" }],
  });
  modelRuntime.registerNativeProvider(faux.provider);
  faux.setResponses(opts.responses ?? []);

  const config = resolveConfig(
    mergeConfigInputs(
      {
        dataDir: path.join(root, "data"),
        workDir: path.join(root, "work"),
        server: { host: "127.0.0.1", port: 0 },
        octelium: {
          mode: "proxy",
          authProxySocket: mock.socketPath,
          domain: "example.com",
        },
        llm: { provider: "faux", model: "faux-1" },
        skills: { enabled: false },
        agent: { generateTitles: false },
      },
      opts.config,
    ),
    {},
  );

  const app = await createApp({
    config,
    catalog: getCatalog(),
    modelRuntime,
    model: faux.getModel(),
    skipSkills: true,
    env: opts.env ?? {},
  });
  const addr = await app.start();

  return {
    app,
    mock,
    faux,
    modelRuntime,
    root,
    baseUrl: `http://127.0.0.1:${addr.port}`,
    close: async () => {
      await app.close();
      await mock.close();
      if (!opts.root) {
        fs.rmSync(root, { recursive: true, force: true });
      }
    },
  };
};

export interface SSEMessage {
  id?: string;
  event?: string;
  data: string;
}

export async function* readSSE(res: Response): AsyncGenerator<SSEMessage> {
  if (!res.body) {
    return;
  }
  const decoder = new TextDecoder();
  let buffer = "";
  for await (const chunk of res.body) {
    buffer += decoder.decode(chunk, { stream: true });
    let idx: number;
    while ((idx = buffer.indexOf("\n\n")) >= 0) {
      const raw = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      const msg: SSEMessage = { data: "" };
      const data: string[] = [];
      for (const line of raw.split("\n")) {
        if (line.startsWith(":")) {
          continue;
        }
        const sep = line.indexOf(":");
        const field = sep < 0 ? line : line.slice(0, sep);
        const value = sep < 0 ? "" : line.slice(sep + 1).replace(/^ /, "");
        if (field === "id") {
          msg.id = value;
        } else if (field === "event") {
          msg.event = value;
        } else if (field === "data") {
          data.push(value);
        }
      }
      if (data.length > 0) {
        msg.data = data.join("\n");
        yield msg;
      }
    }
  }
}

export const collectEvents = async (
  baseUrl: string,
  runId: string,
  onEvent?: (event: AgentEvent) => void | Promise<void>,
  after = 0,
): Promise<AgentEvent[]> => {
  const res = await fetch(`${baseUrl}/v1/runs/${runId}/events?after=${after}`);
  if (res.status === 204) {
    return [];
  }
  if (res.status !== 200) {
    throw new Error(`Unexpected status ${res.status}: ${await res.text()}`);
  }
  const events: AgentEvent[] = [];
  for await (const msg of readSSE(res)) {
    const event = JSON.parse(msg.data) as AgentEvent;
    events.push(event);
    await onEvent?.(event);
  }
  return events;
};

export const postJSON = async (url: string, body: unknown, method = "POST") =>
  fetch(url, {
    method,
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
