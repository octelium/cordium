import { spawn, type ChildProcess } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import * as grpc from "@grpc/grpc-js";
import {
  ExecRequest,
  ExecResponse,
  WorkspaceService,
  type ExecRequest_Request,
} from "@octelium/apis/main/cordiumv1";
import type { JsonValue } from "@protobuf-ts/runtime";
import type { MethodInfo, ServiceType } from "@protobuf-ts/runtime-rpc";
import { SERVICES, shortTypeName } from "../api/catalog.ts";

export type MockHandler = (
  request: JsonValue,
  metadata: grpc.Metadata,
) => JsonValue | JsonValue[] | Promise<JsonValue | JsonValue[]>;

export class MockError extends Error {
  readonly code: grpc.status;

  constructor(code: grpc.status, message: string) {
    super(message);
    this.code = code;
  }
}

export interface MockCall {
  method: string;
  request: JsonValue;
  metadata: grpc.Metadata;
}

export interface ExecIO {
  stdout(data: Uint8Array): void;
  stderr(data: Uint8Array): void;
}

export interface MockAPIOptions {
  onExec?: (request: ExecRequest_Request) => void | Promise<void>;
}

export interface MockAPI {
  socketPath: string;
  calls: MockCall[];
  execs: ExecRequest_Request[];
  close(): Promise<void>;
}

const methodPath = (service: ServiceType, method: MethodInfo) =>
  `/${service.typeName}/${method.name}`;

const definition = (
  service: ServiceType,
  method: MethodInfo,
): grpc.MethodDefinition<object, object> => ({
  path: methodPath(service, method),
  requestStream: !!method.clientStreaming,
  responseStream: !!method.serverStreaming,
  requestSerialize: (value) => Buffer.from(method.I.toBinary(value)),
  requestDeserialize: (value) => method.I.fromBinary(value) as object,
  responseSerialize: (value) => Buffer.from(method.O.toBinary(value)),
  responseDeserialize: (value) => method.O.fromBinary(value) as object,
});

const toServiceError = (err: unknown): Partial<grpc.ServiceError> =>
  err instanceof MockError
    ? { code: err.code, details: err.message }
    : { code: grpc.status.INTERNAL, details: (err as Error).message };

const execHandler =
  (execs: ExecRequest_Request[], opts: MockAPIOptions) =>
  (call: grpc.ServerDuplexStream<ExecRequest, ExecResponse>) => {
    let child: ChildProcess | undefined;
    let done = false;

    const kill = () => {
      if (child?.pid && child.exitCode === null) {
        try {
          process.kill(-child.pid, "SIGKILL");
        } catch {
          child.kill("SIGKILL");
        }
      }
    };

    const write = (response: ExecResponse) => {
      if (!done) {
        call.write(response);
      }
    };

    call.on("data", (req: ExecRequest) => {
      const type = req.type;
      switch (type.oneofKind) {
        case "request": {
          execs.push(type.request);
          void Promise.resolve(opts.onExec?.(type.request))
            .then(() => {
              child = spawn("sh", ["-c", type.request.command], {
                cwd: type.request.workingDir || os.tmpdir(),
                env: {
                  ...process.env,
                  ...Object.fromEntries(
                    type.request.envVars.map((e) => [e.key, e.value]),
                  ),
                },
                detached: true,
                stdio: [
                  type.request.hasStdin ? "pipe" : "ignore",
                  "pipe",
                  "pipe",
                ],
              });
              child.stdout?.on("data", (data: Buffer) =>
                write(
                  ExecResponse.create({
                    type: { oneofKind: "stdout", stdout: { data } },
                  }),
                ),
              );
              child.stderr?.on("data", (data: Buffer) =>
                write(
                  ExecResponse.create({
                    type: { oneofKind: "stderr", stderr: { data } },
                  }),
                ),
              );
              child.on("close", (code) => {
                write(
                  ExecResponse.create({
                    type: { oneofKind: "exit", exit: { code: code ?? -1 } },
                  }),
                );
                done = true;
                call.end();
              });
            })
            .catch((err: Error) => {
              done = true;
              call.destroy(
                Object.assign(new Error(err.message), toServiceError(err)),
              );
            });
          break;
        }
        case "writeData":
          child?.stdin?.write(type.writeData.data);
          break;
        case "kill":
          kill();
          break;
      }
    });
    call.on("cancelled", kill);
    call.on("error", kill);
  };

export const startMockAPI = async (
  handlers: Record<string, MockHandler>,
  opts: MockAPIOptions = {},
): Promise<MockAPI> => {
  const calls: MockCall[] = [];
  const execs: ExecRequest_Request[] = [];
  const server = new grpc.Server();
  const services = [...SERVICES, WorkspaceService];
  const known = new Set<string>();

  for (const service of services) {
    const def: Record<string, grpc.MethodDefinition<object, object>> = {};
    const impl: grpc.UntypedServiceImplementation = {};

    for (const method of service.methods) {
      const id = `${shortTypeName(service.typeName)}/${method.name}`;
      known.add(id);
      if (service === WorkspaceService && method.name === "Exec") {
        def[method.localName] = definition(service, method);
        impl[method.localName] = execHandler(execs, opts);
        continue;
      }
      const fn = handlers[id];
      if (!fn) {
        continue;
      }
      def[method.localName] = definition(service, method);
      if (!method.serverStreaming && !method.clientStreaming) {
        impl[method.localName] = (
          call: grpc.ServerUnaryCall<object, object>,
          callback: grpc.sendUnaryData<object>,
        ) => {
          const request = method.I.toJson(call.request as never);
          calls.push({ method: id, request, metadata: call.metadata });
          Promise.resolve()
            .then(() => fn(request, call.metadata))
            .then((res) =>
              callback(null, method.O.fromJson(res as JsonValue) as object),
            )
            .catch((err) => callback(toServiceError(err), null));
        };
      } else if (method.serverStreaming && !method.clientStreaming) {
        impl[method.localName] = (
          call: grpc.ServerWritableStream<object, object>,
        ) => {
          const request = method.I.toJson(call.request as never);
          calls.push({ method: id, request, metadata: call.metadata });
          Promise.resolve()
            .then(() => fn(request, call.metadata))
            .then((res) => {
              for (const item of res as JsonValue[]) {
                call.write(method.O.fromJson(item) as object);
              }
              call.end();
            })
            .catch((err) =>
              call.destroy(
                Object.assign(new Error(err.message), toServiceError(err)),
              ),
            );
        };
      }
    }

    if (Object.keys(def).length > 0) {
      server.addService(def, impl);
    }
  }

  for (const id of Object.keys(handlers)) {
    if (!known.has(id)) {
      throw new Error(`Unknown method: ${id}`);
    }
  }

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cordium-mockapi-"));
  const socketPath = path.join(dir, "api.sock");
  await new Promise<void>((resolve, reject) =>
    server.bindAsync(
      `unix:${socketPath}`,
      grpc.ServerCredentials.createInsecure(),
      (err) => (err ? reject(err) : resolve()),
    ),
  );

  return {
    socketPath,
    calls,
    execs,
    close: async () => {
      server.forceShutdown();
      fs.rmSync(dir, { recursive: true, force: true });
    },
  };
};
