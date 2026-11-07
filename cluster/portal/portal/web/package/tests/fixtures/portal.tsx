import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import "@xterm/xterm/css/xterm.css";
import "@/index.css";
import React from "react";
import ReactDOM from "react-dom/client";
import { MantineProvider } from "@mantine/core";
import { QueryClientProvider } from "@tanstack/react-query";
import { Provider } from "react-redux";
import {
  BrowserRouter,
  createBrowserRouter,
  Route,
  RouterProvider,
  Routes,
} from "react-router-dom";
import * as UserPB from "@octelium/apis/main/userv1";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { Terminal as XTerm } from "@xterm/xterm";
import { WebglAddon } from "@xterm/addon-webgl";
import theme, { cssVariablesResolver } from "@/utils/theme/mantine";
import { queryClient } from "@/utils";
import { buildConnectPlan } from "@/components/ServiceConnect/plan";
import { buildSetupScript } from "@/components/ServiceConnect/setup";

interface TerminalRecord {
  terminal: XTerm;
  disposed: boolean;
  refreshes: number;
  webglDisposed: number;
}

declare global {
  interface Window {
    portalFixture: {
      sockets: FakeSocket[];
      terminals: TerminalRecord[];
      calls: { method: string; input: unknown }[];
      failServices: boolean;
      failRemove: boolean;
      failTerminals: boolean;
      serviceTotal: number | null;
      missingTerminalIDs: string[];
      logAborts: number;
      refreshTerminals: () => Promise<void>;
      refreshAgent: () => Promise<unknown>;
      states: typeof WsPB.Workspace_Status_State;
      agent: {
        uid: string;
        configured: boolean;
        missing: boolean;
        state: WsPB.Workspace_Status_State;
        failure: string;
        startError: string;
      };
      connect: {
        scripts: string[];
        inputs: string[];
        sizes: { id: string; cols: number; rows: number }[];
        holdSetup: boolean;
        failTool: boolean;
        failCreate: boolean;
        main: () => string;
        output: (text: string) => void;
        closeMain: () => void;
      };
      plan: (
        spec: Partial<UserPB.Service_Spec>,
        name: string,
        displayName?: string,
      ) => {
        tool?: string;
        kubeconfig?: string;
        hint?: string;
        inferred?: string;
        inputs: { text: string; run?: boolean }[];
        script: string;
      };
    };
  }
}

window.__OCTELIUM_GLOBALS__ = { cluster: { domain: "example.test" } };
window.portalFixture = {
  sockets: [],
  terminals: [],
  calls: [],
  failServices: false,
  failRemove: false,
  failTerminals: false,
  serviceTotal: null,
  missingTerminalIDs: [],
  logAborts: 0,
  refreshTerminals: () =>
    queryClient.invalidateQueries({ queryKey: ["workspace/ws/listTerminal"] }),
  refreshAgent: () => queryClient.invalidateQueries({ queryKey: ["agent"] }),
  states: WsPB.Workspace_Status_State,
  agent: {
    uid: "workspace-agent",
    configured: true,
    missing: false,
    state: WsPB.Workspace_Status_State.RUNNING,
    failure: "",
    startError: "",
  },
  connect: {
    scripts: [],
    inputs: [],
    sizes: [],
    holdSetup: false,
    failTool: false,
    failCreate: false,
    main: () => mainTerminal(),
    output: (text) => emitOutput(mainTerminal(), text),
    closeMain: () => {
      const id = mainTerminal();
      emitTerminal(id, { oneofKind: "close", close: {} });
      closeStream(id);
    },
  },
  plan: (spec, name, displayName) => {
    const plan = buildConnectPlan(
      UserPB.Service.create({
        metadata: { name, displayName },
        spec,
        status: { namespace: name.split(".")[1] },
      }),
      "example.test",
    );
    return {
      tool: plan.tool,
      kubeconfig: plan.kubeconfig,
      hint: plan.hint?.text,
      inferred: plan.inferred,
      inputs: plan.commands({ kubeconfig: "/home/alex/.kube/config" }),
      script: buildSetupScript(plan, "example.test", "0123456789ab"),
    };
  },
};
const fixture = window.portalFixture;
const encoder = new TextEncoder();
const decoder = new TextDecoder();
const route = new URLSearchParams(location.search).get("route") ?? "/services";
const colorScheme =
  new URLSearchParams(location.search).get("theme") === "dark"
    ? "dark"
    : "light";
const fullRouter = new URLSearchParams(location.search).has("router");
history.replaceState(null, "", route);

