import type {
  APICallDetails,
  ApprovalBlock,
  ApprovalDecision,
  ArtifactBlock,
  Block,
  ChartBlock,
  CommandDetails,
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
import { Button, Loader, Tooltip } from "@mantine/core";
import {
  IconAlertTriangle,
  IconApi,
  IconArrowRight,
  IconBan,
  IconBrain,
  IconChevronDown,
  IconCircleCheck,
  IconCircleX,
  IconCopy,
  IconDownload,
  IconExternalLink,
  IconFile,
  IconFileText,
  IconInfoCircle,
  IconListDetails,
  IconPlayerStop,
  IconSearch,
  IconShieldQuestion,
  IconStack2,
  IconTable,
  IconTemplate,
  IconTerminal2,
  IconTool,
  IconWorld,
  IconCube,
} from "@tabler/icons-react";
import { AnimatePresence, motion } from "framer-motion";
import * as React from "react";
import { Link } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import { AgentClient } from "./client";
import AgentMarkdown, { CopyButton } from "./Markdown";
import {
  formatDuration,
  formatSize,
  formatValue,
  getResourceRoute,
  riskLabel,
  riskTone,
  saveBlob,
  toCSV,
  toJSON,
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
  ) => Promise<void>;
}

const Collapse = (props: { opened: boolean; children: React.ReactNode }) => (
  <AnimatePresence initial={false}>
    {props.opened && (
      <motion.div
        initial={{ height: 0, opacity: 0 }}
        animate={{ height: "auto", opacity: 1 }}
        exit={{ height: 0, opacity: 0 }}
        transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
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
        "max-h-[320px] overflow-auto rounded-lg border border-line-subtle bg-surface-subtle px-3 py-2",
        props.className,
      )}
    >
      <code className="whitespace-pre font-mono text-[0.76rem] leading-5 text-ink-body">
        {props.children}
      </code>
    </pre>
    <CopyButton
      text={props.children}
      className="absolute right-1.5 top-1.5 opacity-0 transition-opacity group-hover/code:opacity-100"
    />
  </div>
);

const ConsoleOutput = (props: {
  command?: string;
  output?: string;
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
        "console-surface group/console relative overflow-hidden rounded-lg bg-console",
        props.className,
      )}
    >
      <pre
        ref={ref}
        className="max-h-[360px] overflow-auto px-3.5 py-2.5 font-mono text-[0.76rem] leading-5 text-zinc-200"
      >
        {props.command && (
          <code className="block whitespace-pre-wrap break-all text-zinc-400">
            <span className="select-none text-emerald-400">$ </span>
            {props.command}
          </code>
        )}
        {props.output && (
          <code className="mt-1 block whitespace-pre-wrap break-words">
            {props.output}
          </code>
        )}
        {props.live && (
          <span className="mt-1 inline-block h-3.5 w-1.5 animate-pulse bg-zinc-400 align-middle" />
        )}
      </pre>
      {props.output && (
        <CopyButton
          text={props.output}
          className="absolute right-1.5 top-1.5 text-zinc-400 opacity-0 transition-opacity group-hover/console:opacity-100"
        />
      )}
    </div>
  );
};

const Section = (props: {
  label: React.ReactNode;
  children: React.ReactNode;
}) => (
  <div className="mt-3 first:mt-0">
    <p className="mb-1.5 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-ink-subtle">
      {props.label}
    </p>
    {props.children}
  </div>
);

