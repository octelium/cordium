import { strict as assert } from "node:assert";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { describe, it } from "node:test";
import {
  ConfigError,
  DEFAULT_BUILTIN_TOOLS,
  DEFAULT_SKILLS_REPOSITORY,
  getRegionPortalOrigin,
  loadConfig,
  mergeConfigInputs,
  resolveConfig,
  validateConfigInput,
} from "./config.ts";

describe("config", () => {
  it("resolves the defaults from the environment", () => {
    const cfg = resolveConfig(
      {},
      {
        OCTELIUM_DOMAIN: "example.com",
        OCTELIUM_AUTH_PROXY_SOCKET: "/var/run/octelium-proxy.sock",
        CORDIUM_AGENT_DATA_DIR: "/tmp/agent-data",
        CORDIUM_HOSTNAME: "abc.cordium.example.com",
      },
    );
    assert.equal(cfg.dataDir, "/tmp/agent-data");
    assert.equal(cfg.server.host, "0.0.0.0");
    assert.equal(cfg.server.port, 8080);
    assert.equal(cfg.server.cors, false);
    assert.deepEqual(cfg.server.allowedOrigins, [
      "https://cordium.example.com",
      "https://abc.cordium.example.com",
    ]);
    assert.equal(cfg.octelium.mode, "auto");
    assert.equal(cfg.octelium.domain, "example.com");
    assert.equal(cfg.octelium.authProxySocket, "/var/run/octelium-proxy.sock");
    assert.equal(cfg.llm.provider, "octelium");
    assert.equal(cfg.llm.thinkingLevel, "medium");
    assert.deepEqual(cfg.llm.loginProviders, ["anthropic", "openai"]);
    assert.deepEqual(cfg.agent.tools, DEFAULT_BUILTIN_TOOLS);
    assert.equal(cfg.approvals.api, "destructive");
    assert.equal(cfg.approvals.commands, false);
    assert.deepEqual(cfg.skills.repositories, [
      { url: DEFAULT_SKILLS_REPOSITORY, ref: "main" },
    ]);
  });

  it("allows the portal of the Workspace's Region", () => {
    const cfg = resolveConfig(
      {},
      {
        OCTELIUM_DOMAIN: "example.com",
        CORDIUM_HOSTNAME: "abc.eu.cordium.example.com",
      },
    );
    assert.deepEqual(cfg.server.allowedOrigins, [
      "https://cordium.example.com",
      "https://eu.cordium.example.com",
      "https://abc.eu.cordium.example.com",
    ]);

    assert.equal(
      getRegionPortalOrigin("abc.cordium.example.com", "example.com"),
      "https://cordium.example.com",
    );
    assert.equal(
      getRegionPortalOrigin("abc.other.example.com", "example.com"),
      undefined,
    );
    assert.equal(getRegionPortalOrigin("localhost"), undefined);
  });

  it("merges the inputs deeply while replacing arrays and headers", () => {
    const merged = mergeConfigInputs(
      {
        server: { port: 1, allowedOrigins: ["https://a.com"] },
        llm: { model: "a", headers: { x: "1" } },
      },
      undefined,
      {
        server: { host: "127.0.0.1", allowedOrigins: ["https://b.com"] },
        llm: { headers: { y: "2" } },
      },
    );
    assert.deepEqual(merged, {
      server: { port: 1, host: "127.0.0.1", allowedOrigins: ["https://b.com"] },
      llm: { model: "a", headers: { y: "2" } },
    });
  });

  it("rejects unknown and invalid fields", () => {
    assert.throws(
      () => validateConfigInput({ server: { prot: 1 } }, "test"),
      (err: Error) =>
        err instanceof ConfigError && /\/server.*prot/.test(err.message),
    );
    assert.throws(
      () => validateConfigInput({ approvals: { api: "always" } }, "test"),
      ConfigError,
    );
    assert.throws(
      () => validateConfigInput({ server: { port: 70000 } }, "test"),
      ConfigError,
    );
    assert.throws(
      () => validateConfigInput({ agent: { tools: ["read", "nope"] } }, "test"),
      ConfigError,
    );
    assert.throws(
      () => validateConfigInput({ octelium: { insecureTLS: true } }, "test"),
      ConfigError,
    );
    assert.doesNotThrow(() =>
      validateConfigInput(
        {
          llm: {
            provider: "anthropic",
            model: "claude",
            thinkingLevel: "high",
          },
          approvals: { api: "write", commands: true },
          webSearch: { provider: "searxng", baseUrl: "http://searx" },
        },
        "test",
      ),
    );
  });

  it("loads the config file, the JSON env var and the overrides in order", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-config-"));
    try {
      const configPath = path.join(dir, "config.json");
      fs.writeFileSync(
        configPath,
        JSON.stringify({
          server: { port: 9000 },
          llm: { service: "llm", model: "gpt" },
          approvals: { commands: true },
        }),
      );

      const cfg = loadConfig({
        configPath,
        env: {
          CORDIUM_AGENT_CONFIG_JSON: JSON.stringify({
            llm: { model: "claude" },
          }),
        },
        overrides: { server: { port: 9100 }, dataDir: dir },
      });
      assert.equal(cfg.server.port, 9100);
      assert.equal(cfg.llm.service, "llm");
      assert.equal(cfg.llm.model, "claude");
      assert.equal(cfg.approvals.commands, true);
      assert.equal(cfg.approvals.api, "destructive");
      assert.equal(cfg.dataDir, dir);

      const fromDataDir = loadConfig({
        env: { CORDIUM_AGENT_DATA_DIR: dir },
      });
      assert.equal(fromDataDir.server.port, 9000);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it("reports unreadable and malformed config files", () => {
    assert.throws(
      () => loadConfig({ configPath: "/nonexistent/config.json", env: {} }),
      ConfigError,
    );
    assert.throws(
      () =>
        loadConfig({
          env: { CORDIUM_AGENT_CONFIG_JSON: "{" },
          overrides: { dataDir: os.tmpdir() },
        }),
      ConfigError,
    );
  });
});
