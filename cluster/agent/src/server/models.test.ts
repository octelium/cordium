import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as path from "node:path";
import { after, before, describe, it } from "node:test";
import { fauxAssistantMessage, fauxText } from "@earendil-works/pi-ai";
import type {
  AgentInfo,
  CreateConversationResponse,
  ListAuthProvidersResponse,
  LoginSession,
  MarkdownBlock,
  ModelsResponse,
} from "../protocol/index.ts";
import {
  collectEvents,
  createTestApp,
  postJSON,
  type TestApp,
} from "../testutil/app.ts";
import { createFakeOAuthProvider } from "../testutil/oauth.ts";

const getJSON = async <T>(url: string): Promise<T> => {
  const res = await fetch(url);
  assert.equal(res.status, 200, await res.clone().text());
  return (await res.json()) as T;
};

const waitForLogin = async (
  t: TestApp,
  id: string,
  predicate: (session: LoginSession) => boolean,
): Promise<LoginSession> => {
  const deadline = Date.now() + 5000;
  for (;;) {
    const session = await getJSON<LoginSession>(
      `${t.baseUrl}/v1/auth/logins/${id}`,
    );
    if (predicate(session)) {
      return session;
    }
    if (Date.now() > deadline) {
      throw new Error(`Timed out: ${JSON.stringify(session)}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
};

describe("models and logins", () => {
  let t: TestApp;

  before(async () => {
    t = await createTestApp({
      config: { llm: { loginProviders: ["fake-oauth"] } },
    });
    t.modelRuntime.registerNativeProvider(createFakeOAuthProvider());
  });

  after(async () => {
    await t.close();
  });

  it("advertises the capabilities", async () => {
    const info = await getJSON<AgentInfo>(`${t.baseUrl}/v1/info`);
    assert.equal(info.capabilities.models, true);
    assert.equal(info.capabilities.login, true);
  });

  it("lists and switches the models", async () => {
    let body = await getJSON<ModelsResponse>(`${t.baseUrl}/v1/models`);
    assert.equal(body.current?.provider, "faux");
    assert.equal(body.current?.id, "faux-1");
    assert.equal(body.error, undefined);
    assert.ok(
      body.models.some((m) => m.provider === "faux" && m.id === "faux-2"),
    );
    assert.ok(!body.models.some((m) => m.provider === "fake-oauth"));

    const res = await postJSON(
      `${t.baseUrl}/v1/model`,
      { provider: "faux", id: "faux-2", thinkingLevel: "high" },
      "PUT",
    );
    assert.equal(res.status, 200);
    body = (await res.json()) as ModelsResponse;
    assert.equal(body.current?.id, "faux-2");
    assert.equal(body.thinkingLevel, "high");
    assert.deepEqual(
      JSON.parse(
        fs.readFileSync(path.join(t.root, "data", "settings.json"), "utf8"),
      ),
      { model: { provider: "faux", id: "faux-2" }, thinkingLevel: "high" },
    );

    t.faux.setResponses([
      (_context, _options, _state, model) =>
        fauxAssistantMessage([fauxText(`Using ${model.id}`)]),
    ]);
    const created = (await (
      await postJSON(`${t.baseUrl}/v1/conversations`, {
        input: { text: "Which model?" },
      })
    ).json()) as CreateConversationResponse;
    const events = await collectEvents(t.baseUrl, created.run!.id);
    const completed = events.find((e) => e.type === "message.completed");
    assert.ok(completed && completed.type === "message.completed");
    const markdown = completed.message.blocks.find(
      (b) => b.type === "markdown",
    ) as MarkdownBlock;
    assert.equal(markdown.text, "Using faux-2");

    for (const invalid of [
      {},
      { provider: "faux" },
      { provider: "faux", id: "missing" },
      { provider: "faux", id: "faux-1", thinkingLevel: "extreme" },
      { provider: "fake-oauth", id: "fake-oauth-small" },
    ]) {
      const res = await postJSON(`${t.baseUrl}/v1/model`, invalid, "PUT");
      assert.equal(res.status, 400, JSON.stringify(invalid));
    }
  });

  it("signs in via the HTTP API", async () => {
    let providers = await getJSON<ListAuthProvidersResponse>(
      `${t.baseUrl}/v1/auth/providers`,
    );
    assert.deepEqual(
      providers.items.map((p) => [p.id, p.configured]),
      [["fake-oauth", false]],
    );

    const res = await postJSON(`${t.baseUrl}/v1/auth/logins`, {
      provider: "fake-oauth",
    });
    assert.equal(res.status, 201);
    const started = (await res.json()) as LoginSession;
    assert.equal(started.status, "pending");

    const selecting = await waitForLogin(
      t,
      started.id,
      (s) => s.prompt?.type === "select",
    );
    assert.equal(selecting.events[0].type, "auth_url");
    assert.equal(
      (
        await postJSON(
          `${t.baseUrl}/v1/auth/logins/${started.id}/prompts/${selecting.prompt!.id}`,
          { value: "code" },
        )
      ).status,
      200,
    );

    const coding = await waitForLogin(
      t,
      started.id,
      (s) => s.prompt?.type === "manual_code",
    );
    assert.equal(
      (
        await postJSON(
          `${t.baseUrl}/v1/auth/logins/${started.id}/prompts/${selecting.prompt!.id}`,
          { value: "good" },
        )
      ).status,
      409,
    );
    await postJSON(
      `${t.baseUrl}/v1/auth/logins/${started.id}/prompts/${coding.prompt!.id}`,
      { value: "good" },
    );

    const done = await waitForLogin(
      t,
      started.id,
      (s) => s.status !== "pending",
    );
    assert.equal(done.status, "completed");
    assert.equal(done.model?.provider, "fake-oauth");
    assert.equal(done.model?.id, "fake-oauth-small");

    const models = await getJSON<ModelsResponse>(`${t.baseUrl}/v1/models`);
    assert.equal(models.current?.provider, "fake-oauth");
    assert.equal(models.current?.id, "fake-oauth-small");
    assert.ok(
      models.models.some(
        (m) => m.provider === "fake-oauth" && m.id === "fake-oauth-large",
      ),
    );

    providers = await getJSON<ListAuthProvidersResponse>(
      `${t.baseUrl}/v1/auth/providers`,
    );
    assert.deepEqual(
      providers.items.map((p) => [p.id, p.configured, p.oauth]),
      [["fake-oauth", true, true]],
    );

    const cancelled = await fetch(`${t.baseUrl}/v1/auth/logins/${started.id}`, {
      method: "DELETE",
    });
    assert.equal(cancelled.status, 200);
    assert.equal(
      ((await cancelled.json()) as LoginSession).status,
      "completed",
    );

    const logout = await fetch(`${t.baseUrl}/v1/auth/providers/fake-oauth`, {
      method: "DELETE",
    });
    assert.equal(logout.status, 204);
    const afterLogout = await getJSON<ModelsResponse>(`${t.baseUrl}/v1/models`);
    assert.equal(afterLogout.current?.provider, "faux");
    assert.ok(!afterLogout.models.some((m) => m.provider === "fake-oauth"));
  });

  it("validates the login requests", async () => {
    for (const body of [{}, { provider: "openai" }, { provider: 1 }]) {
      const res = await postJSON(`${t.baseUrl}/v1/auth/logins`, body);
      assert.equal(res.status, 400, JSON.stringify(body));
    }
    assert.equal(
      (
        await postJSON(`${t.baseUrl}/v1/auth/logins`, {
          provider: "fake-oauth",
          selectModel: "yes",
        })
      ).status,
      400,
    );
    assert.equal(
      (await fetch(`${t.baseUrl}/v1/auth/logins/unknown`)).status,
      404,
    );
    assert.equal(
      (
        await fetch(`${t.baseUrl}/v1/auth/logins/unknown`, {
          method: "DELETE",
        })
      ).status,
      404,
    );
    assert.equal(
      (
        await fetch(`${t.baseUrl}/v1/auth/providers/openai`, {
          method: "DELETE",
        })
      ).status,
      400,
    );
  });
});
