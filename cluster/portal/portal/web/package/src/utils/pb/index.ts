import Yaml from "js-yaml";
import * as WsPB from "../../apis/cordiumv1/cordiumv1";
import * as MetaPB from "../../apis/metav1/metav1";
import * as UserPB from "../../apis/userv1/userv1";

export type Resource =
  | WsPB.Workspace
  | WsPB.Template
  | WsPB.Secret
  | WsPB.Space
  | WsPB.Secret
  | WsPB.GitProvider
  | WsPB.UserSecret
  | UserPB.Service
  | UserPB.Namespace;
export type ResourceList =
  | WsPB.WorkspaceList
  | WsPB.TemplateList
  | WsPB.SecretList
  | WsPB.SpaceList
  | WsPB.SecretList
  | WsPB.GitProviderList
  | WsPB.UserSecretList
  | UserPB.ServiceList
  | UserPB.NamespaceList;
export type ResourceName = "Workspace" | "Template" | "Secret" | "Space";

export const cloneResource = (arg: Resource): Resource => {
  return WsPB[arg.kind as ResourceName].clone(arg as any) as Resource;
};

export const resourceToJSON = (arg: Resource): string => {
  return WsPB[arg.kind as ResourceName].toJsonString(arg as any);
};
export const resourceSpecToJSON = (arg: Resource): string => {
  return JSON.stringify(
    JSON.parse(WsPB[arg.kind as ResourceName].toJsonString(arg as any))["spec"],
  );
};

export const resourceToYAML = (arg: Resource): string => {
  return Yaml.dump(JSON.parse(resourceToJSON(arg)));
};

export const resourceSpecToYAML = (arg: Resource): string => {
  return Yaml.dump(JSON.parse(resourceSpecToJSON(arg)));
};

export const resourceFromYAML = (arg: string): Resource | undefined => {
  const yamlObj = Yaml.load(arg) as any;
  const kind = yamlObj["kind"] as ResourceName;
  return WsPB[kind].fromJsonString(JSON.stringify(yamlObj));
};

export const getResourceRef = (arg: Resource): MetaPB.ObjectReference => {
  return MetaPB.ObjectReference.create({
    apiVersion: arg.apiVersion,
    kind: arg.kind,
    uid: arg.metadata?.uid,
    name: arg.metadata?.name,
  });
};

export const getShortName = (arg: Resource): string => {
  return getShortNameFromStr(arg.metadata!.name);
};

export const getShortNameFromRef = (arg: MetaPB.ObjectReference): string => {
  return getShortNameFromStr(arg.name);
};

export const getShortNameFromStr = (arg: string): string => {
  return arg.split(".").at(0) ?? "";
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

export const isMemberAdmin = (arg: WsPB.Membership): boolean => {
  return (
    arg.spec?.role === WsPB.Membership_Spec_Role.ADMIN ||
    arg.spec?.role === WsPB.Membership_Spec_Role.OWNER
  );
};
