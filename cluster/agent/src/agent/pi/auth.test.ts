import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { after, before, beforeEach, describe, it } from "node:test";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { HTTPError } from "../../errors.ts";
import { logger, setLogLevel } from "../../log.ts";
import type { LoginSession } from "../../protocol/index.ts";
import { createFakeOAuthProvider } from "../../testutil/oauth.ts";
import { AuthManager, toLoginEvent } from "./auth.ts";

const waitFor = async (
  fn: () => LoginSession | undefined,
  predicate: (session: LoginSession) => boolean,
  timeoutMs = 5000,
): Promise<LoginSession> => {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const session = fn();
    if (session && predicate(session)) {
      return session;
    }
    if (Date.now() > deadline) {
      throw new Error(`Timed out: ${JSON.stringify(session)}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
};

describe("auth", () => {
  let root: string;
  let modelRuntime: ModelRuntime;
  let mgr: AuthManager;
  let logins: { provider: string; selectModel: boolean }[];
  let logouts: string[];

  before(() => {
    setLogLevel("error");
  });

  beforeEach(async () => {
    root = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-auth-"));
    modelRuntime = await ModelRuntime.create({
      authPath: path.join(root, "auth.json"),
      modelsPath: null,
      allowModelNetwork: false,
      refreshOnCreate: false,
    });
    modelRuntime.registerNativeProvider(createFakeOAuthProvider());
    logins = [];
    logouts = [];
    mgr = new AuthManager({
      modelRuntime,
      dataDir: path.join(root, "data"),
      logger,
      providers: ["fake-oauth", "anthropic", "missing"],
      onLogin: async (provider, selectModel) => {
        logins.push({ provider, selectModel });
        return { provider, id: `${provider}-small` };
      },
      onLogout: async (provider) => {
        logouts.push(provider);
      },
    });
  });

  after(() => {
    fs.rmSync(root, { recursive: true, force: true });
  });

  it("lists the supported providers", async () => {
    const items = await mgr.listProviders();
    assert.deepEqual(
      items.map((p) => p.id),
      ["fake-oauth", "anthropic"],
    );
    const fake = items[0];
    assert.equal(fake.name, "Fake (subscription)");
    assert.equal(fake.loginLabel, "Sign in with Fake");
    assert.equal(fake.subscription, true);
    assert.equal(fake.configured, false);
    assert.equal(fake.oauth, false);
  });

  it("signs in via the prompts", async () => {
    const started = mgr.startLogin({ provider: "fake-oauth" });
    assert.equal(started.status, "pending");
    assert.equal(started.provider, "fake-oauth");

    const selecting = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.prompt?.type === "select",
    );
    assert.deepEqual(selecting.events[0], {
      type: "auth_url",
      url: "https://auth.example.com/authorize",
      instructions: "Open the URL",
    });
    assert.deepEqual(
      selecting.prompt?.options?.map((o) => o.id),
      ["browser", "code"],
    );

    mgr.answerLogin(started.id, selecting.prompt!.id, "code");

    const coding = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.prompt?.type === "manual_code",
    );
    assert.equal(coding.prompt?.placeholder, "code#state");
    assert.notEqual(coding.prompt?.id, selecting.prompt?.id);

    assert.throws(
      () => mgr.answerLogin(started.id, selecting.prompt!.id, "good"),
      (err: HTTPError) => err.status === 409,
    );

    mgr.answerLogin(started.id, coding.prompt!.id, "good");

    const done = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.status !== "pending",
    );
    assert.equal(done.status, "completed");
    assert.equal(done.prompt, undefined);
    assert.deepEqual(done.model, {
      provider: "fake-oauth",
      id: "fake-oauth-small",
    });
    assert.deepEqual(done.events.at(-1), {
      type: "progress",
      message: "Exchanging the code",
    });
    assert.deepEqual(logins, [{ provider: "fake-oauth", selectModel: true }]);

    const items = await mgr.listProviders();
    assert.equal(items[0].configured, true);
    assert.equal(items[0].oauth, true);

    const auth = await modelRuntime.getAuth("fake-oauth");
    assert.equal(auth?.auth.apiKey, `token-code-${mgr.getDeviceId()}`);

    await mgr.logout("fake-oauth");
    assert.deepEqual(logouts, ["fake-oauth"]);
    assert.equal((await mgr.listProviders())[0].configured, false);
  });

  it("reports the failed logins", async () => {
    const started = mgr.startLogin({
      provider: "fake-oauth",
      selectModel: false,
    });
    const selecting = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.prompt?.type === "select",
    );
    mgr.answerLogin(started.id, selecting.prompt!.id, "browser");
    const coding = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.prompt?.type === "manual_code",
    );
    mgr.answerLogin(started.id, coding.prompt!.id, "bad");

    const done = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.status !== "pending",
    );
    assert.equal(done.status, "failed");
    assert.equal(done.error?.code, "login_failed");
    assert.match(done.error?.message ?? "", /Invalid authorization code/);
    assert.equal(done.model, undefined);
    assert.deepEqual(logins, []);
    assert.equal((await mgr.listProviders())[0].configured, false);
  });

  it("cancels the logins", async () => {
    const first = mgr.startLogin({ provider: "fake-oauth" });
    await waitFor(
      () => mgr.getLogin(first.id),
      (s) => s.prompt?.type === "select",
    );

    const second = mgr.startLogin({ provider: "fake-oauth" });
    assert.equal(mgr.getLogin(first.id)?.status, "cancelled");
    assert.equal(mgr.getLogin(first.id)?.prompt, undefined);

    const selecting = await waitFor(
      () => mgr.getLogin(second.id),
      (s) => s.prompt?.type === "select",
    );
    const cancelled = mgr.cancelLogin(second.id);
    assert.equal(cancelled.status, "cancelled");
    assert.throws(
      () => mgr.answerLogin(second.id, selecting.prompt!.id, "code"),
      (err: HTTPError) => err.status === 409,
    );

    await new Promise((resolve) => setTimeout(resolve, 50));
    assert.equal(mgr.getLogin(second.id)?.status, "cancelled");
    assert.deepEqual(logins, []);
  });

  it("validates the requests", async () => {
    for (const provider of ["openai", "missing", ""]) {
      assert.throws(
        () => mgr.startLogin({ provider }),
        (err: HTTPError) => err.status === 400,
      );
    }
    await assert.rejects(
      () => mgr.logout("openai"),
      (err: HTTPError) => err.status === 400,
    );
    assert.equal(mgr.getLogin("unknown"), undefined);
    assert.throws(
      () => mgr.cancelLogin("unknown"),
      (err: HTTPError) => err.status === 404,
    );
    assert.throws(
      () => mgr.answerLogin("unknown", "p", "v"),
      (err: HTTPError) => err.status === 404,
    );

    const started = mgr.startLogin({ provider: "fake-oauth" });
    const selecting = await waitFor(
      () => mgr.getLogin(started.id),
      (s) => s.prompt?.type === "select",
    );
    assert.throws(
      () =>
        mgr.answerLogin(started.id, selecting.prompt!.id, "a".repeat(10000)),
      (err: HTTPError) => err.status === 400,
    );
    mgr.cancelLogin(started.id);
  });

  it("keeps a stable device ID", () => {
    const id = mgr.getDeviceId();
    assert.match(id, /^[0-9a-f-]{36}$/);
    assert.equal(mgr.getDeviceId(), id);
    assert.equal(
      fs.readFileSync(path.join(root, "data", "device-id"), "utf8"),
      id,
    );
  });

  it("maps the auth events", () => {
    assert.deepEqual(
      toLoginEvent({
        type: "device_code",
        userCode: "ABCD",
        verificationUri: "https://example.com/device",
        intervalSeconds: 5,
      }),
      {
        type: "device_code",
        userCode: "ABCD",
        verificationUri: "https://example.com/device",
        intervalSeconds: 5,
        expiresInSeconds: undefined,
      },
    );
    assert.deepEqual(
      toLoginEvent({
        type: "info",
        message: "Hello",
        links: [{ url: "https://example.com", label: "Docs" }],
      }),
      {
        type: "info",
        message: "Hello",
        links: [{ url: "https://example.com", label: "Docs" }],
      },
    );
  });
});