const ThinkingView = (props: { block: ThinkingBlock; streaming: boolean }) => {
  const [opened, setOpened] = React.useState(false);
  const { block } = props;

  return (
    <div className="my-1.5">
      <button
        type="button"
        onClick={() => setOpened((value) => !value)}
        aria-expanded={opened}
        className="inline-flex cursor-pointer items-center gap-1.5 rounded-md px-1.5 py-1 text-[0.78rem] font-semibold text-ink-muted outline-none transition-colors duration-150 hover:bg-surface-muted hover:text-ink-soft"
      >
        <IconBrain size={14} className="text-hue-violet" />
        <span className={props.streaming ? "animate-pulse" : undefined}>
          {props.streaming ? "Thinking…" : "Thought process"}
        </span>
        <IconChevronDown
          size={13}
          className={twMerge(
            "transition-transform duration-200",
            opened && "rotate-180",
          )}
        />
      </button>
      <Collapse opened={opened}>
        <div className="ml-2.5 mt-1 max-h-[320px] overflow-y-auto border-l-2 border-hue-violet-line py-1 pl-3.5">
          <p className="whitespace-pre-wrap text-[0.84rem] leading-6 text-ink-muted">
            {block.redacted ? "The reasoning is redacted." : block.text}
          </p>
        </div>
      </Collapse>
    </div>
  );
};

const ToolIcon = (props: { block: ToolBlock }) => {
  const { block } = props;
  switch (block.details?.kind) {
    case "api":
      return <IconApi size={15} />;
    case "command":
      return <IconTerminal2 size={15} />;
    case "file":
      return <IconFileText size={15} />;
    case "workspaces":
      return <IconCube size={15} />;
    case "transfer":
      return <IconCopy size={15} />;
  }
  switch (block.name) {
    case "bash":
    case "workspace_exec":
      return <IconTerminal2 size={15} />;
    case "workspace_list":
    case "workspace_create":
    case "workspace_control":
    case "workspace_logs":
      return <IconCube size={15} />;
    case "workspace_copy":
      return <IconCopy size={15} />;
    case "cordium_api_call":
      return <IconApi size={15} />;
    case "cordium_api_search":
    case "cordium_api_describe":
    case "grep":
    case "find":
      return <IconSearch size={15} />;
    case "read":
    case "write":
    case "edit":
    case "ls":
      return <IconFileText size={15} />;
    case "web_fetch":
    case "web_search":
      return <IconWorld size={15} />;
    default:
      return <IconTool size={15} />;
  }
};

const ToolStatus = (props: { block: ToolBlock }) => {
  const { block } = props;
  switch (block.status) {
    case "pending":
    case "running":
      return <Loader size={13} color="gray" />;
    case "awaiting_approval":
      return (
        <span className="inline-flex items-center gap-1 text-[0.72rem] font-semibold text-hue-amber">
          <IconShieldQuestion size={14} />
          Awaiting approval
        </span>
      );
    case "completed":
      return <IconCircleCheck size={15} className="text-hue-emerald" />;
    case "failed":
      return <IconCircleX size={15} className="text-hue-rose" />;
    case "rejected":
      return (
        <span className="inline-flex items-center gap-1 text-[0.72rem] font-semibold text-hue-rose">
          <IconBan size={14} />
          Rejected
        </span>
      );
    case "cancelled":
      return (
        <span className="inline-flex items-center gap-1 text-[0.72rem] font-semibold text-ink-subtle">
          <IconPlayerStop size={14} />
          Stopped
        </span>
      );
  }
};

const toolDuration = (block: ToolBlock): string | undefined => {
  if (!block.startedAt || !block.completedAt) return undefined;
  const ms =
    new Date(block.completedAt).getTime() - new Date(block.startedAt).getTime();
  return ms >= 0 ? formatDuration(ms) : undefined;
};

