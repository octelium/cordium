import "@mantine/core/styles.css";
import "@/index.css";

import React from "react";
import ReactDOM from "react-dom/client";
import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";

import type {
  AgentEvent,
  AgentInfo,
  AgentSettings,
  Block,
  Conversation,
  Message,
  Run,
} from "@/apis/agent/protocol";
import Chat from "@/pages/Agent/Chat";
import { AgentClient } from "@/pages/Agent/client";
import themeMantine, { cssVariablesResolver } from "@/utils/theme/mantine";

interface Call {
  method: string;
  path: string;
  body?: unknown;
}

declare global {
  interface Window {
    agentCalls: Call[];
  }
}

const prefix = "/agent-api/v1";
const now = new Date();
const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
const at = (ms: number) => new Date(ms).toISOString();

const conversations = new Map<string, Conversation>();
const messages = new Map<string, Message[]>();
const decisions = new Map<
  string,
  (decision: { decision: string; reason?: string }) => void
>();
let counter = 0;

const markdown = (text: string, createdAt: string): Block => ({
  id: `b-${++counter}`,
  type: "markdown",
  text,
  createdAt,
});

const seed = (
  id: string,
  title: string,
  time: number,
  q: string,
  a: Block[],
) => {
  const createdAt = at(time);
  conversations.set(id, {
    id,
    title,
    createdAt,
    updatedAt: createdAt,
    messageCount: 2,
  });
  messages.set(id, [
    {
      id: `${id}-u`,
      conversationId: id,
      role: "user",
      status: "completed",
      createdAt,
      blocks: [markdown(q, createdAt)],
    },
    {
      id: `${id}-a`,
      conversationId: id,
      role: "assistant",
      status: "completed",
      createdAt,
      completedAt: createdAt,
      blocks: a,
    },
  ]);
};

const recent = (ms: number) => Math.max(today.getTime(), now.getTime() - ms);

seed(
  "c1",
  "Audit my running Workspaces",
  recent(60000),
  "Which of my Workspaces are running?",
  [markdown("You have 3 running Workspaces.", at(recent(60000)))],
);
seed(
  "c4",
  "Run the unit tests",
  recent(120000),
  "Create a Workspace for octelium and run the unit tests",
  [
    {
      id: "c4-create",
      type: "tool",
      toolCallId: "call-create",
      name: "workspace_create",
      title: "Create the Workspace unit-tests",
      status: "completed",
      createdAt: at(recent(120000)),
      startedAt: at(recent(120000)),
      completedAt: at(recent(110000)),
      details: {
        kind: "workspaces",
        action: "create",
        items: [
          {
            name: "unit-tests",
            displayName: "Unit tests",
            state: "RUNNING",
            space: "default.george",
            image: "debian:latest",
            repository: "https://github.com/octelium/octelium",
          },
        ],
      },
    },
    {
      id: "c4-exec",
      type: "tool",
      toolCallId: "call-exec",
      name: "workspace_exec",
      title: "Run go test in unit-tests",
      status: "completed",
      createdAt: at(recent(110000)),
      startedAt: at(recent(110000)),
      completedAt: at(recent(100000)),
      output: "ok  \tgithub.com/octelium/octelium/pkg\t0.42s",
      details: {
        kind: "command",
        command: "go test ./...",
        workspace: "unit-tests",
        cwd: "/workspace/repo",
        exitCode: 0,
        durationMs: 10000,
      },
    },
    markdown("All the unit tests passed.", at(recent(100000))),
  ],
);
seed(
  "c2",
  "Copy the build artifacts",
  today.getTime() - 12 * 3600000,
  "Copy /workspace/repo/dist from abc to def",
  [markdown("The directory is copied to def.", at(today.getTime()))],
);
seed(
  "c3",
  "Clean up idle Workspaces",
  today.getTime() - 60 * 86400000,
  "Which Workspaces look idle?",
  [markdown("No Workspace has been idle for 30 days.", at(today.getTime()))],
);

const model = { provider: "faux", id: "faux-1", name: "Faux Opus" };
let settings: AgentSettings = {
  approvals: { api: "destructive", commands: false },
};

const info: AgentInfo = {
  name: "cordium-agent",
  version: "0.1.0",
  protocolVersion: 1,
  status: "ready",
  issues: [],
  model,
  cluster: {
    mode: "proxy",
    user: { name: "george", displayName: "George B." },
  },
  workspace: { name: "agent-george" },
  settings,
  capabilities: {
    blockTypes: [],
    webSearch: false,
    uploads: { maxBytes: 1024 * 1024 },
    models: true,
    login: false,
    settings: true,
  },
};

