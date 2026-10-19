import { Service, Service_Spec_Type } from "@octelium/apis/main/userv1";
import {
  getServiceHostname,
  getServicePrivateFQDN,
  getServicePublicFQDN,
} from "@/utils/octelium";

export const SERVICE_TYPES = [
  { value: "HTTP", label: "HTTP", type: Service_Spec_Type.HTTP },
  { value: "WEB", label: "Web app", type: Service_Spec_Type.WEB },
  { value: "GRPC", label: "gRPC", type: Service_Spec_Type.GRPC },
  { value: "SSH", label: "SSH", type: Service_Spec_Type.SSH },
  {
    value: "KUBERNETES",
    label: "Kubernetes",
    type: Service_Spec_Type.KUBERNETES,
  },
  { value: "POSTGRES", label: "PostgreSQL", type: Service_Spec_Type.POSTGRES },
  { value: "MYSQL", label: "MySQL", type: Service_Spec_Type.MYSQL },
  { value: "TCP", label: "TCP", type: Service_Spec_Type.TCP },
  { value: "UDP", label: "UDP", type: Service_Spec_Type.UDP },
  { value: "DNS", label: "DNS", type: Service_Spec_Type.DNS },
  { value: "SOCKS5", label: "SOCKS5", type: Service_Spec_Type.SOCKS5 },
  { value: "RDP_WEB", label: "RDP Web", type: Service_Spec_Type.RDP_WEB },
  { value: "RDP", label: "RDP", type: Service_Spec_Type.RDP },
  { value: "MCP", label: "MCP", type: Service_Spec_Type.MCP },
  { value: "LLM", label: "LLM", type: Service_Spec_Type.LLM },
];

export const getServiceTypeLabel = (item: Service) =>
  SERVICE_TYPES.find((info) => info.type === item.spec?.type)?.label ??
  "Service";

const rankMatch = (item: Service, search: string) => {
  const hostname = getServiceHostname(item).toLowerCase();
  if (hostname === search) return 0;
  if (hostname.startsWith(search)) return 1;
  if (hostname.includes(search)) return 2;
  return 3;
};

export const searchServices = (
  items: Service[],
  search: string,
  domain: string,
) => {
  const needle = search.trim().toLowerCase();
  if (!needle) return items;
  const tokens = needle.split(/\s+/);

  return items
    .filter((item) => {
      const haystack = [
        getServiceHostname(item),
        item.metadata?.name,
        item.metadata?.displayName,
        item.metadata?.description,
        item.status?.namespace,
        getServiceTypeLabel(item),
        item.spec?.port ? String(item.spec.port) : "",
        getServicePrivateFQDN(item, domain),
        item.spec?.isPublic ? getServicePublicFQDN(item, domain) : "",
        ...(item.status?.addresses ?? []),
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return tokens.every((token) => haystack.includes(token));
    })
    .sort(
      (a, b) =>
        rankMatch(a, needle) - rankMatch(b, needle) ||
        getServiceHostname(a).localeCompare(getServiceHostname(b)),
    );
};
