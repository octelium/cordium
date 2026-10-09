import * as fs from "node:fs";
import { defineTool } from "@earendil-works/pi-coding-agent";
import { Type } from "@earendil-works/pi-ai";
import {
  formatResourceURI,
  type ArtifactBlock,
  type ChartBlock,
  type ChartSpec,
  type ResourceItem,
  type ResourcesBlock,
  type TableBlock,
  type TableColumn,
} from "../../protocol/index.ts";
import type { FileStore } from "../../store/files.ts";
import {
  errorResult,
  readRowsFile,
  requireRunContext,
  resolvePath,
  textResult,
  type ToolDeps,
} from "./util.ts";

export const PRESENTATION_TOOL_NAMES = [
  "present_table",
  "present_chart",
  "present_resources",
  "publish_artifact",
];

export const MAX_TABLE_ROWS = 500;
export const MAX_CHART_ROWS = 2000;
export const MAX_ARTIFACT_BYTES = 512 * 1024 * 1024;
export const MAX_SNAPSHOTS = 50;
export const MAX_SNAPSHOT_CHARS = 16 * 1024;

export interface PresentToolDeps extends ToolDeps {
  files: FileStore;
}

const isRecord = (arg: unknown): arg is Record<string, unknown> =>
  typeof arg === "object" && arg !== null && !Array.isArray(arg);

const loadRows = (
  deps: ToolDeps,
  rows: unknown[] | undefined,
  dataPath: string | undefined,
): Record<string, unknown>[] => {
  if (dataPath) {
    return readRowsFile(resolvePath(deps.workDir, dataPath));
  }
  return (rows ?? []) as Record<string, unknown>[];
};

const columnTypes = Type.Enum([
  "string",
  "number",
  "boolean",
  "datetime",
  "json",
]);

const inferColumnType = (
  rows: Record<string, unknown>[],
  key: string,
): TableColumn["type"] => {
  const values = rows
    .slice(0, 50)
    .map((row) => row[key])
    .filter((v) => v !== undefined && v !== null && v !== "");
  if (values.length === 0) {
    return "string";
  }
  if (values.every((v) => typeof v === "number")) {
    return "number";
  }
  if (values.every((v) => typeof v === "boolean")) {
    return "boolean";
  }
  if (values.some((v) => typeof v === "object")) {
    return "json";
  }
  if (
    values.every(
      (v) => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(v),
    )
  ) {
    return "datetime";
  }
  return "string";
};

export const buildTable = (
  rows: Record<string, unknown>[],
  columns?: TableColumn[],
): Pick<TableBlock, "columns" | "rows" | "totalRows" | "truncated"> => {
  if (!rows.every(isRecord)) {
    throw new Error("Every row must be a JSON object");
  }
  const cols: TableColumn[] =
    columns && columns.length > 0
      ? columns
      : [...new Set(rows.slice(0, 100).flatMap((row) => Object.keys(row)))].map(
          (key) => ({ key }),
        );
  if (cols.length === 0) {
    throw new Error("The table has no columns");
  }

  const resolved = cols.map((col) => ({
    ...col,
    type: col.type ?? inferColumnType(rows, col.key),
  }));

  const shown = rows
    .slice(0, MAX_TABLE_ROWS)
    .map((row) =>
      Object.fromEntries(
        resolved.map((col) => [col.key, row[col.key] ?? null]),
      ),
    );

  return {
    columns: resolved,
    rows: shown,
    totalRows: rows.length,
    truncated: rows.length > MAX_TABLE_ROWS || undefined,
  };
};

