import assert from "node:assert/strict";
import { after, afterEach, before, beforeEach, describe, it } from "node:test";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { createServer } from "vite";

const root = fileURLToPath(new URL("../", import.meta.url));

describe("Agent chat", () => {
  let server;
  let browser;
  let context;
  let page;
  let origin;

  const composer = () => page.getByRole("textbox", { name: "Message" });

  const nav = () => page.getByRole("navigation", { name: "Conversations" });

  const calls = () => page.evaluate(() => window.agentCalls);

  const load = async () => {
    await page.goto(`${origin}tests/fixtures/agentChat.html`);
    await page
      .getByText("Test two branches in parallel", { exact: true })
      .waitFor();
  };

  const open = async (viewport = { width: 1440, height: 900 }) => {
    context = await browser.newContext({ timezoneId: "UTC", viewport });
    page = await context.newPage();
    await load();
  };

  before(async () => {
    server = await createServer({
      root,
      configFile: `${root}vite.config.ts`,
      logLevel: "error",
      server: { host: "127.0.0.1", port: 0, watch: null },
    });
    await server.listen();
    origin = server.resolvedUrls.local[0];
    browser = await chromium.launch();
  });

  after(async () => {
    await browser?.close();
    await server?.close();
  });

  beforeEach(async () => {
    await open();
  });

  afterEach(async () => {
    await context?.close();
  });

  it("shows the empty state and groups the conversations by date", async () => {
    await page.getByRole("heading", { name: /, George$/ }).waitFor();
    for (const label of ["Today", "Yesterday"]) {
      assert.equal(await nav().getByText(label, { exact: true }).count(), 1);
    }
    assert.deepEqual(
      await nav()
        .locator("li button[title]")
        .evaluateAll((items) =>
          items.map((item) => item.getAttribute("title")),
        ),
      [
        "Audit my running Workspaces",
        "Run the unit tests",
        "Copy the build artifacts",
        "Clean up idle Workspaces",
      ],
    );

    await page.getByText("Review my Workspaces", { exact: true }).click();
    assert.equal(
      await composer().inputValue(),
      "Which of my Workspaces are running, what are they for and which ones look idle?",
    );
    await assert.doesNotReject(
      page.waitForFunction(
        () => document.activeElement?.getAttribute("aria-label") === "Message",
      ),
    );
  });

  it("auto-grows the composer without a resize handle", async () => {
    assert.equal(
      await composer().evaluate((el) => getComputedStyle(el).resize),
      "none",
    );
    const initial = (await composer().boundingBox()).height;
    await composer().fill("one\ntwo\nthree\nfour");
    const grown = (await composer().boundingBox()).height;
    assert.ok(grown > initial);
    await composer().fill(Array.from({ length: 60 }, (_, i) => i).join("\n"));
    const capped = (await composer().boundingBox()).height;
    assert.ok(capped <= 320);
    await composer().fill("");
    assert.equal((await composer().boundingBox()).height, initial);
    assert.equal(
      await page.getByRole("button", { name: "Send" }).isDisabled(),
      true,
    );
  });

  it("searches the conversation titles", async () => {
    await page.keyboard.press("Control+k");
    const search = page.getByRole("searchbox", {
      name: "Search conversations",
    });
    await assert.doesNotReject(
      page.waitForFunction(
        () => document.activeElement?.getAttribute("type") === "search",
      ),
    );
    await search.fill("unit tests");
    await nav().getByText("Titles", { exact: true }).waitFor();
    assert.equal(await nav().locator("li button[title]").count(), 1);
    assert.equal(await nav().locator("mark").count(), 2);

    await search.press("Enter");
    await page.getByText("All the unit tests passed.").waitFor();
    assert.equal(new URL(page.url()).searchParams.get("c"), "c4");

    await search.fill("nothing matches this");
    await nav().getByText("No results", { exact: true }).waitFor();
  });

  it("sends a message and streams the response", async () => {
    await composer().fill("List my Workspaces");
    await page.keyboard.press("Enter");
    await page.getByRole("button", { name: "Stop" }).waitFor();
    assert.equal(await composer().inputValue(), "");
    await page.getByText("You have 2 Workspaces.").waitFor();
    await page.getByRole("button", { name: "Send" }).waitFor();
    assert.equal(
      await page
        .locator("[data-message-id]")
        .getByText("List my Workspaces", { exact: true })
        .count(),
      1,
    );
    assert.equal(
      await nav().getByText("List my Workspaces", { exact: true }).count(),
      1,
    );

    const created = (await calls()).find(
      (c) => c.method === "POST" && c.path === "/conversations",
    );
    assert.deepEqual(created.body, { input: { text: "List my Workspaces" } });
    assert.ok(new URL(page.url()).searchParams.get("c")?.startsWith("c-"));
  });

  it("asks for an approval and sends the decision", async () => {
    await composer().fill("Delete the unit-tests Workspace");
    await page.keyboard.press("Enter");
    await page.getByText("Approval required", { exact: true }).waitFor();
    assert.ok((await page.getByText("Destructive").count()) > 0);

    await page.getByRole("button", { name: "Approve" }).click();
    await page.getByText("Approved", { exact: true }).waitFor();
    await page.getByText("You have 2 Workspaces.").waitFor();

    const decision = (await calls()).find((c) =>
      c.path.endsWith("/approvals/ap-1"),
    );
    assert.equal(decision.method, "POST");
    assert.deepEqual(decision.body, { decision: "approve" });
  });

  it("rejects an approval with a reason", async () => {
    await composer().fill("Delete the unit-tests Workspace");
    await page.keyboard.press("Enter");
    await page.getByText("Approval required", { exact: true }).waitFor();

    await page.getByRole("button", { name: "Reject" }).click();
    await page
      .getByRole("textbox", { name: "Reason" })
      .fill("Keep it for the release");
    await page.getByRole("button", { name: "Reject" }).click();
    await page.getByText("Reason: Keep it for the release").waitFor();

    const decision = (await calls()).find((c) =>
      c.path.endsWith("/approvals/ap-1"),
    );
    assert.deepEqual(decision.body, {
      decision: "reject",
      reason: "Keep it for the release",
    });
  });

  it("shows the Workspace cards and the command output of the tools", async () => {
    await nav().getByText("Run the unit tests").click();
    await page.getByText("All the unit tests passed.").waitFor();

    const card = page.getByRole("link", { name: "unit-tests", exact: true });
    assert.equal(await card.getAttribute("href"), "/workspaces/unit-tests");
    assert.equal(await page.getByText("go test ./...").count(), 0);

    await page.getByRole("button", { name: /Run go test in unit-tests/ }).click();
    await page.getByText("go test ./...").waitFor();
    await page.getByText("exit 0", { exact: true }).waitFor();
  });

  it("renames and deletes a conversation", async () => {
    await nav().getByText("Copy the build artifacts").click();
    await page.getByText("The directory is copied to def.").waitFor();

    const header = page.locator("header");
    await header
      .getByRole("button", { name: "Copy the build artifacts" })
      .click();
    const title = header.getByRole("textbox", { name: "Conversation title" });
    await title.fill("Copy dist to def");
    await title.press("Enter");
    await header.getByRole("button", { name: "Copy dist to def" }).waitFor();
    const renamed = (await calls()).find((c) => c.method === "PATCH");
    assert.deepEqual(renamed, {
      method: "PATCH",
      path: "/conversations/c2",
      body: { title: "Copy dist to def" },
    });

    await header.getByRole("button", { name: "Conversation actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Delete" })
      .click();
    await page.getByRole("dialog").waitFor({ state: "detached" });
    assert.equal(new URL(page.url()).searchParams.get("c"), null);
    assert.equal(await nav().getByText("Copy dist to def").count(), 0);
    assert.ok(
      (await calls()).some(
        (c) => c.method === "DELETE" && c.path === "/conversations/c2",
      ),
    );
  });

  it("collapses the sidebar and keeps it collapsed after a reload", async () => {
    await page.getByRole("button", { name: "Hide the sidebar" }).click();
    await page.getByRole("button", { name: "Show conversations" }).waitFor();
    await assert.doesNotReject(
      page.waitForFunction(
        () =>
          document.querySelector("aside")?.getBoundingClientRect().width === 0,
      ),
    );

    await load();
    await page.getByRole("button", { name: "Show conversations" }).waitFor();
    await page.getByRole("button", { name: "Show conversations" }).click();
    await page.getByRole("button", { name: "Hide the sidebar" }).waitFor();
  });

  it("opens the conversations in a drawer on narrow screens", async () => {
    await context.close();
    await open({ width: 390, height: 844 });

    assert.equal(await nav().count(), 0);
    await page.getByRole("button", { name: "Show conversations" }).click();
    await nav().getByText("Audit my running Workspaces").click();
    await page.getByText("You have 3 running Workspaces.").waitFor();
    await nav().waitFor({ state: "detached" });
    assert.equal(new URL(page.url()).searchParams.get("c"), "c1");
  });

  it("changes the approval settings", async () => {
    await page.getByRole("button", { name: "Choose the model" }).click();
    await page.getByRole("button", { name: "Settings" }).click();
    const drawer = page.getByRole("dialog");
    await drawer.getByText("Approvals", { exact: true }).waitFor();
    assert.equal(
      await drawer
        .getByRole("radio", { name: /Risky only/ })
        .getAttribute("aria-checked"),
      "true",
    );

    await drawer.getByRole("radio", { name: /All changes/ }).click();
    await assert.doesNotReject(
      page.waitForFunction(() =>
        window.agentCalls.some(
          (c) => c.method === "PATCH" && c.path === "/settings",
        ),
      ),
    );
    const updated = (await calls()).find(
      (c) => c.method === "PATCH" && c.path === "/settings",
    );
    assert.deepEqual(updated.body, { approvals: { api: "write" } });
    await drawer
      .locator('[role="radio"][aria-checked="true"]', { hasText: "All changes" })
      .waitFor();
  });
});
