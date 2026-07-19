import { queryClient } from "@/utils";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import * as UserPB from "@octelium/apis/main/userv1";
import { getResourceRef, getShortNameFromRef, Resource } from "../pb";

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
  return `/spaces/${getShortNameFromRef(arg)}`;
};

const getPathTemplateRef = (arg: MetaPB.ObjectReference): string => {
  return `/templates/${getShortNameFromRef(arg)}`;
};

export const getPathSpace = (arg: WsPB.Space): string => {
  return getPathSpaceRef(getResourceRef(arg));
};

export const getPathTemplate = (arg: WsPB.Template): string => {
  return `${getPathSpaceRef(arg.status!.spaceRef!)}${getPathTemplateRef(
    getResourceRef(arg),
  )}`;
};

export const getPathWorkspace = (arg: WsPB.Workspace): string => {
  return `/workspaces/${arg.metadata!.name}`;
};

export const invalidateResource = (arg: Resource) => {
  queryClient.invalidateQueries({
    queryKey: [`workspace/get${arg.kind}`, arg.metadata?.uid],
  });
  queryClient.invalidateQueries({
    queryKey: [`workspace/get${arg.kind}`, arg.metadata?.name],
  });
};

export const invalidateWorkspace = (arg: WsPB.Workspace) => {
  invalidateResource(arg);
  queryClient.invalidateQueries({
    queryKey: ["workspace/listWorkspace", 0],
  });
  queryClient.invalidateQueries({
    queryKey: ["workspace/listWorkspace", arg.status?.spaceRef?.uid, 0],
  });
  queryClient.invalidateQueries({
    queryKey: ["workspace/listWorkspace", arg.status?.templateRef?.uid, 0],
  });
};

export const invalidateSpaces = () => {
  queryClient.invalidateQueries({
    queryKey: ["workspace/listSpace"],
  });
};

export const invalidateSpace = (arg: WsPB.Space) => {
  invalidateResource(arg);
  queryClient.invalidateQueries({
    queryKey: ["workspace/listSpace"],
  });
};

export const invalidateTemplate = (arg: WsPB.Template) => {
  invalidateResource(arg);

  queryClient.invalidateQueries({
    queryKey: ["workspace/listTemplate", arg.status?.spaceRef?.uid, 0],
  });
  queryClient.invalidateQueries({
    queryKey: ["workspace/listTemplate", arg.status?.spaceRef?.uid],
  });
};