const terminalIDs = new Set([
  "terminal-main",
  "terminal-worker",
  "terminal-logs",
]);
const terminalTitle = (id: string) => id.replace("terminal-", "");
const terminalHistory = (id: string) =>
  `\x1b]0;${terminalTitle(id)}\x07Welcome to ${terminalTitle(id)}\r\n$ `;

class FakeSocket extends EventTarget {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  readyState = 0;
  binaryType = "arraybuffer";
  bufferedAmount = 0;
  subscriptions = new Set<string>();
  sent: WsPB.ClientMessage[] = [];

  constructor(public url: string) {
    super();
    fixture.sockets.push(this);
    window.setTimeout(() => {
      if (this.readyState !== 0) return;
      this.readyState = 1;
      this.dispatchEvent(new Event("open"));
    }, 300);
  }

  send(data: Uint8Array) {
    const message = WsPB.ClientMessage.fromBinary(data);
    this.sent.push(message);
    if (message.type.oneofKind === "listenTerminalRequest") {
      const id = message.type.listenTerminalRequest.id;
      this.subscriptions.add(id);
      window.setTimeout(() => {
        if (this.readyState !== 1 || !this.subscriptions.has(id)) return;
        this.receive(id, {
          oneofKind: "windowSize",
          windowSize: { rows: 24, cols: 80 },
        });
        this.receive(id, {
          oneofKind: "stdout",
          stdout: { data: encoder.encode(terminalHistory(id)) },
        });
      }, 10);
    } else if (message.type.oneofKind === "listenTerminalEndRequest") {
      this.subscriptions.delete(message.type.listenTerminalEndRequest.id);
    } else if (message.type.oneofKind === "writeTerminalDataRequest") {
      const { id, data } = message.type.writeTerminalDataRequest;
      this.receive(id, { oneofKind: "stdout", stdout: { data } });
    }
  }

  receive(id: string, type: WsPB.ListenTerminalResponse["type"]) {
    if (!this.subscriptions.has(id) || this.readyState !== 1) return;
    const payload = WsPB.ServerMessage.toBinary(
      WsPB.ServerMessage.create({
        type: {
          oneofKind: "listenTerminalEvent",
          listenTerminalEvent: { id, listenTerminalResponse: { type } },
        },
      }),
    );
    this.dispatchEvent(new MessageEvent("message", { data: payload.buffer }));
  }

  close() {
    if (this.readyState === 3) return;
    this.readyState = 3;
    this.dispatchEvent(new CloseEvent("close", { code: 1006 }));
  }
}
window.WebSocket = FakeSocket as unknown as typeof WebSocket;

const originalOpen = XTerm.prototype.open;
XTerm.prototype.open = function (parent) {
  const record = {
    terminal: this,
    disposed: false,
    refreshes: 0,
    webglDisposed: 0,
  };
  fixture.terminals.push(record);
  const originalRefresh = this.refresh.bind(this);
  this.refresh = (start, end) => {
    record.refreshes++;
    originalRefresh(start, end);
  };
  const originalDispose = this.dispose.bind(this);
  this.dispose = () => {
    record.disposed = true;
    originalDispose();
  };
  originalOpen.call(this, parent);
};
const originalLoadAddon = XTerm.prototype.loadAddon;
XTerm.prototype.loadAddon = function (addon) {
  originalLoadAddon.call(this, addon);
  if (!(addon instanceof WebglAddon)) return;
  const record = fixture.terminals.find((item) => item.terminal === this)!;
  const originalDispose = addon.dispose.bind(addon);
  addon.dispose = () => {
    record.webglDisposed++;
    originalDispose();
  };
};

