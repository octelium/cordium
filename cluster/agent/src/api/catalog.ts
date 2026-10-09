import * as fs from "node:fs";
import {
  MainService as CordiumMainService,
  ManagementService as CordiumManagementService,
} from "@octelium/apis/main/cordiumv1";
import { MainService as CoreMainService } from "@octelium/apis/main/corev1";
import { MainService as UserMainService } from "@octelium/apis/main/userv1";
import type { MethodInfo, ServiceType } from "@protobuf-ts/runtime-rpc";
import type { APIRisk } from "../protocol/index.ts";
import { SearchIndex, splitCamelCase, type SearchHit } from "./search.ts";

export const TYPE_PREFIX = "octelium.api.main.";

export const DEFAULT_SCHEMA_PATH = new URL(
  "../../apis/cordium.json",
  import.meta.url,
);

export const SERVICES: ServiceType[] = [
  CordiumMainService,
  CordiumManagementService,
  UserMainService,
  CoreMainService,
];

export const DEFAULT_INDEX_PACKAGES = ["cordium.v1", "user.v1"];

const packageDescriptions: Record<string, string> = {
  "cordium.v1":
    "Cordium: the User's Spaces, Templates, Workspaces, WorkspaceSnapshots, Volumes, Memberships, Secrets, UserSecrets, GitProviders, UserConfig and Regions. The ManagementService (the Cordium ClusterConfig) requires administrator permissions",
  "user.v1":
    "The current User's own view of the Octelium Cluster: their status and the Services and Namespaces that they can access",
  "core.v1":
    "Octelium Cluster administration (Services, Namespaces, Users, Groups, Policies, Sessions, Devices, Credentials, Secrets, IdentityProviders, the ClusterConfig). It usually requires administrator permissions",
};

const packagePriors: Record<string, number> = {
  "cordium.v1": 1,
  "user.v1": 0.97,
  "core.v1": 0.9,
};

const excludedMethods = new Set([
  "octelium.api.main.cordium.v1.MainService/InitializeAgent",
  "octelium.api.main.user.v1.MainService/Connect",
  "octelium.api.main.user.v1.MainService/Disconnect",
  "octelium.api.main.user.v1.MainService/SetServiceConfigs",
]);

const readVerbs = [
  "Get",
  "List",
  "Watch",
  "Listen",
  "Search",
  "Query",
  "Describe",
  "Count",
  "Check",
  "Validate",
  "Evaluate",
  "Is",
  "Has",
];

const destructiveVerbs = [
  "Delete",
  "Remove",
  "Leave",
  "Revoke",
  "Purge",
  "Reset",
  "Terminate",
  "Kill",
];

const knownVerbs = [
  ...readVerbs,
  ...destructiveVerbs,
  "Create",
  "Update",
  "Set",
  "Start",
  "Stop",
  "Restart",
  "Build",
  "CancelBuild",
  "Cancel",
  "Share",
  "Unshare",
  "Generate",
  "Upgrade",
  "Join",
  "Approve",
  "Reject",
  "Review",
  "Initialize",
];

export type MethodKind =
  "unary" | "server_streaming" | "client_streaming" | "bidi_streaming";

export interface APIMethod {
  id: string;
  grpcMethod: string;
  packageName: string;
  serviceName: string;
  name: string;
  verb?: string;
  noun?: string;
  kind: MethodKind;
  risk: APIRisk;
  invocable: boolean;
  description: string;
  requestType: string;
  responseType: string;
  info: MethodInfo;
}

export type JSONSchema = {
  $ref?: string;
  type?: string | string[];
  format?: string;
  contentEncoding?: string;
  description?: string;
  properties?: Record<string, JSONSchema>;
  additionalProperties?: boolean | JSONSchema;
  items?: JSONSchema;
  enum?: string[];
  "x-oneof"?: string;
  "x-oneofs"?: Record<string, string[]>;
  "x-enumDescriptions"?: Record<string, string>;
};

