import { strict as assert } from "node:assert";
import { describe, it } from "node:test";
import type { AgentSessionEvent } from "@earendil-works/pi-coding-agent";
import type {
  AgentEventPayload,
  Message,
  ToolBlock,
} from "../../protocol/index.ts";
import { MessageBuilder } from "../../runs/builder.ts";
import { PiEventMapper, resultText, toolDetails, toolTitle } from "./mapper.ts";

const newMapper = () => {
  const events: AgentEventPayload[] = [];
  const message: Message = {
    id: "m1",
    conversationId: "c1",
    role: "assistant",
    status: "streaming",
    createdAt: "",
    blocks: [],
  };
  const builder = new MessageBuilder(message, (e) => events.push(e));
  const activities: unknown[] = [];
  const mapper = new PiEventMapper(
    builder,
    {
      hiddenTools: new Set(["present_table"]),
      maxOutputChars: 10,
      updateIntervalMs: 0,
    },
    (a) => activities.push(a),
  );
  return { mapper, message, events, activities, builder };
};

const ev = (e: unknown) => e as AgentSessionEvent;

describe("pi event mapper", () => {
  it("formats the tool titles", () => {
    assert.equal(
      toolTitle("bash", { command: "ls -la\necho hi" }),
      "Run ls -la",
    );
    assert.equal(toolTitle("bash", {}), "Run a shell command");
    assert.equal(
      toolTitle("cordium_api_call", {
        method: "cordium.v1.MainService/ListSpace",
      }),
      "Call cordium.v1.MainService/ListSpace",
    );
    assert.equal(
      toolTitle("cordium_api_call", undefined),
      "Call the Cordium API",
    );
    assert.equal(
      toolTitle("workspace_exec", {
        workspace: "abc",
        command: "go test ./...\necho done",
      }),
      "Run go test ./... in abc",
    );
    assert.equal(
      toolTitle("workspace_control", {
        action: "stop",
        workspaces: ["abc", "def"],
      }),
      "Stop abc, def",
    );
    assert.equal(
      toolTitle("workspace_control", { action: "wait", workspaces: ["abc"] }),
      "Wait for abc",
    );
    assert.equal(
      toolTitle("workspace_create", {
        space: "my-space",
        image: "debian:latest",
        repository: { url: "https://github.com/octelium/octelium" },
      }),
      "Create a Workspace in my-space (debian:latest, https://github.com/octelium/octelium)",
    );
    assert.equal(
      toolTitle("workspace_create", { template: "go.my-space" }),
      "Create a Workspace from go.my-space",
    );
    assert.equal(
      toolTitle("workspace_copy", {
        source: "abc:/my/dir",
        destination: "def:/my/dir",
      }),
      "Copy abc:/my/dir to def:/my/dir",
    );
    assert.equal(
      toolTitle("workspace_list", { space: "team.cordium" }),
      "List the Workspaces of team.cordium",
    );
    assert.equal(
      toolTitle("workspace_logs", { workspace: "abc" }),
      "Read the logs of abc",
    );
    assert.equal(
      toolTitle("grep", { pattern: "x", path: "src" }),
      'Search for "x" in src',
    );
    assert.equal(toolTitle("ls", {}), "List .");
    assert.equal(toolTitle("custom_tool", {}), "custom_tool");
  });

  it("maps the tool details", () => {
    assert.deepEqual(
      toolDetails(
        "bash",
        { command: "false" },
        { structuredContent: { exit_code: 1 } },
      ),
      { kind: "command", command: "false", exitCode: 1 },
    );
    assert.deepEqual(
      toolDetails("edit", { path: "a.txt" }, { details: { diff: "-a\n+b" } }),
      { kind: "file", path: "a.txt", operation: "edit", diff: "-a\n+b" },
    );
    assert.deepEqual(toolDetails("read", { path: "a.txt" }, {}), {
      kind: "file",
      path: "a.txt",
      operation: "read",
    });
    const api = {
      kind: "api",
      method: "m",
      grpcMethod: "/m",
      risk: "read",
      response: { big: "x".repeat(70000) },
    };
    assert.deepEqual(toolDetails("cordium_api_call", {}, { details: api }), {
      kind: "api",
      method: "m",
      grpcMethod: "/m",
      risk: "read",
      responseTruncated: true,
    });
    assert.equal(toolDetails("ls", {}, {}), undefined);
    const workspaces = {
      kind: "workspaces",
      items: [{ name: "abc", state: "RUNNING" }],
    };
    assert.deepEqual(
      toolDetails("workspace_list", {}, { details: workspaces }),
      workspaces,
    );
    const transfer = {
      kind: "transfer",
      source: "abc:/a",
      destination: "/b",
      type: "file",
      bytes: 1,
    };
    assert.deepEqual(
      toolDetails("workspace_copy", {}, { details: transfer }),
      transfer,
    );
    assert.equal(
      resultText({
        content: [
          { type: "text", text: "a" },
          { type: "image" },
          { type: "text", text: "b" },
        ],
      }),
      "a\nb",
    );
  });

  it("maps the streamed assistant messages", () => {
    const { mapper, message, events } = newMapper();
    const partial = {
      content: [{ type: "thinking", thinking: "", redacted: true }],
    };
    mapper.handle(
      ev({ type: "message_start", message: { role: "assistant" } }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "thinking_start",
          contentIndex: 0,
          partial,
        },
      }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "thinking_end",
          contentIndex: 0,
          content: "",
          partial,
        },
      }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: { type: "text_start", contentIndex: 1, partial },
      }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "text_delta",
          contentIndex: 1,
          delta: "Hel",
          partial,
        },
      }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "text_delta",
          contentIndex: 1,
          delta: "lo",
          partial,
        },
      }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "text_end",
          contentIndex: 1,
          content: "Hello!",
          partial,
        },
      }),
    );
    mapper.handle(
      ev({
        type: "message_end",
        message: {
          role: "assistant",
          stopReason: "length",
          usage: {
            input: 10,
            output: 5,
            cacheRead: 2,
            cacheWrite: 1,
            totalTokens: 18,
            cost: { total: 0.5 },
          },
        },
      }),
    );

    assert.deepEqual(
      message.blocks.map((b) => [b.type, "text" in b ? b.text : undefined]),
      [
        ["thinking", ""],
        ["markdown", "Hello!"],
        [
          "notice",
          "The response was cut off because the model reached its maximum output length.",
        ],
      ],
    );
    assert.equal(
      message.blocks[0].type === "thinking" && message.blocks[0].redacted,
      true,
    );
    assert.deepEqual(
      events.map((e) => e.type),
      [
        "block.created",
        "block.updated",
        "block.created",
        "block.delta",
        "block.delta",
        "block.updated",
        "block.created",
      ],
    );
    assert.deepEqual(mapper.usage, {
      inputTokens: 10,
      outputTokens: 5,
      cacheReadTokens: 2,
      cacheWriteTokens: 1,
      totalTokens: 18,
      cost: 0.5,
    });
  });

  it("maps the tool executions", () => {
    const { mapper, message } = newMapper();
    mapper.handle(
      ev({ type: "message_start", message: { role: "assistant" } }),
    );
    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "toolcall_start",
          contentIndex: 0,
          partial: {
            content: [
              { type: "toolCall", id: "t1", name: "bash", arguments: {} },
            ],
          },
        },
      }),
    );
    let tool = message.blocks[0] as ToolBlock;
    assert.equal(tool.status, "pending");
    assert.equal(tool.title, "Run a shell command");

    mapper.handle(
      ev({
        type: "message_update",
        message: { role: "assistant" },
        assistantMessageEvent: {
          type: "toolcall_end",
          contentIndex: 0,
          toolCall: {
            type: "toolCall",
            id: "t1",
            name: "bash",
            arguments: { command: "ls" },
          },
          partial: { content: [] },
        },
      }),
    );
    mapper.handle(
      ev({
        type: "tool_execution_start",
        toolCallId: "t1",
        toolName: "bash",
        args: { command: "ls" },
      }),
    );
    tool = message.blocks[0] as ToolBlock;
    assert.equal(tool.status, "running");
    assert.equal(tool.title, "Run ls");
    assert.ok(tool.startedAt);

    mapper.handle(
      ev({
        type: "tool_execution_update",
        toolCallId: "t1",
        toolName: "bash",
        args: {},
        partialResult: { content: [{ type: "text", text: "partial" }] },
      }),
    );
    assert.equal((message.blocks[0] as ToolBlock).output, "partial");

    mapper.handle(
      ev({
        type: "tool_execution_end",
        toolCallId: "t1",
        toolName: "bash",
        result: {
          content: [{ type: "text", text: "a very long output" }],
          structuredContent: { exit_code: 0 },
        },
        isError: false,
      }),
    );
    tool = message.blocks[0] as ToolBlock;
    assert.equal(tool.status, "completed");
    assert.equal(tool.output, "a very lon\n…[truncated]");
    assert.equal(tool.outputTruncated, true);
    assert.deepEqual(tool.details, {
      kind: "command",
      command: "ls",
      exitCode: 0,
    });

    mapper.handle(
      ev({
        type: "tool_execution_start",
        toolCallId: "t2",
        toolName: "present_table",
        args: {},
      }),
    );
    mapper.handle(
      ev({
        type: "tool_execution_end",
        toolCallId: "t2",
        toolName: "present_table",
        result: {},
        isError: false,
      }),
    );
    assert.equal(message.blocks.length, 1);
  });

  it("keeps the rejected status and reports the activities", () => {
    const { mapper, message, builder, activities } = newMapper();
    mapper.handle(
      ev({
        type: "tool_execution_start",
        toolCallId: "t1",
        toolName: "cordium_api_call",
        args: { method: "m" },
      }),
    );
    builder.updateBlock<ToolBlock>(message.blocks[0].id, {
      status: "rejected",
    });
    mapper.handle(
      ev({
        type: "tool_execution_end",
        toolCallId: "t1",
        toolName: "cordium_api_call",
        result: { content: [{ type: "text", text: "no" }] },
        isError: true,
      }),
    );
    assert.equal((message.blocks[0] as ToolBlock).status, "rejected");

    mapper.handle(
      ev({
        type: "auto_retry_start",
        attempt: 1,
        maxAttempts: 3,
        delayMs: 1,
        errorMessage: "boom",
      }),
    );
    mapper.handle(ev({ type: "auto_retry_end", success: true, attempt: 1 }));
    mapper.handle(ev({ type: "compaction_start", reason: "threshold" }));
    mapper.handle(
      ev({
        type: "compaction_end",
        reason: "threshold",
        result: {},
        aborted: false,
        willRetry: false,
      }),
    );
    assert.deepEqual(activities, [
      {
        kind: "retrying",
        message: "Retrying after an error (attempt 1/3): boom",
      },
      undefined,
      {
        kind: "compacting",
        message:
          "Summarizing the earlier conversation to fit the model's context window",
      },
      undefined,
    ]);
    assert.equal(message.blocks.at(-1)?.type, "notice");
  });
});
