import { execFile } from "node:child_process";
import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import { promisify } from "node:util";
import type { Config, SkillsRepository } from "./config.ts";
import type { Logger } from "./log.ts";

const execFileAsync = promisify(execFile);

const git = async (args: string[], timeoutMs: number) =>
  execFileAsync("git", args, {
    timeout: timeoutMs,
    env: { ...process.env, GIT_TERMINAL_PROMPT: "0" },
  });

export const repositoryDir = (
  dataDir: string,
  repo: SkillsRepository,
): string =>
  path.join(
    dataDir,
    "skills",
    crypto
      .createHash("sha256")
      .update(`${repo.url}#${repo.ref ?? ""}`)
      .digest("hex")
      .slice(0, 16),
  );

const syncRepository = async (
  dir: string,
  repo: SkillsRepository,
  timeoutMs: number,
) => {
  if (fs.existsSync(path.join(dir, ".git"))) {
    await git(
      ["-C", dir, "fetch", "--depth", "1", "origin", repo.ref ?? "HEAD"],
      timeoutMs,
    );
    await git(["-C", dir, "checkout", "--force", "FETCH_HEAD"], timeoutMs);
    return;
  }

  const tmp = `${dir}.tmp-${process.pid}`;
  fs.rmSync(tmp, { recursive: true, force: true });
  fs.mkdirSync(path.dirname(dir), { recursive: true });
  await git(
    [
      "clone",
      "--depth",
      "1",
      ...(repo.ref ? ["--branch", repo.ref] : []),
      repo.url,
      tmp,
    ],
    timeoutMs,
  );
  fs.rmSync(dir, { recursive: true, force: true });
  fs.renameSync(tmp, dir);
};

export const syncSkills = async (
  config: Config,
  logger: Logger,
  timeoutMs = 30000,
): Promise<string[]> => {
  if (!config.skills.enabled) {
    return [];
  }

  const ret: string[] = [];
  for (const p of config.skills.paths) {
    if (fs.existsSync(p)) {
      ret.push(p);
    } else {
      logger.warn("Skills path does not exist", { path: p });
    }
  }

  for (const repo of config.skills.repositories) {
    const dir = repositoryDir(config.dataDir, repo);
    try {
      await syncRepository(dir, repo, timeoutMs);
    } catch (err) {
      logger.warn("Could not synchronize the skills repository", {
        url: repo.url,
        ref: repo.ref,
        error: err as Error,
      });
    }
    if (!fs.existsSync(dir)) {
      continue;
    }
    const skillsDir = path.join(dir, repo.path ?? "skills");
    ret.push(fs.existsSync(skillsDir) ? skillsDir : dir);
  }

  return ret;
};
