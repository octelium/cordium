import * as fs from "node:fs";
import { pathToFileURL } from "node:url";
import { createFileRegistry, fromBinary, ScalarType } from "@bufbuild/protobuf";
import { FileDescriptorSetSchema } from "@bufbuild/protobuf/wkt";

const TYPE_PREFIX = "octelium.api.main.";

export const SERVICES = [
  "octelium.api.main.cordium.v1.MainService",
  "octelium.api.main.cordium.v1.ManagementService",
  "octelium.api.main.user.v1.MainService",
  "octelium.api.main.core.v1.MainService",
];

const getSourcePath = (desc) => {
  switch (desc.kind) {
    case "message":
      return desc.parent
        ? [
            ...getSourcePath(desc.parent),
            3,
            desc.parent.proto.nestedType.indexOf(desc.proto),
          ]
        : [4, desc.file.proto.messageType.indexOf(desc.proto)];
    case "enum":
      return desc.parent
        ? [
            ...getSourcePath(desc.parent),
            4,
            desc.parent.proto.enumType.indexOf(desc.proto),
          ]
        : [5, desc.file.proto.enumType.indexOf(desc.proto)];
    case "enum_value":
      return [
        ...getSourcePath(desc.parent),
        2,
        desc.parent.proto.value.indexOf(desc.proto),
      ];
    case "field":
      return [
        ...getSourcePath(desc.parent),
        2,
        desc.parent.proto.field.indexOf(desc.proto),
      ];
    case "oneof":
      return [
        ...getSourcePath(desc.parent),
        8,
        desc.parent.proto.oneofDecl.indexOf(desc.proto),
      ];
    case "service":
      return [6, desc.file.proto.service.indexOf(desc.proto)];
    case "rpc":
      return [
        ...getSourcePath(desc.parent),
        2,
        desc.parent.proto.method.indexOf(desc.proto),
      ];
  }
  return [];
};

const getDescFile = (desc) => {
  switch (desc.kind) {
    case "message":
    case "enum":
    case "service":
      return desc.file;
    default:
      return getDescFile(desc.parent);
  }
};

export const normalizeComment = (comment) =>
  comment
    .split("\n")
    .map((line) => line.trim())
    .join("\n")
    .replace(/\n{2,}/g, "\n\n")
    .trim();

const createCommenter = () => {
  const cache = new Map();
  return (desc) => {
    const file = getDescFile(desc);
    let comments = cache.get(file.name);
    if (!comments) {
      comments = new Map();
      for (const location of file.proto.sourceCodeInfo?.location ?? []) {
        const text = [location.leadingComments, location.trailingComments]
          .filter((c) => !!c && c.trim() !== "")
          .map(normalizeComment)
          .join("\n");
        if (text) {
          comments.set(location.path.join(","), text);
        }
      }
      cache.set(file.name, comments);
    }
    return comments.get(getSourcePath(desc).join(",")) ?? "";
  };
};

const scalarSchema = (scalar) => {
  switch (scalar) {
    case ScalarType.STRING:
      return { type: "string" };
    case ScalarType.BOOL:
      return { type: "boolean" };
    case ScalarType.BYTES:
      return { type: "string", contentEncoding: "base64" };
    case ScalarType.DOUBLE:
    case ScalarType.FLOAT:
      return { type: "number" };
    case ScalarType.INT32:
    case ScalarType.SINT32:
    case ScalarType.SFIXED32:
      return { type: "integer", format: "int32" };
    case ScalarType.UINT32:
    case ScalarType.FIXED32:
      return { type: "integer", format: "uint32", minimum: 0 };
    case ScalarType.INT64:
    case ScalarType.SINT64:
    case ScalarType.SFIXED64:
      return { type: ["integer", "string"], format: "int64" };
    case ScalarType.UINT64:
    case ScalarType.FIXED64:
      return { type: ["integer", "string"], format: "uint64" };
  }
  return {};
};

export const wellKnownSchema = (typeName) => {
  switch (typeName) {
    case "google.protobuf.Timestamp":
      return { type: "string", format: "date-time" };
    case "google.protobuf.Duration":
      return { type: "string", format: "duration" };
    case "google.protobuf.Struct":
      return { type: "object" };
    case "google.protobuf.Value":
      return {};
    case "google.protobuf.ListValue":
      return { type: "array" };
    case "google.protobuf.Empty":
      return { type: "object", additionalProperties: false };
    case "google.protobuf.FieldMask":
      return { type: "string", format: "field-mask" };
    case "google.protobuf.Any":
      return {
        type: "object",
        properties: { "@type": { type: "string" } },
        required: ["@type"],
      };
    case "google.protobuf.StringValue":
      return { type: ["string", "null"] };
    case "google.protobuf.BoolValue":
      return { type: ["boolean", "null"] };
    case "google.protobuf.Int32Value":
    case "google.protobuf.UInt32Value":
      return { type: ["integer", "null"] };
    case "google.protobuf.DoubleValue":
    case "google.protobuf.FloatValue":
      return { type: ["number", "null"] };
    case "google.protobuf.Int64Value":
    case "google.protobuf.UInt64Value":
      return { type: ["integer", "string", "null"], format: "int64" };
    case "google.protobuf.BytesValue":
      return { type: ["string", "null"], contentEncoding: "base64" };
  }
  return undefined;
};

