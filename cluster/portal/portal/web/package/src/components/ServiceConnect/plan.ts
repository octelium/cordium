import { getServiceHostname, getServicePrivateFQDN } from "@/utils/octelium";
import { Service, Service_Spec_Type } from "@octelium/apis/main/userv1";
import { match } from "ts-pattern";

export type ToolID =
  | "ssh"
  | "psql"
  | "mysql"
  | "kubectl"
  | "curl"
  | "dig"
  | "grpcurl"
  | "redis-cli"
  | "nats"
  | "kcat";

export type Facts = Record<string, string>;

export interface ConnectInput {
  text: string;
  run?: boolean;
  cursor?: number;
}

export interface Snippet {
  label: string;
  text: string;
}

export interface ConnectHint {
  text: string;
  tone?: "info" | "warning";
}

export interface ConnectPlan {
  label: string;
  tool?: ToolID;
  host: string;
  port?: number;
  address: string;
  kubeconfig?: string;
  inferred?: string;
  commands: (facts: Facts) => ConnectInput[];
  snippets: Snippet[];
  hint?: ConnectHint;
}

export const toolLabels: Record<ToolID, string> = {
  ssh: "ssh",
  psql: "psql",
  mysql: "mysql",
  kubectl: "kubectl",
  curl: "curl",
  dig: "dig",
  grpcurl: "grpcurl",
  "redis-cli": "redis-cli",
  nats: "nats",
  kcat: "kcat",
};

const sq = (value: string) =>
  /^[\w@%+=:,./-]+$/.test(value) ? value : `'${value.replace(/'/g, `'"'"'`)}'`;

const bin = (facts: Facts, tool: ToolID) => facts[tool] || toolLabels[tool];

const portFlag = (flag: string, port: number | undefined, dflt: number) =>
  port && port !== dflt ? ` ${flag} ${port}` : "";

const hostPort = (host: string, port?: number) =>
  port ? `${host.includes(":") ? `[${host}]` : host}:${port}` : host;

const httpURL = (host: string, port: number | undefined, tls: boolean) => {
  const dflt = tls ? 443 : 80;
  return `${tls ? "https" : "http"}://${port && port !== dflt ? hostPort(host, port) : host}`;
};

const tokens = (value?: string) =>
  (value ?? "")
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter(Boolean);

interface Inference {
  kind: string;
  keys: string[];
  ports: number[];
}

const inferences: Inference[] = [
  {
    kind: "postgres",
    keys: [
      "postgres",
      "postgresql",
      "psql",
      "pgsql",
      "pg",
      "timescale",
      "timescaledb",
      "cockroach",
      "cockroachdb",
      "yugabyte",
      "pgbouncer",
    ],
    ports: [5432, 6432, 26257],
  },
  {
    kind: "mysql",
    keys: ["mysql", "mariadb", "percona", "tidb", "vitess"],
    ports: [3306],
  },
  {
    kind: "redis",
    keys: ["redis", "valkey", "keydb", "dragonfly", "kvrocks"],
    ports: [6379],
  },
  { kind: "nats", keys: ["nats", "jetstream"], ports: [4222] },
  { kind: "kafka", keys: ["kafka", "redpanda"], ports: [9092] },
  { kind: "mongo", keys: ["mongo", "mongodb", "ferretdb"], ports: [27017] },
  {
    kind: "elasticsearch",
    keys: ["elasticsearch", "elastic", "opensearch"],
    ports: [9200],
  },
  { kind: "clickhouse", keys: ["clickhouse"], ports: [8123, 9000] },
  {
    kind: "prometheus",
    keys: ["prometheus", "thanos", "victoriametrics"],
    ports: [9090],
  },
  { kind: "vault", keys: ["vault", "openbao"], ports: [8200] },
  { kind: "consul", keys: ["consul"], ports: [8500] },
  { kind: "minio", keys: ["minio"], ports: [] },
  { kind: "ssh", keys: ["ssh", "sftp", "bastion"], ports: [22] },
  { kind: "dns", keys: ["dns", "coredns", "unbound", "bind9"], ports: [53] },
  {
    kind: "http",
    keys: ["http", "https", "web", "api", "nginx", "caddy", "traefik", "envoy"],
    ports: [80, 443, 8000, 8080, 8443, 3000],
  },
];

const matchKey = (token: string, key: string) =>
  token === key || (key.length >= 5 && token.startsWith(key));

