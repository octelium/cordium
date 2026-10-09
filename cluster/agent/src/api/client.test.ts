import { strict as assert } from "node:assert";
import { after, before, describe, it } from "node:test";
import { status } from "@grpc/grpc-js";
import { getCatalog } from "../testutil/app.ts";
import { MockError, startMockAPI, type MockAPI } from "../testutil/mockapi.ts";
import {
  APIClient,
  APIError,
  errorCodeName,
  InvalidRequestError,
  resolveAPIMode,
  toAPIError,
} from "./client.ts";

describe("API client", () => {
  const catalog = getCatalog();
  let mock: MockAPI;
  let client: APIClient;

  before(async () => {
    mock = await startMockAPI({
      "user.v1.MainService/GetStatus": () => ({
        user: { metadata: { name: "alice" } },
      }),
      "cordium.v1.MainService/GetSpace": (req) => {
        const name = (req as { name?: string }).name;
        if (name !== "demo.alice") {
          throw new MockError(status.NOT_FOUND, `Space ${name} not found`);
        }
        return {
          apiVersion: "cordium/v1",
          kind: "Space",
          metadata: { name, createdAt: "2026-01-02T15:04:05Z" },
          spec: { runtime: { envVars: [{ key: "A", value: "1" }] } },
          status: { type: "USER" },
        };
      },
      "cordium.v1.MainService/WatchWorkspace": () => [
        { create: { item: { metadata: { name: "a" } } } },
        { create: { item: { metadata: { name: "b" } } } },
        { create: { item: { metadata: { name: "c" } } } },
      ],
    });
    client = new APIClient(catalog, {
      mode: "proxy",
      authProxySocket: mock.socketPath,
      userAgent: "test-agent",
    });
  });

  after(async () => {
    await client.close();
    await mock.close();
  });

  it("invokes the unary methods over the auth proxy socket", async () => {
    assert.equal(client.enabled, true);
    const res = await client.call<{ user: { metadata: { name: string } } }>(
      "user.v1.MainService/GetStatus",
    );
    assert.equal(res.user.metadata.name, "alice");
    assert.match(
      String(mock.calls.at(-1)?.metadata.get("user-agent")[0]),
      /^test-agent/,
    );

    const spc = await client.call<Record<string, unknown>>(
      "/octelium.api.main.cordium.v1.MainService/GetSpace",
      { name: "demo.alice" },
    );
    assert.deepEqual(spc, {
      apiVersion: "cordium/v1",
      kind: "Space",
      metadata: { name: "demo.alice", createdAt: "2026-01-02T15:04:05Z" },
      spec: { runtime: { envVars: [{ key: "A", value: "1" }] } },
      status: { type: "USER" },
    });
  });

  it("maps the gRPC errors", async () => {
    await assert.rejects(
      client.call("cordium.v1.MainService/GetSpace", { name: "nope.alice" }),
      (err: Error) =>
        err instanceof APIError &&
        err.code === "not_found" &&
        err.message === "Space nope.alice not found",
    );
    await assert.rejects(
      client.call("cordium.v1.MainService/ListSpace"),
      (err: Error) => err instanceof APIError && err.code === "unimplemented",
    );
    assert.equal(errorCodeName("PERMISSION_DENIED"), "permission_denied");
    assert.equal(errorCodeName("DeadlineExceeded"), "deadline_exceeded");
    assert.equal(toAPIError(new Error("boom")).code, "unknown");
  });

  it("validates the requests before sending them", async () => {
    const count = mock.calls.length;
    await assert.rejects(
      client.call("cordium.v1.MainService/GetSpace", { nam: "x" }),
      (err: Error) =>
        err instanceof InvalidRequestError && /nam/.test(err.message),
    );
    await assert.rejects(
      client.call("cordium.v1.MainService/GetSpace", ["x"]),
      InvalidRequestError,
    );
    await assert.rejects(
      client.call("cordium.v1.MainService/CreateSpace", {
        status: { type: "PERSONAL" },
      }),
      InvalidRequestError,
    );
    await assert.rejects(
      client.call("cordium.v1.MainService/Nope"),
      InvalidRequestError,
    );
    assert.equal(mock.calls.length, count);
  });

  it("collects the server-streaming messages", async () => {
    const method = catalog.resolveMethod(
      "cordium.v1.MainService/WatchWorkspace",
    )!;
    const res = await client.invoke(method, {}, { maxMessages: 2 });
    assert.equal(res.messageCount, 2);
    assert.deepEqual(res.response, [
      { create: { item: { metadata: { name: "a" } } } },
      { create: { item: { metadata: { name: "b" } } } },
    ]);
  });

  it("refuses the disabled mode", async () => {
    const disabled = new APIClient(catalog, { mode: "disabled" });
    assert.equal(disabled.enabled, false);
    assert.equal(disabled.cordium, undefined);
    await assert.rejects(
      disabled.call("user.v1.MainService/GetStatus"),
      (err: Error) => err instanceof APIError && err.code === "unavailable",
    );
    assert.throws(
      () => disabled.requireCordium(),
      (err: Error) => err instanceof APIError && err.code === "unavailable",
    );
  });

  it("resolves the connection mode", () => {
    const base = { timeoutSeconds: 30 };
    assert.deepEqual(
      resolveAPIMode({
        ...base,
        mode: "auto",
        authProxySocket: mock.socketPath,
      }),
      { mode: "proxy" },
    );
    assert.deepEqual(
      resolveAPIMode({
        ...base,
        mode: "auto",
        authProxySocket: "/nonexistent.sock",
        domain: "example.com",
        accessToken: "tok",
      }),
      { mode: "direct" },
    );
    assert.equal(resolveAPIMode({ ...base, mode: "auto" }).mode, "disabled");
    assert.equal(
      resolveAPIMode({
        ...base,
        mode: "proxy",
        authProxySocket: "/nonexistent.sock",
      }).mode,
      "disabled",
    );
    assert.equal(
      resolveAPIMode({ ...base, mode: "direct", domain: "x" }).mode,
      "disabled",
    );
    assert.equal(
      resolveAPIMode({
        ...base,
        mode: "disabled",
        authProxySocket: mock.socketPath,
      }).mode,
      "disabled",
    );
  });
});
