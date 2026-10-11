import type {
  APIRisk,
  ApprovalBlock,
  ApprovalDecision,
  ArtifactBlock,
  Block,
  ChartBlock,
  ErrorBlock,
  NoticeBlock,
  ResourceRef,
  ResourcesBlock,
  TableBlock,
  TableColumn,
  ThinkingBlock,
  ToolBlock,
  TransferDetails,
  WorkspaceSummary,
  WorkspacesDetails,
} from "@/apis/agent/protocol";
import StateBadge from "@/components/StateBadge";
import Tag from "@/components/Tag";
import {
  ActionIcon,
  Button,
  Loader,
  Modal,
  Textarea,
  Tooltip,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconAlertTriangle,
  IconApi,
  IconArrowRight,
  IconArrowsMaximize,
  IconBan,
  IconBook,
  IconBrain,
  IconChartBar,
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconCircleOff,
  IconCube,
  IconDownload,
  IconExternalLink,
  IconFile,
  IconFileExport,
  IconFilePencil,
  IconFileSearch,
  IconFileText,
  IconFolderOpen,
  IconFolderSearch,
  IconInfoCircle,
  IconListCheck,
  IconPhotoDown,
  IconSearch,
  IconShieldCheck,
  IconShieldExclamation,
  IconShieldQuestion,
  IconShieldX,
  IconStack2,
  IconTable,
  IconTemplate,
  IconTerminal2,
  IconTool,
  IconTransfer,
  IconWorld,
  IconX,
  type TablerIcon,
} from "@tabler/icons-react";
import { AnimatePresence, motion } from "framer-motion";
import * as React from "react";
import { Link } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import type { ChartHandle } from "./Chart";
import { AgentClient } from "./client";
import AgentMarkdown from "./Markdown";
import { CopyButton, FileIcon, Shimmer } from "./ui";
import {
  formatBytes,
  formatDuration,
  formatValue,
  getResourceRoute,
  getWorkspacePath,
  riskLabel,
  riskTone,
  saveBlob,
  toCSV,
  toFileName,
  toJSON,
  toolDuration,
  toWorkspaceState,
} from "./utils";

const Chart = React.lazy(() => import("./Chart"));

export interface BlockContext {
  client: AgentClient;
  runId?: string;
  streaming: boolean;
  onDecide?: (
    runId: string,
    approvalId: string,
    decision: ApprovalDecision,
    reason?: string,
  ) => Promise<void>;
}

const ease = [0.22, 1, 0.36, 1] as const;

const Collapse = (props: { opened: boolean; children: React.ReactNode }) => (
  <AnimatePresence initial={false}>
    {props.opened && (
      <motion.div
        initial={{ height: 0, opacity: 0 }}
        animate={{ height: "auto", opacity: 1 }}
        exit={{ height: 0, opacity: 0 }}
        transition={{ duration: 0.25, ease }}
        className="overflow-hidden"
      >
        {props.children}
      </motion.div>
    )}
  </AnimatePresence>
);

const Code = (props: { children: string; className?: string }) => (
  <div className="group/code relative">
    <pre
      className={twMerge(
        "max-h-[320px] overflow-auto rounded-lg border border-line bg-surface-subtle px-3 py-2",
        props.className,
      )}
    >
      <code className="whitespace-pre font-mono text-[0.76rem] leading-5 text-ink-body">
        {props.children}
      </code>
    </pre>
    <span className="absolute right-1.5 top-1.5 rounded-md bg-surface opacity-0 shadow-panel transition-opacity focus-within:opacity-100 group-hover/code:opacity-100 [@media(hover:none)]:opacity-100">
      <CopyButton value={() => props.children} />
    </span>
  </div>
);

const TerminalView = (props: {
  command?: string;
  output?: string;
  workspace?: string;
  cwd?: string;
  exitCode?: number;
  durationMs?: number;
  timedOut?: boolean;
  truncated?: boolean;
  live?: boolean;
  className?: string;
}) => {
  const ref = React.useRef<HTMLPreElement>(null);

  React.useEffect(() => {
    if (props.live && ref.current) {
      ref.current.scrollTop = ref.current.scrollHeight;
    }
  }, [props.output, props.live]);

  return (
    <div
      className={twMerge(
        "console-surface overflow-hidden rounded-lg border border-zinc-800 bg-console",
        props.className,
      )}
    >
      <div className="flex min-h-8 items-center gap-2 border-b border-zinc-800 bg-console-chrome py-1 pl-3 pr-1.5 text-[0.7rem]/4">
        <IconTerminal2 size={12} className="shrink-0 text-zinc-500" />
        <span className="flex min-w-0 flex-1 items-center gap-1.5 font-semibold text-zinc-400">
          {props.workspace ? (
            <Link
              to={getWorkspacePath(props.workspace)}
              className="truncate font-mono transition-colors hover:text-zinc-100"
            >
              {props.workspace}
            </Link>
          ) : (
            <span className="truncate uppercase tracking-[0.08em]">Shell</span>
          )}
          {props.cwd && (
            <span className="hidden truncate font-mono font-normal text-zinc-500 sm:inline">
              {props.cwd}
            </span>
          )}
          {props.truncated && (
            <span className="shrink-0 font-normal text-zinc-500">
              · truncated
            </span>
          )}
        </span>
        {props.timedOut && (
          <span className="shrink-0 font-semibold text-amber-400">
            timed out
          </span>
        )}
        {props.durationMs !== undefined && (
          <span className="shrink-0 font-mono text-zinc-500">
            {formatDuration(props.durationMs)}
          </span>
        )}
        {props.exitCode !== undefined && (
          <span
            className={twMerge(
              "shrink-0 font-mono font-semibold",
              props.exitCode === 0 ? "text-emerald-400" : "text-rose-400",
            )}
          >
            exit {props.exitCode}
          </span>
        )}
        {props.output && (
          <CopyButton dark value={() => props.output ?? ""} label="Copy output" />
        )}
      </div>
      <pre
        ref={ref}
        className="max-h-[320px] overflow-auto px-3 py-2 font-mono text-[0.76rem] leading-5 text-zinc-200"
      >
        {props.command && (
          <code className="block whitespace-pre-wrap break-all text-zinc-400">
            <span className="select-none text-emerald-400">$ </span>
            {props.command}
          </code>
        )}
        {props.output && (
          <code className="block whitespace-pre-wrap break-words">
            {props.output}
          </code>
        )}
        {props.live && (
          <span className="mt-1 inline-block h-3.5 w-1.5 bg-zinc-400 align-middle motion-safe:animate-pulse" />
        )}
      </pre>
    </div>
  );
};

