import type {
  AssistantMessage,
  AssistantMessageEvent,
  ToolCall,
} from "@earendil-works/pi-ai";
import type { AgentSessionEvent } from "@earendil-works/pi-coding-agent";
import type {
  MarkdownBlock,
  NoticeBlock,
  RunActivity,
  ThinkingBlock,
  ToolBlock,
  ToolDetails,
  ToolStatus,
  Usage,
} from "../../protocol/index.ts";
import type { MessageBuilder } from "../../runs/builder.ts";

export interface MapperOptions {
  hiddenTools: Set<string>;
  maxOutputChars?: number;
  updateIntervalMs?: number;
}

const toolDetailKinds = new Set([
  "api",
  "command",
  "file",
  "workspaces",
  "transfer",
  "generic",
]);

const maxDetailsChars = 64 * 1024;

const isRecord = (arg: unknown): arg is Record<string, unknown> =>
  typeof arg === "object" && arg !== null && !Array.isArray(arg);

const str = (arg: unknown): string =>
  typeof arg === "string" ? arg : arg === undefined ? "" : JSON.stringify(arg);

const shorten = (text: string, max = 120): string => {
  const line = text.split("\n")[0].trim();
  return line.length > max ? `${line.slice(0, max - 1)}…` : line;
};

const genericTitles: Record<string, string> = {
  bash: "Run a shell command",
  read: "Read a file",
  write: "Write a file",
  edit: "Edit a file",
  grep: "Search files",
  find: "Find files",
  ls: "List files",
  cordium_api_call: "Call the Cordium API",
  cordium_api_search: "Search the Cordium APIs",
  cordium_api_describe: "Describe a Cordium API",
  workspace_list: "List the Workspaces",
  workspace_create: "Create a Workspace",
  workspace_control: "Control Workspaces",
  workspace_exec: "Run a command in a Workspace",
  workspace_copy: "Copy files",
  workspace_logs: "Read the Workspace logs",
  web_fetch: "Fetch a URL",
  web_search: "Search the web",
};

export const toolTitle = (name: string, args: unknown): string => {
  const a = isRecord(args) ? args : {};
  const specific = (() => {
    switch (name) {
      case "bash":
        return a.command ? `Run ${shorten(str(a.command))}` : "";
      case "read":
        return a.path ? `Read ${str(a.path)}` : "";
      case "write":
        return a.path ? `Write ${str(a.path)}` : "";
      case "edit":
        return a.path ? `Edit ${str(a.path)}` : "";
      case "grep":
        return a.pattern
          ? `Search for "${shorten(str(a.pattern), 60)}"${a.path ? ` in ${str(a.path)}` : ""}`
          : "";
      case "find":
        return a.pattern ? `Find ${shorten(str(a.pattern), 60)}` : "";
      case "ls":
        return `List ${str(a.path) || "."}`;
      case "cordium_api_call":
        return a.method ? `Call ${str(a.method)}` : "";
      case "cordium_api_search":
        return a.query
          ? `Search the APIs for "${shorten(str(a.query), 60)}"`
          : "";
      case "cordium_api_describe":
        return a.method || a.type ? `Describe ${str(a.method ?? a.type)}` : "";
      case "workspace_list":
        return a.space
          ? `List the Workspaces of ${str(a.space)}`
          : a.template
            ? `List the Workspaces of ${str(a.template)}`
            : "List the Workspaces";
      case "workspace_create": {
        const what = [
          a.image ? str(a.image) : "",
          isRecord(a.repository) && a.repository.url
            ? shorten(str(a.repository.url), 60)
            : "",
        ]
          .filter(Boolean)
          .join(", ");
        const where = a.template
          ? ` from ${str(a.template)}`
          : a.space
            ? ` in ${str(a.space)}`
            : "";
        return `Create a Workspace${where}${what ? ` (${what})` : ""}`;
      }
      case "workspace_control": {
        const action = str(a.action);
        const names = Array.isArray(a.workspaces)
          ? a.workspaces.map(str).join(", ")
          : "";
        if (!action || !names) {
          return "";
        }
        return action === "wait"
          ? `Wait for ${shorten(names, 80)}`
          : `${action[0].toUpperCase()}${action.slice(1)} ${shorten(names, 80)}`;
      }
      case "workspace_exec":
        return a.command
          ? `Run ${shorten(str(a.command), 80)}${a.workspace ? ` in ${str(a.workspace)}` : ""}`
          : "";
      case "workspace_copy":
        return a.source && a.destination
          ? `Copy ${shorten(str(a.source), 60)} to ${shorten(str(a.destination), 60)}`
          : "";
      case "workspace_logs":
        return a.workspace ? `Read the logs of ${str(a.workspace)}` : "";
      case "web_fetch":
        return a.url ? `Fetch ${shorten(str(a.url))}` : "";
      case "web_search":
        return a.query
          ? `Search the web for "${shorten(str(a.query), 60)}"`
          : "";
      default:
        return "";
    }
  })();
  return specific || genericTitles[name] || name;
};

