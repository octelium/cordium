import type {
  Block,
  FileInfo,
  Message,
  Run,
  ThinkingBlock,
  ToolBlock,
} from "@/apis/agent/protocol";
import TimeAgo from "@/components/TimeAgo";
import { ActionIcon, Tooltip } from "@mantine/core";
import { Timestamp } from "@octelium/apis/google/protobuf/timestamp";
import { IconAlertCircle, IconRotate } from "@tabler/icons-react";
import { motion } from "framer-motion";
import * as React from "react";
import { twMerge } from "tailwind-merge";
import { BlockView, StepsView, type BlockContext } from "./Blocks";
import { AgentMark, CopyButton, FileIcon, Shimmer } from "./ui";
import {
  formatBytes,
  formatCount,
  formatElapsed,
  messageText,
  modelLabel,
  useNow,
} from "./utils";

type BlockGroup =
  | { type: "steps"; id: string; blocks: (ThinkingBlock | ToolBlock)[] }
  | { type: "block"; id: string; block: Block };

const isStandaloneStep = (block: Block) =>
  block.type === "tool" &&
  block.details?.kind === "workspaces" &&
  block.details.items.length > 0;

const groupBlocks = (blocks: Block[]): BlockGroup[] => {
  const ret: BlockGroup[] = [];
  for (const block of blocks) {
    if (block.type === "thinking" || block.type === "tool") {
      const last = ret.at(-1);
      if (
        last?.type === "steps" &&
        !isStandaloneStep(block) &&
        !isStandaloneStep(last.blocks[0])
      ) {
        last.blocks.push(block);
      } else {
        ret.push({ type: "steps", id: block.id, blocks: [block] });
      }
    } else {
      ret.push({ type: "block", id: block.id, block });
    }
  }
  return ret;
};

const messageStatusLabel: Partial<Record<Message["status"], string>> = {
  failed: "The response failed",
  cancelled: "Stopped",
  interrupted: "Interrupted",
};

const Typing = () => (
  <span
    className="inline-flex items-center gap-1 py-1 text-ink-subtle"
    aria-label="Writing"
  >
    <i className="h-1.5 w-1.5 rounded-full bg-current motion-safe:animate-pulse" />
    <i className="h-1.5 w-1.5 rounded-full bg-current motion-safe:animate-pulse [animation-delay:150ms]" />
    <i className="h-1.5 w-1.5 rounded-full bg-current motion-safe:animate-pulse [animation-delay:300ms]" />
  </span>
);

const runStatusLabel = (run: Run | undefined, message: Message): string => {
  if (run?.activity) return run.activity.message;
  const last = message.blocks.at(-1);
  if (!last || last.type === "thinking") return "Thinking";
  if (
    last.type === "tool" &&
    (last.status === "running" || last.status === "pending")
  ) {
    return last.title || last.name;
  }
  return "Thinking";
};

const RunStatus = (props: { run?: Run; message: Message }) => {
  const { run, message } = props;
  const now = useNow(true);
  const elapsed = run
    ? formatElapsed(now - new Date(run.createdAt).getTime())
    : "";

  if (run?.status === "awaiting_approval") {
    return null;
  }

  if (!run?.activity && message.blocks.at(-1)?.type === "markdown") {
    return <Typing />;
  }

  return (
    <p
      className="mt-2 flex min-w-0 items-center gap-2 text-xs font-semibold"
      aria-live="polite"
    >
      <Shimmer className="min-w-0 truncate">
        {runStatusLabel(run, message)}
      </Shimmer>
      {elapsed && (
        <span className="shrink-0 font-mono text-[0.7rem]/4 font-normal text-ink-subtle">
          {elapsed}
        </span>
      )}
    </p>
  );
};

const AttachmentChip = (props: { file: FileInfo }) => {
  return (
    <span className="inline-flex max-w-[240px] items-center gap-2 rounded-xl border border-line bg-surface py-1 pl-1 pr-2.5 shadow-panel">
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-muted">
        <FileIcon
          mimeType={props.file.mimeType}
          name={props.file.name}
          size={13}
        />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-[0.7rem]/4 font-semibold text-ink-body">
          {props.file.name}
        </span>
        <span className="block text-[0.625rem] leading-3 text-ink-subtle">
          {formatBytes(props.file.size)}
        </span>
      </span>
    </span>
  );
};

const longUserText = 700;