const diffLineClass = (line: string): string => {
  if (line.startsWith("+") && !line.startsWith("+++")) {
    return "bg-hue-emerald-soft text-hue-emerald";
  }
  if (line.startsWith("-") && !line.startsWith("---")) {
    return "bg-hue-rose-soft text-hue-rose";
  }
  if (line.startsWith("@@")) {
    return "text-hue-blue";
  }
  return "text-ink-soft";
};

const DiffView = (props: { diff: string }) => (
  <pre className="max-h-[320px] overflow-auto rounded-lg border border-line bg-surface py-1.5 font-mono text-[0.76rem] leading-5">
    {props.diff
      .split("\n")
      .slice(0, 2000)
      .map((line, idx) => (
        <div key={idx} className={twMerge("px-3", diffLineClass(line))}>
          {line || " "}
        </div>
      ))}
  </pre>
);

const Section = (props: { label: string; children: React.ReactNode }) => (
  <div className="mt-3 first:mt-0">
    <p className="mb-1 text-[0.7rem]/4 font-semibold uppercase tracking-[0.08em] text-ink-muted">
      {props.label}
    </p>
    {props.children}
  </div>
);

const RiskBadge = (props: { risk: APIRisk }) => (
  <Tag tone={riskTone(props.risk)} className="px-1.5 text-[0.68rem] leading-4">
    {riskLabel[props.risk]}
  </Tag>
);

const CardHeader = (props: {
  icon: TablerIcon;
  title: React.ReactNode;
  meta?: React.ReactNode;
  children?: React.ReactNode;
}) => (
  <div className="flex min-h-10 items-center gap-2 border-b border-line-subtle py-1.5 pl-3 pr-1.5">
    <props.icon size={13} className="shrink-0 text-ink-muted" />
    <span className="min-w-0 flex-1 truncate text-xs font-semibold text-ink-body">
      {props.title}
    </span>
    {props.meta !== undefined && (
      <span className="shrink-0 text-[0.7rem]/4 font-semibold text-ink-muted">
        {props.meta}
      </span>
    )}
    {props.children}
  </div>
);

const toolIcons: Record<string, TablerIcon> = {
  cordium_api_call: IconApi,
  cordium_api_search: IconSearch,
  cordium_api_describe: IconBook,
  workspace_list: IconCube,
  workspace_create: IconCube,
  workspace_control: IconCube,
  workspace_logs: IconFileText,
  workspace_exec: IconTerminal2,
  workspace_copy: IconTransfer,
  bash: IconTerminal2,
  read: IconFileText,
  write: IconFilePencil,
  edit: IconFilePencil,
  grep: IconFileSearch,
  find: IconFolderSearch,
  ls: IconFolderOpen,
  web_fetch: IconWorld,
  web_search: IconSearch,
  present_table: IconTable,
  present_chart: IconChartBar,
  present_resources: IconStack2,
  publish_artifact: IconFileExport,
};

const detailsIcons: Record<string, TablerIcon> = {
  api: IconApi,
  command: IconTerminal2,
  file: IconFileText,
  workspaces: IconCube,
  transfer: IconTransfer,
};

const ToolIcon = (props: { block: ToolBlock }) => {
  const { block } = props;
  const Icon =
    toolIcons[block.name] ??
    (block.details ? detailsIcons[block.details.kind] : undefined) ??
    IconTool;
  return <Icon size={13} className="shrink-0 text-ink-subtle" />;
};

const ToolStatusIcon = (props: { block: ToolBlock }) => {
  switch (props.block.status) {
    case "pending":
    case "running":
      return <Loader size={10} color="gray" />;
    case "awaiting_approval":
      return <IconShieldQuestion size={12} className="text-hue-amber" />;
    case "completed":
      return <IconCheck size={11} stroke={3} className="text-hue-emerald" />;
    case "failed":
      return <IconX size={11} stroke={3} className="text-hue-rose" />;
    case "rejected":
      return <IconBan size={11} stroke={2.5} className="text-hue-rose" />;
    case "cancelled":
      return <IconCircleOff size={11} stroke={2.5} className="text-ink-muted" />;
  }
};

const toolStatusLabel: Record<ToolBlock["status"], string> = {
  pending: "Preparing",
  running: "Running",
  awaiting_approval: "Awaiting approval",
  completed: "Done",
  failed: "Failed",
  rejected: "Rejected",
  cancelled: "Stopped",
};

const isToolActive = (block: ToolBlock) =>
  block.status === "pending" || block.status === "running";

const ResourceChip = (props: { resource: ResourceRef }) => {
  const route = getResourceRoute(props.resource);
  const className =
    "inline-flex max-w-full items-center gap-1 rounded-md border border-line bg-surface px-1.5 py-0.5 text-[0.7rem]/4 text-ink-strong";
  const label = (
    <>
      <span className="text-ink-muted">{props.resource.kind}</span>
      <span className="truncate font-semibold">{props.resource.name}</span>
    </>
  );

  return route ? (
    <Link
      to={route}
      className={twMerge(
        className,
        "transition-colors duration-150 hover:border-line-strong hover:bg-surface-hover",
      )}
    >
      {label}
    </Link>
  ) : (
    <span className={className}>{label}</span>
  );
};

