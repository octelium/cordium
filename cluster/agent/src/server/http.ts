import type { IncomingMessage, ServerResponse } from "node:http";
import { badRequest, HTTPError, payloadTooLarge } from "../errors.ts";
import type { ErrorResponse } from "../protocol/index.ts";

export const sendJSON = (
  res: ServerResponse,
  status: number,
  body: unknown,
) => {
  const data = JSON.stringify(body);
  res.writeHead(status, {
    "content-type": "application/json; charset=utf-8",
    "content-length": Buffer.byteLength(data),
    "cache-control": "no-store",
  });
  res.end(data);
};

export const sendNoContent = (res: ServerResponse) => {
  res.writeHead(204, { "cache-control": "no-store" });
  res.end();
};

export const sendError = (res: ServerResponse, err: unknown) => {
  const httpErr =
    err instanceof HTTPError
      ? err
      : new HTTPError(500, "internal", "Internal server error");
  if (res.headersSent) {
    res.destroy();
    return;
  }
  const body: ErrorResponse = {
    error: { code: httpErr.code, message: httpErr.message },
  };
  sendJSON(res, httpErr.status, body);
};

export const readBody = async (
  req: IncomingMessage,
  maxBytes: number,
): Promise<Buffer> => {
  const contentLength = Number(req.headers["content-length"] ?? "0");
  if (contentLength > maxBytes) {
    throw payloadTooLarge(`The request body exceeds ${maxBytes} bytes`);
  }

  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    size += (chunk as Buffer).length;
    if (size > maxBytes) {
      throw payloadTooLarge(`The request body exceeds ${maxBytes} bytes`);
    }
    chunks.push(chunk as Buffer);
  }
  return Buffer.concat(chunks);
};

export const readJSON = async <T>(
  req: IncomingMessage,
  maxBytes = 1024 * 1024,
): Promise<T> => {
  const body = await readBody(req, maxBytes);
  if (body.length === 0) {
    return {} as T;
  }
  const contentType = (req.headers["content-type"] ?? "").toLowerCase();
  if (!contentType.startsWith("application/json")) {
    throw badRequest("The request body must be application/json");
  }
  try {
    const parsed = JSON.parse(body.toString("utf8"));
    if (
      typeof parsed !== "object" ||
      parsed === null ||
      Array.isArray(parsed)
    ) {
      throw new Error("not an object");
    }
    return parsed as T;
  } catch {
    throw badRequest("The request body must be a JSON object");
  }
};

export const originMatches = (origin: string, pattern: string): boolean => {
  if (pattern === "*") {
    return true;
  }
  if (!pattern.includes("*")) {
    return origin === pattern;
  }
  const idx = pattern.indexOf("*.");
  if (idx < 0) {
    return false;
  }
  const prefix = pattern.slice(0, idx);
  const suffix = pattern.slice(idx + 1);
  return (
    origin.startsWith(prefix) &&
    origin.endsWith(suffix) &&
    origin.length > prefix.length + suffix.length &&
    !origin.slice(prefix.length, origin.length - suffix.length).includes("/")
  );
};

export const requestOrigin = (req: IncomingMessage): string | undefined => {
  const host =
    (req.headers["x-forwarded-host"] as string | undefined)
      ?.split(",")[0]
      ?.trim() ?? req.headers.host;
  if (!host) {
    return undefined;
  }
  const proto =
    (req.headers["x-forwarded-proto"] as string | undefined)
      ?.split(",")[0]
      ?.trim() ?? "http";
  return `${proto}://${host}`;
};

export const contentDisposition = (
  type: "inline" | "attachment",
  name: string,
) => {
  const fallback = name.replace(/[^\x20-\x7e]|["\\]/g, "_");
  return `${type}; filename="${fallback}"; filename*=UTF-8''${encodeURIComponent(name)}`;
};
