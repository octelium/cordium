import { strict as assert } from "node:assert";
import { describe, it } from "node:test";
import { getCatalog } from "../testutil/app.ts";
import {
  APICatalog,
  classifyRisk,
  firstSentences,
  SERVICES,
  shortTypeName,
} from "./catalog.ts";
import { SearchIndex, stem, tokenize } from "./search.ts";

describe("search", () => {
  it("tokenizes camel case and stems plurals", () => {
    assert.deepEqual(tokenize("ListWorkspaceSnapshots of the Policies"), [
      "list",
      "workspace",
      "snapshot",
      "policy",
    ]);
    assert.equal(stem("addresses"), "address");
    assert.equal(stem("access"), "access");
  });

  it("ranks the documents with BM25, synonyms and prefixes", () => {
    const index = new SearchIndex<string>();
    index.add("delete-workspace", [
      { text: "DeleteWorkspace", weight: 3, primary: true },
    ]);
    index.add("list-workspace", [
      { text: "ListWorkspace", weight: 3, primary: true },
    ]);
    index.add("list-volume", [
      { text: "ListVolume", weight: 3, primary: true },
    ]);
    assert.equal(index.search("remove sandbox")[0].item, "delete-workspace");
    assert.equal(index.search("all disks")[0].item, "list-volume");
    assert.equal(index.search("volu")[0].item, "list-volume");
    assert.deepEqual(index.search("the"), []);
  });
});