const shortName = (name?: string) => name?.split(".").slice(0, -1).join(".");

const WorkspaceCard = (props: { item: WorkspaceSummary }) => {
  const { item } = props;
  const deleted = item.state === "DELETED";

  return (
    <div
      className={twMerge(
        "flex min-w-0 flex-col gap-2 rounded-xl border border-line bg-surface px-3 py-2.5 shadow-panel",
        deleted && "opacity-60",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-1.5">
            {deleted ? (
              <span className="truncate font-mono text-[0.8rem] font-bold text-ink line-through">
                {item.name}
              </span>
            ) : (
              <Link
                to={getWorkspacePath(item.name)}
                className="truncate font-mono text-[0.8rem] font-bold text-ink hover:underline"
              >
                {item.name}
              </Link>
            )}
            {item.isAgent && <Tag tone="accent">Agent</Tag>}
          </div>
          {item.displayName && (
            <p className="truncate text-[0.7rem]/4 font-medium text-ink-muted">
              {item.displayName}
            </p>
          )}
        </div>
        {deleted ? (
          <Tag tone="danger">Deleted</Tag>
        ) : (
          <StateBadge state={toWorkspaceState(item.state)} />
        )}
      </div>
      <div className="flex flex-wrap gap-1">
        {item.space && (
          <Tag icon={<IconStack2 size={11} />}>
            {shortName(item.space) || item.space}
          </Tag>
        )}
        {item.template && (
          <Tag icon={<IconTemplate size={11} />}>
            {item.template.split(".")[0]}
          </Tag>
        )}
        {item.image && <Tag mono>{item.image}</Tag>}
        {item.isEphemeral && <Tag tone="warning">Ephemeral</Tag>}
      </div>
      {item.repository && (
        <p className="truncate font-mono text-[0.7rem]/4 text-ink-muted">
          {item.repository.replace(/^https:\/\//, "")}
        </p>
      )}
      {item.failure && (
        <p className="flex items-start gap-1 text-[0.7rem]/4 font-semibold text-hue-rose">
          <IconAlertTriangle size={12} className="mt-0.5 shrink-0" />
          <span className="min-w-0 break-words">{item.failure}</span>
        </p>
      )}
      {item.hostname && item.state === "RUNNING" && !item.isAgent && (
        <a
          href={`https://${item.hostname}`}
          target="_blank"
          rel="noreferrer noopener"
          className="inline-flex w-fit max-w-full items-center gap-1 text-[0.7rem]/4 font-semibold text-ink-soft hover:text-ink"
        >
          <IconExternalLink size={12} className="shrink-0" />
          <span className="truncate">{item.hostname}</span>
        </a>
      )}
    </div>
  );
};

const workspacesLimit = 24;

const WorkspacesView = (props: { details: WorkspacesDetails }) => {
  const { details } = props;
  if (details.items.length === 0 && !details.errors?.length) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2">
      {details.items.length > 0 && (
        <div className="grid gap-2 sm:grid-cols-2">
          {details.items.slice(0, workspacesLimit).map((item) => (
            <WorkspaceCard key={item.uid ?? item.name} item={item} />
          ))}
        </div>
      )}
      {details.items.length > workspacesLimit && (
        <p className="text-[0.7rem]/4 font-medium text-ink-muted">
          and {details.items.length - workspacesLimit} more
        </p>
      )}
      {details.errors?.map((err) => (
        <p
          key={err.workspace}
          className="flex items-start gap-1.5 rounded-lg border border-hue-rose-line bg-hue-rose-soft px-2.5 py-1.5 text-xs font-semibold leading-5 text-hue-rose"
        >
          <IconAlertCircle size={13} className="mt-1 shrink-0" />
          <span className="min-w-0 break-words">
            <span className="font-mono">{err.workspace}</span>: {err.message}
          </span>
        </p>
      ))}
    </div>
  );
};

const TransferView = (props: { details: TransferDetails }) => (
  <div className="flex flex-wrap items-center gap-1.5">
    <Tag icon={<IconFile size={11} />} mono className="max-w-full">
      {props.details.source}
    </Tag>
    <IconArrowRight size={13} className="shrink-0 text-ink-subtle" />
    <Tag icon={<IconFile size={11} />} mono className="max-w-full">
      {props.details.destination}
    </Tag>
    <Tag>
      {props.details.type} · {formatBytes(props.details.bytes)}
    </Tag>
  </div>
);

const ToolDetailsView = (props: { block: ToolBlock }) => {
  const { block } = props;
  const details = block.details;

  switch (details?.kind) {
    case "api":
      return (
        <>
          <Section label="Method">
            <div className="flex flex-wrap items-center gap-2">
              <code className="break-all font-mono text-[0.78rem] font-semibold text-ink-strong">
                {details.method}
              </code>
              <RiskBadge risk={details.risk} />
            </div>
          </Section>
          {details.request !== undefined && (
            <Section label="Request">
              <Code>{toJSON(details.request)}</Code>
            </Section>
          )}
          {details.error ? (
            <Section label="Error">
              <p className="text-xs font-semibold text-hue-rose">
                {details.error.code ? `${details.error.code}: ` : ""}
                {details.error.message}
              </p>
            </Section>
          ) : details.response !== undefined ? (
            <Section
              label={
                details.responseTruncated ? "Response (truncated)" : "Response"
              }
            >
              <Code>{toJSON(details.response)}</Code>
            </Section>
          ) : null}
          {details.resultPath && (
            <Section label="Saved result">
              <code className="break-all font-mono text-[0.76rem] text-ink-body">
                {details.resultPath}
              </code>
            </Section>
          )}
          {details.resources && details.resources.length > 0 && (
            <Section label="Resources">
              <div className="flex flex-wrap gap-1">
                {details.resources.slice(0, 50).map((ref) => (
                  <ResourceChip
                    key={`${ref.apiVersion}/${ref.kind}/${ref.name}`}
                    resource={ref}
                  />
                ))}
              </div>
            </Section>
          )}
        </>
      );
    case "command":
      return (
        <>
          <TerminalView
            command={details.command}
            output={block.output}
            workspace={details.workspace}
            cwd={details.cwd}
            exitCode={details.exitCode}
            durationMs={details.durationMs}
            timedOut={details.timedOut}
            truncated={block.outputTruncated}
          />
          {details.logPath && (
            <p className="mt-1.5 text-[0.7rem]/4 font-medium text-ink-muted">
              The full log is saved in the agent's Workspace at{" "}
              <code className="break-all font-mono">{details.logPath}</code>
            </p>
          )}
        </>
      );
    case "transfer":
      return (
        <>
          <TransferView details={details} />
          {block.output && (
            <Section label="Output">
              <Code>{block.output}</Code>
            </Section>
          )}
        </>
      );
    case "workspaces":
      return block.output ? (
        <Section label={block.outputTruncated ? "Output (truncated)" : "Output"}>
          <Code>{block.output}</Code>
        </Section>
      ) : (
        <p className="text-xs text-ink-muted">No output</p>
      );
    case "file":
      return (
        <>
          <Section label={details.operation}>
            <code className="break-all font-mono text-[0.76rem] text-ink-body">
              {details.path}
            </code>
          </Section>
          {details.diff && (
            <Section label="Diff">
              <DiffView diff={details.diff} />
            </Section>
          )}
          {block.output && !details.diff && (
            <Section
              label={block.outputTruncated ? "Output (truncated)" : "Output"}
            >
              <Code>{block.output}</Code>
            </Section>
          )}
        </>
      );
    default:
      return (
        <>
          {block.input !== undefined && (
            <Section label="Input">
              <Code>{toJSON(block.input)}</Code>
            </Section>
          )}
          {block.output && (
            <Section
              label={block.outputTruncated ? "Output (truncated)" : "Output"}
            >
              <Code>{block.output}</Code>
            </Section>
          )}
        </>
      );
  }
};

type StepBlock = ThinkingBlock | ToolBlock;

const liveOutputTools = ["workspace_exec", "bash"];
const progressTools = ["workspace_create", "workspace_control", "workspace_copy"];

const StepIcon = (props: { block: StepBlock; live: boolean }) => {
  if (props.block.type === "thinking") {
    return props.live ? (
      <Loader size={10} color="gray" />
    ) : (
      <IconBrain size={12} stroke={2.2} className="text-hue-violet" />
    );
  }
  return <ToolStatusIcon block={props.block} />;
};

const thinkingLabel = (live: boolean, duration?: number) => {
  if (live) return "Thinking";
  const formatted = duration !== undefined ? formatDuration(duration) : "";
  return formatted && duration! >= 1000
    ? `Thought for ${formatted}`
    : "Thought briefly";
};

const StepRow = (props: {
  block: StepBlock;
  live: boolean;
  endAt?: string;
  isLast: boolean;
}) => {
  const { block, live } = props;
  const [opened, setOpened] = React.useState(false);

  const duration =
    block.type === "tool"
      ? toolDuration(block)
      : props.endAt
        ? new Date(props.endAt).getTime() - new Date(block.createdAt).getTime()
        : undefined;
  const title =
    block.type === "thinking"
      ? thinkingLabel(live, duration)
      : block.title || block.name;
  const active = block.type === "thinking" ? live : isToolActive(block);
  const hasDetails =
    block.type === "thinking"
      ? block.redacted || block.text.trim() !== ""
      : true;
  const risk =
    block.type === "tool" &&
    block.details?.kind === "api" &&
    block.details.risk !== "read"
      ? block.details.risk
      : undefined;
  const failed =
    block.type === "tool" &&
    (block.status === "failed" || block.status === "rejected");
  const output = block.type === "tool" && active ? block.output : undefined;
  const liveOutput =
    !!output && block.type === "tool" && liveOutputTools.includes(block.name);
  const progress =
    !!output && block.type === "tool" && progressTools.includes(block.name);
  const workspaces =
    block.type === "tool" && block.details?.kind === "workspaces"
      ? block.details
      : undefined;

  return (
    <li className="relative">
      {!props.isLast && (
        <span className="absolute bottom-0 left-[9.5px] top-7 w-px bg-line" />
      )}
      <div className="flex items-start gap-2">
        <span className="relative z-[1] mt-[7px] flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-surface ring-1 ring-line">
          <StepIcon block={block} live={live} />
        </span>
        <div className="min-w-0 flex-1 pb-1">
          <button
            type="button"
            disabled={!hasDetails}
            aria-expanded={opened}
            onClick={() => setOpened((value) => !value)}
            className="group/step flex min-h-8 w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1 text-left outline-none transition-colors duration-150 hover:bg-surface-muted focus-visible:bg-surface-muted disabled:cursor-default disabled:hover:bg-transparent"
          >
            {block.type === "tool" && <ToolIcon block={block} />}
            <span
              className={twMerge(
                "min-w-0 flex-1 truncate text-xs font-semibold",
                failed ? "text-hue-rose" : "text-ink-body",
              )}
            >
              {active ? <Shimmer>{title}</Shimmer> : title}
            </span>
            {risk && <RiskBadge risk={risk} />}
            {block.type === "tool" && block.status !== "completed" && (
              <span className="hidden shrink-0 text-[0.7rem]/4 font-semibold text-ink-muted sm:inline">
                {toolStatusLabel[block.status]}
              </span>
            )}
            {block.type === "tool" && duration !== undefined && (
              <span className="shrink-0 font-mono text-[0.7rem]/4 text-ink-subtle">
                {formatDuration(duration)}
              </span>
            )}
            {hasDetails && (
              <IconChevronRight
                size={13}
                stroke={2.4}
                className={twMerge(
                  "shrink-0 text-ink-subtle transition-transform duration-200",
                  opened && "rotate-90",
                )}
              />
            )}
          </button>
          {liveOutput && !opened && (
            <TerminalView className="mb-1.5 ml-2 mt-1" output={output} live />
          )}
          {progress && !opened && (
            <pre className="mb-1 ml-2 whitespace-pre-wrap break-words font-mono text-[0.74rem] leading-5 text-ink-muted">
              {output}
            </pre>
          )}
          {workspaces && (
            <div className="mb-1.5 ml-2 mt-1">
              <WorkspacesView details={workspaces} />
            </div>
          )}
          <Collapse opened={opened}>
            <div className="mb-1.5 ml-2 mt-1 rounded-lg border border-line bg-surface p-3">
              {block.type === "thinking" ? (
                <p className="max-h-[320px] overflow-y-auto whitespace-pre-wrap text-xs leading-5 text-ink-soft">
                  {block.redacted ? "The reasoning is redacted." : block.text}
                </p>
              ) : (
                <ToolDetailsView block={block} />
              )}
            </div>
          </Collapse>
        </div>
      </div>
    </li>
  );
};

const stepsSummary = (blocks: StepBlock[]): string => {
  const tools = blocks.filter((b) => b.type === "tool").length;
  if (tools === 0) return "Reasoning";
  const thought = blocks.some((b) => b.type === "thinking");
  return `${thought ? "Thought and used" : "Used"} ${tools} tool${tools === 1 ? "" : "s"}`;
};

const liveStepLabel = (blocks: StepBlock[]): string => {
  const last = blocks.at(-1);
  if (!last || last.type === "thinking") return "Thinking";
  if (last.status === "awaiting_approval") return "Waiting for your approval";
  return last.title || last.name;
};

export const StepsView = React.memo(
  (props: {
    blocks: StepBlock[];
    live: boolean;
    ends: Record<string, string | undefined>;
  }) => {
    const { blocks, live } = props;
    const [manual, setManual] = React.useState<boolean>();
    const opened = manual ?? live;

    const tools = blocks.filter((b): b is ToolBlock => b.type === "tool");
    const failed = tools.filter(
      (b) => b.status === "failed" || b.status === "rejected",
    ).length;
    const waiting = tools.some((b) => b.status === "awaiting_approval");
    const active = live || tools.some(isToolActive);
    const first = blocks[0];
    const last = blocks.at(-1)!;
    const end =
      last.type === "tool"
        ? (last.completedAt ?? props.ends[last.id])
        : props.ends[last.id];
    const duration =
      !active && end
        ? new Date(end).getTime() - new Date(first.createdAt).getTime()
        : undefined;

    if (blocks.length === 1) {
      return (
        <ol className="my-1.5">
          <StepRow
            block={first}
            live={live}
            endAt={props.ends[first.id]}
            isLast
          />
        </ol>
      );
    }

    return (
      <div className="my-1.5">
        <button
          type="button"
          aria-expanded={opened}
          onClick={() => setManual(!opened)}
          className="flex min-h-8 w-full cursor-pointer items-center gap-2 rounded-lg px-1 py-1 text-left outline-none transition-colors duration-150 hover:bg-surface-muted focus-visible:bg-surface-muted"
        >
          <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-surface ring-1 ring-line">
            {active ? (
              <Loader size={10} color="gray" />
            ) : waiting ? (
              <IconShieldQuestion size={12} className="text-hue-amber" />
            ) : failed > 0 ? (
              <IconAlertCircle size={12} className="text-hue-rose" />
            ) : (
              <IconListCheck size={12} stroke={2.2} className="text-ink-muted" />
            )}
          </span>
          <span className="min-w-0 flex-1 truncate pl-1 text-xs font-semibold text-ink-soft">
            {active ? <Shimmer>{liveStepLabel(blocks)}</Shimmer> : stepsSummary(blocks)}
          </span>
          {failed > 0 && <Tag tone="danger">{failed} failed</Tag>}
          {duration !== undefined && (
            <span className="shrink-0 font-mono text-[0.7rem]/4 text-ink-subtle">
              {formatDuration(duration)}
            </span>
          )}
          <IconChevronDown
            size={13}
            stroke={2.4}
            className={twMerge(
              "mr-1 shrink-0 text-ink-subtle transition-transform duration-200",
              opened && "rotate-180",
            )}
          />
        </button>
        <Collapse opened={opened}>
          <ol className="pb-1 pt-1">
            {blocks.map((block, idx) => (
              <StepRow
                key={block.id}
                block={block}
                live={live && idx === blocks.length - 1}
                endAt={props.ends[block.id]}
                isLast={idx === blocks.length - 1}
              />
            ))}
          </ol>
        </Collapse>
      </div>
    );
  },
);

const ApprovalView = (props: { block: ApprovalBlock; ctx: BlockContext }) => {
  const { block, ctx } = props;
  const [pending, setPending] = React.useState<ApprovalDecision | undefined>();
  const [rejecting, setRejecting] = React.useState(false);
  const [reason, setReason] = React.useState("");

  const decide = async (decision: ApprovalDecision) => {
    if (!ctx.runId || !ctx.onDecide) return;
    setPending(decision);
    try {
      await ctx.onDecide(
        ctx.runId,
        block.approvalId,
        decision,
        decision === "reject" && reason.trim() !== ""
          ? reason.trim()
          : undefined,
      );
    } finally {
      setPending(undefined);
    }
  };

  if (block.status !== "pending") {
    const approved = block.status === "approved";
    const rejected = block.status === "rejected";
    const Icon = approved ? IconShieldCheck : rejected ? IconShieldX : IconCircleOff;
    return (
      <div className="my-2 flex items-start gap-2.5 rounded-xl border border-line bg-surface-subtle px-3 py-2">
        <Icon
          size={15}
          className={twMerge(
            "mt-0.5 shrink-0",
            approved
              ? "text-hue-emerald"
              : rejected
                ? "text-hue-rose"
                : "text-ink-muted",
          )}
        />
        <div className="min-w-0 flex-1 text-xs leading-5">
          <span className="font-semibold text-ink-body">
            {approved ? "Approved" : rejected ? "Rejected" : "Cancelled"}
          </span>
          <span className="text-ink-muted"> · {block.title}</span>
          {block.reason && (
            <p className="text-ink-muted">Reason: {block.reason}</p>
          )}
        </div>
        <RiskBadge risk={block.risk} />
      </div>
    );
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 6, scale: 0.99 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: 0.3, ease }}
      data-approval-pending
      className="my-3 overflow-hidden rounded-xl border border-hue-amber-line bg-surface shadow-raised ring-4 ring-hue-amber-soft"
    >
      <div className="flex items-start gap-3 border-b border-hue-amber-line bg-hue-amber-soft px-4 py-3">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-hue-amber-solid text-white shadow-panel">
          <IconShieldExclamation size={16} stroke={2.1} />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-[0.7rem]/4 font-semibold uppercase tracking-[0.08em] text-hue-amber">
            Approval required
          </p>
          <p className="mt-0.5 break-words text-[0.82rem]/5 font-bold text-ink">
            {block.title}
          </p>
        </div>
        <RiskBadge risk={block.risk} />
      </div>
      <div className="px-4 py-3">
        {block.description && (
          <p className="mb-3 text-xs leading-5 text-ink-soft">
            {block.description}
          </p>
        )}
        {block.preview?.method && (
          <Section label="Method">
            <code className="break-all font-mono text-[0.78rem] font-semibold text-ink-strong">
              {block.preview.method}
            </code>
          </Section>
        )}
        {block.preview?.workspaces && block.preview.workspaces.length > 0 && (
          <Section label="Workspaces">
            <div className="flex flex-wrap gap-1">
              {block.preview.workspaces.map((name) => (
                <Tag key={name} icon={<IconCube size={11} />} mono>
                  {name}
                </Tag>
              ))}
            </div>
          </Section>
        )}
        {block.preview?.request !== undefined &&
          JSON.stringify(block.preview.request) !== "{}" && (
            <Section label="Request">
              <Code>{toJSON(block.preview.request)}</Code>
            </Section>
          )}
        {block.preview?.command && (
          <Section label="Command">
            <TerminalView
              command={block.preview.command}
              workspace={block.preview.workspace}
            />
          </Section>
        )}
        <Collapse opened={rejecting}>
          <Textarea
            className="mt-3"
            size="xs"
            label="Reason"
            description="Optional. The agent sees it and can adjust its plan."
            placeholder="e.g. Use a new ephemeral Workspace instead"
            autosize
            minRows={1}
            maxRows={4}
            maxLength={2000}
            value={reason}
            onChange={(event) => setReason(event.currentTarget.value)}
            styles={{ input: { resize: "none" } }}
          />
        </Collapse>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-2 border-t border-line-subtle bg-surface-subtle px-4 py-2.5">
        <span className="mr-auto text-[0.7rem]/4 text-ink-muted">
          The agent always acts within your own permissions
        </span>
        {rejecting ? (
          <>
            <Button
              size="xs"
              variant="subtle"
              color="gray"
              disabled={!!pending}
              onClick={() => setRejecting(false)}
            >
              Cancel
            </Button>
            <Button
              size="xs"
              color="red"
              loading={pending === "reject"}
              disabled={!!pending || !ctx.runId}
              onClick={() => void decide("reject")}
            >
              Reject
            </Button>
          </>
        ) : (
          <>
            <Button
              size="xs"
              variant="default"
              leftSection={<IconX size={13} stroke={2.5} />}
              disabled={!!pending || !ctx.runId}
              onClick={() => setRejecting(true)}
            >
              Reject
            </Button>
            <Button
              size="xs"
              leftSection={<IconCheck size={13} stroke={2.5} />}
              loading={pending === "approve"}
              disabled={!!pending || !ctx.runId}
              onClick={() => void decide("approve")}
            >
              Approve
            </Button>
          </>
        )}
      </div>
    </motion.div>
  );
};

const formatCell = (value: unknown, column: TableColumn): React.ReactNode => {
  if (value === null || value === undefined || value === "") {
    return <span className="text-ink-faint">—</span>;
  }

  switch (column.type) {
    case "number":
      return typeof value === "number"
        ? `${value.toLocaleString(undefined, { maximumFractionDigits: 4 })}${column.unit ? ` ${column.unit}` : ""}`
        : formatValue(value);
    case "boolean":
      return value ? "Yes" : "No";
    case "datetime": {
      const date = new Date(String(value));
      return Number.isNaN(date.getTime())
        ? formatValue(value)
        : date.toLocaleString();
    }
    case "json":
      return (
        <code className="font-mono text-[0.72rem] text-ink-soft">
          {formatValue(value).slice(0, 200)}
        </code>
      );
    default:
      return formatValue(value);
  }
};

const tablePageSize = 50;

const DataTable = (props: {
  block: TableBlock;
  limit: number;
  className?: string;
}) => {
  const { block } = props;
  const rows = block.rows.slice(0, props.limit);

  return (
    <div className={twMerge("overflow-auto", props.className)}>
      <table className="w-full border-collapse text-xs">
        <thead className="sticky top-0 z-[1] bg-surface-subtle text-ink-soft">
          <tr>
            {block.columns.map((column) => (
              <th
                key={column.key}
                className={twMerge(
                  "whitespace-nowrap border-b border-line px-3 py-2 text-left font-bold",
                  column.type === "number" && "text-right",
                )}
              >
                {column.label ?? column.key}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, idx) => (
            <tr
              key={idx}
              className="transition-colors duration-100 hover:bg-surface-hover"
            >
              {block.columns.map((column) => (
                <td
                  key={column.key}
                  title={formatValue(row[column.key])}
                  className={twMerge(
                    "max-w-[320px] truncate border-b border-line-subtle px-3 py-1.5 align-top text-ink-body",
                    column.type === "number" && "text-right tabular-nums",
                  )}
                >
                  {formatCell(row[column.key], column)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

const cardModalStyles = {
  content: {
    border: "1px solid var(--app-line)",
    borderRadius: "14px",
    boxShadow: "var(--app-shadow-overlay)",
    overflow: "hidden",
  },
};

const TableView = (props: { block: TableBlock }) => {
  const { block } = props;
  const [limit, setLimit] = React.useState(tablePageSize);
  const [expanded, setExpanded] = React.useState(false);

  const download = () =>
    saveBlob(
      new Blob([toCSV(block.columns, block.rows)], { type: "text/csv" }),
      toFileName(block.title ?? "table", "csv"),
    );

  return (
    <div className="my-3 overflow-hidden rounded-xl border border-line bg-surface shadow-panel">
      <CardHeader
        icon={IconTable}
        title={block.title ?? "Table"}
        meta={`${block.totalRows.toLocaleString()} rows`}
      >
        <Tooltip label="Download as CSV">
          <ActionIcon
            size="sm"
            variant="subtle"
            color="gray"
            aria-label="Download as CSV"
            onClick={download}
          >
            <IconDownload size={13} />
          </ActionIcon>
        </Tooltip>
        <Tooltip label="Expand">
          <ActionIcon
            size="sm"
            variant="subtle"
            color="gray"
            aria-label="Expand the table"
            onClick={() => setExpanded(true)}
          >
            <IconArrowsMaximize size={13} />
          </ActionIcon>
        </Tooltip>
      </CardHeader>
      <DataTable block={block} limit={limit} className="max-h-[420px]" />
      {(block.caption || block.rows.length > limit || block.truncated) && (
        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line-subtle px-3 py-1.5 text-[0.7rem]/4 text-ink-muted">
          <span>
            {block.caption}
            {block.truncated &&
              ` Showing ${block.rows.length.toLocaleString()} of ${block.totalRows.toLocaleString()} rows.`}
          </span>
          {block.rows.length > limit && (
            <Button
              size="compact-xs"
              variant="subtle"
              color="gray"
              onClick={() => setLimit((value) => value + tablePageSize * 4)}
            >
              Show more
            </Button>
          )}
        </div>
      )}
      <Modal
        opened={expanded}
        onClose={() => setExpanded(false)}
        size="calc(100vw - 48px)"
        padding={0}
        withCloseButton={false}
        overlayProps={{ backgroundOpacity: 0.3, blur: 1 }}
        transitionProps={{ transition: "pop", duration: 200 }}
        styles={cardModalStyles}
      >
        <CardHeader
          icon={IconTable}
          title={block.title ?? "Table"}
          meta={`${block.totalRows.toLocaleString()} rows`}
        >
          <Button
            size="compact-xs"
            variant="subtle"
            color="gray"
            leftSection={<IconDownload size={12} />}
            onClick={download}
          >
            CSV
          </Button>
          <ActionIcon
            size="sm"
            variant="subtle"
            color="gray"
            aria-label="Close"
            onClick={() => setExpanded(false)}
          >
            <IconX size={14} />
          </ActionIcon>
        </CardHeader>
        <DataTable
          block={block}
          limit={block.rows.length}
          className="max-h-[calc(100dvh-140px)]"
        />
      </Modal>
    </div>
  );
};

const ChartView = (props: { block: ChartBlock }) => {
  const { chart } = props.block;
  const ref = React.useRef<ChartHandle>(null);

  const download = async () => {
    const url = ref.current?.toPNG();
    if (!url) return;
    saveBlob(
      await (await fetch(url)).blob(),
      toFileName(chart.title ?? "chart", "png"),
    );
  };

  return (
    <div className="my-3 overflow-hidden rounded-xl border border-line bg-surface shadow-panel">
      <CardHeader icon={IconChartBar} title={chart.title ?? "Chart"}>
        <Tooltip label="Download as PNG">
          <ActionIcon
            size="sm"
            variant="subtle"
            color="gray"
            aria-label="Download as PNG"
            onClick={() => void download()}
          >
            <IconPhotoDown size={13} />
          </ActionIcon>
        </Tooltip>
      </CardHeader>
      <div className="px-3 pb-2 pt-2.5">
        {chart.description && (
          <p className="mb-1 text-[0.7rem]/4 text-ink-muted">
            {chart.description}
          </p>
        )}
        <React.Suspense
          fallback={
            <div className="flex h-[280px] items-center justify-center">
              <Loader size="sm" color="gray" />
            </div>
          }
        >
          <Chart ref={ref} chart={chart} />
        </React.Suspense>
      </div>
    </div>
  );
};

const ResourcesView = (props: { block: ResourcesBlock }) => {
  const { block } = props;
  const [opened, setOpened] = React.useState<string | undefined>();

  return (
    <div className="my-3 overflow-hidden rounded-xl border border-line bg-surface shadow-panel">
      <CardHeader
        icon={IconStack2}
        title={block.title ?? "Resources"}
        meta={block.resources.length.toLocaleString()}
      />
      <ul className="max-h-[420px] divide-y divide-line-subtle overflow-y-auto">
        {block.resources.map((item) => {
          const route = getResourceRoute(item.ref);
          const isOpened = opened === item.uri;
          return (
            <li key={item.uri} className="px-3 py-1.5">
              <div className="flex min-h-7 items-center gap-2">
                <Tag className="px-1.5 text-[0.68rem] leading-4">
                  {item.ref.kind}
                </Tag>
                {route ? (
                  <Link
                    to={route}
                    className="min-w-0 flex-1 truncate font-mono text-[0.78rem] font-bold text-ink hover:underline"
                  >
                    {item.ref.name}
                  </Link>
                ) : (
                  <span className="min-w-0 flex-1 truncate font-mono text-[0.78rem] font-bold text-ink">
                    {item.ref.name}
                  </span>
                )}
                <span className="hidden text-[0.7rem]/4 text-ink-subtle sm:inline">
                  {item.ref.apiVersion}
                </span>
                {item.snapshot && (
                  <ActionIcon
                    size="sm"
                    variant="subtle"
                    color="gray"
                    aria-label="Show details"
                    aria-expanded={isOpened}
                    onClick={() => setOpened(isOpened ? undefined : item.uri)}
                  >
                    <IconChevronDown
                      size={13}
                      className={twMerge(
                        "transition-transform duration-200",
                        isOpened && "rotate-180",
                      )}
                    />
                  </ActionIcon>
                )}
              </div>
              <Collapse opened={isOpened}>
                <div className="pb-1 pt-1.5">
                  <Code>{toJSON(item.snapshot)}</Code>
                </div>
              </Collapse>
            </li>
          );
        })}
      </ul>
    </div>
  );
};

const ArtifactView = (props: { block: ArtifactBlock; ctx: BlockContext }) => {
  const { artifact } = props.block;
  const { client } = props.ctx;
  const [downloading, setDownloading] = React.useState(false);
  const [preview, setPreview] = React.useState<string | undefined>();
  const [zoomed, setZoomed] = React.useState(false);
  const [error, setError] = React.useState<string | undefined>();
  const isImage = artifact.mimeType.startsWith("image/");

  React.useEffect(() => {
    if (!isImage || artifact.size > 10 * 1024 * 1024) return;
    let url: string | undefined;
    let active = true;
    client
      .downloadArtifact(artifact)
      .then((blob) => {
        if (!active) return;
        url = URL.createObjectURL(blob);
        setPreview(url);
      })
      .catch(() => undefined);
    return () => {
      active = false;
      if (url) URL.revokeObjectURL(url);
    };
  }, [client, artifact, isImage]);

  const download = async () => {
    setDownloading(true);
    setError(undefined);
    try {
      saveBlob(await client.downloadArtifact(artifact), artifact.name);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setDownloading(false);
    }
  };

  return (
    <div className="my-3 overflow-hidden rounded-xl border border-line bg-surface shadow-panel">
      {preview && (
        <button
          type="button"
          aria-label="Enlarge the image"
          onClick={() => setZoomed(true)}
          className="block w-full cursor-zoom-in border-b border-line-subtle bg-surface-subtle"
        >
          <img
            src={preview}
            alt={artifact.title ?? artifact.name}
            className="max-h-[360px] w-full object-contain"
          />
        </button>
      )}
      <div className="flex items-center gap-3 px-3 py-2.5">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-line bg-surface-subtle text-ink-soft">
          <FileIcon
            mimeType={artifact.mimeType}
            name={artifact.name}
            size={16}
          />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-[0.82rem]/5 font-semibold text-ink-strong">
            {artifact.title ?? artifact.name}
          </p>
          <p className="truncate text-[0.7rem]/4 text-ink-muted">
            {artifact.name} · {formatBytes(artifact.size)}
          </p>
          {artifact.description && (
            <p className="mt-0.5 text-xs text-ink-soft">
              {artifact.description}
            </p>
          )}
          {error && <p className="mt-0.5 text-xs text-hue-rose">{error}</p>}
        </div>
        <Button
          size="xs"
          variant="default"
          leftSection={<IconDownload size={13} />}
          loading={downloading}
          onClick={() => void download()}
        >
          <span className="hidden sm:inline">Download</span>
          <span className="sm:hidden">Save</span>
        </Button>
      </div>
      {preview && (
        <Modal
          opened={zoomed}
          onClose={() => setZoomed(false)}
          size="auto"
          padding={0}
          withCloseButton={false}
          overlayProps={{ backgroundOpacity: 0.55, blur: 2 }}
          transitionProps={{ transition: "pop", duration: 200 }}
          styles={{
            content: { background: "transparent", boxShadow: "none" },
          }}
        >
          <img
            src={preview}
            alt={artifact.title ?? artifact.name}
            onClick={() => setZoomed(false)}
            className="max-h-[88dvh] max-w-[92vw] cursor-zoom-out rounded-xl object-contain shadow-overlay"
          />
        </Modal>
      )}
    </div>
  );
};

const NoticeView = (props: { block: NoticeBlock }) => (
  <div
    className={twMerge(
      "my-2 flex items-start gap-2 rounded-lg border px-3 py-2 text-xs leading-5",
      props.block.level === "warning"
        ? "border-hue-amber-line bg-hue-amber-soft text-hue-amber"
        : "border-line bg-surface-subtle text-ink-soft",
    )}
  >
    {props.block.level === "warning" ? (
      <IconAlertTriangle size={14} className="mt-0.5 shrink-0" />
    ) : (
      <IconInfoCircle size={14} className="mt-0.5 shrink-0" />
    )}
    <span className="min-w-0 break-words">{props.block.text}</span>
  </div>
);

const ErrorView = (props: { block: ErrorBlock }) => (
  <div className="my-2 flex items-start gap-2 rounded-lg border border-hue-rose-line bg-hue-rose-soft px-3 py-2 text-xs font-semibold leading-5 text-hue-rose">
    <IconAlertCircle size={14} className="mt-0.5 shrink-0" />
    <span className="min-w-0 break-words">{props.block.message}</span>
  </div>
);

export const BlockView = React.memo(
  (props: { block: Block; ctx: BlockContext }) => {
    const { block, ctx } = props;

    switch (block.type) {
      case "markdown":
        return <AgentMarkdown>{block.text}</AgentMarkdown>;
      case "thinking":
      case "tool":
        return <StepsView blocks={[block]} live={false} ends={{}} />;
      case "approval":
        return <ApprovalView block={block} ctx={ctx} />;
      case "table":
        return <TableView block={block} />;
      case "chart":
        return <ChartView block={block} />;
      case "resources":
        return <ResourcesView block={block} />;
      case "artifact":
        return <ArtifactView block={block} ctx={ctx} />;
      case "notice":
        return <NoticeView block={block} />;
      case "error":
        return <ErrorView block={block} />;
      default:
        return null;
    }
  },
);