export const inferTCPKind = (
  service: Service,
): { kind: string; source: string } | undefined => {
  const sources: { source: string; values: string[] }[] = [
    {
      source: "name",
      values: tokens(service.metadata?.name.split(".")[0]),
    },
    {
      source: "display name",
      values: tokens(service.metadata?.displayName),
    },
  ];

  for (const { source, values } of sources) {
    for (const itm of inferences) {
      if (
        values.some((token) => itm.keys.some((key) => matchKey(token, key)))
      ) {
        return { kind: itm.kind, source };
      }
    }
  }

  const port = service.spec?.port;
  const byPort = inferences.find((itm) => port && itm.ports.includes(port));
  return byPort ? { kind: byPort.kind, source: "port" } : undefined;
};

const sshSnippets: Snippet[] = [
  { label: "uptime", text: "uptime" },
  { label: "whoami", text: "whoami" },
  { label: "df -h", text: "df -h" },
  { label: "free -h", text: "free -h" },
];

const psqlSnippets: Snippet[] = [
  { label: "\\conninfo", text: "\\conninfo" },
  { label: "\\l", text: "\\l" },
  { label: "\\dt", text: "\\dt" },
  { label: "\\du", text: "\\du" },
  { label: "SELECT version();", text: "SELECT version();" },
];

const mysqlSnippets: Snippet[] = [
  { label: "STATUS", text: "STATUS" },
  { label: "SHOW DATABASES;", text: "SHOW DATABASES;" },
  { label: "SHOW TABLES;", text: "SHOW TABLES;" },
  { label: "SELECT VERSION();", text: "SELECT VERSION();" },
];

const kubectlSnippets: Snippet[] = [
  { label: "get pods -A", text: "kubectl get pods -A" },
  { label: "get nodes", text: "kubectl get nodes -o wide" },
  { label: "get namespaces", text: "kubectl get namespaces" },
  {
    label: "recent events",
    text: "kubectl get events -A --sort-by=.lastTimestamp",
  },
  { label: "api-resources", text: "kubectl api-resources" },
];

const redisSnippets: Snippet[] = [
  { label: "PING", text: "PING" },
  { label: "INFO server", text: "INFO server" },
  { label: "DBSIZE", text: "DBSIZE" },
  { label: "SCAN 0", text: "SCAN 0 COUNT 20" },
];

const curlSnippets = (url: string): Snippet[] => [
  { label: "GET", text: `curl -i ${url}` },
  { label: "HEAD", text: `curl -sI ${url}` },
  {
    label: "timing",
    text: `curl -s -o /dev/null -w '%{http_code} %{time_total}s\\n' ${url}`,
  },
];

const digSnippets = (server: string): Snippet[] => [
  { label: "A", text: `dig ${server} example.com +short` },
  { label: "AAAA", text: `dig ${server} example.com AAAA +short` },
  { label: "MX", text: `dig ${server} example.com MX` },
];

const run = (text: string): ConnectInput => ({ text, run: true });
const type = (text: string): ConnectInput => ({ text });