const services = [
  ...Array.from({ length: 25 }, (_, index) =>
    UserPB.Service.create({
      metadata: {
        uid: `service-${index}`,
        name: `service-${String(index).padStart(2, "0")}.default`,
      },
      spec: { type: UserPB.Service_Spec_Type.TCP, port: 4000 + index },
      status: { namespace: "default", addresses: [`10.0.0.${index + 1}`] },
    }),
  ),
  UserPB.Service.create({
    metadata: {
      uid: "web",
      name: "web.default",
      displayName: "Control Plane",
      description: "Private infrastructure management",
    },
    spec: {
      type: UserPB.Service_Spec_Type.WEB,
      port: 443,
      isTLS: true,
      isPublic: true,
    },
    status: { namespace: "default" },
  }),
  UserPB.Service.create({
    metadata: {
      uid: "postgres",
      name: "postgres.data",
      displayName: "Analytics database",
    },
    spec: { type: UserPB.Service_Spec_Type.POSTGRES, port: 5432 },
    status: { namespace: "data", addresses: ["10.10.10.10"] },
  }),
  UserPB.Service.create({
    metadata: { uid: "llm", name: "llm.default" },
    spec: { type: UserPB.Service_Spec_Type.LLM, port: 8080 },
    status: { namespace: "default" },
  }),
  UserPB.Service.create({
    metadata: { uid: "windows", name: "windows.default" },
    spec: { type: UserPB.Service_Spec_Type.RDP, port: 3389 },
    status: { namespace: "default" },
  }),
].sort((a, b) => a.metadata!.name.localeCompare(b.metadata!.name));
const workspace = WsPB.Workspace.create({
  apiVersion: "cordium/v1",
  kind: "Workspace",
  metadata: {
    uid: "workspace-demo",
    name: "demo",
    displayName: "Development workspace",
  },
  spec: {
    applications: [{ name: "app", isDefault: true, port: 3000 }],
    runtime: {},
  },
  status: {
    state: WsPB.Workspace_Status_State.RUNNING,
    hostname: "demo.example.test",
    regionRef: { name: "default" },
  },
});

const agentWorkspace = () =>
  WsPB.Workspace.create({
    apiVersion: "cordium/v1",
    kind: "Workspace",
    metadata: {
      uid: fixture.agent.uid,
      name: "agent",
      displayName: "Cordium Agent",
    },
    spec: { runtime: {} },
    status: {
      state: fixture.agent.state,
      regionRef: { name: "default" },
      failure: fixture.agent.failure
        ? { message: fixture.agent.failure }
        : undefined,
    },
  });

const frame = (type: number, payload: Uint8Array) => {
  const bytes = new Uint8Array(5 + payload.length);
  bytes[0] = type;
  new DataView(bytes.buffer).setUint32(1, payload.length);
  bytes.set(payload, 5);
  return bytes;
};
const trailer = frame(128, encoder.encode("grpc-status: 0\r\n"));
const rpcError = (code: number, message: string) =>
  new Response(
    frame(
      128,
      encoder.encode(`grpc-status: ${code}\r\ngrpc-message: ${message}\r\n`),
    ),
    { headers: { "content-type": "application/grpc-web+proto" } },
  );

const terminalStreams = new Map<
  string,
  ReadableStreamDefaultController<Uint8Array>
>();
const pendingInput = new Map<string, string>();
const connectTerminals: string[] = [];
const mainTerminal = () => connectTerminals.at(-1) ?? "";
const emitTerminal = (
  id: string,
  type: WsPB.ListenTerminalResponse["type"],
) => {
  terminalStreams
    .get(id)
    ?.enqueue(
      frame(
        0,
        WsPB.ListenTerminalResponse.toBinary(
          WsPB.ListenTerminalResponse.create({ type }),
        ),
      ),
    );
};
const emitOutput = (id: string, text: string) =>
  emitTerminal(id, {
    oneofKind: "stdout",
    stdout: { data: encoder.encode(text) },
  });
const closeStream = (id: string) => {
  const controller = terminalStreams.get(id);
  if (!controller) return;
  terminalStreams.delete(id);
  controller.enqueue(trailer);
  controller.close();
};
const runSetup = (id: string, script: string) => {
  fixture.connect.scripts.push(script);
  const nonce = /^N=(\w+)/.exec(script)![1];
  const mark = (body: string) =>
    emitOutput(id, `\r\n@@ocx:${nonce}:${body}\r\n`);
  mark("start");
  mark("step:dns:run");
  if (fixture.connect.holdSetup) return;
  mark("step:dns:ok");
  const tool = /^(?:want|fetch) (\S+) /m.exec(script)?.[1];
  if (tool) {
    mark(
      fixture.connect.failTool
        ? `step:${tool}:fail:root access is not available`
        : `step:${tool}:ok:${tool}`,
    );
  }
  mark("done");
  emitTerminal(id, { oneofKind: "close", close: {} });
  closeStream(id);
};

