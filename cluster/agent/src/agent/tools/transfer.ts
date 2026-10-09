import { execFile } from "node:child_process";
import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import { promisify } from "node:util";
import { shellQuote, type Workspace } from "@octelium/cordium";
import { defineTool } from "@earendil-works/pi-coding-agent";
import { Type } from "@earendil-works/pi-ai";
import type { AgentToolResult } from "@earendil-works/pi-agent-core";
import type { ToolDetails, TransferDetails } from "../../protocol/index.ts";
import { toAPIError, type APIClient } from "../../api/client.ts";
import { errorResult, resolvePath, textResult, type ToolDeps } from "./util.ts";

const execFileAsync = promisify(execFile);

export interface TransferToolDeps extends ToolDeps {
  client: APIClient;
  agentWorkspace?: string;
  tmpDir: string;
  approve(
    toolCallId: string,
    title: string,
    signal?: AbortSignal,
  ): Promise<string | undefined>;
}

export interface Endpoint {
  workspace?: string;
  path: string;
}

type OnUpdate = (
  partial: AgentToolResult<ToolDetails | undefined>,
) => void | undefined;

export const parseEndpoint = (
  arg: string,
  agentWorkspace?: string,
): Endpoint => {
  const match = /^([a-z0-9][a-z0-9-]{0,62}):(.+)$/.exec(arg);
  if (!match) {
    return { path: arg };
  }
  const [, workspace, p] = match;
  if (workspace === agentWorkspace) {
    return { path: p };
  }
  return { workspace, path: p };
};

export const formatEndpoint = (e: Endpoint): string =>
  e.workspace ? `${e.workspace}:${e.path}` : e.path;

const remoteHome = async (ws: Workspace, signal?: AbortSignal) => {
  const res = await ws.exec('printf %s "$HOME"', {
    signal,
    timeoutMs: 60000,
  });
  const home = res.stdout.trim();
  if (res.exitCode !== 0 || !home.startsWith("/")) {
    throw new Error(`Could not get the home directory of ${ws.name}`);
  }
  return home;
};

const resolveRemotePath = async (
  ws: Workspace,
  p: string,
  signal?: AbortSignal,
): Promise<string> => {
  if (p.startsWith("/")) {
    return path.posix.normalize(p);
  }
  const home = await remoteHome(ws, signal);
  if (p === "~") {
    return home;
  }
  return path.posix.join(home, p.startsWith("~/") ? p.slice(2) : p);
};

const remoteType = async (
  ws: Workspace,
  p: string,
  signal?: AbortSignal,
): Promise<"file" | "directory" | "missing"> => {
  const q = shellQuote(p);
  const res = await ws.exec(
    `if [ -d ${q} ]; then echo directory; elif [ -e ${q} ]; then echo file; else echo missing; fi`,
    { signal, timeoutMs: 60000 },
  );
  const out = res.stdout.trim();
  if (out === "directory" || out === "file" || out === "missing") {
    return out;
  }
  throw new Error(`Could not inspect ${ws.name}:${p}`);
};

const runChecked = async (
  ws: Workspace,
  command: string,
  opts: { signal?: AbortSignal; timeoutMs: number },
) => {
  const res = await ws.exec(command, {
    signal: opts.signal,
    timeoutMs: opts.timeoutMs,
  });
  if (res.exitCode !== 0) {
    throw new Error(
      `The command failed in ${ws.name} with exit code ${res.exitCode}: ${res.stderr.trim().slice(0, 500)}`,
    );
  }
};

const downloadOutput = async (
  ws: Workspace,
  command: string,
  localPath: string,
  opts: { signal?: AbortSignal; timeoutMs: number },
): Promise<void> => {
  const file = await fs.promises.open(localPath, "w", 0o600);
  const session = ws.execStream(command, {
    interactive: false,
    maxCaptureBytes: 64 * 1024,
    maxBufferBytes: 32 * 1024 * 1024,
    timeoutMs: opts.timeoutMs,
    signal: opts.signal,
  });
  try {
    for await (const chunk of session) {
      if (chunk.stream === "stdout") {
        await file.write(chunk.data);
      }
    }
    const res = await session.wait();
    if (res.exitCode !== 0) {
      throw new Error(
        `The command failed in ${ws.name} with exit code ${res.exitCode}: ${res.stderr.trim().slice(0, 500)}`,
      );
    }
  } finally {
    session.close();
    await file.close();
  }
};

const tarExcludes = (excludes: string[] = []): string[] =>
  excludes.map((e) => `--exclude=${e}`);

