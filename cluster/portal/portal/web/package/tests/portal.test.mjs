import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
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
  const checkNotification = async (theme, color) => {
    const notification = page.locator(".mantine-Notification-root");
    await notification.waitFor();
    await notification.hover();
    assert.equal(await notification.count(), 1);
    const box = await notification.boundingBox();
    const viewport = page.viewportSize();
    assert.ok(Math.abs(box.x + box.width / 2 - viewport.width / 2) <= 1);
    assert.ok(box.x >= 0 && box.x + box.width <= viewport.width);
    assert.ok(viewport.height - box.y - box.height >= 0);
    assert.ok(viewport.height - box.y - box.height <= 32);

    const appearance = await notification.evaluate((element) => {
      const canvas = document.createElement("canvas").getContext("2d");
      const rgb = (color) => {
        canvas.fillStyle = color;
        canvas.fillRect(0, 0, 1, 1);
        return [...canvas.getImageData(0, 0, 1, 1).data].slice(0, 3);
      };
      const luminance = (color) => {
        const [r, g, b] = rgb(color).map((channel) => {
          const value = channel / 255;
          return value <= 0.04045
            ? value / 12.92
            : ((value + 0.055) / 1.055) ** 2.4;
        });
        return r * 0.2126 + g * 0.7152 + b * 0.0722;
      };
      const background = luminance(getComputedStyle(element).backgroundColor);
      const foreground = luminance(
        getComputedStyle(
          element.querySelector(".mantine-Notification-description"),
        ).color,
      );
      return {
        background,
        contrast:
          (Math.max(background, foreground) + 0.05) /
          (Math.min(background, foreground) + 0.05),
        accent: rgb(getComputedStyle(element, "::before").backgroundColor),
      };
    });
    assert.ok(appearance.contrast >= 4.5);
    assert.ok(
      theme === "dark"
        ? appearance.background < 0.1
        : appearance.background > 0.9,
    );
    assert.ok(
      color === "green"
        ? appearance.accent[1] > appearance.accent[0]
        : appearance.accent[0] > appearance.accent[1],
    );
    return notification;
  };

  for (const theme of ["light", "dark"]) {
    it(`shows and dismisses bottom-center success notifications on mobile in the ${theme} theme`, async () => {
      await page.setViewportSize({ width: 390, height: 844 });
      await load("/workspaces/demo/snapshots", theme);
      await page
        .getByRole("button", { name: "Take snapshot", exact: true })
        .first()
        .click();
      const dialog = page.getByRole("dialog", {
        name: "Take snapshot",
        exact: true,
      });
      await dialog
        .getByRole("textbox", { name: "Name", exact: true })
        .fill("checkpoint");
      await dialog
        .getByRole("button", { name: "Take snapshot", exact: true })
        .click();
      await dialog.waitFor({ state: "detached" });
      await page.getByText("Snapshot started", { exact: true }).waitFor();
      const notification = await checkNotification(theme, "green");
      await screenshot(`notification-success-${theme}-mobile`);
      await notification
        .getByRole("button", { name: "Dismiss notification", exact: true })
        .click();
      await notification.waitFor({ state: "detached" });
    });

    it(`shows bottom-center error notifications above full-screen terminals in the ${theme} theme`, async () => {
      await load("/workspaces/demo/terminals", theme);
      await page.getByRole("tab", { name: "main", exact: true }).waitFor();
      await page
        .getByRole("button", { name: "Toggle full screen", exact: true })
        .click();
      await page.evaluate(() => {
        window.portalFixture.failRemove = true;
      });
      await page
        .getByRole("button", { name: "Close main", exact: true })
        .click();
      const notification = await checkNotification(theme, "red");
      assert.equal(await notification.textContent(), "Service Unavailable");
      await screenshot(`notification-error-${theme}-desktop`);
      await notification
        .getByRole("button", { name: "Dismiss notification", exact: true })
        .click();
      await notification.waitFor({ state: "detached" });
      assert.equal(
        await page.getByRole("tab", { name: "main", exact: true }).isVisible(),
        true,
      );
    });
  }

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

  it("opens the terminal requested by the terminal query parameter", async () => {
    await load("/workspaces/demo/terminals?terminal=terminal-logs");
    await page
      .getByRole("tab", { name: "logs", exact: true, selected: true })
      .waitFor();
    await page.waitForFunction(() => !location.search.includes("terminal="));
    assert.equal(
      await page
        .getByRole("tab", { name: "main", exact: true })
        .getAttribute("aria-selected"),
      "false",
    );

    await page.getByRole("tab", { name: "worker", exact: true }).click();
    await page
      .getByRole("tab", { name: "worker", exact: true, selected: true })
      .waitFor();

    await load("/workspaces/demo/terminals?terminal=terminal-missing");
    await page.getByRole("tab", { name: "main", exact: true }).waitFor();
    await page.waitForFunction(() => !location.search.includes("terminal="));
    assert.equal(await page.getByRole("tab", { selected: true }).count(), 1);
  });

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

  it("uses Plus Jakarta Sans and JetBrains Mono with no trace of Ubuntu", async () => {
    await terminals();
    const fonts = await page.evaluate(async () => {
      await document.fonts.load('700 1em "Plus Jakarta Sans"');
      const probe = document.createElement("span");
      probe.className = "font-mono font-bold";
      document.body.append(probe);
      const body = getComputedStyle(document.body);
      const mono = getComputedStyle(probe);
      const result = {
        body: body.fontFamily,
        bodyWeight: body.fontWeight,
        bodySpacing: parseFloat(body.wordSpacing),
        faces: [...document.fonts].map((face) => face.family),
        mono: mono.fontFamily,
        monoWeight: mono.fontWeight,
        monoLigatures: mono.fontVariantLigatures,
        monoSpacing: mono.wordSpacing,
        terminal: window.portalFixture.terminals[0].terminal.options.fontFamily,
      };
      probe.remove();
      return result;
    });
    assert.match(fonts.body, /^"?Plus Jakarta Sans"?,/);
    assert.equal(fonts.bodyWeight, "500");
    assert.ok(fonts.bodySpacing > 0);
    assert.ok(fonts.faces.length > 0);
    assert.ok(fonts.faces.every((family) => !/ubuntu/i.test(family)));
    assert.match(fonts.mono, /^"?JetBrains Mono"?,/);
    assert.equal(fonts.monoWeight, "400");
    assert.equal(fonts.monoLigatures, "none");
    assert.equal(fonts.monoSpacing, "0px");
    assert.match(fonts.terminal, /^"JetBrains Mono",/);
  });

  it("uses the neutral dark palette of the Octelium portal", async () => {
    await load("/services", "dark");
    const token = (name) =>
      page.evaluate(
        (name) =>
          getComputedStyle(document.documentElement).getPropertyValue(name),
        name,
      );
    assert.match(await token("--app-canvas"), /^oklch\(13\.5% 0?\.002 270\)$/);
    assert.match(await token("--app-surface"), /^oklch\(17% 0?\.002 270\)$/);
    assert.match(await token("--app-line"), /^oklch\(26% 0?\.002 270\)$/);
    assert.match(await token("--app-ink"), /^oklch\(96% 0?\.002 270\)$/);
  });

  it("measures terminals again when JetBrains Mono loads late", async () => {
    const height = () =>
      window.portalFixture.terminals.find((record) => !record.disposed).terminal
        ._core._renderService.dimensions.css.cell.height;
    const loaded = () =>
      page.waitForFunction(
        () =>
          document.fonts.check('400 1em "JetBrains Mono"') &&
          document.fonts.check('700 1em "JetBrains Mono"'),
      );
    await terminals();
    await loaded();
    await page.waitForFunction(() => document.fonts.status === "loaded");
    await page.waitForTimeout(200);
    const expected = await page.evaluate(height);
    await page.route(/jetbrains-mono-.*\.woff2/, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 600));
      await route.continue();
    });
    await terminals();
    await loaded();
    await page.waitForFunction(
      ({ height, expected }) => (0, eval)(height)() === expected,
      { height: height.toString(), expected },
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

  it("disposes terminal renderers and subscriptions when navigating away and recreates them on return", async () => {
    await terminals();
    await page.waitForFunction(
      () => window.portalFixture.sockets.at(-1)?.subscriptions.size === 3,
    );
    await page.getByRole("link", { name: "Services", exact: true }).click();
    await page.getByRole("heading", { name: "Services", exact: true }).waitFor();
    await page.waitForFunction(
      () =>
        window.portalFixture.terminals.every((record) => record.disposed) &&
        window.portalFixture.sockets.every(
          (socket) => socket.subscriptions.size === 0,
        ),
    );
    await page.goBack();
    await page.getByRole("tab", { name: "main", exact: true }).waitFor();
    await page.waitForFunction(
      () => window.portalFixture.sockets.at(-1)?.subscriptions.size === 3,
    );
    assert.equal(
      await page.evaluate(
        () =>
          window.portalFixture.terminals.filter((record) => !record.disposed)
            .length,
      ),
      3,
    );
    assert.equal(await page.getByRole("tab", { selected: true }).count(), 1);
  });

  it("closes every terminal and creates a fresh session from the empty state", async () => {
    await terminals();
    for (const name of ["main", "worker", "logs"]) {
      await page
        .getByRole("button", { name: `Close ${name}`, exact: true })
        .click();
      await page
        .getByRole("tab", { name, exact: true })
        .waitFor({ state: "detached" });
    }
    await page.getByText("No terminal sessions", { exact: true }).waitFor();
    await page.waitForFunction(() =>
      window.portalFixture.terminals.every((record) => record.disposed),
    );
    await page.getByRole("button", { name: "New terminal", exact: true }).click();
    await page
      .getByRole("tab", { name: "shell-1", exact: true, selected: true })
      .waitFor();
    assert.deepEqual(await order(), ["terminal-shell-1"]);
    assert.equal(
      await page.evaluate(
        () =>
          window.portalFixture.calls.filter(
            (call) => call.method === "CreateTerminal",
          ).length,
      ),
      1,
    );
  });

  it("ignores duplicate, missing and malformed saved terminal entries", async () => {
    await context.addInitScript(() =>
      localStorage.setItem(
        "cordium-terminal-tabs:workspace-demo",
        JSON.stringify([
          null,
          { id: "terminal-missing", pinned: true },
          { id: "terminal-worker", pinned: true },
          { id: "terminal-worker", pinned: false },
          { id: "terminal-main", pinned: "true" },
        ]),
      ),
    );
    await load("/workspaces/demo/terminals");
    await page
      .getByRole("tab", { name: "worker (pinned)", exact: true })
      .waitFor();
    assert.deepEqual(await order(), [
      "terminal-worker",
      "terminal-main",
      "terminal-logs",
    ]);
    assert.equal(await page.getByRole("tab", { selected: true }).count(), 1);
    assert.equal(await page.getByRole("tab").count(), 3);
  });

  it("keeps terminal actions usable when saving preferences exceeds the storage quota", async () => {
    await context.addInitScript(() => {
      Storage.prototype.setItem = () => {
        throw new DOMException("Storage full", "QuotaExceededError");
      };
    });
    await terminals();
    await page
      .getByRole("button", { name: "Actions for worker", exact: true })
      .click();
    await page.getByRole("menuitem", { name: "Pin tab", exact: true }).click();
    await page
      .getByRole("tab", { name: "worker (pinned)", exact: true })
      .waitFor();
    await page.getByRole("tab", { name: "main", exact: true }).click();
    await page
      .getByRole("tab", { name: "main", exact: true, selected: true })
      .waitFor();
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

  const connectDrawer = (name) =>
    page.getByRole("dialog", { name: `Connect to ${name}`, exact: true });
  const connectStatus = (drawer, label) =>
    drawer.getByRole("status").filter({ hasText: label }).waitFor();
  const connectText = () =>
    page.evaluate(() => {
      const record = window.portalFixture.terminals.find(
        (record) =>
          !record.disposed &&
          record.terminal.element?.closest('[data-testid="connect-terminal"]'),
      );
      if (!record) return "";
      const buffer = record.terminal.buffer.active;
      return Array.from({ length: buffer.length }, (_, index) =>
        buffer.getLine(index).translateToString(true),
      ).join("\n");
    });
  const rpcCalls = (method) =>
    page.evaluate(
      (method) =>
        window.portalFixture.calls.filter((call) => call.method === method)
          .length,
      method,
    );
  const connectInputs = () =>
    page.evaluate(() => window.portalFixture.connect.inputs);
  const settle = (ms = 200) =>
    page.evaluate(
      (ms) => new Promise((resolve) => window.setTimeout(resolve, ms)),
      ms,
    );
  const connectTo = (name) =>
    page.getByRole("link", { name: `Connect to ${name}`, exact: true }).click();
  const connected = async (route = "/services/postgres.data/connect") => {
    await load(route);
    const drawer = connectDrawer("postgres.data");
    await connectStatus(drawer, "Connected");
    return drawer;
  };
  const closeDrawer = async (drawer) => {
    await drawer.getByRole("button", { name: "Close", exact: true }).click();
    await drawer.waitFor({ state: "detached" });
  };
  const textContrast = (locator) =>
    locator.evaluate((element) => {
      const canvas = document.createElement("canvas").getContext("2d", {
        willReadFrequently: true,
      });
      const parse = (color) => {
        canvas.clearRect(0, 0, 1, 1);
        canvas.fillStyle = color;
        canvas.fillRect(0, 0, 1, 1);
        const [r, g, b, a] = canvas.getImageData(0, 0, 1, 1).data;
        return { rgb: [r, g, b], alpha: a / 255 };
      };
      const layers = [];
      for (let node = element; node; node = node.parentElement) {
        const layer = parse(getComputedStyle(node).backgroundColor);
        if (layer.alpha > 0) layers.push(layer);
        if (layer.alpha === 1) break;
      }
      const background = layers.reduceRight(
        (below, layer) =>
          below.map(
            (channel, index) =>
              layer.rgb[index] * layer.alpha + channel * (1 - layer.alpha),
          ),
        [255, 255, 255],
      );
      const foreground = parse(getComputedStyle(element).color);
      const blended = foreground.rgb.map(
        (channel, index) =>
          channel * foreground.alpha +
          background[index] * (1 - foreground.alpha),
      );
      const luminance = (channels) => {
        const [r, g, b] = channels.map((channel) => {
          const value = channel / 255;
          return value <= 0.04045
            ? value / 12.92
            : ((value + 0.055) / 1.055) ** 2.4;
        });
        return r * 0.2126 + g * 0.7152 + b * 0.0722;
      };
      const light = Math.max(luminance(background), luminance(blended));
      const dark = Math.min(luminance(background), luminance(blended));
      return (light + 0.05) / (dark + 0.05);
    });
  const assertReadable = async (locator, label) => {
    const contrast = await textContrast(locator);
    assert.ok(contrast >= 4.5, `${label} has a contrast of ${contrast}`);
  };

  it("connects from the Services list and removes the terminals on close", async () => {
    await load("/services?q=postgres");
    await connectTo("postgres.data");
    await page.waitForURL(
      (url) =>
        url.pathname === "/services/postgres.data/connect" &&
        url.search === "?q=postgres",
    );
    const drawer = connectDrawer("postgres.data");
    await connectStatus(drawer, "Connected");
    assert.equal(await page.title(), "Connect to postgres.data · Cordium");
    assert.deepEqual(await connectInputs(), ["psql -h postgres.data\r"]);
    assert.ok((await connectText()).includes("psql -h postgres.data"));
    assert.equal(
      await drawer.getByTestId("connect-command").textContent(),
      "psql -h postgres.data",
    );
    assert.equal(await rpcCalls("CreateTerminal"), 2);
    assert.equal(await rpcCalls("RemoveTerminal"), 1);

    await closeDrawer(drawer);
    await page.waitForURL(
      (url) => url.pathname === "/services" && url.search === "?q=postgres",
    );
    await page.waitForFunction(
      () =>
        window.portalFixture.calls.filter(
          (call) => call.method === "RemoveTerminal",
        ).length === 2,
    );
    assert.equal(await page.title(), "Services · Cordium");
    assert.equal(
      await page.evaluate(
        () =>
          window.portalFixture.terminals.filter((record) => !record.disposed)
            .length,
      ),
      0,
    );
  });

  it("opens a deep link above the Services list and accepts short names", async () => {
    await load("/services/web/connect");
    const drawer = connectDrawer("web");
    await connectStatus(drawer, "Connected");
    await page.getByText("service-00", { exact: true }).waitFor();
    assert.deepEqual(await connectInputs(), [
      "curl -sI https://web.local.example.test/",
    ]);
  });

  it("closes the drawer without an animation when motion is reduced", async () => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    const drawer = await connected();
    await closeDrawer(drawer);
    await page.waitForURL((url) => url.pathname === "/services");
  });

  it("explains the Services that cannot be connected to", async () => {
    await load("/services?q=windows");
    await page.getByText("windows", { exact: true }).last().waitFor();
    assert.equal(
      await page.getByRole("link", { name: /^Connect to / }).count(),
      0,
    );

    await load("/services/windows/connect");
    let drawer = connectDrawer("windows");
    await drawer.getByText("This Service has no terminal client").waitFor();
    await drawer.getByText("RDP Services cannot be reached").waitFor();
    assert.equal(await rpcCalls("GetUserConfig"), 0);

    await load("/services/missing.default/connect");
    drawer = connectDrawer("missing.default");
    await drawer.getByText("This Service is not available").waitFor();
    await drawer
      .getByRole("button", { name: "Back to Services", exact: true })
      .click();
    await page.waitForURL((url) => url.pathname === "/services");
  });

  it("keeps the terminal output when the drawer is resized", async () => {
    await connected();
    await page.evaluate(() =>
      window.portalFixture.connect.output("keep this line\r\n"),
    );
    const before = await page.evaluate(
      () => window.portalFixture.connect.sizes.length,
    );
    await page.setViewportSize({ width: 700, height: 600 });
    await page.waitForFunction(
      (before) => window.portalFixture.connect.sizes.length > before,
      before,
    );
    await settle();
    const text = await connectText();
    assert.ok(text.includes("keep this line"));
    assert.ok(text.includes("psql -h postgres.data"));
  });

  it("sends Escape and Tab to the terminal instead of the drawer", async () => {
    const drawer = await connected();
    await page.keyboard.press("Escape");
    await page.keyboard.press("Tab");
    await page.waitForFunction(() =>
      window.portalFixture.connect.inputs.includes("\t"),
    );
    assert.ok((await connectInputs()).includes("\x1b"));
    assert.ok(await drawer.isVisible());

    await drawer
      .getByRole("button", { name: "Start a new session", exact: true })
      .focus();
    await page.keyboard.press("Escape");
    await drawer.waitFor({ state: "detached" });
  });

  it("types the quick commands without running them", async () => {
    const drawer = await connected();
    await drawer
      .getByRole("button", { name: "\\conninfo", exact: true })
      .click();
    await page.waitForFunction(() =>
      window.portalFixture.connect.inputs.includes("\\conninfo"),
    );
  });

  it("continues a kept terminal in the Workspace", async () => {
    const drawer = await connected();
    const id = await page.evaluate(() => window.portalFixture.connect.main());
    await drawer
      .getByRole("button", { name: "Open in Workspace", exact: true })
      .click();
    await page.waitForURL(
      (url) => url.pathname === "/workspaces/agent/terminals",
    );
    await page.locator(`[data-terminal-tab="${id}"]`).waitFor();
    await page
      .locator(`[data-terminal-tab="${id}"] [aria-selected="true"]`)
      .waitFor();
    assert.equal(await rpcCalls("RemoveTerminal"), 1);
    assert.equal(new URL(page.url()).searchParams.get("terminal"), null);
  });

  it("restarts the session after the shell exits", async () => {
    const drawer = await connected();
    await page.evaluate(() => window.portalFixture.connect.closeMain());
    await connectStatus(drawer, "The session has ended");
    await drawer
      .getByRole("button", { name: "New session", exact: true })
      .click();
    await page.waitForFunction(
      () => window.portalFixture.connect.inputs.length === 2,
    );
    await connectStatus(drawer, "Connected");
    assert.deepEqual(await connectInputs(), [
      "psql -h postgres.data\r",
      "psql -h postgres.data\r",
    ]);
  });

  it("reports connection failures and missing clients", async () => {
    const drawer = await connected();
    await page.evaluate(() =>
      window.portalFixture.connect.output(
        "psql: error: connection refused\r\n",
      ),
    );
    const alert = drawer.getByRole("alert");
    await alert.getByText("psql: error: connection refused").waitFor();
    await connectStatus(drawer, "Needs attention");
    await alert.getByRole("button", { name: "Dismiss", exact: true }).click();
    await connectStatus(drawer, "Connected");

    await load("/services");
    await page.evaluate(() => {
      window.portalFixture.connect.failTool = true;
    });
    await connectTo("llm");
    const llm = connectDrawer("llm");
    await llm.getByText("curl could not be installed").first().waitFor();
    await connectStatus(llm, "Needs attention");
    assert.deepEqual(await connectInputs(), [
      "curl -s http://llm:8080/v1/models",
    ]);
  });

  it("skips the setup and opens the shell", async () => {
    await load("/services");
    await page.evaluate(() => {
      window.portalFixture.connect.holdSetup = true;
    });
    await connectTo("postgres.data");
    const drawer = connectDrawer("postgres.data");
    await drawer.getByText("Connecting to postgres.data").waitFor();
    await drawer.locator('[data-step="dns"][data-status="running"]').waitFor();
    await drawer
      .getByRole("button", { name: "Skip and open the shell", exact: true })
      .click();
    await connectStatus(drawer, "Connected");
    assert.deepEqual(await connectInputs(), []);
  });

  it("recovers when the terminals cannot be created", async () => {
    await load("/services");
    await page.evaluate(() => {
      window.portalFixture.connect.failCreate = true;
    });
    await connectTo("postgres.data");
    const drawer = connectDrawer("postgres.data");
    await drawer.getByText("The terminal is not available").waitFor();
    await drawer.getByText("The Workspace is not reachable").waitFor();
    await page.evaluate(() => {
      window.portalFixture.connect.failCreate = false;
    });
    await drawer
      .getByRole("button", { name: "Try again", exact: true })
      .click();
    await connectStatus(drawer, "Connected");
  });

  it("sets up the agent Workspace on request", async () => {
    await load("/services");
    await page.evaluate(() => {
      window.portalFixture.agent.configured = false;
    });
    await connectTo("postgres.data");
    const drawer = connectDrawer("postgres.data");
    await drawer.getByText("Set up your agent").waitFor();
    assert.equal(await rpcCalls("InitializeAgent"), 0);
    await drawer.getByRole("button", { name: "Set up", exact: true }).click();
    await connectStatus(drawer, "Connected");
    assert.equal(await rpcCalls("InitializeAgent"), 1);
  });

  it("starts a stopped agent Workspace automatically", async () => {
    await load("/services");
    await page.evaluate(() => {
      const fixture = window.portalFixture;
      fixture.agent.state = fixture.states.STOPPED;
    });
    await connectTo("postgres.data");
    await connectStatus(connectDrawer("postgres.data"), "Connected");
    assert.equal(await rpcCalls("StartWorkspace"), 1);
  });

  it("does not restart a failed agent Workspace", async () => {
    await load("/services");
    await page.evaluate(() => {
      const fixture = window.portalFixture;
      fixture.agent.state = fixture.states.STOPPED;
      fixture.agent.failure = "The image could not be pulled";
    });
    await connectTo("postgres.data");
    const drawer = connectDrawer("postgres.data");
    await drawer.getByText("Your agent Workspace failed").waitFor();
    await drawer.getByText("The image could not be pulled").waitFor();
    assert.equal(await rpcCalls("StartWorkspace"), 0);
    await drawer.getByRole("link", { name: "View logs", exact: true }).click();
    await page.waitForURL((url) => url.pathname === "/workspaces/agent/logs");
  });

  it("reports an agent Workspace that cannot be started", async () => {
    await load("/services");
    await page.evaluate(() => {
      const fixture = window.portalFixture;
      fixture.agent.state = fixture.states.STOPPED;
      fixture.agent.startError = "The quota is exceeded";
    });
    await connectTo("postgres.data");
    const drawer = connectDrawer("postgres.data");
    await drawer.getByText("could not be started").waitFor();
    await drawer.getByText("The quota is exceeded").waitFor();
    await page.evaluate(() => {
      window.portalFixture.agent.startError = "";
    });
    await drawer.getByRole("button", { name: "Start", exact: true }).click();
    await connectStatus(drawer, "Connected");
  });

  it("replaces a deleted agent Workspace", async () => {
    await load("/services");
    await page.evaluate(() => {
      window.portalFixture.agent.missing = true;
    });
    await connectTo("postgres.data");
    const drawer = connectDrawer("postgres.data");
    await drawer.getByText("does not exist anymore").waitFor();
    await drawer
      .getByRole("button", { name: "Set up again", exact: true })
      .click();
    await connectStatus(drawer, "Connected");
  });

  it("builds a client command for every Service type", async () => {
    await load();
    const plans = await page.evaluate(() => {
      const { plan } = window.portalFixture;
      const types = {
        TCP: 1,
        UDP: 2,
        HTTP: 3,
        SSH: 4,
        KUBERNETES: 5,
        POSTGRES: 6,
        MYSQL: 7,
        GRPC: 8,
        WEB: 9,
        DNS: 10,
        SOCKS5: 11,
        MCP: 13,
        LLM: 14,
      };
      return {
        http: plan({ type: types.HTTP, port: 8080 }, "api.default"),
        https: plan(
          { type: types.HTTP, port: 443, isTLS: true },
          "api.default",
        ),
        web: plan({ type: types.WEB, port: 8080 }, "app.default"),
        mcp: plan({ type: types.MCP, port: 8080 }, "mcp.tools"),
        llm: plan({ type: types.LLM, port: 8080 }, "llm.default"),
        ssh: plan({ type: types.SSH, port: 22 }, "box.default"),
        sshPort: plan({ type: types.SSH, port: 2222 }, "box.ops"),
        postgres: plan({ type: types.POSTGRES, port: 5432 }, "db.default"),
        postgresPort: plan({ type: types.POSTGRES, port: 6000 }, "db.default"),
        mysql: plan({ type: types.MYSQL, port: 3307 }, "db.default"),
        kubernetes: plan({ type: types.KUBERNETES, port: 443 }, "k8s.default"),
        grpc: plan({ type: types.GRPC, port: 8080 }, "rpc.default"),
        grpcTLS: plan(
          { type: types.GRPC, port: 443, isTLS: true },
          "rpc.default",
        ),
        dns: plan({ type: types.DNS, port: 53 }, "dns.default"),
        socks: plan({ type: types.SOCKS5, port: 1080 }, "egress.default"),
        udpDNS: plan({ type: types.UDP, port: 53 }, "resolver.default"),
        udp: plan({ type: types.UDP, port: 9000 }, "stats.default"),
        tcpName: plan({ type: types.TCP, port: 7000 }, "redis-1.default"),
        tcpDisplay: plan(
          { type: types.TCP, port: 7000 },
          "cache.default",
          "MariaDB primary",
        ),
        tcpPort: plan({ type: types.TCP, port: 5432 }, "store.default"),
        tcpHTTP: plan({ type: types.TCP, port: 8080 }, "svc.default"),
        tcp: plan({ type: types.TCP, port: 7000 }, "thing.default"),
      };
    });

    const expected = {
      http: ["curl", "curl -i http://api:8080/", false],
      https: ["curl", "curl -i https://api.local.example.test/", false],
      web: ["curl", "curl -sI http://app:8080/", false],
      llm: ["curl", "curl -s http://llm:8080/v1/models", false],
      ssh: ["ssh", "ssh box", true],
      sshPort: ["ssh", "ssh box.ops -p 2222", true],
      postgres: ["psql", "psql -h db", true],
      postgresPort: ["psql", "psql -h db -p 6000", true],
      mysql: ["mysql", "mysql -h db -P 3307", true],
      kubernetes: ["kubectl", "kubectl get pods -A", false],
      grpc: ["grpcurl", "grpcurl -plaintext rpc:8080 list", false],
      grpcTLS: ["grpcurl", "grpcurl rpc.local.example.test:443 list", false],
      dns: ["dig", "dig @dns example.com", false],
      socks: [
        "curl",
        "curl --socks5-hostname egress:1080 https://example.com -I",
        false,
      ],
      udpDNS: ["dig", "dig @resolver example.com", false],
      tcpName: ["redis-cli", "redis-cli -h redis-1 -p 7000", true],
      tcpDisplay: ["mysql", "mysql -h cache -P 7000 -p -u ", false],
      tcpPort: ["psql", "psql -h store -U ", false],
      tcpHTTP: ["curl", "curl -i http://svc:8080/", false],
    };

    for (const [name, [tool, command, run]] of Object.entries(expected)) {
      const plan = plans[name];
      assert.equal(plan.tool, tool, name);
      assert.equal(plan.inputs.at(-1).text, command, name);
      assert.equal(!!plan.inputs.at(-1).run, run, name);
    }

    assert.ok(
      plans.mcp.inputs[0].text.startsWith("curl -s http://mcp.tools:8080/ "),
    );
    assert.ok(plans.mcp.inputs[0].text.includes('"method":"initialize"'));
    assert.equal(plans.kubernetes.kubeconfig, "k8s");
    assert.equal(
      plans.kubernetes.inputs[0].text,
      "export KUBECONFIG=/home/alex/.kube/config",
    );
    assert.equal(plans.udp.tool, undefined);
    assert.ok(plans.udp.hint.includes("stats:9000"));
    assert.equal(plans.tcp.tool, undefined);
    assert.ok(plans.tcp.hint.includes("thing:7000"));
    assert.equal(
      plans.tcpName.inferred,
      "redis-cli is inferred from the Service name.",
    );
    assert.equal(
      plans.tcpDisplay.inferred,
      "mysql is inferred from the Service display name.",
    );
    assert.equal(
      plans.tcpPort.inferred,
      "psql is inferred from the Service port.",
    );

    for (const [name, plan] of Object.entries(plans)) {
      assert.doesNotThrow(
        () => execFileSync("sh", ["-n", "-c", plan.script]),
        `${name} has an invalid setup script`,
      );
    }
  });

  for (const theme of ["light", "dark"]) {
    it(`fits the drawer on every device width in the ${theme} theme`, async () => {
      const sizes = [
        { name: "phone-small", width: 320, height: 568, drawer: 320 },
        { name: "phone", width: 390, height: 844, drawer: 390 },
        { name: "phone-landscape", width: 844, height: 390, drawer: 844 },
        { name: "phablet", width: 640, height: 900, drawer: 640 },
        { name: "tablet", width: 768, height: 1024, drawer: 768 },
        { name: "tablet-landscape", width: 1024, height: 768, drawer: 960 },
        { name: "laptop", width: 1440, height: 900, drawer: 960 },
        { name: "desktop", width: 1920, height: 1080, drawer: 1152 },
        { name: "wide", width: 2560, height: 1440, drawer: 1536 },
      ];
      await load("/services/postgres.data/connect", theme);
      const drawer = connectDrawer("postgres.data");
      await connectStatus(drawer, "Connected");

      for (const size of sizes) {
        await page.setViewportSize({ width: size.width, height: size.height });
        await settle();
        const box = await drawer.boundingBox();
        assert.equal(box.width, size.drawer, `${size.name} width`);
        assert.equal(box.height, size.height, `${size.name} height`);
        assert.equal(box.x + box.width, size.width, `${size.name} edge`);
        assert.equal(box.y, 0, `${size.name} top`);

        const layout = await drawer.evaluate((element) => {
          const content = element.getBoundingClientRect();
          const rect = (selector) =>
            element.querySelector(selector).getBoundingClientRect();
          const controls = [
            ...element.querySelectorAll("header button, header h2"),
          ].filter((node) => node.getClientRects().length > 0);
          const toolbar = element.querySelector('[role="toolbar"]');
          return {
            overflow: controls.some((node) => {
              const box = node.getBoundingClientRect();
              return box.left < content.left || box.right > content.right;
            }),
            clipped: element.scrollWidth > element.clientWidth,
            page: document.documentElement.scrollWidth,
            terminal: rect('[data-testid="connect-terminal"]'),
            xterm: rect(".xterm-screen"),
            toolbarInside: toolbar.clientWidth <= content.width,
            bottom: rect('[data-testid="connect-size"]').bottom,
            contentBottom: content.bottom,
          };
        });
        assert.equal(layout.overflow, false, `${size.name} header controls`);
        assert.equal(layout.clipped, false, `${size.name} clipping`);
        assert.ok(layout.page <= size.width, `${size.name} page scroll`);
        assert.ok(layout.toolbarInside, `${size.name} toolbar`);
        assert.ok(layout.terminal.height >= 150, `${size.name} terminal`);
        assert.ok(
          layout.terminal.width >= size.drawer - 2,
          `${size.name} terminal width`,
        );
        assert.ok(
          layout.xterm.right <= layout.terminal.right,
          `${size.name} xterm width`,
        );
        assert.ok(layout.bottom <= layout.contentBottom, `${size.name} footer`);
        await screenshot(`connect-${theme}-${size.name}`);
      }
    });

    it(`keeps every connect state readable in the ${theme} theme`, async () => {
      await page.setViewportSize({ width: 1280, height: 800 });
      await load("/services", theme);
      await page.evaluate(() => {
        window.portalFixture.connect.holdSetup = true;
      });
      await connectTo("service-00");
      const drawer = connectDrawer("service-00");
      const card = drawer.locator('.console-surface [role="status"]');
      await drawer.getByText("Connecting to service-00").waitFor();
      await assertReadable(drawer.getByRole("heading"), "title");
      await assertReadable(drawer.getByTestId("connect-command"), "command");
      await assertReadable(drawer.getByRole("note"), "info strip");
      await assertReadable(card.getByText("Connecting to service-00"), "card");
      await assertReadable(
        card.getByText("in your agent Workspace"),
        "subtitle",
      );
      await assertReadable(
        drawer.locator('[data-step="dns"] span').last(),
        "step",
      );
      await settle(400);
      await screenshot(`connect-${theme}-progress`);
      await drawer
        .getByRole("button", { name: "Skip and open the shell", exact: true })
        .click();
      await connectStatus(drawer, "Connected");
      await assertReadable(
        drawer.getByRole("status").filter({ hasText: "Connected" }),
        "status",
      );
      await assertReadable(
        drawer.getByRole("button", { name: "raw TCP", exact: true }),
        "quick command",
      );
      await assertReadable(drawer.getByTitle("agent"), "footer");
      await closeDrawer(drawer);

      await page.evaluate(() => {
        const { connect } = window.portalFixture;
        connect.holdSetup = false;
        connect.failTool = true;
      });
      await connectTo("postgres.data");
      const postgres = connectDrawer("postgres.data");
      const warning = postgres.getByRole("alert");
      await warning.getByText("psql could not be installed").first().waitFor();
      await assertReadable(warning.locator("div").first(), "warning strip");
      await connectStatus(postgres, "Needs attention");
      await page.evaluate(() =>
        window.portalFixture.connect.output(
          "psql: error: connection refused\r\n",
        ),
      );
      await settle(400);
      await screenshot(`connect-${theme}-warning`);
      await closeDrawer(postgres);

      await page.evaluate(() => {
        const fixture = window.portalFixture;
        fixture.connect.failTool = false;
        fixture.agent.configured = false;
        return fixture.refreshAgent();
      });
      await connectTo("llm");
      const llm = connectDrawer("llm");
      await llm.getByText("Set up your agent").waitFor();
      const setup = llm.locator('.console-surface [role="status"]');
      await assertReadable(setup.getByText("Set up your agent"), "setup title");
      await assertReadable(
        setup.getByText("Terminals run in the Workspace"),
        "setup text",
      );
      await assertReadable(
        setup.getByRole("button", { name: "Set up", exact: true }),
        "setup button",
      );
      await settle(400);
      await screenshot(`connect-${theme}-setup`);
    });

    it(`shows the Connect action on every width in the ${theme} theme`, async () => {
      await load("/services?q=web", theme);
      for (const width of [320, 360, 390, 768, 1440]) {
        await page.setViewportSize({ width, height: 800 });
        const link = page.getByRole("link", {
          name: "Connect to web",
          exact: true,
        });
        await link.waitFor();
        const box = await link.boundingBox();
        assert.ok(box.x >= 0 && box.x + box.width <= width);
        assert.ok(
          (await page.evaluate(() => document.documentElement.scrollWidth)) <=
            width,
        );
        assert.equal(
          await page.evaluate(
            () =>
              [...document.querySelectorAll(".mantine-Button-label")].filter(
                (label) => label.scrollWidth > label.clientWidth,
              ).length,
          ),
          0,
          `clipped button labels at ${width}px`,
        );
        await screenshot(`services-connect-${theme}-${width}`);
      }
    });
  }
});