export const generate = (data) => {
  const registry = createFileRegistry(
    fromBinary(FileDescriptorSetSchema, data),
  );
  const getComment = createCommenter();

  const definitions = {};
  const queue = [];

  const ref = (desc) => {
    const wkt = wellKnownSchema(desc.typeName);
    if (wkt) {
      return wkt;
    }
    if (!(desc.typeName in definitions)) {
      definitions[desc.typeName] = null;
      queue.push(desc);
    }
    return { $ref: `#/definitions/${desc.typeName}` };
  };

  const fieldSchema = (field) => {
    const value = (kind) => {
      switch (kind) {
        case "scalar":
          return scalarSchema(field.scalar);
        case "enum":
          return ref(field.enum);
        case "message":
          return ref(field.message);
      }
      return {};
    };

    switch (field.fieldKind) {
      case "list":
        return { type: "array", items: value(field.listKind) };
      case "map":
        return {
          type: "object",
          additionalProperties: value(field.mapKind),
        };
      default:
        return value(field.fieldKind);
    }
  };

  const withDescription = (schema, description) =>
    description ? { ...schema, description } : schema;

  const messageSchema = (desc) => {
    const properties = {};
    const oneofs = {};
    for (const field of desc.fields) {
      let schema = withDescription(fieldSchema(field), getComment(field));
      if (field.oneof) {
        schema = { ...schema, "x-oneof": field.oneof.name };
        oneofs[field.oneof.name] ??= [];
        oneofs[field.oneof.name].push(field.jsonName);
      }
      properties[field.jsonName] = schema;
    }

    const ret = withDescription(
      { type: "object", properties, additionalProperties: false },
      getComment(desc),
    );
    if (Object.keys(oneofs).length > 0) {
      ret["x-oneofs"] = oneofs;
    }
    return ret;
  };

  const enumSchema = (desc) => {
    const descriptions = {};
    for (const value of desc.values) {
      const comment = getComment(value);
      if (comment) {
        descriptions[value.name] = comment;
      }
    }
    const ret = withDescription(
      { type: "string", enum: desc.values.map((v) => v.name) },
      getComment(desc),
    );
    if (Object.keys(descriptions).length > 0) {
      ret["x-enumDescriptions"] = descriptions;
    }
    return ret;
  };

  const services = {};
  for (const serviceName of SERVICES) {
    const service = registry.getService(serviceName);
    if (!service) {
      throw new Error(`Unknown service: ${serviceName}`);
    }
    const methods = {};
    for (const method of service.methods) {
      ref(method.input);
      ref(method.output);
      methods[method.name] = {
        description: getComment(method),
        input: method.input.typeName,
        output: method.output.typeName,
        kind: method.methodKind,
      };
    }
    services[serviceName] = {
      description: getComment(service),
      methods,
    };
  }

  while (queue.length > 0) {
    const desc = queue.shift();
    definitions[desc.typeName] =
      desc.kind === "enum" ? enumSchema(desc) : messageSchema(desc);
  }

  const sorted = Object.fromEntries(
    Object.entries(definitions)
      .filter(([, schema]) => schema !== null)
      .sort(([a], [b]) => a.localeCompare(b)),
  );

  return {
    $schema: "https://json-schema.org/draft/2020-12/schema",
    typePrefix: TYPE_PREFIX,
    services,
    definitions: sorted,
  };
};

if (
  process.argv[1] &&
  pathToFileURL(process.argv[1]).href === import.meta.url
) {
  const [input, output] = process.argv.slice(2);
  if (!input || !output) {
    console.error("Usage: gen-apis.mjs <descriptor set> <output JSON>");
    process.exit(1);
  }
  const ret = generate(fs.readFileSync(input));
  fs.writeFileSync(output, `${JSON.stringify(ret)}\n`);
  console.log(
    `Generated ${output}: ${Object.keys(ret.services).length} services, ${Object.keys(ret.definitions).length} definitions`,
  );
}
