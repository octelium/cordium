import type { ConnectPlan, ToolID } from "./plan";
import { toolLabels } from "./plan";

interface PackageTool {
  bins: string[];
  packages: [string, string, string, string, string];
}

interface DownloadTool {
  bins: string[];
  download: "kubectl" | "grpcurl" | "nats";
}

const tools: Record<ToolID, PackageTool | DownloadTool> = {
  ssh: {
    bins: ["ssh"],
    packages: [
      "openssh-client",
      "openssh-client",
      "openssh-clients",
      "openssh-clients",
      "openssh",
    ],
  },
  psql: {
    bins: ["psql"],
    packages: [
      "postgresql-client",
      "postgresql-client",
      "postgresql",
      "postgresql",
      "postgresql-libs",
    ],
  },
  mysql: {
    bins: ["mysql", "mariadb"],
    packages: [
      "mysql-client,default-mysql-client,mariadb-client",
      "mysql-client,mariadb-client",
      "mysql,mariadb",
      "mariadb-client",
      "mariadb-clients",
    ],
  },
  curl: { bins: ["curl"], packages: ["curl", "curl", "curl", "curl", "curl"] },
  dig: {
    bins: ["dig"],
    packages: [
      "dnsutils,bind9-dnsutils",
      "bind-tools",
      "bind-utils",
      "bind-utils",
      "bind",
    ],
  },
  "redis-cli": {
    bins: ["redis-cli", "valkey-cli"],
    packages: [
      "redis-tools,valkey-tools",
      "redis,valkey",
      "redis,valkey",
      "redis,valkey",
      "valkey,redis",
    ],
  },
  kcat: {
    bins: ["kcat", "kafkacat"],
    packages: ["kcat,kafkacat", "kcat", "kcat", "kcat", "-"],
  },
  kubectl: { bins: ["kubectl"], download: "kubectl" },
  grpcurl: { bins: ["grpcurl"], download: "grpcurl" },
  nats: { bins: ["nats"], download: "nats" },
};

const prelude = `rm -f "$0"
m() { printf '\\n@@ocx:%s:%s\\n' "$N" "$*"; }
has() { command -v "$1" >/dev/null 2>&1; }
pick() { for c in $(echo "$1" | tr , ' '); do has "$c" && { echo "$c"; return 0; }; done; return 1; }
if [ "$(id -u)" = 0 ]; then R=; elif has sudo && sudo -n true >/dev/null 2>&1; then R="sudo -n"; else R=-; fi
m start`;

const packages = `P=
for x in apt-get apk dnf microdnf yum zypper pacman; do has $x && { P=$x; break; }; done
U=
inst() {
  case $P in
    apt-get) [ -n "$U" ] || { $R apt-get update -q; U=1; }; $R env DEBIAN_FRONTEND=noninteractive apt-get install -y -q --no-install-recommends "$1" ;;
    apk) $R apk add --no-cache "$1" ;;
    pacman) $R pacman -S --noconfirm --needed "$1" || $R pacman -Sy --noconfirm --needed "$1" ;;
    zypper) $R zypper -n install "$1" ;;
    *) $R $P install -y "$1" ;;
  esac
}
want() {
  i=$1; b=$2; shift 2
  case $P in apt-get) l=$1;; apk) l=$2;; dnf|microdnf|yum) l=$3;; zypper) l=$4;; pacman) l=$5;; *) l=-;; esac
  if c=$(pick "$b"); then m "step:$i:ok:$c"; return 0; fi
  m "step:$i:run"
  if [ "$R" = - ]; then m "step:$i:fail:root access is not available"; return 1; fi
  if [ -z "$P" ] || [ "$l" = - ]; then m "step:$i:fail:no supported package manager was found"; return 1; fi
  for k in $(echo "$l" | tr , ' '); do inst "$k" </dev/null && pick "$b" >/dev/null && break; done
  if c=$(pick "$b"); then m "step:$i:ok:$c"; else m "step:$i:fail:it could not be installed with $P"; return 1; fi
}`;

const downloads = `dl() { curl -fsSL --retry 2 "$1"; }
arch() { case $(uname -m) in x86_64|amd64) echo amd64;; aarch64|arm64) echo arm64;; *) return 1;; esac; }
ghv() { u=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$1/releases/latest") || return 1; v=\${u##*/}; echo "\${v#v}"; }
put() {
  if [ "$R" != - ]; then $R install -m 0755 "$1" "/usr/local/bin/$2" && return 0; fi
  d=; for x in $(echo "$PATH" | tr : ' '); do [ -d "$x" ] && [ -w "$x" ] && { d=$x; break; }; done
  [ -n "$d" ] || { d=$HOME/.local/bin; mkdir -p "$d"; }
  install -m 0755 "$1" "$d/$2" || return 1
  has "$2" || m "fact:$2=$d/$2"
}
fetch() {
  if c=$(pick "$2"); then m "step:$1:ok:$c"; return 0; fi
  m "step:$1:run"
  has curl || want curl curl curl curl curl curl curl || { m "step:$1:fail:curl is not available"; return 1; }
  t=$(mktemp -d) || return 1
  if (cd "$t" && "get$1"); then m "step:$1:ok:$2"; r=0; else m "step:$1:fail:the download failed"; r=1; fi
  rm -rf "$t"; return $r
}`;

