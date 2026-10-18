import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, afterEach, before, beforeEach, describe, it } from "node:test";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { build, preview } from "vite";

const root = fileURLToPath(new URL("../", import.meta.url));

describe("Cordium portal cluster domain", () => {
  let directory;
  let browser;
  let context;
  let page;
  let errors;
  const servers = new Map();

  before(async () => {
    directory = await mkdtemp(join(tmpdir(), "cordium-portal-domain-"));
    for (const mode of ["production", "development"]) {
      const outDir = join(directory, mode);
      await build({
        root,
        mode,
        logLevel: "silent",
        build: {
          outDir,
          rolldownOptions: {
            input: join(root, "tests/fixtures/domain.html"),
          },
        },
      });
      servers.set(
        mode,
        await preview({
          root,
          logLevel: "silent",
          build: { outDir },
          preview: { host: "127.0.0.1", port: 0 },
        }),
      );
    }
    browser = await chromium.launch();
  });

  after(async () => {
    await browser?.close();
    for (const server of servers.values())
      await new Promise((resolve) => server.httpServer.close(resolve));
    if (directory) await rm(directory, { recursive: true, force: true });
  });

  beforeEach(async () => {
    context = await browser.newContext();
    page = await context.newPage();
    page.setDefaultTimeout(10_000);
    errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
  });

  afterEach(async () => {
    await context?.close();
    assert.deepEqual(errors, []);
  });

  const load = async (mode, globals) => {
    await page.addInitScript((globals) => {
      if (globals === undefined) {
        delete window.__OCTELIUM_GLOBALS__;
      } else {
        window.__OCTELIUM_GLOBALS__ = globals;
      }
    }, globals);
    const origin = servers.get(mode).resolvedUrls.local[0];
    await page.goto(`${origin}tests/fixtures/domain.html`);
    await page.waitForFunction(() => window.portalDomain);
  };

  const request = async (regional) => {
    const requests = [];
    const originalFetch = window.fetch;
    window.fetch = async (url, init) => {
      requests.push({ url, credentials: init.credentials });
      const trailer = new TextEncoder().encode("grpc-status: 0\r\n");
      return new Response(
        new Uint8Array([
          0, 0, 0, 0, 0, 128, 0, 0, 0, trailer.length, ...trailer,
        ]),
        { headers: { "content-type": "application/grpc-web+proto" } },
      );
    };
    try {
      await window.portalDomain.requestStatus();
      document.cookie = "octelium_domain=other.example.test; path=/";
      await window.portalDomain.requestTerminals();
      await window.portalDomain.requestTerminals({ name: "default" });
      if (regional) {
        await window.portalDomain.requestTerminals({ name: "region-1" });
      }
    } finally {
      window.fetch = originalFetch;
    }
    return requests;
  };

  it("uses the server domain without a domain cookie", async () => {
    await load("production", { cluster: { domain: "example.test" } });
    const result = await page.evaluate(() => ({
      domain: window.portalDomain.getDomain(),
      cookie: document.cookie,
    }));
    assert.equal(result.domain, "example.test");
    assert.equal(result.cookie, "");
  });

  it("uses the server domain for default and regional credentialed RPCs despite injected domain cookies", async () => {
    await page.addInitScript(() => {
      document.cookie = "octelium_domain=namespace.example.test; path=/";
    });
    await load("production", { cluster: { domain: "example.test" } });
    const requests = await page.evaluate(request, true);
    const result = await page.evaluate(() => ({
      domain: window.portalDomain.getDomain(),
      isDev: window.portalDomain.isDev(),
      cookie: document.cookie,
    }));
    assert.equal(result.isDev, false);
    assert.equal(result.domain, "example.test");
    assert.ok(result.cookie.includes("octelium_domain=other.example.test"));
    assert.deepEqual(
      requests.map(({ url }) => new URL(url).origin),
      [
        "http://octelium-api.example.test",
        "http://octelium-api.example.test",
        "http://octelium-api.example.test",
        "http://region-1.octelium-api.example.test",
      ],
    );
    assert.ok(requests[0].url.endsWith("/GetStatus"));
    for (const item of requests.slice(1)) {
      assert.ok(item.url.endsWith("/ListTerminal"));
    }
    assert.ok(requests.every(({ credentials }) => credentials === "include"));
  });

  for (const globals of [
    undefined,
    null,
    {},
    { cluster: null },
    { cluster: {} },
    { cluster: { domain: "" } },
    { cluster: { domain: 123 } },
  ]) {
    it(`rejects invalid server globals ${JSON.stringify(globals)} without trusting the cookie`, async () => {
      await page.addInitScript(() => {
        document.cookie = "octelium_domain=namespace.example.test; path=/";
      });
      await load("production", globals);
      const result = await page.evaluate(() =>
        [
          () => window.portalDomain.getDomain(),
          () => window.portalDomain.getClientUser(),
          () => window.portalDomain.getClientWorkspace(),
          () => window.portalDomain.getClientWorkspaceSvc({ name: "region-1" }),
        ].map((call) => {
          try {
            call();
            return "unexpected domain";
          } catch (error) {
            return error.message;
          }
        }),
      );
      assert.deepEqual(result, Array(4).fill("The cluster domain is unavailable"));
    });
  }

  it("keeps development RPCs on the current host without server globals", async () => {
    await page.addInitScript(() => {
      document.cookie = "octelium_domain=namespace.example.test; path=/";
    });
    await load("development", undefined);
    const requests = await page.evaluate(request, false);
    const result = await page.evaluate(() => ({
      domain: window.portalDomain.getDomain(),
      isDev: window.portalDomain.isDev(),
    }));
    assert.equal(result.isDev, true);
    assert.equal(result.domain, new URL(page.url()).host);
    assert.equal(requests.length, 3);
    assert.ok(
      requests.every(({ url }) => new URL(url).origin === new URL(page.url()).origin),
    );
    assert.ok(requests.every(({ credentials }) => credentials === "include"));
  });
});
