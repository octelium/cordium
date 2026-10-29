import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { Readable } from "node:stream";
import { afterEach, beforeEach, describe, it } from "node:test";
import type { Message } from "../protocol/index.ts";
import { SettingsStore } from "./settings.ts";
import { ConversationStore, isValidID } from "./conversations.ts";
import {
  FileStore,
  FileTooLargeError,
  guessMimeType,
  sanitizeFileName,
} from "./files.ts";

const message = (conversationId: string, text: string): Message => ({
  id: `m-${text}`,
  conversationId,
  role: "assistant",
  status: "streaming",
  createdAt: new Date().toISOString(),
  blocks: [{ id: "b1", type: "markdown", text, createdAt: "" }],
});

describe("conversation store", () => {
  let dir: string;

  beforeEach(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-store-"));
  });

  afterEach(() => {
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("creates, updates, lists and deletes conversations", () => {
    const store = new ConversationStore(dir);
    store.init();
    const a = store.create();
    assert.equal(a.title, "New conversation");
    assert.equal(a.messageCount, 0);
    const b = store.create("  Audit  ");
    assert.equal(b.title, "Audit");

    store.appendMessage(a.id, message(a.id, "hello"));
    assert.equal(store.get(a.id)?.messageCount, 1);
    assert.equal(store.list()[0].id, a.id);
    assert.deepEqual(
      store.getMessages(a.id).map((m) => m.id),
      ["m-hello"],
    );

    store.update(b.id, { title: "Renamed", activeRunId: "r1" });
    assert.equal(store.get(b.id)?.title, "Renamed");
    assert.equal(store.get(b.id)?.activeRunId, "r1");
    store.update(b.id, { activeRunId: undefined });
    assert.equal(store.get(b.id)?.activeRunId, undefined);

    store.setSessionFile(a.id, "/tmp/session.jsonl");
    assert.equal(store.getSessionFile(a.id), "/tmp/session.jsonl");
    assert.equal("sessionFile" in store.get(a.id)!, false);

    assert.equal(store.delete(a.id), true);
    assert.equal(store.get(a.id), undefined);
    assert.equal(store.delete(a.id), false);
  });

  it("reloads the conversations and recovers the interrupted messages", () => {
    const store = new ConversationStore(dir);
    store.init();
    const conv = store.create("Recover");
    store.update(conv.id, { activeRunId: "r1" });
    store.savePending(conv.id, message(conv.id, "partial"));
    store.setSessionFile(conv.id, "/tmp/s.jsonl");

    const reloaded = new ConversationStore(dir);
    const recovered = reloaded.init();
    assert.equal(recovered.length, 1);
    assert.equal(recovered[0].status, "interrupted");
    const loaded = reloaded.get(conv.id)!;
    assert.equal(loaded.title, "Recover");
    assert.equal(loaded.activeRunId, undefined);
    assert.equal(reloaded.getSessionFile(conv.id), "/tmp/s.jsonl");
    assert.deepEqual(
      reloaded.getMessages(conv.id).map((m) => [m.id, m.status]),
      [["m-partial", "interrupted"]],
    );
    assert.equal(reloaded.readPending(conv.id), undefined);
  });

  it("validates the IDs", () => {
    assert.equal(isValidID("2b3f6a0e-1c2d-4e5f-8a9b-0c1d2e3f4a5b"), true);
    assert.equal(isValidID("../etc"), false);
    assert.equal(isValidID(""), false);
    assert.equal(isValidID("a".repeat(65)), false);
  });
});

describe("file store", () => {
  let dir: string;

  beforeEach(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-files-"));
  });

  afterEach(() => {
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("saves the uploads and enforces the size limit", async () => {
    const store = new FileStore(dir);
    store.init();
    const info = await store.saveUpload(
      "../../evil name.json",
      undefined,
      Readable.from([Buffer.from('{"a":1}')]),
      100,
    );
    assert.equal(info.name, "evil name.json");
    assert.equal(info.mimeType, "application/json");
    assert.equal(info.size, 7);
    assert.ok(info.path.startsWith(path.join(dir, "uploads")));
    assert.deepEqual(store.getUpload(info.id), info);
    assert.equal(store.getUpload("../x"), undefined);

    await assert.rejects(
      store.saveUpload(
        "big.bin",
        undefined,
        Readable.from([Buffer.alloc(200)]),
        100,
      ),
      FileTooLargeError,
    );
    assert.equal(fs.readdirSync(path.join(dir, "uploads")).length, 1);
  });

  it("publishes the artifacts as copies", () => {
    const store = new FileStore(dir);
    store.init();
    const source = path.join(dir, "report.csv");
    fs.writeFileSync(source, "a,b\n1,2\n");
    const info = store.publishArtifact(source, { title: "Report" });
    assert.equal(info.name, "report.csv");
    assert.equal(info.mimeType, "text/csv");
    assert.equal(info.size, 8);
    assert.equal(info.url, `/v1/artifacts/${info.id}/content`);

    fs.writeFileSync(source, "changed");
    const stored = store.getArtifact(info.id)!;
    assert.equal(fs.readFileSync(stored.filePath, "utf8"), "a,b\n1,2\n");
    assert.throws(() => store.publishArtifact(dir));
    assert.equal(store.getArtifact("nope"), undefined);
  });

  it("sanitizes the names and guesses the MIME types", () => {
    assert.equal(sanitizeFileName("a/b/c.txt"), "c.txt");
    assert.equal(sanitizeFileName("..\\..\\x.txt"), "x.txt");
    assert.equal(sanitizeFileName(".hidden"), "_hidden");
    assert.equal(sanitizeFileName("a\u0000b?.txt"), "a_b_.txt");
    assert.equal(sanitizeFileName(""), "file");
    assert.equal(guessMimeType("x.PNG"), "image/png");
    assert.equal(guessMimeType("x.unknown"), "application/octet-stream");
  });
});

describe("store recovery and file boundaries", () => {
  let dir: string;

  beforeEach(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-boundaries-"));
  });

  afterEach(() => {
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("keeps valid messages when a journal contains a truncated record", () => {
    const store = new ConversationStore(dir);
    store.init();
    const conv = store.create();
    store.appendMessage(conv.id, message(conv.id, "first"));
    fs.appendFileSync(
      path.join(dir, conv.id, "messages.jsonl"),
      '{"partial":\n',
    );
    store.appendMessage(conv.id, message(conv.id, "last"));
    assert.deepEqual(
      store.getMessages(conv.id).map((m) => m.id),
      ["m-first", "m-last"],
    );
  });

  it("recovers a pending message only once across repeated restarts", () => {
    const store = new ConversationStore(dir);
    store.init();
    const conv = store.create();
    store.savePending(conv.id, message(conv.id, "pending"));
    const first = new ConversationStore(dir);
    assert.equal(first.init().length, 1);
    const second = new ConversationStore(dir);
    assert.deepEqual(second.init(), []);
    assert.equal(second.getMessages(conv.id).length, 1);
    assert.equal(second.get(conv.id)?.messageCount, 1);
  });

  it("does not duplicate a pending message already appended before a crash", () => {
    const store = new ConversationStore(dir);
    store.init();
    const conv = store.create();
    const pending = message(conv.id, "pending");
    store.savePending(conv.id, pending);
    store.appendMessage(conv.id, { ...pending, status: "completed" });
    const reloaded = new ConversationStore(dir);
    reloaded.init();
    assert.equal(reloaded.getMessages(conv.id).length, 1);
    assert.equal(reloaded.get(conv.id)?.messageCount, 1);
    assert.equal(reloaded.getMessages(conv.id)[0].status, "completed");
  });

  it("ignores malformed metadata without preventing valid conversations from loading", () => {
    const store = new ConversationStore(dir);
    store.init();
    const conv = store.create("Valid");
    for (const [id, metadata] of [
      ["bad-json", "{"],
      ["bad-null", "null"],
      ["bad-array", "[]"],
    ]) {
      fs.mkdirSync(path.join(dir, id));
      fs.writeFileSync(path.join(dir, id, "conversation.json"), metadata);
    }
    const reloaded = new ConversationStore(dir);
    assert.doesNotThrow(() => reloaded.init());
    assert.deepEqual(
      reloaded.list().map((c) => c.id),
      [conv.id],
    );
  });

  it("does not expose mutable conversation metadata to callers", () => {
    const store = new ConversationStore(dir);
    store.init();
    const conv = store.create("Original");
    conv.title = "Changed";
    store.list()[0].title = "Changed again";
    assert.equal(store.get(conv.id)?.title, "Original");
    assert.equal(store.update("missing", { title: "Ignored" }), undefined);
    assert.deepEqual(store.getMessages("missing"), []);
  });

  it("accepts exactly the upload limit and cleans up a multi-chunk overflow", async () => {
    const store = new FileStore(dir);
    store.init();
    const info = await store.saveUpload(
      "exact.bin",
      undefined,
      Readable.from([Buffer.alloc(3), Buffer.alloc(5)]),
      8,
    );
    assert.equal(info.size, 8);
    await assert.rejects(
      store.saveUpload(
        "overflow.bin",
        undefined,
        Readable.from([Buffer.alloc(4), Buffer.alloc(5)]),
        8,
      ),
      FileTooLargeError,
    );
    assert.deepEqual(fs.readdirSync(store.uploadsDir), [info.id]);
    const empty = await store.saveUpload(
      "empty.txt",
      undefined,
      Readable.from([]),
      0,
    );
    assert.equal(empty.size, 0);
  });

  it("removes partial upload data when the source stream fails", async () => {
    const store = new FileStore(dir);
    store.init();
    const failure = new Error("source failed");
    const source = Readable.from(
      (async function* () {
        yield Buffer.from("partial");
        throw failure;
      })(),
    );
    await assert.rejects(
      store.saveUpload("partial.txt", undefined, source, 100),
      failure,
    );
    assert.deepEqual(fs.readdirSync(store.uploadsDir), []);
  });

  it("reloads upload metadata and keeps uploaded files private", async () => {
    const store = new FileStore(dir);
    store.init();
    const info = await store.saveUpload(
      "data.json",
      "application/octet-stream",
      Readable.from(["{}"]),
      2,
    );
    const reloaded = new FileStore(dir);
    reloaded.init();
    assert.deepEqual(reloaded.getUpload(info.id), info);
    assert.equal(info.mimeType, "application/json");
    assert.equal(fs.statSync(info.path).mode & 0o777, 0o600);
    assert.equal(fs.statSync(path.dirname(info.path)).mode & 0o777, 0o700);
    fs.writeFileSync(path.join(store.uploadsDir, info.id, ".meta.json"), "{");
    assert.equal(store.getUpload(info.id), undefined);
  });

  it("does not return metadata for an upload whose content was removed", async () => {
    const store = new FileStore(dir);
    store.init();
    const info = await store.saveUpload(
      "removed.txt",
      undefined,
      Readable.from(["x"]),
      1,
    );
    fs.unlinkSync(info.path);
    assert.equal(store.getUpload(info.id), undefined);
  });

  it("accepts long UTF-8 filenames within filesystem byte limits", async () => {
    const store = new FileStore(dir);
    store.init();
    const info = await store.saveUpload(
      `${"é".repeat(140)}.txt`,
      undefined,
      Readable.from(["x"]),
      1,
    );
    assert.ok(Buffer.byteLength(info.name) <= 255);
    assert.equal(fs.readFileSync(info.path, "utf8"), "x");
  });

  it("reloads artifact metadata and ignores missing artifact content", () => {
    const store = new FileStore(dir);
    store.init();
    const source = path.join(dir, "source.txt");
    fs.writeFileSync(source, "content");
    const info = store.publishArtifact(source, {
      name: "../report.txt",
      conversationId: "conversation",
      title: "Report",
    });
    const reloaded = new FileStore(dir);
    reloaded.init();
    const artifact = reloaded.getArtifact(info.id)!;
    assert.equal(artifact.info.name, "report.txt");
    assert.equal(artifact.conversationId, "conversation");
    fs.unlinkSync(artifact.filePath);
    assert.equal(reloaded.getArtifact(info.id), undefined);
    assert.equal(reloaded.getArtifact("../../outside"), undefined);
  });
});

describe("settings store", () => {
  let dir: string;

  beforeEach(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-settings-"));
  });

  afterEach(() => {
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("persists settings and removes fields explicitly cleared by an update", () => {
    const store = new SettingsStore(dir);
    assert.deepEqual(store.read(), {});
    store.write({
      model: { provider: "test", id: "model" },
      thinkingLevel: "high",
      approvals: { api: "write", commands: true },
    });
    const updated = store.update({ thinkingLevel: undefined });
    assert.equal(updated.thinkingLevel, undefined);
    assert.deepEqual(new SettingsStore(dir).read(), {
      model: { provider: "test", id: "model" },
      approvals: { api: "write", commands: true },
    });
    assert.equal(
      fs.statSync(path.join(dir, "settings.json")).mode & 0o777,
      0o600,
    );
  });

  it("ignores corrupt or structurally invalid settings", () => {
    const store = new SettingsStore(dir);
    for (const input of ["{", "null", "[]", '"text"']) {
      fs.writeFileSync(path.join(dir, "settings.json"), input);
      assert.deepEqual(store.read(), {});
    }
    fs.writeFileSync(
      path.join(dir, "settings.json"),
      JSON.stringify({
        model: { provider: 1, id: "x" },
        thinkingLevel: false,
        approvals: { api: "unknown", commands: "true" },
      }),
    );
    assert.deepEqual(store.read(), { approvals: {} });
  });
});
