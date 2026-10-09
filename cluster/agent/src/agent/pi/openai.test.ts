import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as http from "node:http";
import * as os from "node:os";
import * as path from "node:path";
import { after, before, describe, it } from "node:test";
import { createApp, type App } from "../../app.ts";
import { resolveConfig } from "../../config.ts";
import { setLogLevel } from "../../log.ts";
import type {
  CreateConversationResponse,
  Message,
  ToolBlock,
} from "../../protocol/index.ts";
import { collectEvents, getCatalog, postJSON } from "../../testutil/app.ts";
import { startMockAPI, type MockAPI } from "../../testutil/mockapi.ts";

interface ChatRequest {
  model: string;
  stream: boolean;
  messages: { role: string; content: unknown }[];
  tools?: { function: { name: string } }[];
}

const chunk = (delta: unknown, finishReason: string | null = null) =>
  `data: ${JSON.stringify({
    id: "chatcmpl-1",
    object: "chat.completion.chunk",
    created: 1,
    model: "test-model",
    choices: [{ index: 0, delta, finish_reason: finishReason }],
  })}\n\n`;

describe("openai-compatible LLM Service", () => {
  let dir: string;
  let mock: MockAPI;
  let llm: http.Server;
  let app: App;
  let baseUrl: string;
  const requests: { path: string; auth?: string; body: ChatRequest }[] = [];

  before(async () => {
    setLogLevel("error");
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-openai-"));
    mock = await startMockAPI({
      "user.v1.MainService/GetStatus": () => ({
        user: { metadata: { name: "alice" } },
      }),
      "user.v1.MainService/GetService": () => {
        throw new Error("must not be called when baseUrl is set");
      },
      "cordium.v1.MainService/GetSpace": () => ({
        apiVersion: "cordium/v1",
        kind: "Space",
        metadata: { name: "demo.alice" },
        spec: {},
        status: { type: "USER", userRef: { name: "alice" } },
      }),
    });

    llm = http.createServer(async (req, res) => {
      let raw = "";
      for await (const c of req) {
        raw += c;
      }
      const body = JSON.parse(raw) as ChatRequest;
      requests.push({
        path: req.url ?? "",
        auth: req.headers.authorization,
        body,
      });
      res.writeHead(200, { "content-type": "text/event-stream" });
      const last = body.messages.at(-1);
      if (last?.role === "tool") {
        res.write(chunk({ role: "assistant", content: "The Space " }));
        res.write(chunk({ content: "is a personal one." }));
        res.write(chunk({}, "stop"));
      } else {
        res.write(chunk({ role: "assistant", content: "Checking." }));
        res.write(
          chunk({
            tool_calls: [
              {
                index: 0,
                id: "call_1",
                type: "function",
                function: { name: "cordium_api_call", arguments: "" },
              },
            ],
          }),
        );
        res.write(
          chunk({
            tool_calls: [
              {
                index: 0,
                function: {
                  arguments: JSON.stringify({
                    method: "cordium.v1.MainService/GetSpace",
                    request: { name: "demo.alice" },
                  }),
                },
              },
            ],
          }),
        );
        res.write(chunk({}, "tool_calls"));
      }
      res.end("data: [DONE]\n\n");
    });
    await new Promise<void>((resolve) => llm.listen(0, "127.0.0.1", resolve));
    const llmPort = (llm.address() as { port: number }).port;

    const config = resolveConfig(
      {
        dataDir: path.join(dir, "data"),
        workDir: path.join(dir, "work"),
        server: { host: "127.0.0.1", port: 0 },
        octelium: {
          mode: "proxy",
          authProxySocket: mock.socketPath,
          domain: "example.com",
        },
        llm: {
          service: "llm",
          baseUrl: `http://127.0.0.1:${llmPort}`,
          model: "test-model",
          thinkingLevel: "off",
        },
        skills: { enabled: false },
        agent: { generateTitles: false, maxRetries: 0 },
      },
      {},
    );
    app = await createApp({
      config,
      catalog: getCatalog(),
      skipSkills: true,
      env: {},
    });
    const addr = await app.start();
    baseUrl = `http://127.0.0.1:${addr.port}`;
  });

  after(async () => {
    await app.close();
    await mock.close();
    await new Promise<void>((resolve) => llm.close(() => resolve()));
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("runs the agent loop against the LLM Service", async () => {
    const res = await postJSON(`${baseUrl}/v1/conversations`, {
      input: { text: "What kind of Space is demo?" },
    });
    const created = (await res.json()) as CreateConversationResponse;
    const events = await collectEvents(baseUrl, created.run!.id);
    const completed = events.find((e) => e.type === "message.completed");
    assert.ok(completed?.type === "message.completed");
    const message: Message = completed.message;

    assert.equal(message.status, "completed");
    assert.equal(message.model?.provider, "octelium");
    assert.equal(message.model?.id, "test-model");
    assert.deepEqual(
      message.blocks.map((b) => b.type),
      ["markdown", "tool", "markdown"],
    );
    const tool = message.blocks[1] as ToolBlock;
    assert.equal(tool.status, "completed");
    assert.ok(tool.details?.kind === "api");
    assert.equal(
      (tool.details.response as { status: { type: string } }).status.type,
      "USER",
    );
    assert.deepEqual(tool.details.resources, [
      { apiVersion: "cordium/v1", kind: "Space", name: "demo.alice" },
    ]);
    const answer = message.blocks[2];
    assert.ok(answer.type === "markdown");
    assert.equal(answer.text, "The Space is a personal one.");

    assert.equal(requests.length, 2);
    assert.equal(requests[0].path, "/v1/chat/completions");
    assert.equal(requests[0].auth, "Bearer octelium");
    assert.equal(requests[0].body.model, "test-model");
    assert.equal(requests[0].body.stream, true);
    const system = requests[0].body.messages[0];
    assert.match(String(system.content), /You are the Cordium Agent/);
    const tools = (requests[0].body.tools ?? []).map((t) => t.function.name);
    for (const name of [
      "bash",
      "workspace_list",
      "workspace_create",
      "workspace_control",
      "workspace_exec",
      "workspace_copy",
      "workspace_logs",
      "cordium_api_search",
      "cordium_api_describe",
      "cordium_api_call",
      "present_table",
      "present_chart",
      "present_resources",
      "publish_artifact",
      "web_fetch",
    ]) {
      assert.ok(tools.includes(name), name);
    }
    assert.ok(!tools.includes("web_search"));
    assert.equal(requests[1].body.messages.at(-1)?.role, "tool");
  });
});