export const buildChart = (
  spec: Omit<ChartSpec, "rows">,
  rows: Record<string, unknown>[],
): ChartSpec => {
  if (!rows.every(isRecord)) {
    throw new Error("Every row must be a JSON object");
  }
  if (rows.length === 0) {
    throw new Error("The chart has no data rows");
  }
  if (spec.series.length === 0) {
    throw new Error("The chart must have at least one series");
  }

  const keys = new Set(rows.slice(0, 100).flatMap((row) => Object.keys(row)));
  const missing = [spec.x.key, ...spec.series.map((s) => s.key)].filter(
    (key) => !keys.has(key),
  );
  if (missing.length > 0) {
    throw new Error(
      `The rows do not contain the key(s): ${missing.join(", ")}. Available keys: ${[...keys].join(", ")}`,
    );
  }

  const normalized = rows.slice(0, MAX_CHART_ROWS).map((row) => {
    const ret: Record<string, string | number | null> = {};
    const x = row[spec.x.key];
    ret[spec.x.key] =
      typeof x === "number" || typeof x === "string"
        ? x
        : x == null
          ? null
          : String(x);
    for (const s of spec.series) {
      const v = row[s.key];
      const num =
        typeof v === "number" ? v : typeof v === "string" ? Number(v) : NaN;
      ret[s.key] = Number.isFinite(num) ? num : null;
    }
    return ret;
  });

  if (
    spec.series.every((s) => normalized.every((row) => row[s.key] === null))
  ) {
    throw new Error("The series values must be numeric");
  }

  return { ...spec, rows: normalized };
};

