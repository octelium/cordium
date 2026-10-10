import { strict as assert } from "node:assert";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { describe, it } from "node:test";
import { fileURLToPath } from "node:url";
import { resolvePublishVersion } from "./publish-version.mjs";

const defaults = {
  eventName: "push",
  refType: "branch",
  refName: "dev",
  baseVersion: "0.1.0",
  runNumber: "123",
  runAttempt: "1",
};

describe("publishing versions", () => {
  it("publishes each branch under its own npm tag", () => {
    for (const refName of ["main", "dev", "b-feature", "b-w146"]) {
      assert.deepEqual(resolvePublishVersion({ ...defaults, refName }), {
        version: `0.1.0-${refName}.123.1`,
        tag: refName,
      });
    }
  });

  it("generates different versions for new runs and retries", () => {
    const versions = [
      defaults,
      { ...defaults, runNumber: "124" },
      { ...defaults, runAttempt: "2" },
    ].map((input) => resolvePublishVersion(input).version);
    assert.equal(new Set(versions).size, versions.length);
  });

  it("normalizes prerelease identifiers while preserving npm tags", () => {
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        refName: "b-feature.name_1",
        baseVersion: "1.2.3-rc.1+build.1",
      }),
      { version: "1.2.3-b-feature-name-1.123.1", tag: "b-feature.name_1" },
    );
  });

  it("publishes version tags and stable releases as latest", () => {
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        refType: "tag",
        refName: "v1.2.3",
      }),
      { version: "1.2.3", tag: "latest" },
    );
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        eventName: "release",
        release: { tag_name: "v2.3.4", prerelease: false },
      }),
      { version: "2.3.4", tag: "latest" },
    );
  });

  it("publishes prerelease versions and prerelease releases as next", () => {
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        refType: "tag",
        refName: "v1.2.3-rc.1",
      }),
      { version: "1.2.3-rc.1", tag: "next" },
    );
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        eventName: "release",
        release: { tag_name: "1.2.3", prerelease: true },
      }),
      { version: "1.2.3", tag: "next" },
    );
  });

  it("uses the manual npm tag for unique branch builds", () => {
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        eventName: "workflow_dispatch",
        inputs: { tag: "canary" },
      }),
      { version: "0.1.0-canary.123.1", tag: "canary" },
    );
  });

  it("supports manual semver overrides and selected Git tags", () => {
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        eventName: "workflow_dispatch",
        inputs: { tag: "latest", version: "v3.2.1" },
      }),
      { version: "3.2.1", tag: "latest" },
    );
    assert.deepEqual(
      resolvePublishVersion({
        ...defaults,
        eventName: "workflow_dispatch",
        refType: "tag",
        refName: "v3.2.1-rc.2",
        inputs: { tag: "beta" },
      }),
      { version: "3.2.1-rc.2", tag: "beta" },
    );
  });

  it("rejects invalid npm tags and workflow output injection", () => {
    for (const tag of [
      undefined,
      "",
      " ",
      "v1.2",
      "1.2.3",
      "x",
      "foo/bar",
      "--dry-run",
      "dev\ntag=latest",
    ]) {
      assert.throws(
        () =>
          resolvePublishVersion({
            ...defaults,
            eventName: "workflow_dispatch",
            inputs: { tag },
          }),
        /Invalid npm dist-tag/,
      );
    }
  });

  it("rejects invalid release and manual versions", () => {
    for (const version of ["1.2", "01.2.3", "1.2.3-01", "1.2.3\ntag=latest"]) {
      assert.throws(() =>
        resolvePublishVersion({
          ...defaults,
          refType: "tag",
          refName: `v${version}`,
        }),
      );
      assert.throws(
        () =>
          resolvePublishVersion({
            ...defaults,
            eventName: "workflow_dispatch",
            inputs: { tag: "latest", version },
          }),
        /Invalid publishing version/,
      );
    }
    assert.throws(
      () =>
        resolvePublishVersion({ ...defaults, refType: "tag", refName: "v" }),
      /Invalid publishing version/,
    );
  });

  it("rejects invalid base versions, run identifiers and unsupported events", () => {
    for (const input of [
      { baseVersion: "invalid" },
      { runNumber: "0" },
      { runAttempt: "01" },
      { eventName: "pull_request" },
    ]) {
      assert.throws(() => resolvePublishVersion({ ...defaults, ...input }));
    }
  });

  it("writes GitHub Actions outputs from the event payload and environment", () => {
    const dir = fs.mkdtempSync(
      path.join(os.tmpdir(), "cordium-agent-publish-"),
    );
    try {
      const eventPath = path.join(dir, "event.json");
      const outputPath = path.join(dir, "output");
      fs.writeFileSync(
        eventPath,
        JSON.stringify({ inputs: { tag: "canary", version: "v2.0.0-rc.1" } }),
      );
      execFileSync(
        process.execPath,
        [fileURLToPath(new URL("./publish-version.mjs", import.meta.url))],
        {
          env: {
            ...process.env,
            GITHUB_EVENT_PATH: eventPath,
            GITHUB_OUTPUT: outputPath,
            GITHUB_EVENT_NAME: "workflow_dispatch",
            GITHUB_REF_TYPE: "branch",
            GITHUB_REF_NAME: "main",
            GITHUB_RUN_NUMBER: "123",
            GITHUB_RUN_ATTEMPT: "1",
          },
        },
      );
      assert.equal(
        fs.readFileSync(outputPath, "utf8"),
        "version=2.0.0-rc.1\ntag=canary\n",
      );
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