const tcpPlan = (
  service: Service,
  host: string,
  port: number | undefined,
): ConnectPlan => {
  const inferred = inferTCPKind(service);
  const address = hostPort(host, port);
  const base = { host, port, address };
  const note = (client: string) =>
    inferred
      ? `${client} is inferred from the Service ${inferred.source}.`
      : undefined;
  const noCreds =
    "TCP Services do not inject upstream credentials, so the client might ask for them.";

  return match(inferred?.kind)
    .with("postgres", () => ({
      ...base,
      label: "psql",
      tool: "psql" as const,
      inferred: note("psql"),
      hint: { text: noCreds },
      commands: (facts: Facts) => [
        type(
          `${bin(facts, "psql")} -h ${host}${portFlag("-p", port, 5432)} -U `,
        ),
      ],
      snippets: psqlSnippets,
    }))
    .with("mysql", () => ({
      ...base,
      label: "mysql",
      tool: "mysql" as const,
      inferred: note("mysql"),
      hint: { text: noCreds },
      commands: (facts: Facts) => [
        type(
          `${bin(facts, "mysql")} -h ${host}${portFlag("-P", port, 3306)} -p -u `,
        ),
      ],
      snippets: mysqlSnippets,
    }))
    .with("redis", () => ({
      ...base,
      label: "redis-cli",
      tool: "redis-cli" as const,
      inferred: note("redis-cli"),
      commands: (facts: Facts) => [
        run(
          `${bin(facts, "redis-cli")} -h ${host}${portFlag("-p", port, 6379)}`,
        ),
      ],
      snippets: redisSnippets,
    }))
    .with("nats", () => ({
      ...base,
      label: "nats",
      tool: "nats" as const,
      inferred: note("nats"),
      commands: (facts: Facts) => [
        run(`export NATS_URL=nats://${address}`),
        type(`${bin(facts, "nats")} rtt`),
      ],
      snippets: [
        { label: "rtt", text: "nats rtt" },
        { label: "subscribe >", text: "nats sub '>'" },
        { label: "streams", text: "nats stream ls" },
        { label: "server info", text: "nats server info" },
      ],
    }))
    .with("kafka", () => ({
      ...base,
      label: "kcat",
      tool: "kcat" as const,
      inferred: note("kcat"),
      commands: (facts: Facts) => [
        type(`${bin(facts, "kcat")} -b ${address} -L`),
      ],
      snippets: [
        { label: "metadata", text: `kcat -b ${address} -L` },
        { label: "consume", text: `kcat -b ${address} -C -t ` },
      ],
    }))
    .with("mongo", () => ({
      ...base,
      label: "Terminal",
      inferred: note("MongoDB"),
      hint: {
        text: `mongosh is not installed automatically. Once it is available, connect with mongosh mongodb://${address}`,
      },
      commands: () => [],
      snippets: [
        { label: "mongosh", text: `mongosh mongodb://${address}` },
        { label: "raw TCP", text: `curl -v telnet://${address}` },
      ],
    }))
    .with("ssh", () => ({
      ...base,
      label: "ssh",
      tool: "ssh" as const,
      inferred: note("ssh"),
      hint: { text: noCreds },
      commands: () => [type(`ssh ${host}${portFlag("-p", port, 22)}`)],
      snippets: sshSnippets,
    }))
    .with("dns", () => ({
      ...base,
      label: "dig",
      tool: "dig" as const,
      inferred: note("dig"),
      commands: () => [
        type(`dig @${host}${portFlag("-p", port, 53)} example.com`),
      ],
      snippets: digSnippets(`@${host}${portFlag("-p", port, 53)}`),
    }))
    .with(
      "elasticsearch",
      "clickhouse",
      "prometheus",
      "vault",
      "consul",
      "minio",
      "http",
      (kind) => {
        const url = httpURL(host, port, false);
        const path = match(kind)
          .with("elasticsearch", () => "/_cluster/health?pretty")
          .with("clickhouse", () =>
            port === 9000 ? "" : "/?query=SELECT%20version()",
          )
          .with("prometheus", () => "/api/v1/status/buildinfo")
          .with("vault", () => "/v1/sys/health")
          .with("consul", () => "/v1/status/leader")
          .with("minio", () => "/minio/health/live")
          .otherwise(() => "/");
        return {
          ...base,
          label: "curl",
          tool: "curl" as const,
          inferred: note("curl"),
          commands: () => [type(`curl -i ${sq(`${url}${path}`)}`)],
          snippets: curlSnippets(url),
        };
      },
    )
    .otherwise(() => ({
      ...base,
      label: "Terminal",
      hint: {
        text: `No specific client is known for this Service. Any client in this terminal reaches it at ${address}.`,
      },
      commands: () => [],
      snippets: [{ label: "raw TCP", text: `curl -v telnet://${address}` }],
    }));
};