export const createTransferTool = (deps: TransferToolDeps) =>
  defineTool({
    name: "workspace_copy",
    label: "Copy files",
    description:
      'Copy a file or a directory between two Workspaces, or between a Workspace and the agent\'s own Workspace, through the Cordium API (no SSH needed). Endpoints are "<workspace>:<path>" for a Workspace or a plain path for the agent\'s own filesystem. The destination is the full path of the copy: a file is written (or overwritten) there and the contents of a directory are merged into it, e.g. copying "abc:/my/dir" to "def:/my/dir" makes /my/dir of def contain the files of /my/dir of abc. Both Workspaces must be running and have tar and gzip installed for directories.',
    parameters: Type.Object({
      source: Type.String({
        description: 'e.g. "abc:/workspace/repo/dist" or "/tmp/report.csv"',
      }),
      destination: Type.String({
        description: 'e.g. "def:/workspace/repo/dist" or "/tmp/report.csv"',
      }),
      excludes: Type.Optional(
        Type.Array(Type.String(), {
          description:
            'Directories only: tar exclude patterns, e.g. ["node_modules", ".git"]',
        }),
      ),
      timeoutSeconds: Type.Optional(
        Type.Integer({
          minimum: 10,
          maximum: 7200,
          description: "The overall deadline (default 1800)",
        }),
      ),
    }),
    async execute(toolCallId, params, signal, onUpdate) {
      const src = parseEndpoint(params.source, deps.agentWorkspace);
      const dst = parseEndpoint(params.destination, deps.agentWorkspace);
      const timeoutMs = (params.timeoutSeconds ?? 1800) * 1000;
      const report = (text: string) =>
        (onUpdate as OnUpdate | undefined)?.({
          content: [{ type: "text", text }],
          details: undefined,
        });

      const denied = await deps.approve(
        toolCallId,
        `Copy ${formatEndpoint(src)} to ${formatEndpoint(dst)}`,
        signal,
      );
      if (denied) {
        return errorResult(denied);
      }

      const tmp = path.join(
        deps.tmpDir,
        `transfer-${crypto.randomBytes(6).toString("hex")}`,
      );

      try {
        const cordium = deps.client.requireCordium();
        const getWorkspace = async (name: string) => {
          const ws = await cordium.workspaces.get(name, { signal });
          if (!ws.isReady) {
            throw new Error(
              `The Workspace ${name} is not running. Start it first with workspace_control (action "start")`,
            );
          }
          return ws;
        };

        const srcWS = src.workspace
          ? await getWorkspace(src.workspace)
          : undefined;
        const dstWS = dst.workspace
          ? await getWorkspace(dst.workspace)
          : undefined;

        const srcPath = srcWS
          ? await resolveRemotePath(srcWS, src.path, signal)
          : resolvePath(deps.workDir, src.path);
        const dstPath = dstWS
          ? await resolveRemotePath(dstWS, dst.path, signal)
          : resolvePath(deps.workDir, dst.path);

        let type: "file" | "directory" | "missing";
        if (srcWS) {
          type = await remoteType(srcWS, srcPath, signal);
        } else {
          try {
            type = fs.statSync(srcPath).isDirectory() ? "directory" : "file";
          } catch {
            type = "missing";
          }
        }
        if (type === "missing") {
          return errorResult(
            `The source ${formatEndpoint({ workspace: src.workspace, path: srcPath })} does not exist`,
          );
        }

        fs.mkdirSync(tmp, { recursive: true, mode: 0o700 });
        let bytes: number;

        if (type === "file") {
          if (!srcWS && !dstWS) {
            fs.mkdirSync(path.dirname(dstPath), { recursive: true });
            fs.copyFileSync(srcPath, dstPath);
            bytes = fs.statSync(dstPath).size;
          } else if (srcWS && !dstWS) {
            report(`Downloading ${srcWS.name}:${srcPath}`);
            await srcWS.files.download(srcPath, dstPath, {
              signal,
              timeoutMs,
            });
            bytes = fs.statSync(dstPath).size;
          } else {
            let local = srcPath;
            if (srcWS) {
              local = path.join(tmp, "file");
              report(`Downloading ${srcWS.name}:${srcPath}`);
              await srcWS.files.download(srcPath, local, {
                signal,
                timeoutMs,
              });
            }
            bytes = fs.statSync(local).size;
            report(`Uploading to ${dstWS!.name}:${dstPath}`);
            await dstWS!.files.upload(local, dstPath, { signal, timeoutMs });
          }
        } else {
          const archive = path.join(tmp, "data.tgz");
          const excludes = tarExcludes(params.excludes);
          report(
            `Packing ${formatEndpoint({ workspace: src.workspace, path: srcPath })}`,
          );
          if (srcWS) {
            await downloadOutput(
              srcWS,
              ["tar", "-C", srcPath, "-czf", "-", ...excludes, "."]
                .map(shellQuote)
                .join(" "),
              archive,
              { signal, timeoutMs },
            );
          } else {
            await execFileAsync(
              "tar",
              ["-C", srcPath, "-czf", archive, ...excludes, "."],
              { signal, timeout: timeoutMs },
            );
          }
          bytes = fs.statSync(archive).size;

          if (dstWS) {
            const remoteArchive = `/tmp/cordium-agent-${path.basename(tmp)}.tgz`;
            report(`Uploading to ${dstWS.name}`);
            await dstWS.files.upload(archive, remoteArchive, {
              signal,
              timeoutMs,
            });
            report(`Extracting into ${dstWS.name}:${dstPath}`);
            const a = shellQuote(remoteArchive);
            const d = shellQuote(dstPath);
            await runChecked(
              dstWS,
              `mkdir -p ${d} && tar -C ${d} -xzf ${a}; rc=$?; rm -f ${a}; exit $rc`,
              { signal, timeoutMs },
            );
          } else {
            fs.mkdirSync(dstPath, { recursive: true });
            await execFileAsync("tar", ["-C", dstPath, "-xzf", archive], {
              signal,
              timeout: timeoutMs,
            });
          }
        }

        const details: TransferDetails = {
          kind: "transfer",
          source: formatEndpoint({ workspace: src.workspace, path: srcPath }),
          destination: formatEndpoint({
            workspace: dst.workspace,
            path: dstPath,
          }),
          type,
          bytes,
        };
        return textResult(
          `Copied the ${type} ${details.source} to ${details.destination} (${bytes} bytes${type === "directory" ? " compressed" : ""}).`,
          details,
        );
      } catch (err) {
        const apiErr = toAPIError(err);
        return errorResult(
          `Could not copy ${params.source} to ${params.destination}: ${apiErr.message}`,
        );
      } finally {
        fs.rmSync(tmp, { recursive: true, force: true });
      }
    },
  });