export const createPresentationTools = (deps: PresentToolDeps) => {
  const table = defineTool({
    name: "present_table",
    label: "Present table",
    description: `Render a table in the User's chat UI. Prefer it over Markdown tables for more than a few rows or columns. Provide either "rows" (an array of objects) or "dataPath" (a JSON array, JSONL or CSV file). At most ${MAX_TABLE_ROWS} rows are shown.`,
    parameters: Type.Object({
      title: Type.Optional(Type.String()),
      caption: Type.Optional(Type.String()),
      columns: Type.Optional(
        Type.Array(
          Type.Object({
            key: Type.String({
              description: "The key of the value in each row",
            }),
            label: Type.Optional(Type.String()),
            type: Type.Optional(columnTypes),
            unit: Type.Optional(Type.String()),
          }),
          {
            description:
              "The columns to show. Inferred from the rows if omitted.",
          },
        ),
      ),
      rows: Type.Optional(
        Type.Array(Type.Object({}, { additionalProperties: true })),
      ),
      dataPath: Type.Optional(
        Type.String({ description: "Path to a JSON, JSONL or CSV data file" }),
      ),
    }),
    async execute(_toolCallId, params) {
      const ctx = requireRunContext(deps);
      let result;
      try {
        result = buildTable(
          loadRows(deps, params.rows, params.dataPath),
          params.columns as TableColumn[] | undefined,
        );
      } catch (err) {
        return errorResult(
          `Could not render the table: ${(err as Error).message}`,
        );
      }
      const block = ctx.builder.addBlock<TableBlock>({
        type: "table",
        title: params.title,
        caption: params.caption,
        ...result,
      });
      return textResult(
        `Rendered a table (${result.totalRows} rows${result.truncated ? `, ${MAX_TABLE_ROWS} shown` : ""}) as block ${block.id}. Do not repeat its content in your answer.`,
      );
    },
  });

  const chart = defineTool({
    name: "present_chart",
    label: "Present chart",
    description: `Render a chart in the User's chat UI (line, area, bar, pie or scatter). Each row is an object holding the x value under x.key and one numeric value per series key. For a pie chart, x.key is the category and the first series is the value. Provide either "rows" or "dataPath" (a JSON array, JSONL or CSV file). At most ${MAX_CHART_ROWS} rows are used.`,
    parameters: Type.Object({
      chartType: Type.Enum(["line", "area", "bar", "pie", "scatter"]),
      title: Type.Optional(Type.String()),
      description: Type.Optional(Type.String()),
      x: Type.Object({
        key: Type.String(),
        label: Type.Optional(Type.String()),
        type: Type.Optional(Type.Enum(["time", "category", "number"])),
      }),
      y: Type.Optional(
        Type.Object({
          label: Type.Optional(Type.String()),
          unit: Type.Optional(Type.String()),
        }),
      ),
      series: Type.Array(
        Type.Object({
          key: Type.String(),
          label: Type.Optional(Type.String()),
          unit: Type.Optional(Type.String()),
        }),
        { minItems: 1 },
      ),
      stacked: Type.Optional(Type.Boolean()),
      rows: Type.Optional(
        Type.Array(Type.Object({}, { additionalProperties: true })),
      ),
      dataPath: Type.Optional(
        Type.String({ description: "Path to a JSON, JSONL or CSV data file" }),
      ),
    }),
    async execute(_toolCallId, params) {
      const ctx = requireRunContext(deps);
      let spec: ChartSpec;
      try {
        const { rows, dataPath, ...rest } = params;
        spec = buildChart(
          rest as Omit<ChartSpec, "rows">,
          loadRows(deps, rows, dataPath),
        );
      } catch (err) {
        return errorResult(
          `Could not render the chart: ${(err as Error).message}`,
        );
      }
      const block = ctx.builder.addBlock<ChartBlock>({
        type: "chart",
        chart: spec,
      });
      return textResult(
        `Rendered a ${spec.chartType} chart with ${spec.rows.length} data points as block ${block.id}.`,
      );
    },
  });

  const resources = defineTool({
    name: "present_resources",
    label: "Present resources",
    description:
      "Show one or more Cordium/Octelium resources (e.g. Workspaces, Spaces, Templates, WorkspaceSnapshots, Volumes, Services) as interactive items in the User's chat UI that link to their pages in the Cordium web portal. Use the resources' apiVersion (e.g. cordium/v1, user/v1), kind and name exactly as returned by the APIs.",
    parameters: Type.Object({
      title: Type.Optional(Type.String()),
      resources: Type.Array(
        Type.Object({
          apiVersion: Type.String({ description: 'e.g. "cordium/v1"' }),
          kind: Type.String({ description: 'e.g. "Workspace"' }),
          name: Type.String(),
        }),
        { minItems: 1, maxItems: 200 },
      ),
    }),
    async execute(_toolCallId, params) {
      const ctx = requireRunContext(deps);
      const items: ResourceItem[] = params.resources.map((ref, i) => {
        const snapshot = i < MAX_SNAPSHOTS ? ctx.resources.get(ref) : undefined;
        return {
          ref,
          uri: formatResourceURI(ref),
          snapshot:
            snapshot && JSON.stringify(snapshot).length <= MAX_SNAPSHOT_CHARS
              ? snapshot
              : undefined,
        };
      });
      const block = ctx.builder.addBlock<ResourcesBlock>({
        type: "resources",
        title: params.title,
        resources: items,
      });
      return textResult(
        `Rendered ${items.length} resource(s) as block ${block.id}.`,
      );
    },
  });

  const artifact = defineTool({
    name: "publish_artifact",
    label: "Publish artifact",
    description:
      "Publish a file that you created (e.g. a report, a CSV/JSON export, a YAML manifest, an image, an archive) so that the User can preview or download it from the chat UI. Write the file first, then publish it. Never paste large file contents into your answer.",
    parameters: Type.Object({
      path: Type.String({ description: "Path of the file to publish" }),
      title: Type.Optional(Type.String()),
      description: Type.Optional(Type.String()),
      name: Type.Optional(
        Type.String({
          description: "The download file name (defaults to the file's name)",
        }),
      ),
      mimeType: Type.Optional(Type.String()),
    }),
    async execute(_toolCallId, params) {
      const ctx = requireRunContext(deps);
      const filePath = resolvePath(deps.workDir, params.path);
      let size: number;
      try {
        const stat = fs.statSync(filePath);
        if (!stat.isFile()) {
          return errorResult(`Not a regular file: ${filePath}`);
        }
        size = stat.size;
      } catch (err) {
        return errorResult(
          `Could not access ${filePath}: ${(err as Error).message}`,
        );
      }
      if (size > MAX_ARTIFACT_BYTES) {
        return errorResult(
          `The file is too large to be published (${size} bytes, the maximum is ${MAX_ARTIFACT_BYTES})`,
        );
      }

      const info = deps.files.publishArtifact(filePath, {
        name: params.name,
        title: params.title,
        description: params.description,
        mimeType: params.mimeType,
        conversationId: deps.conversationId,
      });
      const block = ctx.builder.addBlock<ArtifactBlock>({
        type: "artifact",
        artifact: info,
      });
      return textResult(
        `Published the artifact "${info.name}" (${info.size} bytes, ${info.mimeType}) as block ${block.id}.`,
      );
    },
  });

  return [table, chart, resources, artifact];
};
