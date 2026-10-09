export class HTTPError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export const badRequest = (message: string) =>
  new HTTPError(400, "invalid_argument", message);

export const unauthorized = (message = "Unauthorized") =>
  new HTTPError(401, "unauthenticated", message);

export const forbidden = (message = "Forbidden") =>
  new HTTPError(403, "permission_denied", message);

export const notFound = (message = "Not found") =>
  new HTTPError(404, "not_found", message);

export const conflict = (message: string) =>
  new HTTPError(409, "conflict", message);

export const gone = (message: string) => new HTTPError(410, "gone", message);

export const payloadTooLarge = (message: string) =>
  new HTTPError(413, "payload_too_large", message);

export const tooManyRequests = (message: string) =>
  new HTTPError(429, "resource_exhausted", message);
