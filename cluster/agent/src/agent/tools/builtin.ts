import * as fs from "node:fs";
import * as path from "node:path";
import {
  createBashToolDefinition,
  createEditToolDefinition,
  createFindToolDefinition,
  createGrepToolDefinition,
  createLsToolDefinition,
  createReadToolDefinition,
  createWriteToolDefinition,
  type ToolDefinition,
} from "@earendil-works/pi-coding-agent";
import type { SettingsProvider } from "../types.ts";
import { errorResult, requireRunContext, type ToolDeps } from "./util.ts";

const factories: Record<string, (cwd: string) => ToolDefinition> = {
  read: (cwd) => createReadToolDefinition(cwd) as unknown as ToolDefinition,
  bash: (cwd) => createBashToolDefinition(cwd) as unknown as ToolDefinition,
  edit: (cwd) => createEditToolDefinition(cwd) as unknown as ToolDefinition,
  write: (cwd) => createWriteToolDefinition(cwd) as unknown as ToolDefinition,
  grep: (cwd) => createGrepToolDefinition(cwd) as unknown as ToolDefinition,
  find: (cwd) => createFindToolDefinition(cwd) as unknown as ToolDefinition,
  ls: (cwd) => createLsToolDefinition(cwd) as unknown as ToolDefinition,
};

export const BUILTIN_TOOL_NAMES = Object.keys(factories);

const requiredBinaries: Record<string, string[]> = {
  grep: ["rg"],
  find: ["fd", "fdfind"],
};

export const findBinary = (
  names: string[],
  envPath = process.env.PATH ?? "",
): string | undefined => {
  for (const dir of envPath.split(path.delimiter)) {
    if (!dir) {
      continue;
    }
    for (const name of names) {
      const candidate = path.join(dir, name);
      try {
        fs.accessSync(candidate, fs.constants.X_OK);
        if (fs.statSync(candidate).isFile()) {
          return candidate;
        }
      } catch {
        continue;
      }
    }
  }
  return undefined;
};

export const filterAvailableTools = (
  names: string[],
  envPath = process.env.PATH ?? "",
): { available: string[]; unavailable: string[] } => {
  const available: string[] = [];
  const unavailable: string[] = [];
  for (const name of names) {
    const binaries = requiredBinaries[name];
    if (binaries && !findBinary(binaries, envPath)) {
      unavailable.push(name);
    } else {
      available.push(name);
    }
  }
  return { available, unavailable };
};

export interface BuiltinToolDeps extends ToolDeps {
  names: string[];
  settings: SettingsProvider;
}

const withBashApproval = (
  deps: BuiltinToolDeps,
  def: ToolDefinition,
): ToolDefinition => ({
  ...def,
  async execute(toolCallId, params, signal, onUpdate, ctx) {
    if (!deps.settings.get().approvals.commands) {
      return def.execute(toolCallId, params, signal, onUpdate, ctx);
    }
    const run = requireRunContext(deps);
    const command = String((params as { command?: unknown }).command ?? "");
    const decision = await run.requestApproval(
      {
        toolCallId,
        title: "Run a shell command in the agent's Workspace",
        risk: "write",
        preview: { command },
      },
      signal,
    );
    if (!decision.approved) {
      return errorResult(
        `The User did not approve running this command${decision.reason ? `. Reason: ${decision.reason}` : ""}. Do not retry it unless the User asks you to.`,
      );
    }
    return def.execute(toolCallId, params, signal, onUpdate, ctx);
  },
});

export const createBuiltinTools = (deps: BuiltinToolDeps): ToolDefinition[] =>
  deps.names.map((name) => {
    const factory = factories[name];
    if (!factory) {
      throw new Error(
        `Unknown built-in tool "${name}". The available tools are: ${BUILTIN_TOOL_NAMES.join(", ")}`,
      );
    }
    const def = factory(deps.workDir);
    return name === "bash" ? withBashApproval(deps, def) : def;
  });
