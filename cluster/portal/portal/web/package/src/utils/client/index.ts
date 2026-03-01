import { ObjectReference } from "@/apis/metav1/metav1";
import * as grpcWeb from "@protobuf-ts/grpcweb-transport";

import { getDomain, isDev } from "..";
import * as WsGRPC from "../../apis/cordiumv1/cordiumv1.client";
import * as UserGRPC from "../../apis/userv1/userv1.client";

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