const originalFetch = window.fetch;
window.fetch = async (url, init) => {
  const path = new URL(String(url), location.origin).pathname;
  const service = [
    UserPB.MainService,
    WsPB.MainService,
    WsPB.WorkspaceService,
  ].find((item) => path.includes(item.typeName));
  const method = service?.methods.find((item) =>
    path.endsWith(`/${item.name}`),
  );
  if (!method) return originalFetch(url, init);
  const requestBody = init!.body;
  const bytes =
    typeof requestBody === "string"
      ? Uint8Array.from(atob(requestBody), (character) =>
          character.charCodeAt(0),
        )
      : new Uint8Array(requestBody as ArrayBuffer);
  const input = method.I.fromBinary(bytes.subarray(5));
  fixture.calls.push({ method: method.name, input });
  let result: object = {};
  switch (method.name) {
    case "GetStatus":
      result = {
        user: {
          metadata: { name: "alex", displayName: "Alex Morgan" },
          spec: { email: "alex@example.test" },
        },
      };
      break;
    case "ListSpace":
      result = {
        items: [
          {
            metadata: { uid: "default-space", name: "default.alex" },
            spec: {},
            status: { type: WsPB.Space_Status_Type.USER },
          },
        ],
      };
      break;
    case "GetUserConfig":
    case "InitializeAgent":
      if (method.name === "InitializeAgent") {
        if (fixture.agent.missing) fixture.agent.uid = "workspace-agent-new";
        fixture.agent.configured = true;
        fixture.agent.missing = false;
      }
      result = {
        status: {
          agentWorkspaceRef: fixture.agent.configured
            ? { uid: fixture.agent.uid, name: "agent" }
            : undefined,
        },
      };
      break;
    case "StartWorkspace":
      if (fixture.agent.startError) {
        return rpcError(9, fixture.agent.startError);
      }
      fixture.agent.state = WsPB.Workspace_Status_State.RUNNING;
      break;
    case "GetService": {
      const name = (input as { name: string }).name;
      const item = services.find(
        (service) =>
          service.metadata!.name === name ||
          service.metadata!.name === `${name}.default`,
      );
      if (!item) return rpcError(5, "The Service does not exist");
      result = item;
      break;
    }
    case "GetWorkspace": {
      const { name, uid } = input as { name: string; uid: string };
      if (uid.startsWith("workspace-agent")) {
        if (fixture.agent.missing) {
          return rpcError(5, "The Workspace does not exist");
        }
        result = agentWorkspace();
        break;
      }
      const item = WsPB.Workspace.clone(workspace);
      if (name === "other") {
        item.metadata!.uid = "workspace-other";
        item.metadata!.name = "other";
        item.metadata!.displayName = "Other workspace";
      }
      result = item;
      break;
    }
    case "ListService": {
      if (fixture.failServices) return new Response(null, { status: 503 });
      const options = input as UserPB.ListServiceOptions;
      const filtered = services.filter(
        (item) =>
          (!options.namespace ||
            item.status?.namespace === options.namespace) &&
          (!options.type || item.spec?.type === options.type),
      );
      const page = options.common?.page ?? 0;
      const itemsPerPage = options.common?.itemsPerPage || 10;
      result = {
        items: filtered.slice(page * itemsPerPage, (page + 1) * itemsPerPage),
        listResponseMeta: {
          page,
          itemsPerPage,
          totalCount: fixture.serviceTotal ?? filtered.length,
        },
      };
      break;
    }
    case "ListNamespace":
      result = {
        items: ["default", "data"].map((name) => ({ metadata: { name } })),
      };
      break;
    case "ListWorkspaceSnapshot":
      result = {
        items: [],
        listResponseMeta: { totalCount: 0, itemsPerPage: 10 },
      };
      break;
    case "ListTerminal": {
      if (fixture.failTerminals) return new Response(null, { status: 503 });
      const workspaceID = (input as WsPB.ListTerminalRequest).workspaceRef?.uid;
      const ids =
        workspaceID === "workspace-other"
          ? ["terminal-other"]
          : [...terminalIDs];
      result = {
        items: ids
          .filter((id) => !fixture.missingTerminalIDs.includes(id))
          .map((id) => ({ id })),
      };
      break;
    }
    case "CreateTerminal": {
      if (fixture.connect.failCreate) {
        return rpcError(14, "The Workspace is not reachable");
      }
      const id = `terminal-shell-${terminalIDs.size + 1}`;
      terminalIDs.add(id);
      if ((input as WsPB.CreateTerminalRequest).cols === 200) {
        connectTerminals.unshift(id);
      } else {
        connectTerminals.push(id);
      }
      result = { id };
      break;
    }
    case "ListenTerminal": {
      const id = (input as WsPB.ListenTerminalRequest).id;
      if (!terminalIDs.has(id)) return rpcError(5, "Terminal not found");
      return new Response(
        new ReadableStream({
          start(controller) {
            terminalStreams.set(id, controller);
            emitTerminal(id, {
              oneofKind: "windowSize",
              windowSize: { rows: 24, cols: 80 },
            });
            emitOutput(id, "$ ");
            init?.signal?.addEventListener(
              "abort",
              () => {
                if (terminalStreams.get(id) === controller) closeStream(id);
              },
              { once: true },
            );
          },
        }),
        { headers: { "content-type": "application/grpc-web+proto" } },
      );
    }
    case "WriteTerminalData": {
      const { id, data } = input as WsPB.WriteTerminalDataRequest;
      const text = (pendingInput.get(id) ?? "") + decoder.decode(data);
      if (text.startsWith(" echo ")) {
        const setup = /^ echo (\S+) \| base64 -d > \S+ && sh \S+; exit\r$/.exec(
          text,
        );
        if (setup) {
          pendingInput.delete(id);
          runSetup(id, atob(setup[1]));
        } else {
          pendingInput.set(id, text);
        }
        break;
      }
      fixture.connect.inputs.push(text);
      emitOutput(id, text.replace(/\r/g, "\r\n"));
      break;
    }
    case "SetTerminalWindowSize": {
      const { id, cols, rows } = input as WsPB.SetTerminalWindowSizeRequest;
      fixture.connect.sizes.push({ id, cols, rows });
      emitTerminal(id, {
        oneofKind: "windowSize",
        windowSize: { cols, rows },
      });
      break;
    }
    case "RemoveTerminal":
      if (fixture.failRemove)
        return new Response(null, {
          status: 503,
          statusText: "Service Unavailable",
        });
      terminalIDs.delete((input as WsPB.RemoveTerminalRequest).id);
      closeStream((input as WsPB.RemoveTerminalRequest).id);
      break;
    case "ListenLog":
      return new Response(
        new ReadableStream({
          start(controller) {
            controller.enqueue(
              frame(
                0,
                method.O.toBinary(
                  method.O.create({
                    data: encoder.encode("Starting tasks"),
                  } as never),
                ),
              ),
            );
            init?.signal?.addEventListener(
              "abort",
              () => {
                fixture.logAborts++;
                controller.enqueue(trailer);
                controller.close();
              },
              { once: true },
            );
          },
        }),
        { headers: { "content-type": "application/grpc-web+proto" } },
      );
  }
  const body = frame(0, method.O.toBinary(method.O.create(result)));
  return new Response(new Uint8Array([...body, ...trailer]), {
    headers: { "content-type": "application/grpc-web+proto" },
  });
};

