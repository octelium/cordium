import { strict as assert } from "node:assert";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { after, before, describe, it } from "node:test";
import { resolveConfig } from "./config.ts";
import { logger, setLogLevel } from "./log.ts";
import { repositoryDir, syncSkills } from "./skills.ts";

const git = (cwd: string, ...args: string[]) =>
  execFileSync("git", args, {
    cwd,
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "t",
      GIT_AUTHOR_EMAIL: "t@example.com",
      GIT_COMMITTER_NAME: "t",
      GIT_COMMITTER_EMAIL: "t@example.com",
    },
  });

describe("skills", () => {
  let dir: string;
  let repo: string;

  before(() => {
    setLogLevel("error");
    dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-agent-skills-"));
    repo = path.join(dir, "repo");
    fs.mkdirSync(path.join(repo, "skills", "demo"), { recursive: true });
    fs.writeFileSync(
      path.join(repo, "skills", "demo", "SKILL.md"),
      "---\nname: demo\ndescription: Demo skill\n---\nv1\n",
    );
    git(repo, "init", "-q", "-b", "main");
    git(repo, "add", ".");
    git(repo, "commit", "-q", "-m", "v1");
  });

  after(() => {
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it("clones and updates the skills repositories", async () => {
    const local = path.join(dir, "local-skills");
    fs.mkdirSync(local);
    const config = resolveConfig(
      {
        dataDir: path.join(dir, "data"),
        skills: {
          paths: [local, path.join(dir, "missing")],
          repositories: [{ url: repo, ref: "main" }],
        },
      },
      {},
    );

    const paths = await syncSkills(config, logger);
    const cloned = repositoryDir(config.dataDir, { url: repo, ref: "main" });
    assert.deepEqual(paths, [local, path.join(cloned, "skills")]);
    assert.match(
      fs.readFileSync(path.join(cloned, "skills", "demo", "SKILL.md"), "utf8"),
      /v1/,
    );

    fs.writeFileSync(
      path.join(repo, "skills", "demo", "SKILL.md"),
      "---\nname: demo\ndescription: Demo skill\n---\nv2\n",
    );
    git(repo, "commit", "-q", "-am", "v2");
    await syncSkills(config, logger);
    assert.match(
      fs.readFileSync(path.join(cloned, "skills", "demo", "SKILL.md"), "utf8"),
      /v2/,
    );
  });

  it("tolerates unreachable repositories and the disabled mode", async () => {
    const config = resolveConfig(
      {
        dataDir: path.join(dir, "data2"),
        skills: { repositories: [{ url: path.join(dir, "nope") }] },
      },
      {},
    );
    assert.deepEqual(await syncSkills(config, logger), []);

    const disabled = resolveConfig(
      { dataDir: path.join(dir, "data3"), skills: { enabled: false } },
      {},
    );
    assert.deepEqual(await syncSkills(disabled, logger), []);
  });
});
