import { strict as assert } from "node:assert";
import { describe, it } from "node:test";
import { buildSystemPrompt, type PromptContext } from "./prompt.ts";

const base: PromptContext = {
  domain: "example.com",
  user: { name: "alice", displayName: "Alice" },
  workspace: { name: "abc" },
  workDir: "/workspace",
  apiMode: "proxy",
  apiIndex: "cordium.v1 — Cordium\n  MainService: Space[List|Get]",
  tools: ["bash", "read", "workspace_exec", "cordium_api_call"],
  date: new Date("2026-10-03T10:00:00Z"),
};

describe("system prompt", () => {
  it("describes the environment, Cordium, the tools and the presentation rules", () => {
    const prompt = buildSystemPrompt(base);
    assert.match(prompt, /^You are the Cordium Agent/);
    assert.match(prompt, /\(domain: example\.com\)/);
    assert.match(prompt, /own Cordium Workspace "abc"/);
    assert.match(prompt, /`octelium\.alice`/);
    assert.match(prompt, /Your working directory is \/workspace/);
    assert.match(prompt, /never stop, restart or delete this Workspace/);
    assert.match(prompt, /the Octelium User "alice" \(Alice\)/);
    assert.match(prompt, /<service>\.<namespace>\.local\.example\.com/);
    assert.match(prompt, /Never run `octelium login`/);
    assert.match(prompt, /`default\.default\.alice`/);
    assert.match(prompt, /https:\/\/<workspace>\.cordium\.example\.com/);
    assert.match(prompt, /issue the tool calls in parallel/);
    assert.match(prompt, /paused until the User approves them/);
    assert.match(prompt, /octelium:\/\/resource\/<apiVersion>\/<Kind>\/<name>/);
    assert.match(prompt, /# Current date\n2026-10-03/);
    assert.match(
      prompt,
      /# API index\n.*\ncordium\.v1 — Cordium\n  MainService: Space\[List\|Get\]/,
    );
    assert.doesNotMatch(prompt, /NOT available/);
    assert.doesNotMatch(prompt, /Additional instructions/);
  });

  it("adapts to the configuration", () => {
    const prompt = buildSystemPrompt({
      ...base,
      domain: undefined,
      user: undefined,
      workspace: undefined,
      tools: ["cordium_api_call"],
      apiMode: "disabled",
      apiUnavailableReason: "no socket",
      append: "  Always answer in French.  ",
    });
    assert.match(prompt, /the signed-in Octelium User/);
    assert.match(prompt, /`octelium\.<USER>`/);
    assert.match(prompt, /\.local\.<DOMAIN>/);
    assert.doesNotMatch(prompt, /octeliumctl/);
    assert.match(prompt, /NOT available to you \(no socket\)/);
    assert.match(
      prompt,
      /# Additional instructions\nAlways answer in French\.$/,
    );
  });
});