const ResourceChip = (props: { resource: ResourceRef }) => {
  const route = getResourceRoute(props.resource);
  const label = (
    <>
      <span className="text-ink-subtle">{props.resource.kind}</span>
      <span className="font-semibold">{props.resource.name}</span>
    </>
  );
  const className =
    "inline-flex items-center gap-1 rounded-md border border-line bg-surface px-1.5 py-0.5 text-[0.72rem] text-ink-strong";

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

const WorkspaceCard = (props: { item: WorkspaceSummary }) => {
  const { item } = props;
  const deleted = item.state === "DELETED";
  const short = (name?: string) => name?.split(".").slice(0, -1).join(".");

  return (
    <div
      className={twMerge(
        "flex min-w-0 flex-col gap-2 rounded-xl border border-line bg-surface px-3 py-2.5 shadow-panel",
        deleted && "opacity-60",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            {deleted ? (
              <span className="font-mono text-[0.85rem] font-bold text-ink line-through">
                {item.name}
              </span>
            ) : (
              <Link
                to={`/workspaces/${encodeURIComponent(item.name)}`}
                className="font-mono text-[0.85rem] font-bold text-ink hover:underline"
              >
                {item.name}
              </Link>
            )}
            {item.isAgent && <Tag tone="accent">Agent</Tag>}
          </div>
          {item.displayName && (
            <p className="truncate text-[0.75rem] font-medium text-ink-muted">
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
            {short(item.space) || item.space}
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
        <p className="truncate font-mono text-[0.72rem] text-ink-muted">
          {item.repository.replace(/^https:\/\//, "")}
        </p>
      )}
      {item.failure && (
        <p className="flex items-start gap-1 text-[0.75rem] font-semibold text-hue-rose">
          <IconAlertTriangle size={13} className="mt-0.5 shrink-0" />
          <span className="min-w-0 break-words">{item.failure}</span>
        </p>
      )}
      {item.hostname && item.state === "RUNNING" && !item.isAgent && (
        <a
          href={`https://${item.hostname}`}
          target="_blank"
          rel="noreferrer noopener"
          className="inline-flex w-fit items-center gap-1 text-[0.72rem] font-semibold text-ink-soft hover:text-ink"
        >
          <IconExternalLink size={12} />
          {item.hostname}
        </a>
      )}
    </div>
  );
};

const WorkspacesView = (props: { details: WorkspacesDetails }) => {
  const { details } = props;
  if (details.items.length === 0 && !details.errors?.length) {
    return null;
  }
  return (
    <div className="flex flex-col gap-2">
      {details.items.length > 0 && (
        <div className="grid gap-2 sm:grid-cols-2">
          {details.items.slice(0, 24).map((item) => (
            <WorkspaceCard key={item.uid ?? item.name} item={item} />
          ))}
        </div>
      )}
      {details.items.length > 24 && (
        <p className="text-[0.75rem] font-medium text-ink-muted">
          and {details.items.length - 24} more
        </p>
      )}
      {details.errors?.map((err) => (
        <p
          key={err.workspace}
          className="flex items-start gap-1.5 rounded-lg border border-hue-rose-line bg-hue-rose-soft px-2.5 py-1.5 text-[0.75rem] font-semibold text-hue-rose"
        >
          <IconCircleX size={14} className="mt-0.5 shrink-0" />
          <span className="min-w-0 break-words">
            <span className="font-mono">{err.workspace}</span>: {err.message}
          </span>
        </p>
      ))}
    </div>
  );
};

const CommandView = (props: { block: ToolBlock; details: CommandDetails }) => {
  const { block, details } = props;
  const ok = details.exitCode === 0;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-1.5">
        {details.workspace ? (
          <Link to={`/workspaces/${encodeURIComponent(details.workspace)}`}>
            <Tag icon={<IconCube size={11} />} mono>
              {details.workspace}
            </Tag>
          </Link>
        ) : (
          <Tag icon={<IconCube size={11} />}>Agent Workspace</Tag>
        )}
        {details.cwd && <Tag mono>{details.cwd}</Tag>}
        {details.exitCode !== undefined && (
          <Tag tone={ok ? "success" : "danger"}>exit {details.exitCode}</Tag>
        )}
        {details.timedOut && <Tag tone="warning">Timed out</Tag>}
        {details.durationMs !== undefined && (
          <Tag>{formatDuration(details.durationMs)}</Tag>
        )}
      </div>
      <ConsoleOutput command={details.command} output={block.output} />
      {(block.outputTruncated || details.logPath) && (
        <p className="text-[0.72rem] font-medium text-ink-muted">
          The output is truncated
          {details.logPath && (
            <>
              ; the full log is saved in the agent's Workspace at{" "}
              <code className="font-mono">{details.logPath}</code>
            </>
          )}
          .
        </p>
      )}
    </div>
  );
};

const APIView = (props: { details: APICallDetails }) => {
  const { details } = props;
  return (
    <>
      <Section label="Method">
        <div className="flex flex-wrap items-center gap-2">
          <code className="font-mono text-[0.8rem] font-semibold text-ink-strong">
            {details.method}
          </code>
          <Tag tone={riskTone(details.risk)}>{riskLabel[details.risk]}</Tag>
        </div>
      </Section>
      {details.request !== undefined && (
        <Section label="Request">
          <Code>{toJSON(details.request)}</Code>
        </Section>
      )}
      {details.error ? (
        <Section label="Error">
          <p className="text-[0.8rem] font-semibold text-hue-rose">
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
          <code className="font-mono text-[0.76rem] text-ink-soft">
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
};

const TransferView = (props: { details: TransferDetails }) => (
  <div className="flex flex-wrap items-center gap-2 text-[0.8rem]">
    <Tag icon={<IconFile size={11} />} mono>
      {props.details.source}
    </Tag>
    <IconArrowRight size={14} className="text-ink-subtle" />
    <Tag icon={<IconFile size={11} />} mono>
      {props.details.destination}
    </Tag>
    <Tag>
      {props.details.type} · {formatSize(props.details.bytes)}
    </Tag>
  </div>
);

const DiffView = (props: { diff: string }) => (
  <pre className="max-h-[320px] overflow-auto rounded-lg border border-line-subtle bg-surface-subtle py-2 font-mono text-[0.76rem] leading-5">
    {props.diff.split("\n").map((line, idx) => (
      <div
        key={idx}
        className={twMerge(
          "px-3",
          line.startsWith("+") && "bg-hue-emerald-soft text-hue-emerald",
          line.startsWith("-") && "bg-hue-rose-soft text-hue-rose",
          !line.startsWith("+") && !line.startsWith("-") && "text-ink-body",
        )}
      >
        {line || " "}
      </div>
    ))}
  </pre>
);

const ToolDetailsView = (props: { block: ToolBlock }) => {
  const { block } = props;
  const details = block.details;

  switch (details?.kind) {
    case "api":
      return <APIView details={details} />;
    case "command":
      return <CommandView block={block} details={details} />;
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
    case "file":
      return (
        <>
          <Section label={details.operation}>
            <code className="font-mono text-[0.78rem] text-ink-soft">
              {details.path}
            </code>
          </Section>
          {details.diff && (
            <Section label="Diff">
              <DiffView diff={details.diff} />
            </Section>
          )}
          {block.output && !details.diff && (
            <Section label="Output">
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

const ToolView = (props: { block: ToolBlock }) => {
  const { block } = props;
  const [opened, setOpened] = React.useState(false);
  const active = block.status === "running" || block.status === "pending";
  const live =
    active &&
    !!block.output &&
    (block.name === "workspace_exec" || block.name === "bash");
  const progress =
    active &&
    !!block.output &&
    (block.name === "workspace_create" ||
      block.name === "workspace_control" ||
      block.name === "workspace_copy");
  const duration = toolDuration(block);
  const workspaces =
    block.details?.kind === "workspaces" ? block.details : undefined;

  return (
    <div className="px-3 py-2">
      <button
        type="button"
        onClick={() => setOpened((value) => !value)}
        aria-expanded={opened}
        className="group/tool flex w-full cursor-pointer items-center gap-2.5 text-left outline-none"
      >
        <span
          className={twMerge(
            "flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-ink-soft",
            active && "text-ink",
          )}
        >
          <ToolIcon block={block} />
        </span>
        <span
          className={twMerge(
            "min-w-0 flex-1 truncate text-[0.84rem] font-semibold text-ink-strong",
            active && "animate-pulse",
          )}
        >
          {block.title || block.name}
        </span>
        {duration && !active && (
          <span className="hidden text-[0.72rem] font-medium text-ink-subtle sm:inline">
            {duration}
          </span>
        )}
        <span className="flex shrink-0 items-center">
          <ToolStatus block={block} />
        </span>
        <IconChevronDown
          size={14}
          className={twMerge(
            "shrink-0 text-ink-subtle transition-transform duration-200 group-hover/tool:text-ink-soft",
            opened && "rotate-180",
          )}
        />
      </button>

      {live && !opened && (
        <ConsoleOutput className="ml-[38px] mt-2" output={block.output} live />
      )}
      {progress && !opened && (
        <pre className="ml-[38px] mt-1.5 whitespace-pre-wrap font-mono text-[0.74rem] leading-5 text-ink-muted">
          {block.output}
        </pre>
      )}
      {workspaces && (
        <div className="ml-[38px] mt-2">
          <WorkspacesView details={workspaces} />
        </div>
      )}

      <Collapse opened={opened}>
        <div className="ml-[38px] mt-2 rounded-xl border border-line-subtle bg-surface-subtle/60 p-3">
          {workspaces ? (
            block.output && (
              <Section label="Output">
                <Code>{block.output}</Code>
              </Section>
            )
          ) : (
            <ToolDetailsView block={block} />
          )}
        </div>
      </Collapse>
    </div>
  );
};

export const ToolGroup = (props: { blocks: ToolBlock[] }) => (
  <div className="my-2 divide-y divide-line-subtle overflow-hidden rounded-xl border border-line bg-surface-subtle/50">
    {props.blocks.map((block) => (
      <ToolView key={block.id} block={block} />
    ))}
  </div>
);

const ApprovalView = (props: { block: ApprovalBlock; ctx: BlockContext }) => {
  const { block, ctx } = props;
  const [pending, setPending] = React.useState<ApprovalDecision | undefined>();
  const isPending = block.status === "pending";

  const decide = async (decision: ApprovalDecision) => {
    if (!ctx.runId || !ctx.onDecide) return;
    setPending(decision);
    try {
      await ctx.onDecide(ctx.runId, block.approvalId, decision);
    } finally {
      setPending(undefined);
    }
  };

  if (!isPending) {
    return (
      <div className="my-1.5 flex items-center gap-2 px-1 text-[0.78rem] font-medium text-ink-muted">
        {block.status === "approved" ? (
          <IconCircleCheck size={15} className="text-hue-emerald" />
        ) : (
          <IconBan size={15} className="text-ink-subtle" />
        )}
        <span className="min-w-0 truncate">
          {block.status === "approved"
            ? "Approved"
            : block.status === "rejected"
              ? "Rejected"
              : "Cancelled"}
          : {block.title}
          {block.reason && block.status === "rejected"
            ? ` (${block.reason})`
            : ""}
        </span>
      </div>
    );
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      className="my-2 overflow-hidden rounded-xl border border-hue-amber-line bg-hue-amber-soft"
    >
      <div className="flex items-start gap-3 px-4 py-3">
        <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface text-hue-amber shadow-panel">
          <IconShieldQuestion size={17} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-[0.9rem] font-bold text-ink">{block.title}</p>
            <Tag tone={riskTone(block.risk)}>{riskLabel[block.risk]}</Tag>
          </div>
          {block.description && (
            <p className="mt-1 text-[0.8rem] leading-5 text-ink-soft">
              {block.description}
            </p>
          )}
          {block.preview?.method && (
            <Section label="Method">
              <code className="font-mono text-[0.78rem] font-semibold text-ink-strong">
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
                <Code className="bg-surface">
                  {toJSON(block.preview.request)}
                </Code>
              </Section>
            )}
          {block.preview?.command && (
            <Section
              label={
                block.preview.workspace
                  ? `Command in ${block.preview.workspace}`
                  : "Command"
              }
            >
              <ConsoleOutput command={block.preview.command} />
            </Section>
          )}
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              size="xs"
              loading={pending === "approve"}
              disabled={!!pending || !ctx.runId}
              leftSection={<IconCircleCheck size={14} />}
              onClick={() => void decide("approve")}
            >
              Approve
            </Button>
            <Button
              size="xs"
              variant="default"
              loading={pending === "reject"}
              disabled={!!pending || !ctx.runId}
              leftSection={<IconBan size={14} />}
              onClick={() => void decide("reject")}
            >
              Reject
            </Button>
          </div>
        </div>
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
        <code className="font-mono text-[0.72rem] text-ink-muted">
          {formatValue(value).slice(0, 200)}
        </code>
      );
    default:
      return formatValue(value);
  }
};

const tablePageSize = 50;

const BlockCard = (props: {
  icon: React.ReactNode;
  title: React.ReactNode;
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
}) => (
  <div className="my-2.5 overflow-hidden rounded-xl border border-line bg-surface shadow-panel">
    <div className="flex items-center gap-2 border-b border-line-subtle bg-surface-subtle px-3 py-2">
      <span className="shrink-0 text-ink-subtle">{props.icon}</span>
      <span className="min-w-0 flex-1 truncate text-[0.8rem] font-bold text-ink-strong">
        {props.title}
      </span>
      {props.meta && (
        <span className="text-[0.72rem] font-semibold text-ink-subtle">
          {props.meta}
        </span>
      )}
      {props.actions}
    </div>
    {props.children}
    {props.footer && (
      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line-subtle px-3 py-1.5 text-[0.72rem] text-ink-muted">
        {props.footer}
      </div>
    )}
  </div>
);

const TableView = (props: { block: TableBlock }) => {
  const { block } = props;
  const [limit, setLimit] = React.useState(tablePageSize);
  const rows = block.rows.slice(0, limit);

  return (
    <BlockCard
      icon={<IconTable size={14} />}
      title={block.title ?? "Table"}
      meta={`${block.totalRows.toLocaleString()} rows`}
      actions={
        <Tooltip label="Download as CSV">
          <Button
            size="compact-xs"
            variant="subtle"
            color="gray"
            leftSection={<IconDownload size={12} />}
            onClick={() =>
              saveBlob(
                new Blob([toCSV(block.columns, block.rows)], {
                  type: "text/csv",
                }),
                `${(block.title ?? "table").replace(/[^\w.-]+/g, "_")}.csv`,
              )
            }
          >
            CSV
          </Button>
        </Tooltip>
      }
      footer={
        (block.caption || block.rows.length > limit || block.truncated) && (
          <>
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
          </>
        )
      }
    >
      <div className="max-h-[420px] overflow-auto">
        <table className="w-full border-collapse text-[0.8rem]">
          <thead className="sticky top-0 bg-surface text-ink-soft">
            <tr>
              {block.columns.map((column) => (
                <th
                  key={column.key}
                  className="whitespace-nowrap border-b border-line px-3 py-2 text-left font-bold"
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
    </BlockCard>
  );
};

const ChartView = (props: { block: ChartBlock }) => {
  const { chart } = props.block;
  return (
    <BlockCard
      icon={<IconListDetails size={14} />}
      title={chart.title ?? "Chart"}
    >
      <div className="px-3 py-2.5">
        {chart.description && (
          <p className="mb-1 text-[0.75rem] text-ink-muted">
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
          <Chart chart={chart} />
        </React.Suspense>
      </div>
    </BlockCard>
  );
};

const ResourcesView = (props: { block: ResourcesBlock }) => {
  const { block } = props;
  const [opened, setOpened] = React.useState<string | undefined>();

  return (
    <BlockCard
      icon={<IconStack2 size={14} />}
      title={block.title ?? "Resources"}
      meta={block.resources.length}
    >
      <ul className="max-h-[420px] divide-y divide-line-subtle overflow-y-auto">
        {block.resources.map((item) => {
          const route = getResourceRoute(item.ref);
          const isOpened = opened === item.uri;
          return (
            <li key={item.uri} className="px-3 py-2">
              <div className="flex items-center gap-2">
                <Tag>{item.ref.kind}</Tag>
                {route ? (
                  <Link
                    to={route}
                    className="min-w-0 flex-1 truncate font-mono text-[0.8rem] font-bold text-ink hover:underline"
                  >
                    {item.ref.name}
                  </Link>
                ) : (
                  <span className="min-w-0 flex-1 truncate font-mono text-[0.8rem] font-bold text-ink">
                    {item.ref.name}
                  </span>
                )}
                {item.snapshot && (
                  <button
                    type="button"
                    aria-label="Show details"
                    aria-expanded={isOpened}
                    onClick={() => setOpened(isOpened ? undefined : item.uri)}
                    className="flex h-6 w-6 cursor-pointer items-center justify-center rounded-md text-ink-subtle hover:bg-surface-muted hover:text-ink"
                  >
                    <IconChevronDown
                      size={13}
                      className={twMerge(
                        "transition-transform duration-200",
                        isOpened && "rotate-180",
                      )}
                    />
                  </button>
                )}
              </div>
              <Collapse opened={isOpened}>
                <div className="mt-2">
                  <Code>{toJSON(item.snapshot)}</Code>
                </div>
              </Collapse>
            </li>
          );
        })}
      </ul>
    </BlockCard>
  );
};

const ArtifactView = (props: { block: ArtifactBlock; ctx: BlockContext }) => {
  const { artifact } = props.block;
  const { client } = props.ctx;
  const [downloading, setDownloading] = React.useState(false);
  const [preview, setPreview] = React.useState<string | undefined>();
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
    <div className="my-2.5 overflow-hidden rounded-xl border border-line bg-surface shadow-panel">
      {preview && (
        <img
          src={preview}
          alt={artifact.title ?? artifact.name}
          className="max-h-[360px] w-full border-b border-line-subtle bg-surface-subtle object-contain"
        />
      )}
      <div className="flex items-center gap-3 px-3 py-2.5">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-soft">
          <IconFileText size={17} />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-[0.85rem] font-bold text-ink">
            {artifact.title ?? artifact.name}
          </p>
          <p className="truncate text-[0.72rem] text-ink-muted">
            {artifact.name} · {formatSize(artifact.size)} · {artifact.mimeType}
          </p>
          {artifact.description && (
            <p className="mt-0.5 text-[0.78rem] text-ink-soft">
              {artifact.description}
            </p>
          )}
          {error && (
            <p className="mt-0.5 text-[0.75rem] text-hue-rose">{error}</p>
          )}
        </div>
        <Button
          size="xs"
          variant="default"
          leftSection={<IconDownload size={13} />}
          loading={downloading}
          onClick={() => void download()}
        >
          Download
        </Button>
      </div>
    </div>
  );
};

const NoticeView = (props: { block: NoticeBlock }) => (
  <div
    className={twMerge(
      "my-2 flex items-start gap-2 rounded-lg border px-3 py-2 text-[0.8rem]",
      props.block.level === "warning"
        ? "border-hue-amber-line bg-hue-amber-soft text-hue-amber"
        : "border-line bg-surface-subtle text-ink-soft",
    )}
  >
    {props.block.level === "warning" ? (
      <IconAlertTriangle size={15} className="mt-0.5 shrink-0" />
    ) : (
      <IconInfoCircle size={15} className="mt-0.5 shrink-0" />
    )}
    <span className="min-w-0 break-words">{props.block.text}</span>
  </div>
);

const ErrorView = (props: { block: ErrorBlock }) => (
  <div className="my-2 flex items-start gap-2 rounded-lg border border-hue-rose-line bg-hue-rose-soft px-3 py-2 text-[0.8rem] font-semibold text-hue-rose">
    <IconCircleX size={15} className="mt-0.5 shrink-0" />
    <span className="min-w-0 break-words">{props.block.message}</span>
  </div>
);

export const BlockView = (props: {
  block: Block;
  ctx: BlockContext;
  isLast: boolean;
}) => {
  const { block, ctx } = props;

  switch (block.type) {
    case "markdown":
      return <AgentMarkdown>{block.text}</AgentMarkdown>;
    case "thinking":
      return (
        <ThinkingView block={block} streaming={ctx.streaming && props.isLast} />
      );
    case "tool":
      return <ToolGroup blocks={[block]} />;
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
};