export const buildConnectPlan = (
  service: Service,
  domain: string,
): ConnectPlan => {
  const spec = service.spec;
  const name = getServiceHostname(service);
  const fqdn = getServicePrivateFQDN(service, domain);
  const port = spec?.port || undefined;
  const tls = !!spec?.isTLS;
  const host = tls ? fqdn : name;
  const address = hostPort(host, port);
  const base = { host, port, address };

  return match(spec?.type ?? Service_Spec_Type.UNSET)
    .with(Service_Spec_Type.SSH, () => ({
      ...base,
      label: "ssh",
      tool: "ssh" as const,
      commands: () => [run(`ssh ${host}${portFlag("-p", port, 22)}`)],
      snippets: sshSnippets,
    }))
    .with(Service_Spec_Type.POSTGRES, () => ({
      ...base,
      label: "psql",
      tool: "psql" as const,
      commands: (facts: Facts) => [
        run(`${bin(facts, "psql")} -h ${host}${portFlag("-p", port, 5432)}`),
      ],
      snippets: psqlSnippets,
    }))
    .with(Service_Spec_Type.MYSQL, () => ({
      ...base,
      label: "mysql",
      tool: "mysql" as const,
      commands: (facts: Facts) => [
        run(
          [
            bin(facts, "mysql"),
            `-h ${host}`,
            ...(port && port !== 3306 ? [`-P ${port}`] : []),
            ...(facts["mysql.flavor"] === "mariadb"
              ? ["--skip-ssl-verify-server-cert"]
              : []),
          ].join(" "),
        ),
      ],
      snippets: mysqlSnippets,
    }))
    .with(Service_Spec_Type.KUBERNETES, () => ({
      ...base,
      label: "kubectl",
      tool: "kubectl" as const,
      kubeconfig: name,
      commands: (facts: Facts) => [
        ...(facts.kubeconfig
          ? [run(`export KUBECONFIG=${sq(facts.kubeconfig)}`)]
          : []),
        type(`${bin(facts, "kubectl")} get pods -A`),
      ],
      snippets: kubectlSnippets,
    }))
    .with(
      Service_Spec_Type.HTTP,
      Service_Spec_Type.WEB,
      Service_Spec_Type.MCP,
      Service_Spec_Type.LLM,
      (serviceType) => {
        const url = httpURL(host, port, tls);
        return {
          ...base,
          label: "curl",
          tool: "curl" as const,
          commands: (): ConnectInput[] =>
            match(serviceType)
              .with(Service_Spec_Type.LLM, () => [
                type(`curl -s ${url}/v1/models`),
              ])
              .with(Service_Spec_Type.MCP, () => [
                type(
                  `curl -s ${url}/ -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' -d ${sq(
                    JSON.stringify({
                      jsonrpc: "2.0",
                      id: 1,
                      method: "initialize",
                      params: {
                        protocolVersion: "2025-06-18",
                        capabilities: {},
                        clientInfo: { name: "curl", version: "1.0.0" },
                      },
                    }),
                  )}`,
                ),
              ])
              .with(Service_Spec_Type.WEB, () => [type(`curl -sI ${url}/`)])
              .otherwise(() => [type(`curl -i ${url}/`)]),
          snippets: curlSnippets(`${url}/`),
        };
      },
    )
    .with(Service_Spec_Type.GRPC, () => {
      const target = hostPort(host, port ?? (tls ? 443 : 80));
      const flag = tls ? "" : " -plaintext";
      return {
        ...base,
        label: "grpcurl",
        tool: "grpcurl" as const,
        commands: (facts: Facts) => [
          type(`${bin(facts, "grpcurl")}${flag} ${target} list`),
        ],
        snippets: [
          { label: "list", text: `grpcurl${flag} ${target} list` },
          { label: "describe", text: `grpcurl${flag} ${target} describe` },
          {
            label: "health",
            text: `grpcurl${flag} ${target} grpc.health.v1.Health/Check`,
          },
        ],
      };
    })
    .with(Service_Spec_Type.DNS, () => {
      const server = `@${host}${portFlag("-p", port, 53)}`;
      return {
        ...base,
        label: "dig",
        tool: "dig" as const,
        commands: () => [type(`dig ${server} example.com`)],
        snippets: digSnippets(server),
      };
    })
    .with(Service_Spec_Type.SOCKS5, () => ({
      ...base,
      label: "curl",
      tool: "curl" as const,
      commands: () => [
        type(`curl --socks5-hostname ${address} https://example.com -I`),
      ],
      snippets: [
        {
          label: "egress IP",
          text: `curl --socks5-hostname ${address} https://checkip.amazonaws.com`,
        },
      ],
    }))
    .with(Service_Spec_Type.UDP, () =>
      port === 53
        ? {
            ...base,
            label: "dig",
            tool: "dig" as const,
            commands: () => [type(`dig @${host} example.com`)],
            snippets: digSnippets(`@${host}`),
          }
        : {
            ...base,
            label: "Terminal",
            hint: {
              text: `This is a UDP Service. Any UDP client in this terminal reaches it at ${address}.`,
            },
            commands: () => [],
            snippets: [],
          },
    )
    .otherwise(() => tcpPlan(service, host, port));
};

export const mainCommand = (plan: ConnectPlan, facts: Facts = {}) =>
  plan.commands(facts).at(-1)?.text;