export const resultText = (result: unknown): string => {
  if (!isRecord(result) || !Array.isArray(result.content)) {
    return typeof result === "string" ? result : "";
  }
  return result.content
    .filter(
      (c): c is { type: "text"; text: string } =>
        isRecord(c) && c.type === "text" && typeof c.text === "string",
    )
    .map((c) => c.text)
    .join("\n");
};

export const toolDetails = (
  name: string,
  args: unknown,
  result: unknown,
): ToolDetails | undefined => {
  const a = isRecord(args) ? args : {};
  const r = isRecord(result) ? result : {};

  let details: ToolDetails | undefined;
  if (isRecord(r.details) && toolDetailKinds.has(str(r.details.kind))) {
    details = r.details as unknown as ToolDetails;
  } else {
    switch (name) {
      case "bash": {
        const structured = isRecord(r.structuredContent)
          ? r.structuredContent
          : {};
        details = {
          kind: "command",
          command: str(a.command),
          exitCode:
            typeof structured.exit_code === "number"
              ? structured.exit_code
              : undefined,
        };
        break;
      }
      case "edit":
        details = {
          kind: "file",
          path: str(a.path),
          operation: "edit",
          diff:
            isRecord(r.details) && typeof r.details.diff === "string"
              ? r.details.diff
              : undefined,
        };
        break;
      case "write":
      case "read":
        details = { kind: "file", path: str(a.path), operation: name };
        break;
    }
  }

  if (!details) {
    return undefined;
  }
  if (JSON.stringify(details).length > maxDetailsChars) {
    if (details.kind === "file" && details.diff) {
      return {
        ...details,
        diff: `${details.diff.slice(0, maxDetailsChars / 2)}\n…[truncated]`,
      };
    }
    if (details.kind === "api") {
      const { response: _, ...rest } = details;
      return { ...rest, responseTruncated: true };
    }
    return undefined;
  }
  return details;
};

export const emptyUsage = (): Usage => ({
  inputTokens: 0,
  outputTokens: 0,
  cacheReadTokens: 0,
  cacheWriteTokens: 0,
  totalTokens: 0,
  cost: 0,
});

export class PiEventMapper {
  readonly usage: Usage = emptyUsage();
  lastAssistant?: AssistantMessage;
  private builder: MessageBuilder;
  private opts: Required<MapperOptions>;
  private setActivity: (activity: RunActivity | undefined) => void;
  private turn = 0;
  private contentBlocks = new Map<string, string>();
  private lastUpdates = new Map<string, number>();

  constructor(
    builder: MessageBuilder,
    opts: MapperOptions,
    setActivity: (activity: RunActivity | undefined) => void,
  ) {
    this.builder = builder;
    this.opts = {
      maxOutputChars: 8000,
      updateIntervalMs: 300,
      ...opts,
    };
    this.setActivity = setActivity;
  }

