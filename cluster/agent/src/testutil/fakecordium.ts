import * as crypto from "node:crypto";
import * as grpc from "@grpc/grpc-js";
import type { JsonValue } from "@protobuf-ts/runtime";
import { MockError, type MockHandler } from "./mockapi.ts";

type JsonObject = { [key: string]: JsonValue };

interface FakeWorkspace {
  resource: JsonObject;
  steps: string[];
  failure?: string;
}

const startSteps = ["INITIALIZING", "PULLING_IMAGE", "PREPARING", "RUNNING"];

const asObject = (arg: JsonValue | undefined): JsonObject =>
  arg && typeof arg === "object" && !Array.isArray(arg)
    ? (arg as JsonObject)
    : {};

export class FakeCordium {
  readonly user: string;
  readonly domain: string;
  readonly workspaces = new Map<string, FakeWorkspace>();
  private counter = 0;

  constructor(user = "alice", domain = "example.com") {
    this.user = user;
    this.domain = domain;
  }

  add(
    name: string,
    state = "RUNNING",
    extra: { space?: string; template?: string; image?: string } = {},
  ): JsonObject {
    const space = extra.space ?? `default.${this.user}`;
    const resource: JsonObject = {
      apiVersion: "cordium/v1",
      kind: "Workspace",
      metadata: {
        name,
        uid: crypto.randomUUID(),
        createdAt: new Date(Date.now() + this.counter++ * 1000).toISOString(),
      },
      spec: extra.image ? { image: { registry: { url: extra.image } } } : {},
      status: {
        state,
        userRef: { name: this.user },
        spaceRef: { name: space },
        templateRef: { name: extra.template ?? `default.${space}` },
        ...(state === "RUNNING"
          ? {
              hostname: `${name}.cordium.${this.domain}`,
              regionRef: { name: "default" },
            }
          : {}),
      },
    };
    this.workspaces.set(name, { resource, steps: [] });
    return resource;
  }

  private find(ref: JsonValue | undefined): FakeWorkspace {
    const r = asObject(ref);
    for (const ws of this.workspaces.values()) {
      const metadata = asObject(ws.resource.metadata);
      if (
        (r.name && metadata.name === r.name) ||
        (r.uid && metadata.uid === r.uid)
      ) {
        return ws;
      }
    }
    throw new MockError(
      grpc.status.NOT_FOUND,
      `Workspace ${String(r.name ?? r.uid)} does not exist`,
    );
  }

  private advance(ws: FakeWorkspace) {
    const step = ws.steps.shift();
    if (!step) {
      return;
    }
    const status = asObject(ws.resource.status);
    const name = String(asObject(ws.resource.metadata).name);
    if (step === "FAILED") {
      status.state = "STOPPED";
      status.failure = { message: ws.failure ?? "The run failed" };
      delete status.hostname;
      return;
    }
    status.state = step;
    if (step === "RUNNING") {
      status.hostname = `${name}.cordium.${this.domain}`;
      status.regionRef = { name: "default" };
    }
    if (step === "STOPPED") {
      delete status.hostname;
    }
  }

  get(name: string): JsonObject | undefined {
    return this.workspaces.get(name)?.resource;
  }

  handlers(): Record<string, MockHandler> {
    return {
      "cordium.v1.MainService/CreateWorkspace": (req) => {
        const r = asObject(req);
        const name = `w${(this.workspaces.size + 1).toString(36)}x`;
        const status = asObject(r.status);
        const template = String(
          asObject(status.templateRef).name ?? `default.default.${this.user}`,
        );
        const space = template.split(".").slice(1).join(".");
        const resource = this.add(name, "STOPPED", {
          space: space.includes(".") ? space : `${space}.${this.user}`,
          template,
        });
        resource.spec = r.spec ?? {};
        const displayName = asObject(r.metadata).displayName;
        if (displayName !== undefined) {
          asObject(resource.metadata).displayName = displayName;
        }
        return resource;
      },
      "cordium.v1.MainService/GetWorkspace": (req) => {
        const ws = this.find(req);
        this.advance(ws);
        return ws.resource;
      },
      "cordium.v1.MainService/ListWorkspace": () => ({
        apiVersion: "cordium/v1",
        kind: "WorkspaceList",
        items: [...this.workspaces.values()].map((ws) => ws.resource),
        listResponseMeta: {
          totalCount: this.workspaces.size,
          page: 0,
          itemsPerPage: 100,
          hasMore: false,
        },
      }),
      "cordium.v1.MainService/StartWorkspace": (req) => {
        const ws = this.find(asObject(req).workspaceRef);
        const status = asObject(ws.resource.status);
        if (status.state !== "STOPPED") {
          throw new MockError(
            grpc.status.ALREADY_EXISTS,
            "The Workspace is already running",
          );
        }
        delete status.failure;
        status.state = "INIT_REQUEST";
        const image = JSON.stringify(ws.resource.spec ?? {});
        ws.steps = image.includes("fail")
          ? ["INITIALIZING", "PULLING_IMAGE", "FAILED"]
          : [...startSteps];
        ws.failure = image.includes("fail")
          ? "Could not pull the image"
          : undefined;
        return {};
      },
      "cordium.v1.MainService/StopWorkspace": (req) => {
        const ws = this.find(asObject(req).workspaceRef);
        asObject(ws.resource.status).state = "STOPPING_REQUEST";
        ws.steps = ["STOPPING", "STOPPED"];
        return {};
      },
      "cordium.v1.MainService/DeleteWorkspace": (req) => {
        const ws = this.find(req);
        this.workspaces.delete(String(asObject(ws.resource.metadata).name));
        return {};
      },
    };
  }
}