const downloaders: Record<DownloadTool["download"], string> = {
  kubectl: `getkubectl() {
  a=$(arch) && v=$(dl https://dl.k8s.io/release/stable.txt) || return 1
  u=https://dl.k8s.io/release/$v/bin/linux/$a/kubectl
  dl "$u" > kubectl && echo "$(dl "$u.sha256")  kubectl" | sha256sum -c - >/dev/null && put kubectl kubectl
}`,
  grpcurl: `getgrpcurl() {
  a=$(arch) && v=$(ghv fullstorydev/grpcurl) || return 1
  [ "$a" = amd64 ] && a=x86_64
  f=grpcurl_\${v}_linux_$a.tar.gz; b=https://github.com/fullstorydev/grpcurl/releases/download/v$v
  dl "$b/$f" > "$f" && dl "$b/grpcurl_\${v}_checksums.txt" | grep " $f\\$" | sha256sum -c - >/dev/null && tar -xzf "$f" grpcurl && put grpcurl grpcurl
}`,
  nats: `getnats() {
  a=$(arch) && v=$(ghv nats-io/natscli) || return 1
  has unzip || want unzip unzip unzip unzip unzip unzip unzip || return 1
  f=nats-$v-linux-$a.zip; b=https://github.com/nats-io/natscli/releases/download/v$v
  dl "$b/$f" > "$f" && dl "$b/SHA256SUMS" | grep " $f\\$" | sha256sum -c - >/dev/null && unzip -qjo "$f" '*/nats' && put nats nats
}`,
};

const shq = (value: string) => `'${value.replace(/'/g, `'"'"'`)}'`;

const dnsStep = (host: string) => `m step:dns:run
if has getent; then
  k=0; until getent hosts ${shq(host)} >/dev/null 2>&1; do k=$((k+1)); [ $k -ge 30 ] && break; sleep 1; done
  if [ $k -lt 30 ]; then m step:dns:ok; else m "step:dns:fail:${host} does not resolve yet"; fi
else
  m step:dns:skip
fi`;

const kubeconfigStep = (name: string, domain: string) => `m step:kubeconfig:run
f="$HOME/.kube/${name}.\${OCTELIUM_DOMAIN:-${domain}}"
if has octelium; then
  k=0; until octelium config ${shq(name)} </dev/null; do k=$((k+1)); [ $k -ge 5 ] && break; sleep 2; done
  if [ $k -lt 5 ] && [ -f "$f" ]; then m "fact:kubeconfig=$f"; m step:kubeconfig:ok; else m "step:kubeconfig:fail:octelium config failed"; fi
else
  m "step:kubeconfig:fail:the octelium CLI is not available"
fi`;

const toolStep = (id: ToolID) => {
  const tool = tools[id];
  const bins = tool.bins.join(",");
  if ("download" in tool) return `fetch ${id} ${bins}`;
  const step = `want ${id} ${bins} ${tool.packages.join(" ")}`;
  return id === "mysql"
    ? `${step} && { mysql --version 2>/dev/null | grep -qi mariadb || ! has mysql; } && m fact:mysql.flavor=mariadb`
    : step;
};

export interface SetupStepInfo {
  id: string;
  label: string;
}

export const setupSteps = (plan: ConnectPlan): SetupStepInfo[] => [
  { id: "dns", label: `Reach ${plan.host}` },
  ...(plan.tool ? [{ id: plan.tool, label: toolLabels[plan.tool] }] : []),
  ...(plan.kubeconfig
    ? [{ id: "kubeconfig", label: `Configure kubectl for ${plan.kubeconfig}` }]
    : []),
];

export const buildSetupScript = (
  plan: ConnectPlan,
  domain: string,
  nonce: string,
): string => {
  const tool = plan.tool ? tools[plan.tool] : undefined;
  const download = tool && "download" in tool ? tool.download : undefined;
  const needsPackages = !!tool;
  return [
    `N=${nonce}`,
    prelude,
    ...(needsPackages ? [packages] : []),
    ...(download ? [downloads, downloaders[download]] : []),
    dnsStep(plan.host),
    ...(plan.tool ? [toolStep(plan.tool)] : []),
    ...(plan.kubeconfig ? [kubeconfigStep(plan.kubeconfig, domain)] : []),
    "m done",
    "",
  ].join("\n");
};

export const setupCommand = (script: string, nonce: string) => {
  const path = `/tmp/.cordium-connect-${nonce}.sh`;
  return ` echo ${btoa(script)} | base64 -d > ${path} && sh ${path}; exit\r`;
};

export type SetupEvent =
  | { type: "start" }
  | { type: "done" }
  | { type: "fact"; key: string; value: string }
  | {
      type: "step";
      id: string;
      status: "run" | "ok" | "fail" | "skip";
      detail?: string;
    };

export const parseSetupLine = (
  line: string,
  nonce: string,
): SetupEvent | undefined => {
  const prefix = `@@ocx:${nonce}:`;
  const idx = line.indexOf(prefix);
  if (idx < 0) return undefined;
  const body = line.slice(idx + prefix.length).trim();
  if (body === "start") return { type: "start" };
  if (body === "done") return { type: "done" };
  if (body.startsWith("fact:")) {
    const fact = body.slice(5);
    const eq = fact.indexOf("=");
    return eq > 0
      ? { type: "fact", key: fact.slice(0, eq), value: fact.slice(eq + 1) }
      : undefined;
  }
  const step = /^step:([\w.-]+):(run|ok|fail|skip)(?::(.*))?$/.exec(body);
  return step
    ? {
        type: "step",
        id: step[1],
        status: step[2] as "run" | "ok" | "fail" | "skip",
        detail: step[3] || undefined,
      }
    : undefined;
};
