import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { after, before, describe, it } from "node:test";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { resolveConfig, type ConfigInput } from "../../config.ts";
import { logger, setLogLevel } from "../../log.ts";
import { APIClient } from "../../api/client.ts";
import { getCatalog } from "../../testutil/app.ts";
import { startMockAPI, type MockAPI } from "../../testutil/mockapi.ts";
import {
  defaultServiceURL,
  ModelResolutionError,
  resolveModel,
  serviceURLFromUserService,
  withBasePath,
} from "./model.ts";

const llmConfig = (llm: ConfigInput["llm"]) => resolveConfig({ llm }, {}).llm;

describe("model resolution", () => {
  let dir: string;
  let mock: MockAPI;
  let octelium: APIClient;

  const newRuntime = () =>
    ModelRuntime.create({
      authPath: path.join(dir, `auth-${Math.random()}.json`),
      modelsPath: null,
      allowModelNetwork: false,
      refreshOnCreate: false,
    });

  before(async () => {
    setLogLevel("error");
    process.env.PI_OFFLINE = "1";
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-model-"));
    mock = await startMockAPI({
      "user.v1.MainService/GetService": (req) => {
        if ((req as { name: string }).name !== "llm.ai") {
          throw new Error("not found");
        }
        return {
          metadata: { name: "llm.ai" },
          spec: { port: 8080, type: "LLM" },
          status: { primaryHostname: "llm.ai" },
        };
      },
    });
    octelium = new APIClient(getCatalog(), {
      mode: "proxy",
      authProxySocket: mock.socketPath,
    });
  });

  after(async () => {
    await octelium.close();
    await mock.close();
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("derives the URLs of the Services", () => {
    assert.equal(
      defaultServiceURL("llm", "example.com"),
      "http://llm.default.local.example.com",
    );
    assert.equal(defaultServiceURL("llm.ai"), "http://llm.ai");
    assert.equal(defaultServiceURL("llm"), "http://llm");
    assert.equal(
      serviceURLFromUserService(
        {
          spec: { port: 443, isTLS: true },
          status: { primaryHostname: "llm.local.example.com" },
        },
        "llm",
        "example.com",
      ),
      "https://llm.local.example.com",
    );
    assert.equal(
      serviceURLFromUserService(
        { spec: { port: 8080 } },
        "llm.ai",
        "example.com",
      ),
      "http://llm.ai.local.example.com:8080",
    );
    assert.equal(
      serviceURLFromUserService(
        { spec: { port: 80 }, status: { primaryHostname: "llm" } },
        "llm",
        "example.com",
      ),
      "http://llm.local.example.com",
    );
    assert.equal(
      serviceURLFromUserService(
        { spec: { port: 80 }, status: { primaryHostname: "llm" } },
        "llm",
      ),
      "http://llm",
    );
    assert.equal(
      withBasePath("http://llm", "openai-completions"),
      "http://llm/v1",
    );
    assert.equal(
      withBasePath("http://llm/", "openai-responses"),
      "http://llm/v1",
    );
    assert.equal(
      withBasePath("http://llm/custom/v2", "openai-completions"),
      "http://llm/custom/v2",
    );
    assert.equal(
      withBasePath("http://llm", "anthropic-messages"),
      "http://llm",
    );
    assert.equal(
      withBasePath("http://llm", "google-generative-ai"),
      "http://llm/v1beta",
    );
  });

  it("resolves an Octelium LLM Service through the User API", async () => {
    const modelRuntime = await newRuntime();
    const resolved = await resolveModel(
      llmConfig({
        service: "llm.ai",
        model: "claude-sonnet-4-5",
        api: "anthropic-messages",
      }),
      { modelRuntime, octelium, domain: "example.com", logger },
    );
    assert.equal(resolved.baseUrl, "http://llm.ai.local.example.com:8080");
    assert.equal(resolved.model.provider, "octelium");
    assert.equal(resolved.model.id, "claude-sonnet-4-5");
    assert.equal(resolved.model.api, "anthropic-messages");
    assert.equal(
      resolved.model.baseUrl,
      "http://llm.ai.local.example.com:8080",
    );
    assert.equal(resolved.model.reasoning, true);
    assert.ok(resolved.model.contextWindow >= 200000);
    assert.equal(resolved.info.provider, "octelium");
  });

  it("falls back to the default Service URL and discovers the model", async () => {
    const modelRuntime = await newRuntime();
    const urls: string[] = [];
    const fetchImpl = (async (url: string | URL) => {
      urls.push(url.toString());
      return new Response(JSON.stringify({ data: [{ id: "my-model" }] }));
    }) as typeof fetch;
    const resolved = await resolveModel(llmConfig({ service: "gateway" }), {
      modelRuntime,
      octelium,
      domain: "example.com",
      logger,
      fetchImpl,
    });
    assert.deepEqual(urls, [
      "http://gateway.default.local.example.com/v1/models",
    ]);
    assert.equal(resolved.model.id, "my-model");
    assert.equal(resolved.model.api, "openai-completions");
    assert.equal(resolved.model.contextWindow, 128000);
    assert.equal(resolved.model.reasoning, false);
  });

  it("uses the explicit model metadata of the custom providers", async () => {
    const modelRuntime = await newRuntime();
    const resolved = await resolveModel(
      llmConfig({
        provider: "custom",
        baseUrl: "https://api.example.net/openai/v1",
        model: "local-model",
        apiKey: "k",
        contextWindow: 32000,
        maxTokens: 4000,
        reasoning: true,
        input: ["text", "image"],
      }),
      { modelRuntime, logger },
    );
    assert.equal(resolved.model.baseUrl, "https://api.example.net/openai/v1");
    assert.equal(resolved.model.contextWindow, 32000);
    assert.equal(resolved.model.maxTokens, 4000);
    assert.deepEqual(resolved.model.input, ["text", "image"]);
  });

  it("resolves the built-in providers", async () => {
    const modelRuntime = await newRuntime();
    const resolved = await resolveModel(
      llmConfig({
        provider: "anthropic",
        model: "claude-sonnet-4-5",
        apiKey: "k",
      }),
      { modelRuntime, logger },
    );
    assert.equal(resolved.model.provider, "anthropic");
    assert.equal(resolved.model.api, "anthropic-messages");
    assert.equal(modelRuntime.hasConfiguredAuth("anthropic"), true);

    await assert.rejects(
      resolveModel(llmConfig({ provider: "anthropic", model: "nope" }), {
        modelRuntime,
        logger,
      }),
      /Unknown model anthropic\/nope/,
    );
    await assert.rejects(
      resolveModel(llmConfig({ provider: "nope", model: "x" }), {
        modelRuntime,
        logger,
      }),
      /Unknown LLM provider/,
    );
  });

  it("reports the invalid configurations", async () => {
    const modelRuntime = await newRuntime();
    await assert.rejects(
      resolveModel(llmConfig({}), { modelRuntime, logger }),
      ModelResolutionError,
    );
    await assert.rejects(
      resolveModel(llmConfig({ provider: "custom", model: "x" }), {
        modelRuntime,
        logger,
      }),
      /requires llm.baseUrl/,
    );
    await assert.rejects(
      resolveModel(llmConfig({ provider: "openai" }), { modelRuntime, logger }),
      /llm.model field is required/,
    );
    const failingFetch = (async () =>
      new Response("", { status: 500 })) as unknown as typeof fetch;
    await assert.rejects(
      resolveModel(llmConfig({ baseUrl: "http://gw" }), {
        modelRuntime,
        logger,
        fetchImpl: failingFetch,
      }),
      /could not be discovered/,
    );
  });
});
