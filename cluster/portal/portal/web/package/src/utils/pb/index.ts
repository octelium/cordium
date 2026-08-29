import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import * as UserPB from "@octelium/apis/main/userv1";
import { dump, load as loadYaml } from "js-yaml";

export type Resource =
  | WsPB.Workspace
  | WsPB.Template
  | WsPB.Secret
  | WsPB.Space
  | WsPB.GitProvider
  | WsPB.UserSecret
  | UserPB.Service
  | UserPB.Namespace;

export type ResourceName =
  | "Workspace"
  | "Template"
  | "Secret"
  | "Space"
  | "GitProvider"
  | "UserSecret";

interface ResourceCodec {
  clone(arg: object): object;
  toJsonString(arg: object): string;
  fromJsonString(arg: string): object;
}

const codecFor = (kind: string): ResourceCodec =>
  WsPB[kind as ResourceName] as unknown as ResourceCodec;

export const cloneResource = <T extends Resource>(arg: T): T => {
  return codecFor(arg.kind).clone(arg) as T;
};

export const resourceToJSON = (arg: Resource): string => {
  return codecFor(arg.kind).toJsonString(arg);
};

export const resourceSpecToJSON = (arg: Resource): string => {
  const parsed = JSON.parse(resourceToJSON(arg)) as Record<string, unknown>;
  return JSON.stringify(parsed["spec"] ?? {});
};

export const resourceToYAML = (arg: Resource): string => {
  return dump(JSON.parse(resourceToJSON(arg)));
};

export const resourceSpecToYAML = (arg: Resource): string => {
  return dump(JSON.parse(resourceSpecToJSON(arg)));
};

export const resourceFromYAML = (arg: string): Resource | undefined => {
  const yamlObj = loadYaml(arg) as Record<string, unknown> | undefined;
  const kind = yamlObj?.["kind"];
  if (typeof kind !== "string" || !(kind in WsPB)) {
    return undefined;
  }
  return codecFor(kind).fromJsonString(JSON.stringify(yamlObj)) as Resource;
};

export const getResourceRef = (arg: Resource): MetaPB.ObjectReference => {
  return MetaPB.ObjectReference.create({
    apiVersion: arg.apiVersion,
    kind: arg.kind,
    uid: arg.metadata?.uid,
    name: arg.metadata?.name,
  });
};

export const getShortNameFromStr = (arg: string): string => {
  return arg.split(".").at(0) ?? "";
};

export const getShortName = (arg: Resource): string => {
  return getShortNameFromStr(arg.metadata!.name);
};

export const getShortNameFromRef = (arg: MetaPB.ObjectReference): string => {
  return getShortNameFromStr(arg.name);
};

export const getDisplayName = (arg: Resource): string => {
  return arg.metadata?.displayName || getShortName(arg);
};

export const canStopWorkspace = (arg: WsPB.Workspace): boolean => {
  switch (arg.status?.state) {
    case WsPB.Workspace_Status_State.STOPPING_REQUEST:
    case WsPB.Workspace_Status_State.STOPPING:
    case WsPB.Workspace_Status_State.STOPPED:
      return false;
    default:
      return true;
  }
};

export const isWorkspaceStopped = (arg: WsPB.Workspace): boolean => {
  return arg.status?.state === WsPB.Workspace_Status_State.STOPPED;
};

export const isWorkspaceRunning = (arg: WsPB.Workspace): boolean => {
  return arg.status?.state === WsPB.Workspace_Status_State.RUNNING;
};

export const isMemberAdmin = (arg: WsPB.Membership): boolean => {
  return (
    arg.spec?.role === WsPB.Membership_Spec_Role.ADMIN ||
    arg.spec?.role === WsPB.Membership_Spec_Role.OWNER
  );
};

export const isMemberOwner = (arg: WsPB.Membership): boolean => {
  return arg.spec?.role === WsPB.Membership_Spec_Role.OWNER;
};

export const isOrgSpace = (arg: WsPB.Space): boolean => {
  return arg.status?.type === WsPB.Space_Status_Type.ORGANIZATION;
};

export const getSpaceTypeLabel = (arg: WsPB.Space): string => {
  return isOrgSpace(arg) ? "Organization" : "Personal";
};

export const getRoleLabel = (role: WsPB.Membership_Spec_Role): string => {
  switch (role) {
    case WsPB.Membership_Spec_Role.OWNER:
      return "Owner";
    case WsPB.Membership_Spec_Role.ADMIN:
      return "Admin";
    case WsPB.Membership_Spec_Role.USER:
      return "Member";
    default:
      return "Unknown";
  }
};