export interface APISchemas {
  services: Record<
    string,
    {
      description?: string;
      methods: Record<
        string,
        { description?: string; input: string; output: string; kind: string }
      >;
    }
  >;
  definitions: Record<string, JSONSchema>;
}

export const shortTypeName = (typeName: string): string =>
  typeName.startsWith(TYPE_PREFIX)
    ? typeName.slice(TYPE_PREFIX.length)
    : typeName;

const getPackageName = (serviceTypeName: string): string => {
  const short = shortTypeName(serviceTypeName);
  return short.slice(0, short.lastIndexOf("."));
};

const getVerb = (name: string): { verb?: string; noun?: string } => {
  const verb = knownVerbs
    .filter(
      (v) =>
        name.startsWith(v) &&
        (name.length === v.length || /[A-Z]/.test(name[v.length])),
    )
    .sort((a, b) => b.length - a.length)[0];
  if (!verb) {
    return {};
  }
  return { verb, noun: name.slice(verb.length) || undefined };
};

export const classifyRisk = (methodName: string): APIRisk => {
  const { verb } = getVerb(methodName);
  if (verb && readVerbs.includes(verb)) {
    return "read";
  }
  if (verb && destructiveVerbs.includes(verb)) {
    return "destructive";
  }
  if (
    /Token|Password|Secret|Credential|Membership|ClusterConfig|Share[A-Z]/.test(
      methodName,
    )
  ) {
    return "sensitive";
  }
  return "write";
};

const getMethodKind = (info: MethodInfo): MethodKind => {
  if (info.serverStreaming && info.clientStreaming) {
    return "bidi_streaming";
  }
  if (info.serverStreaming) {
    return "server_streaming";
  }
  if (info.clientStreaming) {
    return "client_streaming";
  }
  return "unary";
};

export const firstSentences = (text: string, max = 200): string => {
  const flat = text.replace(/\s+/g, " ").trim();
  if (flat.length <= max) {
    return flat;
  }
  const cut = flat.slice(0, max);
  let end = -1;
  for (const match of cut.matchAll(/\. /g)) {
    const before = cut.slice(0, match.index);
    if (!/(\b(i\.e|e\.g|etc|vs)|\.\.)$/.test(before)) {
      end = match.index;
    }
  }
  if (end > max / 3) {
    return cut.slice(0, end + 1);
  }
  return `${cut.trimEnd()}…`;
};

