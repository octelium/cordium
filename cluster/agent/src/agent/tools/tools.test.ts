import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { describe, it } from "node:test";
import type { WebSearchConfig } from "../../config.ts";
import { needsApproval } from "./api.ts";
import { buildChart, buildTable, MAX_TABLE_ROWS } from "./present.ts";
import { parseCSV, readRowsFile, resolvePath, truncateText } from "./util.ts";
import { decodeEntities, htmlToText, runWebSearch } from "./web.ts";

describe("presentation tools", () => {
  it("builds the tables and infers the column types", () => {
    const table = buildTable([
      {
        name: "a",
        count: 1,
        ok: true,
        at: "2026-01-01T00:00:00Z",
        meta: { x: 1 },
      },
      { name: "b", count: 2, ok: false, at: "2026-01-02T00:00:00Z" },
    ]);
    assert.deepEqual(
      table.columns.map((c) => [c.key, c.type]),
      [
        ["name", "string"],
        ["count", "number"],
        ["ok", "boolean"],
        ["at", "datetime"],
        ["meta", "json"],
      ],
    );
    assert.deepEqual(table.rows[1].meta, null);
    assert.equal(table.totalRows, 2);
    assert.equal(table.truncated, undefined);

    const many = buildTable(
      Array.from({ length: MAX_TABLE_ROWS + 5 }, (_, i) => ({ i })),
      [{ key: "i", label: "Index" }],
    );
    assert.equal(many.rows.length, MAX_TABLE_ROWS);
    assert.equal(many.totalRows, MAX_TABLE_ROWS + 5);
    assert.equal(many.truncated, true);
    assert.equal(many.columns[0].label, "Index");

    assert.throws(() => buildTable([]), /no columns/);
    assert.throws(() => buildTable([1 as unknown as Record<string, unknown>]));
  });

  it("builds and validates the charts", () => {
    const chart = buildChart(
      {
        chartType: "line",
        x: { key: "t", type: "time" },
        series: [{ key: "v" }, { key: "w" }],
      },
      [
        { t: "2026-01-01", v: 1, w: "2" },
        { t: "2026-01-02", v: "x", w: 3 },
      ],
    );
    assert.deepEqual(chart.rows, [
      { t: "2026-01-01", v: 1, w: 2 },
      { t: "2026-01-02", v: null, w: 3 },
    ]);

    assert.throws(
      () =>
        buildChart(
          { chartType: "bar", x: { key: "t" }, series: [{ key: "missing" }] },
          [{ t: 1, v: 1 }],
        ),
      /missing.*Available keys: t, v/,
    );
    assert.throws(
      () =>
        buildChart(
          { chartType: "bar", x: { key: "t" }, series: [{ key: "v" }] },
          [],
        ),
      /no data/,
    );
    assert.throws(
      () =>
        buildChart(
          { chartType: "bar", x: { key: "t" }, series: [{ key: "v" }] },
          [{ t: 1, v: "a" }],
        ),
      /numeric/,
    );
  });

  it("reads the data files", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-rows-"));
    try {
      const csv = path.join(dir, "a.csv");
      fs.writeFileSync(csv, 'name,note\r\n"a, b","say ""hi"""\r\nc,\r\n');
      assert.deepEqual(readRowsFile(csv), [
        { name: "a, b", note: 'say "hi"' },
        { name: "c", note: "" },
      ]);

      const jsonl = path.join(dir, "a.jsonl");
      fs.writeFileSync(jsonl, '{"a":1}\n\n{"a":2}\n');
      assert.deepEqual(readRowsFile(jsonl), [{ a: 1 }, { a: 2 }]);

      const json = path.join(dir, "a.json");
      fs.writeFileSync(json, JSON.stringify({ items: [{ a: 1 }] }));
      assert.deepEqual(readRowsFile(json), [{ a: 1 }]);

      fs.writeFileSync(json, JSON.stringify({ a: 1 }));
      assert.throws(() => readRowsFile(json), /JSON array/);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it("formats the helpers", () => {
    assert.deepEqual(parseCSV(""), []);
    assert.deepEqual(truncateText("abcdef", 3), {
      text: "abc\n…[truncated]",
      truncated: true,
    });
    assert.equal(resolvePath("/work", "a/b.txt"), "/work/a/b.txt");
    assert.equal(resolvePath("/work", "/tmp/x"), "/tmp/x");
  });
});

describe("approval policy", () => {
  it("decides which risks need an approval", () => {
    assert.equal(needsApproval("read", "write"), false);
    assert.equal(needsApproval("write", "write"), true);
    assert.equal(needsApproval("sensitive", "write"), true);
    assert.equal(needsApproval("destructive", "write"), true);
    assert.equal(needsApproval("read", "destructive"), false);
    assert.equal(needsApproval("write", "destructive"), false);
    assert.equal(needsApproval("sensitive", "destructive"), true);
    assert.equal(needsApproval("destructive", "destructive"), true);
    assert.equal(needsApproval("destructive", "never"), false);
    assert.equal(needsApproval("sensitive", "never"), false);
  });
});

describe("web tools", () => {
  it("converts HTML to text", () => {
    const { title, text } =
      htmlToText(`<html><head><title>Hello &amp; welcome</title>
<style>body{}</style><script>alert(1)</script></head>
<body><h1>Title</h1><p>Some <b>bold</b> text&nbsp;here.</p>
<ul><li>One</li><li>Two</li></ul>
<a href="https://octelium.com/docs">Docs</a> <a href="#top">Top</a>
<!-- comment --></body></html>`);
    assert.equal(title, "Hello & welcome");
    assert.equal(
      text,
      "# Title\nSome bold text here.\n\n- One\n- Two\n\nDocs (https://octelium.com/docs) Top",
    );
    assert.equal(decodeEntities("&#x41;&#66;&lt;&unknown;"), "AB<&unknown;");
  });

  it("queries the web search providers", async () => {
    const requests: { url: string; init?: RequestInit }[] = [];
    const fetchImpl = (async (url: URL | string, init?: RequestInit) => {
      requests.push({ url: url.toString(), init });
      const body = url.toString().includes("brave")
        ? {
            web: {
              results: [{ title: "T", url: "https://a", description: "D" }],
            },
          }
        : { results: [{ title: "T2", url: "https://b", content: "C" }] };
      return new Response(JSON.stringify(body), { status: 200 });
    }) as typeof fetch;

    const brave: WebSearchConfig = { provider: "brave", apiKey: "k" };
    assert.deepEqual(
      await runWebSearch(brave, "octelium", 3, undefined, fetchImpl),
      [{ title: "T", url: "https://a", snippet: "D" }],
    );
    assert.match(
      requests[0].url,
      /^https:\/\/api\.search\.brave\.com\/res\/v1\/web\/search\?q=octelium&count=3$/,
    );
    assert.equal(
      (requests[0].init?.headers as Record<string, string>)[
        "x-subscription-token"
      ],
      "k",
    );

    const tavily: WebSearchConfig = {
      provider: "tavily",
      baseUrl: "http://search.local.example.com",
    };
    assert.deepEqual(await runWebSearch(tavily, "q", 2, undefined, fetchImpl), [
      { title: "T2", url: "https://b", snippet: "C" },
    ]);
    assert.equal(requests[1].url, "http://search.local.example.com/search");
    assert.equal(requests[1].init?.method, "POST");

    await assert.rejects(
      runWebSearch({ provider: "searxng" }, "q", 2, undefined, fetchImpl),
      /baseUrl/,
    );
  });
});

describe("web_fetch", () => {
  it("fetches and converts the pages", async () => {
    const http = await import("node:http");
    const { createWebTools } = await import("./web.ts");
    const server = http.createServer((req, res) => {
      if (req.url === "/page") {
        res.writeHead(200, { "content-type": "text/html; charset=utf-8" });
        res.end(
          "<html><head><title>Page</title></head><body><p>Hello world</p></body></html>",
        );
        return;
      }
      res.writeHead(404, { "content-type": "text/plain" });
      res.end("missing");
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const port = (server.address() as { port: number }).port;

    try {
      const [webFetch] = createWebTools({
        conversationId: "c1",
        workDir: os.tmpdir(),
        runtime: { current: () => undefined },
      });
      const exec = (args: Record<string, unknown>) =>
        webFetch.execute(
          "call-1",
          args as never,
          undefined,
          undefined,
          undefined as never,
        );

      const page = await exec({ url: `http://127.0.0.1:${port}/page` });
      const text = page.content
        .map((c) => (c.type === "text" ? c.text : ""))
        .join("");
      assert.match(text, /Status: 200 OK/);
      assert.match(text, /Title: Page/);
      assert.match(text, /Hello world/);
      assert.equal(page.isError, undefined);

      const missing = await exec({ url: `http://127.0.0.1:${port}/nope` });
      assert.equal(missing.isError, true);

      const invalid = await exec({ url: "file:///etc/passwd" });
      assert.equal(invalid.isError, true);
    } finally {
      server.close();
    }
  });
});

describe("built-in tools", () => {
  it("drops the tools whose binaries are missing", async () => {
    const { filterAvailableTools, findBinary, createBuiltinTools } =
      await import("./builtin.ts");
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-bin-"));
    const settings = {
      get: () => ({
        approvals: { api: "destructive" as const, commands: false },
      }),
    };
    try {
      const rg = path.join(dir, "rg");
      fs.writeFileSync(rg, "#!/bin/sh\n", { mode: 0o755 });
      assert.equal(findBinary(["rg"], dir), rg);
      assert.equal(findBinary(["fd", "fdfind"], dir), undefined);
      assert.deepEqual(
        filterAvailableTools(["read", "grep", "find", "bash"], dir),
        {
          available: ["read", "grep", "bash"],
          unavailable: ["find"],
        },
      );
      const tools = createBuiltinTools({
        conversationId: "c1",
        workDir: dir,
        runtime: { current: () => undefined },
        names: ["read", "bash"],
        settings,
      });
      assert.deepEqual(
        tools.map((t) => t.name),
        ["read", "bash"],
      );
      assert.throws(
        () =>
          createBuiltinTools({
            conversationId: "c1",
            workDir: dir,
            runtime: { current: () => undefined },
            names: ["nope"],
            settings,
          }),
        /Unknown built-in tool "nope"/,
      );
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