const UserMessage = (props: { message: Message; pending?: boolean }) => {
  const { message } = props;
  const [expanded, setExpanded] = React.useState(false);
  const text = messageText(message);
  const isLong = text.length > longUserText || text.split("\n").length > 12;

  return (
    <div className="group/msg flex w-full flex-col items-end gap-1.5">
      {message.attachments && message.attachments.length > 0 && (
        <div className="flex max-w-[85%] flex-wrap justify-end gap-1.5">
          {message.attachments.map((file) => (
            <AttachmentChip key={file.id} file={file} />
          ))}
        </div>
      )}
      {text && (
        <div
          className={twMerge(
            "relative max-w-[min(85%,40rem)] rounded-2xl rounded-br-md bg-surface-muted px-4 py-2.5 transition-opacity duration-300",
            props.pending && "opacity-60",
          )}
        >
          <p
            className={twMerge(
              "whitespace-pre-wrap break-words text-[0.82rem]/6 text-ink-strong",
              isLong && !expanded && "max-h-60 overflow-hidden",
            )}
          >
            {text}
          </p>
          {isLong && (
            <div
              className={twMerge(
                "flex justify-end",
                !expanded &&
                  "absolute inset-x-0 bottom-0 h-16 items-end rounded-b-2xl bg-gradient-to-t from-surface-muted via-surface-muted/90 to-transparent px-4 pb-2",
              )}
            >
              <button
                type="button"
                onClick={() => setExpanded((value) => !value)}
                className="cursor-pointer text-[0.7rem]/4 font-semibold text-ink-soft hover:text-ink"
              >
                {expanded ? "Show less" : "Show more"}
              </button>
            </div>
          )}
        </div>
      )}
      {!props.pending && (
        <div className="flex h-6 items-center gap-1 text-[0.7rem]/4 text-ink-subtle opacity-0 transition-opacity duration-150 focus-within:opacity-100 group-hover/msg:opacity-100 [@media(hover:none)]:opacity-100">
          <TimeAgo rfc3339={Timestamp.fromDate(new Date(message.createdAt))} />
          {text && <CopyButton value={() => text} label="Copy message" />}
        </div>
      )}
    </div>
  );
};

const UsageInfo = (props: { message: Message }) => {
  const usage = props.message.usage;
  if (!usage || usage.totalTokens === 0) return null;

  return (
    <Tooltip
      multiline
      label={
        <span className="block text-[0.7rem] leading-5">
          Input: {usage.inputTokens.toLocaleString()}
          <br />
          Output: {usage.outputTokens.toLocaleString()}
          {usage.cacheReadTokens > 0 && (
            <>
              <br />
              Cache read: {usage.cacheReadTokens.toLocaleString()}
            </>
          )}
          {usage.cacheWriteTokens > 0 && (
            <>
              <br />
              Cache write: {usage.cacheWriteTokens.toLocaleString()}
            </>
          )}
          {usage.cost > 0 && (
            <>
              <br />
              Cost: ${usage.cost.toFixed(4)}
            </>
          )}
        </span>
      }
    >
      <span className="cursor-default tabular-nums">
        {formatCount(usage.totalTokens)} tokens
      </span>
    </Tooltip>
  );
};

type MessageContext = Omit<BlockContext, "runId" | "streaming">;

