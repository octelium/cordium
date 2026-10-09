import { strict as assert } from "node:assert";
import { describe, it } from "node:test";
import {
  formatResourceURI,
  isTerminalRunStatus,
  parseResourceURI,
  resourceKey,
} from "./index.ts";

describe("protocol", () => {
  it("formats and parses resource URIs", () => {
    const uri = formatResourceURI({
      apiVersion: "core/v1",
      kind: "Service",
      name: "nginx.default",
    });
    assert.equal(uri, "octelium://resource/core/v1/Service/nginx.default");
    assert.deepEqual(parseResourceURI(uri), {
      apiVersion: "core/v1",
      kind: "Service",
      name: "nginx.default",
    });

    const encoded = formatResourceURI({
      apiVersion: "cordium/v1",
      kind: "Workspace",
      name: "a b/c",
    });
    assert.equal(parseResourceURI(encoded)?.name, "a b/c");
  });

  it("rejects invalid resource URIs", () => {
    assert.equal(parseResourceURI("https://example.com"), undefined);
    assert.equal(
      parseResourceURI("octelium://resource/core/Service/x"),
      undefined,
    );
    assert.equal(
      parseResourceURI("octelium://resource/core/v1/Service/"),
      undefined,
    );
    assert.equal(
      parseResourceURI("octelium://resource/core/v1/Service/%E0%A4%A"),
      undefined,
    );
  });

  it("computes resource keys and terminal statuses", () => {
    assert.equal(
      resourceKey({ apiVersion: "core/v1", kind: "User", name: "alice" }),
      "core/v1/User/alice",
    );
    assert.equal(isTerminalRunStatus("completed"), true);
    assert.equal(isTerminalRunStatus("cancelled"), true);
    assert.equal(isTerminalRunStatus("awaiting_approval"), false);
  });
});
