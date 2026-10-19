import assert from "node:assert/strict";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, afterEach, before, beforeEach, describe, it } from "node:test";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { build, preview } from "vite";

const root = fileURLToPath(new URL("../", import.meta.url));
const artifacts = process.env.CORDIUM_PORTAL_ARTIFACT_DIR;

describe("Portal navigation, Services and terminals", () => {
  let directory;
  let server;
  let browser;
  let context;
  let page;
  let errors;

  before(async () => {
    directory = await mkdtemp(join(tmpdir(), "cordium-portal-ui-"));
    await build({
      root,
      mode: "production",
      logLevel: "silent",
      build: {
        outDir: directory,
        sourcemap: false,
        rolldownOptions: { input: join(root, "tests/fixtures/portal.html") },
      },
    });
    server = await preview({
      root,
      logLevel: "silent",
      build: { outDir: directory },
      preview: { host: "127.0.0.1", port: 0 },
    });
    browser = await chromium.launch({ args: ["--enable-unsafe-swiftshader"] });
    if (artifacts) await mkdir(artifacts, { recursive: true });
  });

  after(async () => {
    await browser?.close();
    if (server)
      await new Promise((resolve) => server.httpServer.close(resolve));
    if (directory) await rm(directory, { recursive: true, force: true });
  });

  beforeEach(async () => {
    context = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
    });
    page = await context.newPage();
    page.setDefaultTimeout(15_000);
    errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
  });

  afterEach(async () => {
    await context?.close();
    assert.deepEqual(errors, []);
  });

  const load = async (route = "/services", theme = "light") => {
    const origin = server.resolvedUrls.local[0];
    await page.goto(
      `${origin}tests/fixtures/portal.html?${new URLSearchParams({ route, theme })}`,
    );
    await page
      .getByRole("heading", {
        name: route.startsWith("/services")
          ? "Services"
          : route.startsWith("/workspaces/other")
            ? "Other workspace"
            : "Development workspace",
        exact: true,
      })
      .waitFor();
  };
  const terminals = async () => {
    await load("/workspaces/demo/terminals");
    await page.getByRole("tab", { name: "main", exact: true }).waitFor();
    await page.getByRole("tab", { name: "worker", exact: true }).waitFor();
    await page.getByRole("tab", { name: "logs", exact: true }).waitFor();
  };
  const order = () =>
    page
      .locator("[data-terminal-tab]")
      .evaluateAll((tabs) => tabs.map((tab) => tab.dataset.terminalTab));
  const terminalText = (id = "terminal-main") =>
    page.evaluate((id) => {
      const record = window.portalFixture.terminals.find(
        (record) =>
          !record.disposed &&
          record.terminal.element?.closest("[role=tabpanel]")?.id ===
            `terminal-panel-${id}`,
      );
      if (!record) return "";
      const buffer = record.terminal.buffer.active;
      return Array.from({ length: buffer.length }, (_, index) =>
        buffer.getLine(index).translateToString(true),
      ).join("\n");
    }, id);
  const screenshot = async (name) => {
    if (artifacts)
      await page.screenshot({
        path: join(artifacts, `${name}.png`),
        fullPage: true,
      });
  };

  it("persists the collapsed desktop sidebar and keeps the selected route accessible", async () => {
    await load();
    await page
      .getByRole("button", { name: "Collapse sidebar", exact: true })
      .click();
    await page.waitForFunction(
      () =>
        document
          .querySelector('[aria-label="Portal sidebar"]')
          .getBoundingClientRect().width === 64,
    );
    assert.equal(
      await page
        .getByRole("link", { name: "Services", exact: true })
        .getAttribute("aria-current"),
      "page",
    );
    await load();
    await page
      .getByRole("button", { name: "Expand sidebar", exact: true })
      .waitFor();
    assert.equal(
      await page.getByRole("link", { name: "Workspaces", exact: true }).count(),
      1,
    );
  });

  it("opens mobile navigation in a focus-trapped drawer and closes it after navigation", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await load();
    await page
      .getByRole("button", { name: "Open navigation", exact: true })
      .click();
    const drawer = page.getByRole("dialog", { name: "Portal navigation" });
    await drawer.waitFor();
    await page.keyboard.press("Shift+Tab");
    assert.equal(
      await drawer.evaluate((element) =>
        element.contains(document.activeElement),
      ),
      true,
    );
    await drawer.getByRole("link", { name: "Workspaces", exact: true }).click();
    await drawer.waitFor({ state: "detached" });
    assert.equal(new URL(page.url()).pathname, "/workspaces");
  });

  it("searches beyond the current page and resets pagination when filters change", async () => {
    await load("/services?common.page=2");
    await page
      .getByRole("searchbox", { name: "Search Services" })
      .fill("service-24 4024");
    await page.waitForFunction(
      () => new URL(location.href).searchParams.get("q") === "service-24 4024",
    );
    await page.getByText("service-24", { exact: true }).waitFor();
    assert.equal(new URL(page.url()).searchParams.has("common.page"), false);
    assert.equal(
      await page.locator('[aria-label^="Show details of"]').count(),
      1,
    );
    const request = await page.evaluate(
      () =>
        window.portalFixture.calls
          .filter((call) => call.method === "ListService")
          .at(-1).input,
    );
    assert.equal(request.common.itemsPerPage, 500);
    assert.equal(request.common.page, 0);
    await page
      .getByRole("searchbox", { name: "Search Services" })
      .press("Escape");
    await page.waitForFunction(
      () => !new URL(location.href).searchParams.has("q"),
    );
    assert.equal(
      await page
        .getByRole("searchbox", { name: "Search Services" })
        .inputValue(),
      "",
    );
  });

  it("uses server Namespace and type filters with persistent URLs and removable chips", async () => {
    await load();
    await page
      .getByRole("combobox", { name: "Filter by type", exact: true })
      .click();
    await page.getByRole("option", { name: "PostgreSQL", exact: true }).click();
    await page.getByText("postgres.data", { exact: true }).waitFor();
    await page
      .getByRole("combobox", { name: "Filter by Namespace", exact: true })
      .click();
    await page.getByRole("option", { name: "data", exact: true }).click();
    await page
      .getByRole("button", { name: "Remove namespace filter", exact: true })
      .waitFor();
    const route = `${new URL(page.url()).pathname}${new URL(page.url()).search}`;
    await load(route, "dark");
    await page.getByText("postgres.data", { exact: true }).waitFor();
    assert.equal(
      await page
        .getByRole("button", { name: "Remove type filter", exact: true })
        .count(),
      1,
    );
    await page
      .getByRole("button", { name: "Clear filters", exact: true })
      .click();
    await page.waitForFunction(() => location.search === "");
    await page
      .getByRole("combobox", { name: "Filter by type", exact: true })
      .click();
    await page.getByRole("option", { name: "LLM", exact: true }).click();
    await page.getByText("llm", { exact: true }).waitFor();
  });

  it("shows empty search results and makes the search coverage limit explicit", async () => {
    await load();
    await page
      .getByRole("searchbox", { name: "Search Services" })
      .fill("no-such-service");
    await page.getByText("No matching Services", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Clear filters", exact: true })
      .last()
      .click();
    await page.evaluate(() => {
      window.portalFixture.serviceTotal = 900;
    });
    await page
      .getByRole("searchbox", { name: "Search Services" })
      .fill("10.10.10.10");
    await page.waitForFunction(
      () => new URL(location.href).searchParams.get("q") === "10.10.10.10",
    );
    await page.getByText("postgres.data", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Refresh Services", exact: true })
      .click();
    await page
      .getByText(/Search covers the first .* of 900 Services/)
      .waitFor();
  });

  it("retains loaded Services after refresh failures and supports retry", async () => {
    await load();
    await page.getByText("service-00", { exact: true }).waitFor();
    await page.evaluate(() => {
      window.portalFixture.failServices = true;
    });
    await page
      .getByRole("button", { name: "Refresh Services", exact: true })
      .click();
    await page
      .getByText("Refreshing failed. Showing the last loaded Services.")
      .waitFor();
    assert.equal(
      await page.getByText("service-00", { exact: true }).isVisible(),
      true,
    );
    await page.evaluate(() => {
      window.portalFixture.failServices = false;
    });
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await page
      .getByText("Refreshing failed. Showing the last loaded Services.")
      .waitFor({ state: "detached" });
  });

  it("groups workspace actions horizontally and keeps YAML and stop confirmation accessible", async () => {
    await load("/workspaces/demo");
    const group = page.getByRole("group", { name: "Workspace actions" });
    const boxes = await Promise.all([
      group.getByRole("link", { name: "Open", exact: true }).boundingBox(),
      group.getByRole("button", { name: "Stop", exact: true }).boundingBox(),
      group
        .getByRole("button", { name: "View YAML", exact: true })
        .boundingBox(),
    ]);
    assert.ok(boxes.every((box) => box.y === boxes[0].y));
    await group.getByRole("button", { name: "View YAML", exact: true }).click();
    await page.getByRole("dialog").waitFor();
    await page.keyboard.press("Escape");
    await page.getByRole("dialog").waitFor({ state: "detached" });
    await group.getByRole("button", { name: "Stop", exact: true }).click();
    await page.getByRole("dialog", { name: "Stop this workspace?" }).waitFor();
  });

  it("pins, reorders and restores tab preferences while preserving running terminal instances", async () => {
    await terminals();
    await page
      .getByRole("button", { name: "Actions for logs", exact: true })
      .click();
    await page.getByRole("menuitem", { name: "Pin tab", exact: true }).click();
    await page
      .getByRole("tab", { name: "logs (pinned)", exact: true })
      .waitFor();
    assert.deepEqual(await order(), [
      "terminal-logs",
      "terminal-main",
      "terminal-worker",
    ]);
    await page.getByRole("tab", { name: "worker", exact: true }).click();
    await page
      .getByRole("tab", { name: "worker", exact: true })
      .press("Alt+Shift+ArrowLeft");
    assert.deepEqual(await order(), [
      "terminal-logs",
      "terminal-worker",
      "terminal-main",
    ]);
    const live = await page.evaluate(
      () =>
        window.portalFixture.terminals.filter((record) => !record.disposed)
          .length,
    );
    assert.equal(live, 3);
    await load("/workspaces/demo/terminals");
    await page
      .getByRole("tab", { name: "logs (pinned)", exact: true })
      .waitFor();
    assert.deepEqual(await order(), [
      "terminal-logs",
      "terminal-worker",
      "terminal-main",
    ]);
    await page
      .getByRole("button", { name: "Actions for logs (pinned)", exact: true })
      .click();
    await page
      .getByRole("menuitem", { name: "Unpin tab", exact: true })
      .click();
    await page.getByRole("tab", { name: "logs", exact: true }).waitFor();
    assert.equal(
      await page
        .locator('[data-terminal-tab="terminal-logs"]')
        .getAttribute("data-pinned"),
      "false",
    );
  });

  it("switches terminal tabs with wheel scrolling in normal and full-screen modes", async () => {
    await terminals();
    for (const fullscreen of [false, true]) {
      if (fullscreen) {
        await page
          .getByRole("button", { name: "Toggle full screen", exact: true })
          .click();
      }
      const scrollY = await page.evaluate(() => window.scrollY);
      for (const [deltaX, deltaY, expected] of [
        [0, 80, "worker"],
        [0, 80, "logs"],
        [0, 80, "logs"],
        [-80, 0, "worker"],
        [0, -80, "main"],
        [0, -80, "main"],
      ]) {
        await page.getByRole("tablist", { name: "Terminal sessions" }).hover();
        await page.waitForTimeout(110);
        await page.mouse.wheel(deltaX, deltaY);
        await page.waitForFunction(
          (title) =>
            document
              .getElementById(`terminal-tab-terminal-${title}`)
              ?.getAttribute("aria-selected") === "true",
          expected,
        );
      }
      assert.equal(await page.evaluate(() => window.scrollY), scrollY);
    }
  });

  for (const theme of ["light", "dark"]) {
    it(`keeps full-screen tab menus clickable in the ${theme} theme`, async () => {
      await load("/workspaces/demo/terminals", theme);
      await page.getByRole("tab", { name: "logs", exact: true }).waitFor();
      const fullscreen = page.getByRole("button", {
        name: "Toggle full screen",
        exact: true,
      });
      await fullscreen.click();
      await page
        .getByRole("button", { name: "Actions for logs", exact: true })
        .click();
      await page
        .getByRole("menuitem", { name: "Pin tab", exact: true })
        .click();
      await page
        .getByRole("tab", { name: "logs (pinned)", exact: true })
        .waitFor();
      await page
        .getByRole("button", { name: "Actions for logs (pinned)", exact: true })
        .click();
      await page
        .getByRole("menuitem", { name: "Unpin tab", exact: true })
        .click();
      await page
        .getByRole("tab", { name: "logs", exact: true })
        .click({ button: "right" });
      await page
        .getByRole("menuitem", { name: "Move right", exact: true })
        .click();
      assert.deepEqual(await order(), [
        "terminal-main",
        "terminal-logs",
        "terminal-worker",
      ]);
      await page
        .getByRole("button", { name: "Actions for worker", exact: true })
        .click();
      await page
        .getByRole("menu", { name: "Actions for worker", exact: true })
        .getByRole("menuitem", { name: "Close terminal", exact: true })
        .click();
      await page
        .getByRole("tab", { name: "worker", exact: true })
        .waitFor({ state: "detached" });
      assert.equal(await fullscreen.getAttribute("aria-pressed"), "true");
    });
  }

  it("keeps pinned tabs and titles when the terminal list refreshes", async () => {
    await terminals();
    await page
      .getByRole("button", { name: "Actions for worker", exact: true })
      .click();
    await page.getByRole("menuitem", { name: "Pin tab", exact: true }).click();
    await page.evaluate(() => window.portalFixture.refreshTerminals());
    await page
      .getByRole("tab", { name: "worker (pinned)", exact: true })
      .waitFor();
    assert.deepEqual(await order(), [
      "terminal-worker",
      "terminal-main",
      "terminal-logs",
    ]);
    assert.equal(
      await page.getByRole("tab", { name: "main", exact: true }).count(),
      1,
    );
  });

  it("retains sessions after failed closes and tolerates invalid saved preferences", async () => {
    await context.addInitScript(() =>
      localStorage.setItem("cordium-terminal-tabs:workspace-demo", "{invalid"),
    );
    await terminals();
    await page.evaluate(() => {
      window.portalFixture.failRemove = true;
    });
    await page.getByRole("button", { name: "Close main", exact: true }).click();
    await page.waitForFunction(() =>
      window.portalFixture.calls.some(
        (call) => call.method === "RemoveTerminal",
      ),
    );
    assert.equal(
      await page.getByRole("tab", { name: "main", exact: true }).isVisible(),
      true,
    );
    await page.evaluate(() => {
      window.portalFixture.failRemove = false;
    });
    await page.getByRole("button", { name: "Close main", exact: true }).click();
    await page
      .getByRole("tab", { name: "main", exact: true })
      .waitFor({ state: "detached" });
  });

  it("reorders tabs by dragging and keeps arrow-key selection and close history valid", async () => {
    await terminals();
    const main = page.locator('[data-terminal-tab="terminal-main"]');
    const worker = page.locator('[data-terminal-tab="terminal-worker"]');
    await main.dragTo(worker, { targetPosition: { x: 170, y: 16 } });
    assert.deepEqual(await order(), [
      "terminal-worker",
      "terminal-main",
      "terminal-logs",
    ]);
    await page.getByRole("tab", { name: "main", exact: true }).focus();
    await page.keyboard.press("ArrowRight");
    assert.equal(
      await page
        .getByRole("tab", { name: "logs", exact: true })
        .getAttribute("aria-selected"),
      "true",
    );
    await page.getByRole("tab", { name: "logs", exact: true }).click();
    await page.getByRole("button", { name: "Close logs", exact: true }).click();
    await page
      .getByRole("tab", { name: "logs", exact: true })
      .waitFor({ state: "detached" });
    assert.equal(
      await page
        .getByRole("tab", { name: "main", exact: true })
        .getAttribute("aria-selected"),
      "true",
    );
    assert.equal(
      await page.evaluate(
        () =>
          window.portalFixture.terminals.filter((record) => !record.disposed)
            .length,
      ),
      2,
    );
  });

  it("restores subscriptions and geometry after a socket error without losing tabs or duplicating history", async () => {
    await terminals();
    await page.evaluate(() =>
      window.portalFixture.sockets.at(-1).dispatchEvent(new Event("error")),
    );
    await page.getByText("Reconnecting…", { exact: true }).waitFor();
    assert.equal(await page.getByRole("tab").count(), 3);
    await page.waitForFunction(
      () =>
        window.portalFixture.sockets.length > 1 &&
        window.portalFixture.sockets.at(-1).subscriptions.size === 3,
    );
    await page
      .getByText("Reconnecting…", { exact: true })
      .waitFor({ state: "detached" });
    await page.waitForFunction(() =>
      window.portalFixture.sockets
        .at(-1)
        .sent.some(
          (message) =>
            message.type.oneofKind === "setTerminalWindowSizeRequest",
        ),
    );
    assert.equal((await terminalText()).split("Welcome to main").length - 1, 1);
    assert.equal(
      await page.evaluate(
        () =>
          window.portalFixture.terminals.filter((record) => !record.disposed)
            .length,
      ),
      3,
    );
  });

  it("keeps terminal screens mounted when a reconnect list refresh fails", async () => {
    await terminals();
    const instances = await page.evaluate(
      () => window.portalFixture.terminals.length,
    );
    await page.evaluate(() => {
      window.portalFixture.failTerminals = true;
      window.portalFixture.sockets.at(-1).close();
    });
    await page
      .getByText("Could not refresh terminal sessions", { exact: true })
      .waitFor();
    assert.equal(await page.getByRole("tab").count(), 3);
    assert.equal(
      await page.evaluate(() => window.portalFixture.terminals.length),
      instances,
    );
    await page.evaluate(() => {
      window.portalFixture.failTerminals = false;
    });
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await page
      .getByText("Could not refresh terminal sessions", { exact: true })
      .waitFor({ state: "detached" });
    assert.equal(
      await page.evaluate(() => window.portalFixture.terminals.length),
      instances,
    );
  });

  it("reconciles terminals removed remotely while disconnected", async () => {
    await terminals();
    await page.evaluate(() => {
      window.portalFixture.missingTerminalIDs = ["terminal-worker"];
      window.portalFixture.sockets.at(-1).close();
    });
    await page.getByText("Reconnecting…", { exact: true }).waitFor();
    await page
      .getByRole("tab", { name: "worker", exact: true })
      .waitFor({ state: "detached" });
    assert.deepEqual(await order(), ["terminal-main", "terminal-logs"]);
    assert.equal(
      await page
        .getByRole("tab", { name: "main", exact: true })
        .getAttribute("aria-selected"),
      "true",
    );
  });

  it("keeps sessions and saved preferences scoped to their Workspace", async () => {
    await terminals();
    await page
      .getByRole("button", { name: "Actions for worker", exact: true })
      .click();
    await page.getByRole("menuitem", { name: "Pin tab", exact: true }).click();
    await page.evaluate(() => {
      history.pushState(null, "", "/workspaces/other/terminals");
      dispatchEvent(new PopStateEvent("popstate"));
    });
    await page
      .getByRole("heading", { name: "Other workspace", exact: true })
      .waitFor();
    await page.getByRole("tab", { name: "other", exact: true }).waitFor();
    assert.deepEqual(await order(), ["terminal-other"]);
    assert.deepEqual(
      await page.evaluate(() =>
        JSON.parse(
          localStorage.getItem("cordium-terminal-tabs:workspace-other"),
        ).map((tab) => tab.id),
      ),
      ["terminal-other"],
    );
    assert.equal(
      await page.evaluate(
        () =>
          JSON.parse(
            localStorage.getItem("cordium-terminal-tabs:workspace-demo"),
          )[0].id,
      ),
      "terminal-worker",
    );
  });

  it("does not restore subscriptions for sessions closed during a disconnect", async () => {
    await terminals();
    await page.evaluate(() => window.portalFixture.sockets.at(-1).close());
    await page.getByText("Reconnecting…", { exact: true }).waitFor();
    await page
      .getByRole("button", { name: "Close worker", exact: true })
      .click();
    await page
      .getByRole("tab", { name: "worker", exact: true })
      .waitFor({ state: "detached" });
    await page.waitForFunction(
      () =>
        window.portalFixture.sockets.length > 1 &&
        window.portalFixture.sockets.at(-1).subscriptions.size === 2,
    );
    assert.equal(
      await page.evaluate(() =>
        window.portalFixture.sockets
          .at(-1)
          .subscriptions.has("terminal-worker"),
      ),
      false,
    );
  });

  it("preserves the screen and terminal instances across full screen and redraws on resume", async () => {
    await terminals();
    const before = await terminalText();
    const count = await page.evaluate(
      () => window.portalFixture.terminals.length,
    );
    await page
      .getByRole("button", { name: "Toggle full screen", exact: true })
      .click();
    assert.equal(
      await page
        .getByRole("button", { name: "Toggle full screen", exact: true })
        .getAttribute("aria-pressed"),
      "true",
    );
    await page.keyboard.press("Escape");
    assert.equal(await terminalText(), before);
    assert.equal(
      await page.evaluate(() => window.portalFixture.terminals.length),
      count,
    );
    const refreshes = await page.evaluate(() =>
      window.portalFixture.terminals
        .filter((record) => !record.disposed)
        .reduce((sum, record) => sum + record.refreshes, 0),
    );
    await page.evaluate(() =>
      document.dispatchEvent(new Event("visibilitychange")),
    );
    await page.waitForFunction(
      (count) =>
        window.portalFixture.terminals
          .filter((record) => !record.disposed)
          .reduce((sum, record) => sum + record.refreshes, 0) > count,
      refreshes,
    );
  });

  it("falls back to a working renderer after WebGL context loss without resizing the page", async (test) => {
    await terminals();
    const supported = await page.evaluate(() => {
      const canvas = document.querySelector(
        "#terminal-panel-terminal-main .xterm-screen canvas:not(.xterm-link-layer)",
      );
      const context = canvas?.getContext("webgl2");
      const extension = context?.getExtension("WEBGL_lose_context");
      if (!extension) return false;
      extension.loseContext();
      return true;
    });
    if (!supported)
      return test.skip("WebGL context loss is unavailable in this browser");
    await page.waitForFunction(() =>
      window.portalFixture.terminals.some(
        (record) => !record.disposed && record.webglDisposed > 0,
      ),
    );
    await page.locator("#terminal-panel-terminal-main .xterm-rows").waitFor();
    assert.match(await terminalText(), /Welcome to main/);
    assert.equal(await page.getByRole("tab").count(), 3);
  });

  it("aborts the log stream when leaving the logs tab", async () => {
    await load("/workspaces/demo/logs");
    await page.waitForFunction(() =>
      window.portalFixture.calls.some((call) => call.method === "ListenLog"),
    );
    const before = await page.evaluate(() => window.portalFixture.logAborts);
    await page.getByRole("link", { name: "Terminals", exact: true }).click();
    await page.waitForFunction(
      (count) => window.portalFixture.logAborts > count,
      before,
    );
    await page.getByRole("tab", { name: "main", exact: true }).waitFor();
  });

  for (const theme of ["light", "dark"]) {
    for (const width of [1440, 390]) {
      it(`fits Services and terminal controls in the ${theme} theme at ${width}px`, async () => {
        await page.setViewportSize({ width, height: 1000 });
        await load("/services", theme);
        await page.getByText("service-00", { exact: true }).waitFor();
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
          true,
        );
        await screenshot(`services-${theme}-${width}`);
        await load("/workspaces/demo/terminals", theme);
        await page.getByRole("tab", { name: "main", exact: true }).waitFor();
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
          true,
        );
        assert.equal(
          await page
            .getByRole("button", { name: "Toggle full screen", exact: true })
            .isVisible(),
          true,
        );
        await screenshot(`terminals-${theme}-${width}`);
      });
    }
  }
});