const refName = (ref: string): string => ref.replace(/^#\/definitions\//, "");

export interface DescribeOptions {
  maxChars?: number;
  maxDepth?: number;
  verbose?: boolean;
}

export class APICatalog {
  readonly schemas: APISchemas;
  private methodsById = new Map<string, APIMethod>();
  private methodsByLowerId = new Map<string, APIMethod>();
  private index = new SearchIndex<APIMethod>();
  private services: ServiceType[];

  constructor(schemas: APISchemas, services: ServiceType[] = SERVICES) {
    this.schemas = schemas;
    this.services = services;
    this.build();
  }

  static fromJSON(data: string): APICatalog {
    return new APICatalog(JSON.parse(data) as APISchemas);
  }

  static load(filePath: string | URL = DEFAULT_SCHEMA_PATH): APICatalog {
    return APICatalog.fromJSON(fs.readFileSync(filePath, "utf8"));
  }

  private build() {
    for (const service of this.services) {
      const packageName = getPackageName(service.typeName);
      const serviceSchema = this.schemas.services[service.typeName];
      const serviceDescription = serviceSchema?.description ?? "";
      const serviceShortName = service.typeName.slice(
        service.typeName.lastIndexOf(".") + 1,
      );

      for (const info of service.methods) {
        if (excludedMethods.has(`${service.typeName}/${info.name}`)) {
          continue;
        }

        const { verb, noun } = getVerb(info.name);
        const kind = getMethodKind(info);
        const description =
          serviceSchema?.methods[info.name]?.description ?? "";
        const entry: APIMethod = {
          id: `${shortTypeName(service.typeName)}/${info.name}`,
          grpcMethod: `/${service.typeName}/${info.name}`,
          packageName,
          serviceName: service.typeName,
          name: info.name,
          verb,
          noun,
          kind,
          risk: classifyRisk(info.name),
          invocable: kind === "unary" || kind === "server_streaming",
          description,
          requestType: shortTypeName(info.I.typeName),
          responseType: shortTypeName(info.O.typeName),
          info,
        };

        this.methodsById.set(entry.id, entry);
        this.methodsByLowerId.set(entry.id.toLowerCase(), entry);

        this.index.add(
          entry,
          [
            { text: info.name, weight: 3, primary: true },
            { text: noun ?? "", weight: 2 },
            { text: `${packageName} ${serviceShortName}`, weight: 1.5 },
            { text: description, weight: 1 },
            {
              text: `${entry.requestType} ${entry.responseType}`,
              weight: 1,
            },
            {
              text: firstSentences(
                this.schemas.definitions[info.O.typeName]?.description ?? "",
                160,
              ),
              weight: 0.5,
            },
            { text: packageDescriptions[packageName] ?? "", weight: 0.3 },
            { text: firstSentences(serviceDescription, 200), weight: 0.3 },
          ],
          packagePriors[packageName] ?? 0.9,
        );
      }
    }
  }

  get methods(): APIMethod[] {
    return [...this.methodsById.values()];
  }

  resolveMethod(input: string): APIMethod | undefined {
    let id = input.trim();
    if (id.startsWith("/")) {
      id = id.slice(1);
    }
    if (id.startsWith(TYPE_PREFIX)) {
      id = id.slice(TYPE_PREFIX.length);
    }
    if (!id.includes("/")) {
      const idx = id.lastIndexOf(".");
      if (idx > 0) {
        id = `${id.slice(0, idx)}/${id.slice(idx + 1)}`;
      }
    }
    return (
      this.methodsById.get(id) ?? this.methodsByLowerId.get(id.toLowerCase())
    );
  }

  resolveType(input: string): string | undefined {
    const name = input.trim().replace(/^\./, "");
    for (const candidate of [name, `${TYPE_PREFIX}${name}`]) {
      if (candidate in this.schemas.definitions) {
        return candidate;
      }
    }
    return undefined;
  }

  search(query: string, limit = 10): SearchHit<APIMethod>[] {
    return this.index.search(query, limit);
  }

  formatSearchResults(hits: SearchHit<APIMethod>[]): string {
    if (hits.length === 0) {
      return "No matching methods found. Try other keywords (e.g. the resource kind such as Workspace, Space, Template, Service, or an action such as list, create, delete).";
    }

    return hits
      .map((hit, i) => {
        const m = hit.item;
        const description = m.description
          ? ` — ${firstSentences(m.description, 160)}`
          : "";
        const flags = [m.risk, m.kind !== "unary" ? m.kind : undefined]
          .filter(Boolean)
          .join(", ");
        const invocable = m.invocable ? "" : " (not invocable)";
        return `${i + 1}. ${m.id}${description} [${flags}]${invocable}\n   request: ${m.requestType} → response: ${m.responseType}`;
      })
      .join("\n");
  }

  formatIndex(packages: string[] = DEFAULT_INDEX_PACKAGES): string {
    const lines: string[] = [];
    const omitted: string[] = [];
    const byPackage = new Map<string, ServiceType[]>();
    for (const service of this.services) {
      const pkg = getPackageName(service.typeName);
      byPackage.set(pkg, [...(byPackage.get(pkg) ?? []), service]);
    }

    for (const [pkg, services] of byPackage) {
      const description = packageDescriptions[pkg];
      if (!packages.includes(pkg)) {
        omitted.push(`${pkg}${description ? ` — ${description}` : ""}`);
        continue;
      }
      lines.push(`${pkg}${description ? ` — ${description}` : ""}`);
      for (const service of services) {
        const methods = this.methods.filter(
          (m) => m.serviceName === service.typeName && m.invocable,
        );
        if (methods.length === 0) {
          continue;
        }
        const groups = new Map<string, string[]>();
        const singles: string[] = [];
        for (const m of methods) {
          if (m.noun && m.verb) {
            groups.set(m.noun, [...(groups.get(m.noun) ?? []), m.verb]);
          } else {
            singles.push(m.name);
          }
        }
        const parts = [...groups.entries()].map(([noun, verbs]) =>
          verbs.length === 1
            ? `${verbs[0]}${noun}`
            : `${noun}[${verbs.join("|")}]`,
        );
        const name = service.typeName.slice(
          service.typeName.lastIndexOf(".") + 1,
        );
        lines.push(`  ${name}: ${[...parts, ...singles].join(", ")}`);
      }
    }

    for (const line of omitted) {
      lines.push(`${line} (not listed here, use cordium_api_search)`);
    }
    return lines.join("\n");
  }

  describeMethod(method: APIMethod, opts: DescribeOptions = {}): string {
    const lines = [
      `Method: ${method.id}`,
      `gRPC method: ${method.grpcMethod}`,
      `Kind: ${method.kind}${method.invocable ? "" : " (not invocable by this agent)"}`,
      `Risk: ${method.risk}`,
    ];
    if (method.description) {
      lines.push(`Description: ${method.description.replace(/\n+/g, " ")}`);
    }
    lines.push(
      `Request type: ${method.requestType}`,
      `Response type: ${method.responseType}`,
      "",
      "Request schema (proto3 JSON mapping, every field is optional unless its description says otherwise):",
      this.renderTypes(method.info.I.typeName, opts),
      "",
      "Response summary:",
      this.renderTypes(method.info.O.typeName, {
        maxChars: 2500,
        maxDepth: 0,
        verbose: false,
      }),
    );
    return lines.join("\n");
  }

  describeType(typeName: string, opts: DescribeOptions = {}): string {
    const schema = this.schemas.definitions[typeName];
    if (schema?.enum) {
      return this.renderEnumBlock(typeName, schema);
    }
    return [
      "Schema (proto3 JSON mapping, every field is optional unless its description says otherwise):",
      this.renderTypes(typeName, opts),
    ].join("\n");
  }

  private renderEnumBlock(typeName: string, schema: JSONSchema): string {
    const lines = [`enum ${shortTypeName(typeName)} {`];
    if (schema.description) {
      lines.unshift(`// ${firstSentences(schema.description, 300)}`);
    }
    for (const value of schema.enum ?? []) {
      const c = schema["x-enumDescriptions"]?.[value];
      lines.push(`  "${value}"${c ? `  // ${firstSentences(c, 160)}` : ""}`);
    }
    lines.push("}");
    return lines.join("\n");
  }

  private renderTypes(root: string, opts: DescribeOptions): string {
    const maxChars = opts.maxChars ?? 14000;
    const maxDepth = opts.maxDepth ?? 3;
    const verbose = opts.verbose ?? false;

    const blocks: string[] = [];
    const seen = new Set<string>([root]);
    const queue: { name: string; depth: number }[] = [{ name: root, depth: 0 }];
    const skipped: string[] = [];
    let used = 0;

    while (queue.length > 0) {
      const { name, depth } = queue.shift()!;
      const schema = this.schemas.definitions[name];
      if (!schema) {
        continue;
      }
      const nested: string[] = [];
      const block = this.renderMessageBlock(
        name,
        schema,
        depth === 0 || verbose,
        nested,
      );
      if (used + block.length > maxChars && blocks.length > 0) {
        skipped.push(shortTypeName(name));
        continue;
      }
      blocks.push(block);
      used += block.length;

      for (const child of nested) {
        if (seen.has(child)) {
          continue;
        }
        seen.add(child);
        if (depth + 1 > maxDepth) {
          skipped.push(shortTypeName(child));
          continue;
        }
        queue.push({ name: child, depth: depth + 1 });
      }
    }

    if (skipped.length > 0) {
      blocks.push(
        `// Not expanded (use cordium_api_describe with {"type": "<name>"} to see them): ${skipped.join(", ")}`,
      );
    }
    return blocks.join("\n\n");
  }

  private renderMessageBlock(
    name: string,
    schema: JSONSchema,
    verbose: boolean,
    nested: string[],
  ): string {
    const lines: string[] = [];
    if (schema.description) {
      lines.push(
        `// ${firstSentences(schema.description, verbose ? 600 : 240)}`,
      );
    }

    lines.push(`${shortTypeName(name)} {`);
    const renderField = (key: string, field: JSONSchema, indent: string) => {
      const type = this.renderFieldType(field, nested);
      const c = field.description
        ? `  // ${firstSentences(field.description, verbose ? 400 : 200)}`
        : "";
      lines.push(`${indent}${key}: ${type}${c}`);
    };

    const properties = schema.properties ?? {};
    const oneofs = schema["x-oneofs"] ?? {};
    const rendered = new Set<string>();
    for (const [key, field] of Object.entries(properties)) {
      const oneof = field["x-oneof"];
      if (!oneof) {
        renderField(key, field, "  ");
        continue;
      }
      if (rendered.has(oneof)) {
        continue;
      }
      rendered.add(oneof);
      lines.push(`  oneof ${oneof} (set at most one) {`);
      for (const member of oneofs[oneof] ?? [key]) {
        renderField(member, properties[member], "    ");
      }
      lines.push("  }");
    }
    lines.push("}");
    return lines.join("\n");
  }

  private renderFieldType(field: JSONSchema, nested: string[]): string {
    if (field.$ref) {
      const name = refName(field.$ref);
      const def = this.schemas.definitions[name];
      if (def?.enum) {
        const values = def.enum.map((v) => `"${v}"`);
        const shown =
          values.length > 16
            ? `${values.slice(0, 16).join(" | ")} | …`
            : values.join(" | ");
        return `enum ${shortTypeName(name)} (${shown})`;
      }
      nested.push(name);
      return shortTypeName(name);
    }

    const types = Array.isArray(field.type)
      ? field.type
      : field.type
        ? [field.type]
        : [];
    const nullable = types.includes("null") ? " | null" : "";

    if (types.includes("array")) {
      return field.items
        ? `${this.renderFieldType(field.items, nested)}[]`
        : "array (arbitrary JSON values)";
    }
    if (types.includes("object")) {
      if (field.properties?.["@type"]) {
        return 'object with "@type" (type URL) and the fields of that type';
      }
      if (field.additionalProperties && field.additionalProperties !== true) {
        return `map<string, ${this.renderFieldType(field.additionalProperties, nested)}>`;
      }
      if (field.additionalProperties === false) {
        return "{}";
      }
      return "object (arbitrary JSON object)";
    }
    switch (field.format) {
      case "date-time":
        return 'timestamp (RFC 3339 string, e.g. "2026-01-02T15:04:05Z")';
      case "duration":
        return 'duration (string with "s" suffix, e.g. "3.5s")';
      case "field-mask":
        return 'string (comma-separated field paths, e.g. "spec.port,metadata.labels")';
      case "int64":
      case "uint64":
        return `${field.format} (number or decimal string)${nullable}`;
      case "int32":
      case "uint32":
        return `${field.format}${nullable}`;
    }
    if (field.contentEncoding === "base64") {
      return `bytes (base64 string)${nullable}`;
    }
    if (types.includes("string")) {
      return `string${nullable}`;
    }
    if (types.includes("boolean")) {
      return `bool${nullable}`;
    }
    if (types.includes("integer")) {
      return `integer${nullable}`;
    }
    if (types.includes("number")) {
      return `number${nullable}`;
    }
    return "any JSON value";
  }
}

export const methodTitle = (method: APIMethod): string =>
  `${splitCamelCase(method.name)} (${method.packageName})`;