const AssistantMessage = (props: {
  message: Message;
  ctx: MessageContext;
  streaming: boolean;
  run?: Run;
  isLast: boolean;
  onRetry?: () => void;
}) => {
  const { message, streaming } = props;
  const ctx = React.useMemo<BlockContext>(
    () => ({ ...props.ctx, runId: message.runId, streaming }),
    [props.ctx, message.runId, streaming],
  );
  const groups = React.useMemo(
    () => groupBlocks(message.blocks),
    [message.blocks],
  );
  const ends = React.useMemo(() => {
    const ret: Record<string, string | undefined> = {};
    message.blocks.forEach((block, idx) => {
      ret[block.id] = message.blocks[idx + 1]?.createdAt ?? message.completedAt;
    });
    return ret;
  }, [message.blocks, message.completedAt]);

  const status = messageStatusLabel[message.status];
  const text = streaming ? "" : messageText(message);
  const showError =
    message.error && !message.blocks.some((b) => b.type === "error");

  return (
    <div className="group/msg flex w-full gap-3">
      <AgentMark live={streaming} className="mt-0.5 hidden sm:flex" />
      <div className="min-w-0 flex-1">
        {groups.map((group, idx) =>
          group.type === "steps" ? (
            <StepsView
              key={group.id}
              blocks={group.blocks}
              live={streaming && idx === groups.length - 1}
              ends={ends}
            />
          ) : (
            <BlockView key={group.id} block={group.block} ctx={ctx} />
          ),
        )}
        {streaming && <RunStatus run={props.run} message={message} />}
        {!streaming && (status || showError) && (
          <p className="mt-2 flex items-start gap-1.5 text-xs font-semibold text-ink-muted">
            <IconAlertCircle size={13} className="mt-0.5 shrink-0" />
            <span className="min-w-0 break-words">
              {status}
              {showError ? `: ${message.error!.message}` : ""}
            </span>
          </p>
        )}
        {!streaming && (
          <div
            className={twMerge(
              "-ml-1 mt-1.5 flex h-7 items-center gap-0.5 text-[0.7rem]/4 text-ink-subtle transition-opacity duration-150 focus-within:opacity-100 [@media(hover:none)]:opacity-100",
              props.isLast
                ? "opacity-100"
                : "opacity-0 group-hover/msg:opacity-100",
            )}
          >
            {text && <CopyButton value={() => text} label="Copy response" />}
            {props.onRetry && (
              <Tooltip label="Retry">
                <ActionIcon
                  size="sm"
                  variant="subtle"
                  color="gray"
                  aria-label="Retry"
                  onClick={props.onRetry}
                >
                  <IconRotate size={13} />
                </ActionIcon>
              </Tooltip>
            )}
            <span className="ml-1.5 flex min-w-0 items-center gap-1.5 truncate">
              {message.model && (
                <>
                  <span className="truncate">{modelLabel(message.model)}</span>
                  <span aria-hidden>·</span>
                </>
              )}
              <TimeAgo
                rfc3339={Timestamp.fromDate(
                  new Date(message.completedAt ?? message.createdAt),
                )}
              />
              {message.usage && message.usage.totalTokens > 0 && (
                <>
                  <span aria-hidden>·</span>
                  <UsageInfo message={message} />
                </>
              )}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

export const MessageView = React.memo(
  (props: {
    message: Message;
    ctx: MessageContext;
    streaming: boolean;
    run?: Run;
    isLast: boolean;
    pending?: boolean;
    onRetry?: () => void;
  }) => (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
      data-message-id={props.message.id}
    >
      {props.message.role === "user" ? (
        <UserMessage message={props.message} pending={props.pending} />
      ) : (
        <AssistantMessage
          message={props.message}
          ctx={props.ctx}
          streaming={props.streaming}
          run={props.run}
          isLast={props.isLast}
          onRetry={props.onRetry}
        />
      )}
    </motion.div>
  ),
);

export const TranscriptSkeleton = () => (
  <div
    className="flex w-full flex-col gap-8 motion-safe:animate-pulse"
    role="status"
    aria-label="Loading the conversation"
  >
    <div className="ml-auto h-10 w-2/5 rounded-2xl rounded-br-md bg-surface-muted" />
    <div className="flex gap-3">
      <div className="hidden h-7 w-7 shrink-0 rounded-lg bg-surface-muted sm:block" />
      <div className="flex-1 space-y-2.5 pt-1">
        <div className="h-3 w-11/12 rounded-full bg-surface-muted" />
        <div className="h-3 w-4/5 rounded-full bg-surface-muted" />
        <div className="h-3 w-2/3 rounded-full bg-surface-muted" />
        <div className="mt-4 h-24 w-full rounded-xl bg-surface-muted" />
      </div>
    </div>
    <div className="ml-auto h-10 w-1/3 rounded-2xl rounded-br-md bg-surface-muted" />
  </div>
);

export const PendingAssistant = () => (
  <motion.div
    initial={{ opacity: 0, y: 8 }}
    animate={{ opacity: 1, y: 0 }}
    transition={{ duration: 0.25, delay: 0.1, ease: [0.22, 1, 0.36, 1] }}
    className="flex w-full gap-3"
  >
    <AgentMark live className="mt-0.5 hidden sm:flex" />
    <p className="flex h-7 items-center text-xs font-semibold">
      <Shimmer>Thinking</Shimmer>
    </p>
  </motion.div>
);