const json = (body: unknown, status = 200) =>
  new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });

const runs = new Map<string, { run: Run; text: string }>();

const startRun = (conversationId: string, text: string): Run => {
  const id = `r-${++counter}`;
  const run: Run = {
    id,
    conversationId,
    status: "running",
    createdAt: at(Date.now()),
    userMessageId: `${id}-u`,
    assistantMessageId: `${id}-a`,
    lastSeq: 0,
  };
  runs.set(id, { run, text });
  return run;
};

window.agentCalls = [];
const realFetch = window.fetch.bind(window);

window.fetch = async (
  input: Parameters<typeof fetch>[0],
  init?: Parameters<typeof fetch>[1],
) => {
  const url = new URL(
    typeof input === "string" || input instanceof URL ? input : input.url,
    window.location.href,
  );
  if (!url.pathname.startsWith(prefix)) {
    return realFetch(input, init);
  }

  const path = url.pathname.slice(prefix.length);
  const method = init?.method ?? "GET";
  const body =
    typeof init?.body === "string" ? JSON.parse(init.body) : undefined;
  window.agentCalls.push({ method, path: `${path}${url.search}`, body });

  if (path === "/conversations" && method === "GET") {
    return json({
      items: [...conversations.values()].sort((a, b) =>
        b.updatedAt.localeCompare(a.updatedAt),
      ),
    });
  }

  if (path === "/conversations" && method === "POST") {
    const id = `c-${++counter}`;
    const conversation: Conversation = {
      id,
      title: body.input.text,
      createdAt: at(Date.now()),
      updatedAt: at(Date.now()),
      messageCount: 0,
    };
    conversations.set(id, conversation);
    messages.set(id, []);
    const run = startRun(id, body.input.text);
    conversation.activeRunId = run.id;
    return json({ conversation, run }, 201);
  }

  const conversationMatch = /^\/conversations\/([^/]+)$/.exec(path);
  if (conversationMatch) {
    const id = conversationMatch[1];
    const conversation = conversations.get(id);
    if (!conversation) return json({ error: { message: "Not found" } }, 404);
    if (method === "PATCH") {
      conversation.title = body.title;
      return json(conversation);
    }
    if (method === "DELETE") {
      conversations.delete(id);
      return json(undefined, 204);
    }
    return json({ conversation, messages: messages.get(id) ?? [] });
  }

  const runMatch = /^\/conversations\/([^/]+)\/runs$/.exec(path);
  if (runMatch) {
    return json(startRun(runMatch[1], body.input.text), 201);
  }

  const approvalMatch = /^\/runs\/([^/]+)\/approvals\/([^/]+)$/.exec(path);
  if (approvalMatch) {
    decisions.get(approvalMatch[2])?.(body);
    return json(runs.get(approvalMatch[1])?.run);
  }

  if (path === "/models") {
    return json({ current: model, models: [model] });
  }

  if (path === "/settings") {
    if (method === "PATCH") {
      settings = {
        approvals: { ...settings.approvals, ...body.approvals },
      };
    }
    return json(settings);
  }

  if (path === "/info") {
    return json(info);
  }

  return json({ error: { message: "Not found" } }, 404);
};

const sleep = (ms: number) =>
  new Promise((resolve) => window.setTimeout(resolve, ms));

class FakeEventSource {
  static readonly CLOSED = 2;
  readyState = 1;
  onerror: (() => void) | null = null;
  private listeners = new Map<string, ((msg: { data: string }) => void)[]>();
  private seq = 0;

  constructor(url: string) {
    const runId = /\/runs\/([^/]+)\/events/.exec(url)?.[1] ?? "";
    window.setTimeout(() => void this.play(runId), 30);
  }

  addEventListener(type: string, fn: (msg: { data: string }) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
  }

  close() {
    this.readyState = FakeEventSource.CLOSED;
  }

  private emit(payload: Record<string, unknown> & { type: string }, run: Run) {
    if (this.readyState === FakeEventSource.CLOSED) return;
    const event = {
      ...payload,
      seq: ++this.seq,
      ts: at(Date.now()),
      conversationId: run.conversationId,
      runId: run.id,
    } as AgentEvent;
    for (const fn of this.listeners.get(event.type) ?? []) {
      fn({ data: JSON.stringify(event) });
    }
  }