describe("catalog", () => {
  const catalog = getCatalog();

  it("loads the Cordium, user and core APIs", () => {
    const packages = new Set(catalog.methods.map((m) => m.packageName));
    assert.deepEqual([...packages].sort(), [
      "cordium.v1",
      "core.v1",
      "user.v1",
    ]);
    assert.ok(
      catalog.methods.filter((m) => m.packageName === "cordium.v1").length > 50,
    );
    for (const id of [
      "cordium.v1.MainService/InitializeAgent",
      "user.v1.MainService/Connect",
      "user.v1.MainService/SetServiceConfigs",
      "cordium.v1.WorkspaceService/Exec",
    ]) {
      assert.equal(catalog.resolveMethod(id), undefined, id);
    }
    const watch = catalog.resolveMethod(
      "cordium.v1.MainService/WatchWorkspace",
    );
    assert.equal(watch?.kind, "server_streaming");
    assert.equal(watch?.invocable, true);
  });

  it("documents every method from the generated schemas", () => {
    for (const service of SERVICES) {
      const schema = catalog.schemas.services[service.typeName];
      assert.ok(schema, service.typeName);
      for (const method of service.methods) {
        assert.ok(schema.methods[method.name], method.name);
        assert.ok(
          catalog.schemas.definitions[method.I.typeName] ||
            method.I.typeName.startsWith("google.protobuf."),
          method.I.typeName,
        );
      }
    }
    const method = catalog.resolveMethod(
      "cordium.v1.MainService/CreateWorkspace",
    )!;
    assert.match(method.description, /created in the STOPPED state/);
    assert.equal(method.requestType, "cordium.v1.Workspace");
  });

  it("resolves the method IDs in their different forms", () => {
    for (const id of [
      "cordium.v1.MainService/ListSpace",
      "/octelium.api.main.cordium.v1.MainService/ListSpace",
      "octelium.api.main.cordium.v1.MainService.ListSpace",
      "cordium.v1.MainService.ListSpace",
      "CORDIUM.V1.MAINSERVICE/LISTSPACE",
    ]) {
      assert.equal(
        catalog.resolveMethod(id)?.grpcMethod,
        "/octelium.api.main.cordium.v1.MainService/ListSpace",
        id,
      );
    }
    assert.equal(
      catalog.resolveMethod("cordium.v1.MainService/Nope"),
      undefined,
    );
    assert.equal(
      catalog.resolveType("cordium.v1.Workspace.Spec"),
      "octelium.api.main.cordium.v1.Workspace.Spec",
    );
    assert.equal(
      catalog.resolveType(".octelium.api.main.meta.v1.GetOptions"),
      "octelium.api.main.meta.v1.GetOptions",
    );
    assert.equal(catalog.resolveType("cordium.v1.Nope"), undefined);
  });

  it("finds the relevant methods", () => {
    const top = (q: string) => catalog.search(q, 3).map((h) => h.item.id);
    assert.equal(top("list spaces")[0], "cordium.v1.MainService/ListSpace");
    assert.equal(
      top("start sandbox")[0],
      "cordium.v1.MainService/StartWorkspace",
    );
    assert.ok(
      top("snapshot a workspace").includes(
        "cordium.v1.MainService/CreateWorkspaceSnapshot",
      ),
    );
    assert.ok(
      top("expose workspace port").includes(
        "cordium.v1.MainService/ShareWorkspacePort",
      ),
    );
    assert.equal(
      top("create volume")[0],
      "cordium.v1.MainService/CreateVolume",
    );
    assert.ok(top("list services").includes("user.v1.MainService/ListService"));
    assert.match(
      catalog.formatSearchResults(catalog.search("list templates", 2)),
      /^1\. cordium\.v1\.MainService\/ListTemplate — .*\[read\]\n   request: cordium\.v1\.ListTemplateOptions/,
    );
    assert.match(catalog.formatSearchResults([]), /^No matching methods/);
  });

  it("classifies the risks", () => {
    const risk = (id: string) => catalog.resolveMethod(id)?.risk;
    assert.equal(risk("cordium.v1.MainService/ListWorkspace"), "read");
    assert.equal(risk("cordium.v1.MainService/GetUserConfig"), "read");
    assert.equal(risk("cordium.v1.MainService/CreateWorkspace"), "write");
    assert.equal(risk("cordium.v1.MainService/StopWorkspace"), "write");
    assert.equal(risk("cordium.v1.MainService/BuildTemplate"), "write");
    assert.equal(risk("cordium.v1.MainService/DeleteSpace"), "destructive");
    assert.equal(risk("cordium.v1.MainService/LeaveSpace"), "destructive");
    assert.equal(risk("cordium.v1.MainService/CreateSecret"), "sensitive");
    assert.equal(risk("cordium.v1.MainService/UpdateUserSecret"), "sensitive");
    assert.equal(risk("cordium.v1.MainService/CreateMembership"), "sensitive");
    assert.equal(
      risk("cordium.v1.MainService/ShareWorkspacePort"),
      "sensitive",
    );
    assert.equal(risk("cordium.v1.MainService/UnshareWorkspacePort"), "write");
    assert.equal(
      risk("cordium.v1.ManagementService/UpdateClusterConfig"),
      "sensitive",
    );
    assert.equal(
      risk("core.v1.MainService/GenerateCredentialToken"),
      "sensitive",
    );
    assert.equal(classifyRisk("DoSomething"), "write");
  });

  it("describes the methods with their JSON schemas", () => {
    const text = catalog.describeMethod(
      catalog.resolveMethod("cordium.v1.MainService/ListWorkspace")!,
    );
    assert.match(text, /^Method: cordium\.v1\.MainService\/ListWorkspace\n/);
    assert.match(text, /Risk: read/);
    assert.match(text, /cordium\.v1\.ListWorkspaceOptions \{/);
    assert.match(text, /common: meta\.v1\.CommonListOptions/);
    assert.match(
      text,
      /  oneof filter \(set at most one\) \{\n    spaceRef: meta\.v1\.ObjectReference.*\n    templateRef: meta\.v1\.ObjectReference/,
    );
    assert.match(text, /Response summary:\n.*\n?cordium\.v1\.WorkspaceList \{/);

    const ws = catalog.describeType(
      "octelium.api.main.cordium.v1.Workspace.Status",
      { maxDepth: 0 },
    );
    assert.match(
      ws,
      /state: enum cordium\.v1\.Workspace\.Status\.State \("UNKNOWN" \| "INIT_REQUEST"/,
    );
    assert.match(ws, /timestamp \(RFC 3339 string/);
    assert.match(ws, /Not expanded/);

    const state = catalog.describeType(
      "octelium.api.main.cordium.v1.Workspace.Status.State",
    );
    assert.match(
      state,
      /^\/\/ .*\nenum cordium\.v1\.Workspace\.Status\.State \{/,
    );
    assert.match(state, /  "RUNNING"  \/\/ /);

    const config = catalog.describeType(
      "octelium.api.main.cordium.v1.ClusterConfig.Spec.Agent",
    );
    assert.match(config, /config: object \(arbitrary JSON object\)/);
    assert.match(config, /isDisabled: bool/);
  });

  it("lists the API index of the prompt packages only", () => {
    const index = catalog.formatIndex();
    assert.match(index, /^cordium\.v1 — /);
    assert.match(
      index,
      /\n  MainService: .*Workspace\[Create\|Update\|Delete\|List\|Start\|Stop\|Get\|Watch\]/,
    );
    assert.match(index, /\nuser\.v1 — /);
    assert.match(
      index,
      /\ncore\.v1 — .*\(not listed here, use cordium_api_search\)$/,
    );
    assert.doesNotMatch(index, /InitializeAgent/);
    assert.match(catalog.formatIndex(["core.v1"]), /^core\.v1 — /);
  });

  it("formats the helpers", () => {
    assert.equal(
      shortTypeName("octelium.api.main.cordium.v1.Workspace"),
      "cordium.v1.Workspace",
    );
    assert.equal(
      shortTypeName("google.protobuf.Struct"),
      "google.protobuf.Struct",
    );
    assert.equal(firstSentences("Short."), "Short.");
    assert.equal(
      firstSentences(
        `First sentence, i.e. not cut. Second one. ${"x".repeat(300)}`,
        60,
      ),
      "First sentence, i.e. not cut. Second one.",
    );
    assert.match(firstSentences("y".repeat(300), 50), /…$/);
  });

  it("loads custom schemas", () => {
    const custom = APICatalog.fromJSON(
      JSON.stringify({ services: {}, definitions: {} }),
    );
    const method = custom.resolveMethod("cordium.v1.MainService/ListSpace");
    assert.equal(method?.description, "");
    assert.match(custom.describeMethod(method!), /Request schema/);
  });
});