const { default: store } = await import("@/store");
const { default: Root } = await import("@/pages");
const { default: routerRoot } = await import("@/pages/router");
const { default: Services } = await import("@/pages/Services/List");
const { default: Connect } = await import("@/pages/Services/Connect");
const { default: Workspace } = await import("@/pages/Workspaces/Item");
const { default: Terminals } =
  await import("@/pages/Workspaces/Item/Terminals");
const { default: Logs } = await import("@/pages/Workspaces/Item/Logs");
const { default: Snapshots } =
  await import("@/pages/Workspaces/Item/Snapshots");

const portalRouter = () => {
  const root = routerRoot();
  root.children![0].children!.push({
    path: "crash",
    Component: () => {
      throw new Error("Render failed");
    },
  });
  return createBrowserRouter([root]);
};

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Provider store={store}>
      <MantineProvider
        theme={theme}
        cssVariablesResolver={cssVariablesResolver}
        forceColorScheme={colorScheme}
      >
        <QueryClientProvider client={queryClient}>
          {fullRouter ? (
            <RouterProvider router={portalRouter()} />
          ) : (
            <BrowserRouter>
              <Routes>
                <Route element={<Root />}>
                  <Route path="services" element={<Services />}>
                    <Route path=":name/connect" element={<Connect />} />
                  </Route>
                  <Route path="workspaces/:name" element={<Workspace />}>
                    <Route index element={<p>Workspace details</p>} />
                    <Route path="terminals" element={<Terminals />} />
                    <Route path="logs" element={<Logs />} />
                    <Route path="snapshots" element={<Snapshots />} />
                  </Route>
                  <Route path="*" element={<p>Portal resources</p>} />
                </Route>
              </Routes>
            </BrowserRouter>
          )}
        </QueryClientProvider>
      </MantineProvider>
    </Provider>
  </React.StrictMode>,
);