  private async play(runId: string) {
    const entry = runs.get(runId);
    if (!entry || this.readyState === FakeEventSource.CLOSED) return;
    const { run, text } = entry;
    const createdAt = at(Date.now());
    const user: Message = {
      id: run.userMessageId,
      conversationId: run.conversationId,
      runId: run.id,
      role: "user",
      status: "completed",
      createdAt,
      blocks: [markdown(text, createdAt)],
    };
    const assistant: Message = {
      id: run.assistantMessageId,
      conversationId: run.conversationId,
      runId: run.id,
      role: "assistant",
      status: "streaming",
      createdAt,
      blocks: [],
    };

    this.emit({ type: "run.started", run }, run);
    await sleep(150);
    this.emit({ type: "message.created", message: user }, run);
    this.emit({ type: "message.created", message: assistant }, run);

    if (text.toLowerCase().includes("delete")) {
      const tool: Block = {
        id: "tool-1",
        type: "tool",
        toolCallId: "call-1",
        name: "cordium_api_call",
        title: "Call cordium.v1.MainService/DeleteWorkspace",
        status: "awaiting_approval",
        createdAt,
        details: {
          kind: "api",
          method: "cordium.v1.MainService/DeleteWorkspace",
          grpcMethod:
            "/octelium.api.main.cordium.v1.MainService/DeleteWorkspace",
          risk: "destructive",
          request: { name: "unit-tests" },
        },
      };
      const approval: Block = {
        id: "approval-1",
        type: "approval",
        approvalId: "ap-1",
        toolCallId: "call-1",
        title: "Delete Workspace (cordium.v1)",
        risk: "destructive",
        status: "pending",
        createdAt,
        preview: {
          method: "cordium.v1.MainService/DeleteWorkspace",
          request: { name: "unit-tests" },
        },
      };
      this.emit(
        { type: "block.created", messageId: assistant.id, block: tool },
        run,
      );
      this.emit(
        { type: "block.created", messageId: assistant.id, block: approval },
        run,
      );
      this.emit(
        {
          type: "run.updated",
          run: { ...run, status: "awaiting_approval" },
        },
        run,
      );
      const decision = await new Promise<{ decision: string; reason?: string }>(
        (resolve) => decisions.set("ap-1", resolve),
      );
      const approved = decision.decision === "approve";
      const resolved = {
        ...approval,
        status: approved ? "approved" : "rejected",
        reason: decision.reason,
      } as Block;
      const done = {
        ...tool,
        status: approved ? "completed" : "rejected",
      } as Block;
      this.emit(
        { type: "block.updated", messageId: assistant.id, block: resolved },
        run,
      );
      this.emit(
        { type: "block.updated", messageId: assistant.id, block: done },
        run,
      );
      this.emit({ type: "run.updated", run }, run);
      assistant.blocks.push(done, resolved);
    }

    const answer = markdown("", at(Date.now()));
    this.emit(
      { type: "block.created", messageId: assistant.id, block: answer },
      run,
    );
    for (const chunk of ["You have ", "**2** ", "Workspaces."]) {
      await sleep(120);
      this.emit(
        {
          type: "block.delta",
          messageId: assistant.id,
          blockId: answer.id,
          text: chunk,
        },
        run,
      );
    }

    const completed: Message = {
      ...assistant,
      status: "completed",
      completedAt: at(Date.now()),
      blocks: [
        ...assistant.blocks,
        { ...answer, text: "You have **2** Workspaces." } as Block,
      ],
    };
    messages.set(run.conversationId, [
      ...(messages.get(run.conversationId) ?? []),
      user,
      completed,
    ]);
    const conversation = conversations.get(run.conversationId)!;
    delete conversation.activeRunId;
    conversation.updatedAt = completed.completedAt!;
    conversation.messageCount += 2;

    this.emit({ type: "message.completed", message: completed }, run);
    this.emit({ type: "conversation.updated", conversation }, run);
    this.emit(
      {
        type: "run.completed",
        run: {
          ...run,
          status: "completed",
          completedAt: completed.completedAt,
        },
      },
      run,
    );
  }
}

window.EventSource = FakeEventSource as unknown as typeof EventSource;

const client = new AgentClient(`${window.location.origin}/agent-api`);
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

window.history.replaceState(null, "", "/agent");

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <MantineProvider
      theme={themeMantine}
      cssVariablesResolver={cssVariablesResolver}
    >
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Routes>
            <Route
              path="/agent"
              element={
                <div style={{ height: "100dvh" }}>
                  <Chat client={client} info={info} />
                </div>
              }
            />
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </MantineProvider>
  </React.StrictMode>,
);
