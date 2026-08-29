import { queryClient } from "@/utils";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import * as UserPB from "@octelium/apis/main/userv1";
import { getResourceRef, getShortNameFromStr, Resource } from "../pb";

const ORGANIZATION_SPACE_PARENT = "cordium";

const getSpacePathName = (name: string, organization: boolean): string => {
  const shortName = getShortNameFromStr(name);
  return organization ? `@${shortName}` : shortName;
};

export const getSpaceResourceName = (
  pathName: string,
  userName?: string,
): string | undefined => {
  if (pathName.startsWith("@")) {
    const shortName = pathName.slice(1);
    return shortName ? `${shortName}.${ORGANIZATION_SPACE_PARENT}` : undefined;
  }

  if (pathName.includes(".")) {
    return pathName;
  }

  return userName ? `${pathName}.${userName}` : undefined;
};

export const getServiceHostname = (arg: UserPB.Service): string => {
  if (arg.status!.namespace === `default`) {
    return arg.metadata!.name.split(".")[0];
  }
  return `${arg.metadata!.name}`;
};

export const getServicePrivateFQDN = (
  arg: UserPB.Service,
  domain: string,
): string => {
  return `${getServiceHostname(arg)}.local.${domain}`;
};

export const getServicePublicFQDN = (
  arg: UserPB.Service,
  domain: string,
): string => {
  return `${getServiceHostname(arg)}.${domain}`;
};

export const getServicePublicURL = (
  arg: UserPB.Service,
  domain: string,
): string => {
  return `https://${getServicePublicFQDN(arg, domain)}`;
};

export const getPathSpaceRef = (arg: MetaPB.ObjectReference): string => {
  return `/spaces/${getSpacePathName(
    arg.name,
    arg.name.endsWith(`.${ORGANIZATION_SPACE_PARENT}`),
  )}`;
};

export const getPathSpace = (arg: WsPB.Space): string => {
  return `/spaces/${getSpacePathName(
    arg.metadata!.name,
    arg.status?.type === WsPB.Space_Status_Type.ORGANIZATION,
  )}`;
};

export const getPathTemplateRef = (
  spaceRef: MetaPB.ObjectReference,
  templateRef: MetaPB.ObjectReference,
): string => {
  return `${getPathSpaceRef(spaceRef)}/templates/${getShortNameFromStr(
    templateRef.name,
  )}`;
};

export const getPathTemplate = (arg: WsPB.Template): string => {
  return getPathTemplateRef(arg.status!.spaceRef!, getResourceRef(arg));
};

export const getPathWorkspace = (arg: WsPB.Workspace): string => {
  return `/workspaces/${arg.metadata!.name}`;
};

export const getWorkspaceURL = (arg: WsPB.Workspace): string | undefined => {
  return arg.status?.hostname ? `https://${arg.status.hostname}` : undefined;
};

export const getApplicationURL = (
  arg: WsPB.Workspace,
  app: WsPB.Workspace_Spec_Application,
): string | undefined => {
  if (!arg.status?.hostname) {
    return undefined;
  }
  return app.isDefault
    ? `https://${arg.status.hostname}`
    : `https://${app.name}_${arg.status.hostname}`;
};

export const invalidateResource = (arg: Resource) => {
  queryClient.invalidateQueries({
    queryKey: [`workspace/get${arg.kind}`, arg.metadata?.uid],
  });
  queryClient.invalidateQueries({
    queryKey: [`workspace/get${arg.kind}`, arg.metadata?.name],
  });
};

export const invalidateWorkspaces = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listWorkspace"] });
};

export const invalidateWorkspace = (arg: WsPB.Workspace) => {
  invalidateResource(arg);
  invalidateWorkspaces();
};

export const invalidateSpaces = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listSpace"] });
};

export const invalidateSpace = (arg: WsPB.Space) => {
  invalidateResource(arg);
  invalidateSpaces();
};

export const invalidateTemplates = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listTemplate"] });
};

export const invalidateTemplate = (arg: WsPB.Template) => {
  invalidateResource(arg);
  invalidateTemplates();
};

export const invalidateSecrets = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listSecret"] });
};

export const invalidateGitProviders = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listGitProvider"] });
};

export const invalidateMemberships = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listMembership"] });
};

export const invalidateUserSecrets = () => {
  queryClient.invalidateQueries({ queryKey: ["workspace/listUserSecret"] });
};