  private truncate(text: string): {
    output: string;
    outputTruncated?: boolean;
  } {
    return text.length <= this.opts.maxOutputChars
      ? { output: text }
      : {
          output: `${text.slice(0, this.opts.maxOutputChars)}\n…[truncated]`,
          outputTruncated: true,
        };
  }

  handle(event: AgentSessionEvent) {
    switch (event.type) {
      case "message_start":
        if (event.message.role === "assistant") {
          this.turn++;
        }
        break;
      case "message_update":
        if (event.message.role === "assistant") {
          this.handleAssistantEvent(event.assistantMessageEvent);
        }
        break;
      case "message_end":
        if (event.message.role === "assistant") {
          this.handleAssistantEnd(event.message as AssistantMessage);
        }
        break;
      case "tool_execution_start":
        this.handleToolStart(event.toolCallId, event.toolName, event.args);
        break;
      case "tool_execution_update":
        this.handleToolUpdate(
          event.toolCallId,
          event.toolName,
          event.partialResult,
        );
        break;
      case "tool_execution_end":
        this.handleToolEnd(
          event.toolCallId,
          event.toolName,
          event.result,
          event.isError,
        );
        break;
      case "auto_retry_start":
        this.setActivity({
          kind: "retrying",
          message: `Retrying after an error (attempt ${event.attempt}/${event.maxAttempts}): ${event.errorMessage}`,
        });
        break;
      case "auto_retry_end":
        this.setActivity(undefined);
        break;
      case "compaction_start":
        this.setActivity({
          kind: "compacting",
          message:
            "Summarizing the earlier conversation to fit the model's context window",
        });
        break;
      case "compaction_end":
        this.setActivity(undefined);
        if (event.result && !event.aborted) {
          this.builder.addBlock<NoticeBlock>({
            type: "notice",
            level: "info",
            text: "The earlier conversation was summarized to fit the model's context window.",
          });
        }
        break;
    }
  }

  private key(contentIndex: number): string {
    return `${this.turn}:${contentIndex}`;
  }

  private handleAssistantEvent(e: AssistantMessageEvent) {
    switch (e.type) {
      case "text_start": {
        const block = this.builder.addBlock<MarkdownBlock>({
          type: "markdown",
          text: "",
        });
        this.contentBlocks.set(this.key(e.contentIndex), block.id);
        break;
      }
      case "text_delta": {
        const id = this.contentBlocks.get(this.key(e.contentIndex));
        if (id) {
          this.builder.appendText(id, e.delta);
        }
        break;
      }
      case "text_end": {
        const id = this.contentBlocks.get(this.key(e.contentIndex));
        const block = id ? this.builder.getBlock<MarkdownBlock>(id) : undefined;
        if (block && block.text !== e.content) {
          this.builder.updateBlock<MarkdownBlock>(block.id, {
            text: e.content,
          });
        }
        break;
      }
      case "thinking_start": {
        const block = this.builder.addBlock<ThinkingBlock>({
          type: "thinking",
          text: "",
        });
        this.contentBlocks.set(this.key(e.contentIndex), block.id);
        break;
      }
      case "thinking_delta": {
        const id = this.contentBlocks.get(this.key(e.contentIndex));
        if (id) {
          this.builder.appendText(id, e.delta);
        }
        break;
      }
      case "thinking_end": {
        const id = this.contentBlocks.get(this.key(e.contentIndex));
        const block = id ? this.builder.getBlock<ThinkingBlock>(id) : undefined;
        const content = e.partial.content[e.contentIndex];
        const redacted =
          content?.type === "thinking" && content.redacted ? true : undefined;
        if (block && (block.text !== e.content || redacted)) {
          this.builder.updateBlock<ThinkingBlock>(block.id, {
            text: e.content,
            redacted,
          });
        }
        break;
      }
      case "toolcall_start": {
        const content = e.partial.content[e.contentIndex];
        if (content?.type === "toolCall" && content.id && content.name) {
          this.ensureToolBlock(content.id, content.name, undefined, "pending");
        }
        break;
      }
      case "toolcall_end":
        this.ensureToolBlock(
          e.toolCall.id,
          e.toolCall.name,
          (e.toolCall as ToolCall).arguments,
          "pending",
        );
        break;
    }
  }

