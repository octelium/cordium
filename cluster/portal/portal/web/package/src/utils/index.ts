const isDevVal = import.meta.env.MODE === "development";
import type { RpcError } from "@protobuf-ts/runtime-rpc";
import toast from "react-hot-toast";

import { QueryClient } from "@tanstack/react-query";

export function isDev(): boolean {
  return isDevVal;
}

const isWebgl2SupportedFn = (() => {
  let isSupported = window.WebGL2RenderingContext ? undefined : false;
  return () => {
    if (isSupported === undefined) {
      const canvas = document.createElement("canvas");
      const gl = canvas.getContext("webgl2", {
        depth: false,
        antialias: false,
      });
      isSupported = gl instanceof window.WebGL2RenderingContext;
    }
    return isSupported;
  };
})();

export const isWebgl2Supported = isWebgl2SupportedFn();

export const onError = (err: RpcError) => {
  toast.error(err.message);
};

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30000,
      retry: 1,
    },
  },
});

export const toNumOrZero = (arg: string | null | undefined): number => {
  if (!arg) {
    return 0;
  }

  try {
    return parseInt(arg, 10);
  } catch {
    return 0;
  }
};

let __domain: string | undefined;

export const getDomain = (): string => {
  if (isDev()) {
    return window.location.host;
  }

  if (__domain) {
    return __domain;
  }

  __domain =
    ("; " + window.document.cookie)
      .split("; octelium_domain=")
      .pop()
      ?.split(";")
      .shift() ?? "";

  return __domain;
};

export const truncateUtf8 = (
  input: string,
  maxBytes: number,
  options?: { suffix?: string },
) => {
  if (maxBytes <= 0) return "";

  const encoder = new TextEncoder();

  const suffix = options?.suffix ?? "";
  const suffixBytes = suffix ? encoder.encode(suffix).length : 0;

  const totalBytes = encoder.encode(input).length;
  if (totalBytes <= maxBytes) return input;

  const allowedForBody = Math.max(0, maxBytes - suffixBytes);

  let out = "";
  let used = 0;

  for (const ch of input) {
    const b = encoder.encode(ch).length;
    if (used + b > allowedForBody) break;
    out += ch;
    used += b;
  }

  if (suffix && used + suffixBytes <= maxBytes) {
    return out + suffix;
  }

  return out;
};

export const formatMegabytes = (megabytes: number): string => {
  if (megabytes <= 0) return "—";
  if (megabytes >= 1000) {
    const gb = megabytes / 1000;
    return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`;
  }
  return `${megabytes} MB`;
};

export const formatMillicores = (millicores: number): string => {
  if (millicores <= 0) return "—";
  if (millicores >= 1000) {
    const cores = millicores / 1000;
    return `${Number.isInteger(cores) ? cores : cores.toFixed(1)} vCPU`;
  }
  return `${millicores}m vCPU`;
};

export const pluralize = (count: number, singular: string, plural?: string) => {
  return count === 1 ? singular : (plural ?? `${singular}s`);
};
