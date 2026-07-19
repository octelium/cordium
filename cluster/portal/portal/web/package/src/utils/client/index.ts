import { ObjectReference } from "@octelium/apis/main/metav1";
import * as grpcWeb from "@protobuf-ts/grpcweb-transport";

import * as WsGRPC from "@octelium/apis/main/cordiumv1";
import * as UserGRPC from "@octelium/apis/main/userv1";
import { getDomain, isDev } from "..";

const getTransport = () => {
  const domain = getDomain();
  const scheme = location.protocol === "https:" ? "https" : "http";

  let baseUrl = `${scheme}://octelium-api.${domain}`;

  if (isDev()) {
    baseUrl = `https://${window.location.host}`;
  }

  return new grpcWeb.GrpcWebFetchTransport({
    baseUrl,

    fetchInit: {
      credentials: "include",
    },
  });
};

const getTransportRegionRef = (regionRef: ObjectReference | undefined) => {
  if (!regionRef || regionRef.name === `default`) {
    return getTransport();
  }

  const domain = getDomain();
  const scheme = location.protocol === "https:" ? "https" : "http";

  return new grpcWeb.GrpcWebFetchTransport({
    baseUrl: `${scheme}://${regionRef.name}.octelium-api.${domain}`,

    fetchInit: {
      credentials: "include",
    },
  });
};

export const getClientUser = (): UserGRPC.MainServiceClient => {
  return new UserGRPC.MainServiceClient(getTransport());
};

export const getClientWorkspace = (): WsGRPC.MainServiceClient => {
  return new WsGRPC.MainServiceClient(getTransport());
};

export const getClientWorkspaceSvc = (
  regionRef: ObjectReference | undefined,
): WsGRPC.WorkspaceServiceClient => {
  return new WsGRPC.WorkspaceServiceClient(getTransportRegionRef(regionRef));
};