  private handleAssistantEnd(message: AssistantMessage) {
    this.lastAssistant = message;
    const u = message.usage;
    if (u) {
      this.usage.inputTokens += u.input ?? 0;
      this.usage.outputTokens += u.output ?? 0;
      this.usage.cacheReadTokens += u.cacheRead ?? 0;
      this.usage.cacheWriteTokens += u.cacheWrite ?? 0;
      this.usage.totalTokens += u.totalTokens ?? 0;
      this.usage.cost += u.cost?.total ?? 0;
    }
    if (message.stopReason === "length") {
      this.builder.addBlock<NoticeBlock>({
        type: "notice",
        level: "warning",
        text: "The response was cut off because the model reached its maximum output length.",
      });
    }
  }

  private ensureToolBlock(
    toolCallId: string,
    name: string,
    args: unknown,
    status: ToolStatus,
  ): ToolBlock | undefined {
    if (this.opts.hiddenTools.has(name)) {
      return undefined;
    }
    const existing = this.builder.getToolBlock(toolCallId);
    if (existing) {
      if (
        args !== undefined &&
        JSON.stringify(args) !== JSON.stringify(existing.input)
      ) {
        return this.builder.updateBlock<ToolBlock>(existing.id, {
          input: args,
          title: toolTitle(name, args),
        });
      }
      return existing;
    }
    return this.builder.addBlock<ToolBlock>({
      type: "tool",
      toolCallId,
      name,
      title: toolTitle(name, args),
      status,
      input: args,
    });
  }

  private handleToolStart(toolCallId: string, name: string, args: unknown) {
    const block = this.ensureToolBlock(toolCallId, name, args, "running");
    if (block && block.status === "pending") {
      this.builder.updateBlock<ToolBlock>(block.id, {
        status: "running",
        startedAt: new Date().toISOString(),
      });
    } else if (block && !block.startedAt) {
      this.builder.updateBlock<ToolBlock>(block.id, {
        startedAt: new Date().toISOString(),
      });
    }
  }

  private handleToolUpdate(toolCallId: string, name: string, partial: unknown) {
    if (this.opts.hiddenTools.has(name)) {
      return;
    }
    const block = this.builder.getToolBlock(toolCallId);
    if (!block) {
      return;
    }
    const now = Date.now();
    const last = this.lastUpdates.get(toolCallId) ?? 0;
    if (now - last < this.opts.updateIntervalMs) {
      return;
    }
    this.lastUpdates.set(toolCallId, now);
    const text = resultText(partial);
    if (text !== "") {
      this.builder.updateBlock<ToolBlock>(block.id, this.truncate(text));
    }
  }

  private handleToolEnd(
    toolCallId: string,
    name: string,
    result: unknown,
    isError: boolean,
  ) {
    if (this.opts.hiddenTools.has(name)) {
      return;
    }
    const block =
      this.builder.getToolBlock(toolCallId) ??
      this.ensureToolBlock(toolCallId, name, undefined, "running");
    if (!block) {
      return;
    }
    this.lastUpdates.delete(toolCallId);
    const status: ToolStatus =
      block.status === "rejected"
        ? "rejected"
        : isError
          ? "failed"
          : "completed";
    const { output, outputTruncated } = this.truncate(resultText(result));
    this.builder.updateBlock<ToolBlock>(block.id, {
      status,
      output,
      outputTruncated,
      details: toolDetails(name, block.input, result),
      completedAt: new Date().toISOString(),
    });
  }
}
