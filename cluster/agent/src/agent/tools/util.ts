import * as fs from "node:fs";
import * as path from "node:path";
import type { AgentToolResult } from "@earendil-works/pi-agent-core";
import type { ToolDetails } from "../../protocol/index.ts";
import type { ToolContextProvider, ToolRunContext } from "../types.ts";

export interface ToolDeps {
  conversationId: string;
  workDir: string;
  runtime: ToolContextProvider;
}

export const textResult = (
  text: string,
  details?: ToolDetails,
  isError = false,
): AgentToolResult<ToolDetails | undefined> => ({
  content: [{ type: "text", text }],
  details,
  isError: isError || undefined,
});

export const errorResult = (text: string, details?: ToolDetails) =>
  textResult(text, details, true);

export const truncateText = (
  text: string,
  max: number,
): { text: string; truncated: boolean } =>
  text.length <= max
    ? { text, truncated: false }
    : { text: `${text.slice(0, max)}\n…[truncated]`, truncated: true };

export const resolvePath = (workDir: string, p: string): string =>
  path.resolve(
    workDir,
    p.startsWith("~/") ? path.join(process.env.HOME ?? "", p.slice(2)) : p,
  );

export const requireRunContext = (deps: ToolDeps): ToolRunContext => {
  const ctx = deps.runtime.current(deps.conversationId);
  if (!ctx) {
    throw new Error("This tool can only be used within an active run");
  }
  return ctx;
};

export const writeJSONFile = (filePath: string, data: unknown) => {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, `${JSON.stringify(data, null, 2)}\n`);
};

export const parseCSV = (text: string): Record<string, string>[] => {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let inQuotes = false;

  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inQuotes) {
      if (c === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += c;
      }
      continue;
    }
    if (c === '"') {
      inQuotes = true;
    } else if (c === ",") {
      row.push(field);
      field = "";
    } else if (c === "\n" || c === "\r") {
      if (c === "\r" && text[i + 1] === "\n") {
        i++;
      }
      row.push(field);
      field = "";
      if (row.some((v) => v !== "")) {
        rows.push(row);
      }
      row = [];
    } else {
      field += c;
    }
  }
  row.push(field);
  if (row.some((v) => v !== "")) {
    rows.push(row);
  }

  const [header, ...data] = rows;
  if (!header) {
    return [];
  }
  return data.map((values) =>
    Object.fromEntries(header.map((key, i) => [key.trim(), values[i] ?? ""])),
  );
};

export const readRowsFile = (filePath: string): Record<string, unknown>[] => {
  const raw = fs.readFileSync(filePath, "utf8");
  const ext = path.extname(filePath).toLowerCase();

  if (ext === ".csv") {
    return parseCSV(raw);
  }

  if (ext === ".jsonl" || ext === ".ndjson") {
    return raw
      .split("\n")
      .filter((line) => line.trim() !== "")
      .map((line) => JSON.parse(line));
  }

  const parsed = JSON.parse(raw);
  if (Array.isArray(parsed)) {
    return parsed;
  }
  if (parsed && typeof parsed === "object") {
    for (const key of ["rows", "items", "data", "results"]) {
      if (Array.isArray(parsed[key])) {
        return parsed[key];
      }
    }
  }
  throw new Error(
    "The data file must contain a JSON array of objects (or an object with a rows/items/data array), JSONL or CSV",
  );
};
