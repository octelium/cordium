import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { after, before, beforeEach, describe, it } from "node:test";
import type {
  AgentSettings,
  CommandDetails,
  Message,
  TransferDetails,
  WorkspacesDetails,
} from "../../protocol/index.ts";
import { APIClient } from "../../api/client.ts";
import { MessageBuilder } from "../../runs/builder.ts";
import { FakeCordium } from "../../testutil/fakecordium.ts";
import { getCatalog } from "../../testutil/app.ts";
import { startMockAPI, type MockAPI } from "../../testutil/mockapi.ts";
import { ResourceCache } from "../resources.ts";
import type { ApprovalRequest, ToolRunContext } from "../types.ts";
import { formatEndpoint, parseEndpoint } from "./transfer.ts";
import {
  backgroundCommand,
  createWorkspaceTools,
  formatSummary,
  matchesName,
} from "./workspaces.ts";

interface ToolResult {
  content: { type: string; text?: string }[];
  details?: unknown;
  isError?: boolean;
}

const text = (res: ToolResult) =>
  res.content.map((c) => (c.type === "text" ? c.text : "")).join("");

describe("workspace tools", () => {
  let dir: string;
  let fake: FakeCordium;
  let mock: MockAPI;
  let client: APIClient;
  let settings: AgentSettings;
  let approvals: ApprovalRequest[];
  let decision: boolean;
  let tools: ReturnType<typeof createWorkspaceTools>;

  const run = async (
    name: string,
    params: Record<string, unknown>,
    updates?: string[],
  ): Promise<ToolResult> => {
    const tool = tools.find((t) => t.name === name)!;
    return (await tool.execute(
      "call-1",
      params as never,
      undefined,
      ((partial: ToolResult) => updates?.push(text(partial))) as never,
      undefined as never,
    )) as ToolResult;
  };

  before(async () => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-tools-"));
    fake = new FakeCordium();
    mock = await startMockAPI({
      ...fake.handlers(),
      "cordium.v1.WorkspaceService/ListenLog": () => [
        {
          type: "CLONING_REPO",
          mode: "STDOUT",
          data: Buffer.from("Cloning into 'repo'...\n").toString("base64"),
        },
        {
          type: "TASK",
          mode: "STDERR",
          data: Buffer.from("npm warn\r\ninstalled\n").toString("base64"),
        },
      ],
    });
    client = new APIClient(getCatalog(), {
      mode: "proxy",
      authProxySocket: mock.socketPath,
      domain: "example.com",
    });

    const message: Message = {
      id: "m1",
      conversationId: "c1",
      role: "assistant",
      status: "streaming",
      createdAt: new Date().toISOString(),
      blocks: [],
    };
    const ctx: ToolRunContext = {
      runId: "r1",
      conversationId: "c1",
      builder: new MessageBuilder(message, () => {}),
      resources: new ResourceCache(),
      resultsDir: path.join(dir, "results"),
      requestApproval: async (req) => {
        approvals.push(req);
        return { approved: decision, reason: decision ? undefined : "no" };
      },
    };

    tools = createWorkspaceTools({
      conversationId: "c1",
      workDir: path.join(dir, "work"),
      runtime: { current: () => ctx },
      client,
      settings: { get: () => settings },
      agentWorkspace: "agt",
      tmpDir: path.join(dir, "tmp"),
      pollIntervalMs: 5,
    });
  });

  after(async () => {
    await client.close();
    await mock.close();
    fs.rmSync(dir, { recursive: true, force: true });
  });

  beforeEach(() => {
    fake.workspaces.clear();
    fake.add("agt", "RUNNING", { space: "octelium.alice" });
    fake.add("abc", "RUNNING", { image: "debian:latest" });
    fake.add("def", "STOPPED", { space: "proj.alice" });
    settings = { approvals: { api: "destructive", commands: false } };
    approvals = [];
    decision = true;
  });

  it("lists and filters the Workspaces", async () => {
    let res = await run("workspace_list", {});
    const details = res.details as WorkspacesDetails;
    assert.equal(details.kind, "workspaces");
    assert.deepEqual(
      details.items.map((i) => i.name),
      ["agt", "abc", "def"],
    );
    assert.equal(details.items[0].isAgent, true);
    assert.equal(details.items[1].image, "debian:latest");
    assert.equal(details.items[1].hostname, "abc.cordium.example.com");
    assert.match(text(res), /^3 matching Workspace\(s\):/);
    assert.match(
      text(res),
      /- agt: RUNNING · Space octelium\.alice .*\(this agent's own Workspace\)/,
    );

    res = await run("workspace_list", { space: "proj" });
    assert.deepEqual(
      (res.details as WorkspacesDetails).items.map((i) => i.name),
      ["def"],
    );
    res = await run("workspace_list", { state: "running", limit: 1 });
    assert.match(text(res), /^2 matching Workspace\(s\), 1 shown:/);
    res = await run("workspace_list", { query: "nope" });
    assert.equal(text(res), "No matching Workspaces found.");
  });

  it("creates, starts and waits for a Workspace", async () => {
    const updates: string[] = [];
    const res = await run(
      "workspace_create",
      {
        displayName: "Octelium tests",
        space: "proj",
        image: "debian:latest",
        repository: {
          url: "https://github.com/octelium/octelium",
          branch: "dev",
        },
        env: { CI: "true" },
        resources: { cpu: 4000 },
      },
      updates,
    );
    assert.equal(res.isError, undefined, text(res));
    assert.match(text(res), /^The Workspace \w+ is RUNNING\./);
    assert.match(text(res), /cloned into \/workspace\/repo/);

    const call = mock.calls.find(
      (c) => c.method === "cordium.v1.MainService/CreateWorkspace",
    );
    assert.deepEqual(call?.request, {
      metadata: { displayName: "Octelium tests" },
      spec: {
        image: { registry: { url: "debian:latest" } },
        repository: {
          url: "https://github.com/octelium/octelium",
          cloneOptions: { branch: "dev" },
        },
        runtime: { envVars: [{ key: "CI", value: "true" }] },
        limit: { cpu: { millicores: 4000 } },
      },
      status: { templateRef: { name: "default.proj" } },
    });

    const details = res.details as WorkspacesDetails;
    assert.equal(details.action, "create");
    assert.equal(details.items[0].state, "RUNNING");
    assert.equal(details.items[0].space, "proj.alice");
    assert.match(updates[0], /^\w+: created\n\(0s\)$/);
    assert.deepEqual(approvals, []);
  });

  it("reports failed starts and honors the options", async () => {
    let res = await run("workspace_create", { image: "registry/fail:1" });
    assert.equal(res.isError, true);
    assert.match(text(res), /start failed: Could not pull the image/);
    assert.equal(
      (res.details as WorkspacesDetails).items[0].failure,
      "Could not pull the image",
    );

    res = await run("workspace_create", {
      template: "py",
      space: "proj",
      start: false,
    });
    assert.match(text(res), /^Created the Workspace \w+ \(STOPPED\)/);
    assert.deepEqual(
      (mock.calls.at(-1)?.request as { status: unknown }).status,
      { templateRef: { name: "py.proj" } },
    );

    res = await run("workspace_create", { spec: { unknownField: 1 } });
    assert.equal(res.isError, true);
    assert.match(text(res), /^Invalid Workspace configuration/);

    settings.approvals.api = "write";
    decision = false;
    const count = mock.calls.length;
    res = await run("workspace_create", { image: "debian" });
    assert.equal(res.isError, true);
    assert.match(text(res), /did not approve/);
    assert.equal(approvals[0].risk, "write");
    assert.equal(mock.calls.length, count);
  });

  it("controls several Workspaces in parallel", async () => {
    let res = await run("workspace_control", {
      action: "stop",
      workspaces: ["abc", "def"],
    });
    assert.equal(text(res), "abc: STOPPED\ndef: STOPPED");
    assert.deepEqual(approvals, []);

    res = await run("workspace_control", {
      action: "start",
      workspaces: ["abc"],
    });
    assert.equal(text(res), "abc: RUNNING");

    res = await run("workspace_control", {
      action: "restart",
      workspaces: ["abc", "nope"],
    });
    assert.equal(res.isError, undefined);
    assert.match(
      text(res),
      /^abc: RUNNING\nnope: failed: Workspace nope does not exist \(not_found\)$/,
    );
    assert.deepEqual((res.details as WorkspacesDetails).errors, [
      {
        workspace: "nope",
        message: "Workspace nope does not exist (not_found)",
      },
    ]);

    res = await run("workspace_control", {
      action: "stop",
      workspaces: ["agt"],
    });
    assert.equal(res.isError, true);
    assert.match(text(res), /agent's own Workspace/);

    res = await run("workspace_control", {
      action: "wait",
      workspaces: ["agt", "def"],
    });
    assert.equal(text(res), "agt: RUNNING\ndef: STOPPED");

    decision = false;
    res = await run("workspace_control", {
      action: "delete",
      workspaces: ["def"],
    });
    assert.equal(res.isError, true);
    assert.equal((approvals as ApprovalRequest[]).at(-1)?.risk, "destructive");
    assert.deepEqual((approvals as ApprovalRequest[]).at(-1)?.preview, {
      workspaces: ["def"],
    });
    assert.ok(fake.get("def"));

    decision = true;
    res = await run("workspace_control", {
      action: "delete",
      workspaces: ["def"],
    });
    assert.equal(text(res), "def: deleted");
    assert.equal(fake.get("def"), undefined);
  });

  it("runs commands in another Workspace", async () => {
    const cwd = fs.mkdtempSync(path.join(dir, "cwd-"));
    let res = await run("workspace_exec", {
      workspace: "abc",
      command: 'echo "hello $NAME" && pwd && echo oops >&2; exit 3',
      cwd,
      env: { NAME: "cordium" },
    });
    assert.equal(res.isError, true);
    assert.match(text(res), /^Exit code: 3\n/);
    assert.match(text(res), /hello cordium/);
    assert.match(text(res), new RegExp(cwd));
    assert.match(text(res), /oops/);
    const details = res.details as CommandDetails;
    assert.equal(details.kind, "command");
    assert.equal(details.workspace, "abc");
    assert.equal(details.exitCode, 3);
    const exec = mock.execs.at(-1)!;
    assert.equal(exec.workingDir, cwd);
    assert.equal(exec.hasStdin, false);
    assert.ok(exec.workspaceRef?.uid || exec.workspaceRef?.name);

    res = await run("workspace_exec", {
      workspace: "abc",
      command: "seq 1 20000",
    });
    assert.equal(res.isError, undefined);
    assert.match(text(res), /characters omitted, the full output is saved to /);
    assert.match(text(res), /\n20000\n?$/);
    const logPath = (res.details as CommandDetails).logPath!;
    assert.equal(fs.readFileSync(logPath, "utf8").split("\n")[19999], "20000");

    res = await run("workspace_exec", {
      workspace: "abc",
      command: "sleep 30",
      timeoutSeconds: 1,
    });
    assert.equal(res.isError, true);
    assert.match(text(res), /^The command was killed after 1s/);
    assert.equal((res.details as CommandDetails).timedOut, true);

    res = await run("workspace_exec", { workspace: "def", command: "ls" });
    assert.equal(res.isError, true);
    assert.match(text(res), /is STOPPED\. Start it first/);

    settings.approvals.commands = true;
    decision = false;
    const count = mock.execs.length;
    res = await run("workspace_exec", { workspace: "abc", command: "ls" });
    assert.equal(res.isError, true);
    assert.deepEqual(approvals[0].preview, {
      command: "ls",
      workspace: "abc",
    });
    assert.equal(mock.execs.length, count);
  });

  it("runs commands in the background", async () => {
    const res = await run("workspace_exec", {
      workspace: "abc",
      command: "echo started; exit 4",
      background: true,
    });
    assert.equal(res.isError, undefined, text(res));
    const log = /written to (\/tmp\/cordium-agent\/\S+\.log) /.exec(
      text(res),
    )?.[1];
    assert.ok(log, text(res));
    assert.match(text(res), /\(PID \d+\)/);
    try {
      const deadline = Date.now() + 5000;
      while (!fs.existsSync(`${log}.exit`) && Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 20));
      }
      await new Promise((resolve) => setTimeout(resolve, 50));
      assert.equal(fs.readFileSync(`${log}.exit`, "utf8").trim(), "4");
      assert.equal(fs.readFileSync(log, "utf8"), "started\n");
    } finally {
      fs.rmSync(log!, { force: true });
      fs.rmSync(`${log}.exit`, { force: true });
    }
    assert.match(
      backgroundCommand("make test", "x"),
      /setsid nohup sh -c 'sh -c '\\''make test'\\''; echo \$\? > \/tmp\/cordium-agent\/x\.log\.exit' > \/tmp\/cordium-agent\/x\.log 2>&1 < \/dev\/null &/,
    );
  });

  it("copies files and directories between Workspaces", async () => {
    const src = path.join(dir, "src");
    fs.mkdirSync(path.join(src, "sub", "node_modules"), { recursive: true });
    fs.writeFileSync(path.join(src, "a.txt"), "hello");
    fs.writeFileSync(
      path.join(src, "sub", "b.bin"),
      Buffer.from([0, 1, 2, 255]),
    );
    fs.writeFileSync(path.join(src, "sub", "node_modules", "big"), "x");
    const dst = path.join(dir, "dst");

    let res = await run("workspace_copy", {
      source: `abc:${src}`,
      destination: `def:${dst}`,
    });
    assert.equal(res.isError, true);
    assert.match(text(res), /def is not running/);

    fake.add("def", "RUNNING");
    res = await run("workspace_copy", {
      source: `abc:${src}`,
      destination: `def:${dst}`,
      excludes: ["node_modules"],
    });
    assert.equal(res.isError, undefined, text(res));
    assert.equal(fs.readFileSync(path.join(dst, "a.txt"), "utf8"), "hello");
    assert.deepEqual(
      [...fs.readFileSync(path.join(dst, "sub", "b.bin"))],
      [0, 1, 2, 255],
    );
    assert.equal(fs.existsSync(path.join(dst, "sub", "node_modules")), false);
    const details = res.details as TransferDetails;
    assert.equal(details.type, "directory");
    assert.equal(details.source, `abc:${src}`);
    assert.ok(details.bytes > 0);
    assert.deepEqual(approvals, []);

    const local = path.join(dir, "local", "copy.txt");
    res = await run("workspace_copy", {
      source: `abc:${path.join(src, "a.txt")}`,
      destination: local,
    });
    assert.equal(res.isError, undefined, text(res));
    assert.equal(fs.readFileSync(local, "utf8"), "hello");

    const remote = path.join(dir, "remote", "up.txt");
    res = await run("workspace_copy", {
      source: local,
      destination: `def:${remote}`,
    });
    assert.equal(res.isError, undefined, text(res));
    assert.equal(fs.readFileSync(remote, "utf8"), "hello");
    assert.equal((res.details as TransferDetails).type, "file");

    res = await run("workspace_copy", {
      source: `agt:${src}`,
      destination: path.join(dir, "mirror"),
    });
    assert.equal(res.isError, undefined, text(res));
    assert.equal(
      fs.readFileSync(path.join(dir, "mirror", "a.txt"), "utf8"),
      "hello",
    );

    res = await run("workspace_copy", {
      source: `abc:${path.join(dir, "missing")}`,
      destination: local,
    });
    assert.equal(res.isError, true);
    assert.match(text(res), /does not exist/);
    assert.deepEqual(fs.readdirSync(path.join(dir, "tmp")), []);
  });

  it("reads the initialization logs", async () => {
    let res = await run("workspace_logs", { workspace: "abc", waitSeconds: 1 });
    assert.equal(res.isError, undefined, text(res));
    assert.match(
      text(res),
      /\[cloningRepo\] Cloning into 'repo'\.\.\.\n\[task\] npm warn\n\[task\] installed$/,
    );
    res = await run("workspace_logs", { workspace: "abc", tail: 1 });
    assert.match(text(res), /\(last 1 of 3 lines\)\n\[task\] installed$/);
    res = await run("workspace_logs", { workspace: "def" });
    assert.match(text(res), /not available since the Workspace is STOPPED/);
  });

  it("formats the helpers", () => {
    assert.deepEqual(parseEndpoint("abc:/a/b"), {
      workspace: "abc",
      path: "/a/b",
    });
    assert.deepEqual(parseEndpoint("abc:/a/b", "abc"), { path: "/a/b" });
    assert.deepEqual(parseEndpoint("/a:b"), { path: "/a:b" });
    assert.deepEqual(parseEndpoint("rel/path"), { path: "rel/path" });
    assert.equal(formatEndpoint({ workspace: "abc", path: "~/x" }), "abc:~/x");
    assert.equal(matchesName("my-space.alice", "my-space"), true);
    assert.equal(matchesName("my-space2.alice", "my-space"), false);
    assert.equal(matchesName(undefined, "x"), false);
    assert.equal(
      formatSummary({
        name: "abc",
        displayName: "Tests",
        state: "STOPPED",
        failure: "boom",
        isEphemeral: true,
      }),
      '- abc "Tests": STOPPED · ephemeral (last run failed: boom)',
    );
  });
});
