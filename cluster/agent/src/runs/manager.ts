import type {
  AgentEvent,
  AgentEventPayload,
  ApprovalBlock,
  ApprovalDecision,
  Conversation,
  FileInfo,
  Message,
  Run,
  RunActivity,
  RunInput,
  RunSnapshot,
  RunStatus,
  ToolBlock,
} from "../protocol/index.ts";
import type {
  AgentBackend,
  ApprovalRequest,
  ApprovalResult,
  BackendRunResult,
  ToolContextProvider,
  ToolRunContext,
} from "../agent/types.ts";
import { ResourceCache } from "../agent/resources.ts";
import { badRequest, conflict, notFound, tooManyRequests } from "../errors.ts";
import type { Logger } from "../log.ts";
import { newID, type ConversationStore } from "../store/conversations.ts";
import type { FileStore } from "../store/files.ts";
import { MessageBuilder } from "./builder.ts";

export const DEFAULT_CONVERSATION_TITLE = "New conversation";

const maxInputChars = 200_000;

interface PendingApproval {
  blockId: string;
  toolCallId: string;
  finish: (result: ApprovalResult, status: ApprovalBlock["status"]) => void;
}

interface ActiveRun {
  run: Run;
  message: Message;
  builder: MessageBuilder;
  events: AgentEvent[];
  listeners: Set<(event: AgentEvent) => void>;
  controller: AbortController;
  approvals: Map<string, PendingApproval>;
  pendingDeltas: Map<string, { messageId: string; text: string }>;
  flushTimer?: NodeJS.Timeout;
  persistTimer?: NodeJS.Timeout;
  done: boolean;
  donePromise: Promise<void>;
  resolveDone: () => void;
}

export interface Subscription {
  replay: AgentEvent[];
  finished: boolean;
  unsubscribe: () => void;
}

export interface RunManagerOptions {
  store: ConversationStore;
  files: FileStore;
  backend: AgentBackend;
  logger: Logger;
  maxConcurrentRuns: number;
  generateTitles: boolean;
  retentionMs?: number;
}

export const truncateTitle = (text: string, max = 60): string => {
  const flat = text.replace(/\s+/g, " ").trim();
  return flat.length <= max ? flat : `${flat.slice(0, max - 1).trimEnd()}…`;
};

export class RunManager implements ToolContextProvider {
  private opts: RunManagerOptions;
  private runs = new Map<string, ActiveRun>();
  private activeByConversation = new Map<string, ActiveRun>();
  private resourceCaches = new Map<string, ResourceCache>();
  private retentionMs: number;

  constructor(opts: RunManagerOptions) {
    this.opts = opts;
    this.retentionMs = opts.retentionMs ?? 10 * 60 * 1000;
  }

  get activeRunCount(): number {
    return this.activeByConversation.size;
  }

  startRun(conversationId: string, input: RunInput): Run {
    const { store, files } = this.opts;
    const conversation = store.get(conversationId);
    if (!conversation) {
      throw notFound(`Conversation not found: ${conversationId}`);
    }
    if (this.activeByConversation.has(conversationId)) {
      throw conflict("The conversation already has an active run");
    }
    if (this.activeByConversation.size >= this.opts.maxConcurrentRuns) {
      throw tooManyRequests("Too many concurrent runs");
    }

    const text = typeof input?.text === "string" ? input.text.trim() : "";
    const attachmentIDs = Array.isArray(input?.attachments)
      ? input.attachments
      : [];
    if (text.length > maxInputChars) {
      throw badRequest(`The input exceeds ${maxInputChars} characters`);
    }

    const attachments: FileInfo[] = [];
    for (const id of attachmentIDs) {
      const file = typeof id === "string" ? files.getUpload(id) : undefined;
      if (!file) {
        throw badRequest(`Unknown attachment: ${String(id)}`);
      }
      attachments.push(file);
    }

    if (text === "" && attachments.length === 0) {
      throw badRequest("The input text must not be empty");
    }

    const now = new Date().toISOString();
    const runId = newID();

    const userMessage: Message = {
      id: newID(),
      conversationId,
      role: "user",
      status: "completed",
      createdAt: now,
      completedAt: now,
      runId,
      blocks:
        text === ""
          ? []
          : [{ id: "b1", type: "markdown", text, createdAt: now }],
      attachments: attachments.length > 0 ? attachments : undefined,
    };
    store.appendMessage(conversationId, userMessage);

    const message: Message = {
      id: newID(),
      conversationId,
      role: "assistant",
      status: "streaming",
      createdAt: now,
      runId,
      blocks: [],
      model: this.opts.backend.model,
    };

    const run: Run = {
      id: runId,
      conversationId,
      status: "running",
      createdAt: now,
      userMessageId: userMessage.id,
      assistantMessageId: message.id,
      lastSeq: 0,
    };

    let resolveDone = () => {};
    const donePromise = new Promise<void>((resolve) => {
      resolveDone = resolve;
    });

    const ar: ActiveRun = {
      run,
      message,
      builder: undefined as unknown as MessageBuilder,
      events: [],
      listeners: new Set(),
      controller: new AbortController(),
      approvals: new Map(),
      pendingDeltas: new Map(),
      done: false,
      donePromise,
      resolveDone,
    };
    ar.builder = new MessageBuilder(
      message,
      (payload) => this.emit(ar, payload),
      () => this.schedulePersist(ar),
    );

    this.runs.set(runId, ar);
    this.activeByConversation.set(conversationId, ar);
    let current = store.update(conversationId, { activeRunId: runId })!;

    this.push(ar, { type: "run.started", run: structuredClone(run) });
    this.push(ar, {
      type: "message.created",
      message: structuredClone(userMessage),
    });
    this.push(ar, {
      type: "message.created",
      message: structuredClone(message),
    });

    const isFirst = conversation.title === DEFAULT_CONVERSATION_TITLE;
    if (isFirst && text !== "") {
      current = store.update(conversationId, { title: truncateTitle(text) })!;
      this.push(ar, {
        type: "conversation.updated",
        conversation: structuredClone(current),
      });
      if (this.opts.generateTitles) {
        void this.generateTitle(conversationId, text, current.title);
      }
    }

    void this.execute(ar, current, text, attachments);

    return structuredClone(run);
  }

  private async execute(
    ar: ActiveRun,
    conversation: Conversation,
    text: string,
    attachments: FileInfo[],
  ) {
    let result: BackendRunResult;
    try {
      result = await this.opts.backend.run(
        { text, attachments },
        {
          runId: ar.run.id,
          conversation,
          builder: ar.builder,
          signal: ar.controller.signal,
          setActivity: (activity) => this.setActivity(ar, activity),
        },
      );
    } catch (err) {
      this.opts.logger.warn("Run failed", {
        runId: ar.run.id,
        error: err as Error,
      });
      result = {
        status: "failed",
        error: { message: (err as Error).message ?? String(err) },
      };
    }

    try {
      this.finalize(ar, result);
    } catch (err) {
      this.opts.logger.error("Could not finalize run", {
        runId: ar.run.id,
        error: err as Error,
      });
    }
  }

  private finalize(ar: ActiveRun, result: BackendRunResult) {
    const { store } = this.opts;
    const { run, message, builder } = ar;
    const cancelled = ar.controller.signal.aborted;
    const status: RunStatus = cancelled ? "cancelled" : result.status;
    const now = new Date().toISOString();

    for (const approval of [...ar.approvals.values()]) {
      approval.finish({ approved: false, reason: "cancelled" }, "cancelled");
    }

    for (const block of message.blocks) {
      if (
        block.type === "tool" &&
        (block.status === "pending" ||
          block.status === "running" ||
          block.status === "awaiting_approval")
      ) {
        builder.updateBlock<ToolBlock>(block.id, {
          status: cancelled ? "cancelled" : "failed",
          completedAt: now,
        });
      }
    }

    if (
      status === "failed" &&
      result.error &&
      !message.blocks.some((b) => b.type === "error")
    ) {
      builder.addBlock({
        type: "error",
        message: result.error.message,
        code: result.error.code,
      });
    }

    message.status =
      status === "completed"
        ? "completed"
        : status === "cancelled"
          ? "cancelled"
          : "failed";
    message.completedAt = now;
    message.model ??= this.opts.backend.model;
    if (result.usage) {
      message.usage = result.usage;
    }
    if (result.error && status === "failed") {
      message.error = result.error;
    }

    run.status = status;
    run.completedAt = now;
    delete run.activity;
    if (result.usage) {
      run.usage = result.usage;
    }
    if (result.error && status === "failed") {
      run.error = result.error;
    }

    this.flushDeltas(ar);
    if (ar.persistTimer) {
      clearTimeout(ar.persistTimer);
      ar.persistTimer = undefined;
    }

    store.appendMessage(run.conversationId, message);
    store.clearPending(run.conversationId);
    store.update(run.conversationId, { activeRunId: undefined });

    ar.done = true;
    this.activeByConversation.delete(run.conversationId);

    this.push(ar, {
      type: "message.completed",
      message: structuredClone(message),
    });
    this.push(ar, {
      type:
        status === "completed"
          ? "run.completed"
          : status === "cancelled"
            ? "run.cancelled"
            : "run.failed",
      run: structuredClone(run),
    });

    ar.listeners.clear();
    ar.resolveDone();

    setTimeout(() => {
      if (this.runs.get(run.id) === ar) {
        this.runs.delete(run.id);
      }
    }, this.retentionMs).unref();
  }

  private emit(ar: ActiveRun, payload: AgentEventPayload) {
    if (payload.type === "block.delta") {
      const pending = ar.pendingDeltas.get(payload.blockId);
      if (pending) {
        pending.text += payload.text;
      } else {
        ar.pendingDeltas.set(payload.blockId, {
          messageId: payload.messageId,
          text: payload.text,
        });
      }
      if (!ar.flushTimer) {
        ar.flushTimer = setTimeout(() => this.flushDeltas(ar), 30);
      }
      return;
    }

    this.flushDeltas(ar);
    this.push(ar, payload);
  }

  private flushDeltas(ar: ActiveRun) {
    if (ar.flushTimer) {
      clearTimeout(ar.flushTimer);
      ar.flushTimer = undefined;
    }
    if (ar.pendingDeltas.size === 0) {
      return;
    }
    const pending = [...ar.pendingDeltas.entries()];
    ar.pendingDeltas.clear();
    for (const [blockId, { messageId, text }] of pending) {
      this.push(ar, { type: "block.delta", messageId, blockId, text });
    }
  }

  private push(ar: ActiveRun, payload: AgentEventPayload) {
    const event = {
      seq: ++ar.run.lastSeq,
      ts: new Date().toISOString(),
      conversationId: ar.run.conversationId,
      runId: ar.run.id,
      ...payload,
    } as AgentEvent;
    ar.events.push(event);
    for (const listener of ar.listeners) {
      try {
        listener(event);
      } catch (err) {
        this.opts.logger.debug("Event listener failed", {
          error: err as Error,
        });
      }
    }
  }

  private schedulePersist(ar: ActiveRun) {
    if (ar.persistTimer || ar.done) {
      return;
    }
    ar.persistTimer = setTimeout(() => {
      ar.persistTimer = undefined;
      if (!ar.done) {
        try {
          this.opts.store.savePending(ar.run.conversationId, ar.message);
        } catch (err) {
          this.opts.logger.warn("Could not persist the pending message", {
            error: err as Error,
          });
        }
      }
    }, 1000);
    ar.persistTimer.unref();
  }

  private setStatus(ar: ActiveRun, status: RunStatus) {
    if (ar.done || ar.run.status === status) {
      return;
    }
    ar.run.status = status;
    this.emit(ar, { type: "run.updated", run: structuredClone(ar.run) });
  }

  private setActivity(ar: ActiveRun, activity: RunActivity | undefined) {
    if (ar.done) {
      return;
    }
    if (activity) {
      ar.run.activity = activity;
    } else if (ar.run.activity) {
      delete ar.run.activity;
    } else {
      return;
    }
    this.emit(ar, { type: "run.updated", run: structuredClone(ar.run) });
  }

  private async generateTitle(
    conversationId: string,
    text: string,
    fallbackTitle: string,
  ) {
    try {
      const title = await this.opts.backend.generateTitle(text);
      if (!title) {
        return;
      }
      const conversation = this.opts.store.get(conversationId);
      if (!conversation || conversation.title !== fallbackTitle) {
        return;
      }
      const updated = this.opts.store.update(conversationId, { title });
      const ar = this.activeByConversation.get(conversationId);
      if (updated && ar && !ar.done) {
        this.emit(ar, {
          type: "conversation.updated",
          conversation: structuredClone(updated),
        });
      }
    } catch (err) {
      this.opts.logger.debug("Could not generate a title", {
        error: err as Error,
      });
    }
  }

  getRun(runId: string): Run | undefined {
    const ar = this.runs.get(runId);
    return ar ? structuredClone(ar.run) : undefined;
  }

  getSnapshot(runId: string): RunSnapshot | undefined {
    const ar = this.runs.get(runId);
    if (!ar) {
      return undefined;
    }
    this.flushDeltas(ar);
    return {
      run: structuredClone(ar.run),
      message: structuredClone(ar.message),
    };
  }

  getActiveSnapshot(conversationId: string): RunSnapshot | undefined {
    const ar = this.activeByConversation.get(conversationId);
    return ar ? this.getSnapshot(ar.run.id) : undefined;
  }

  subscribe(
    runId: string,
    afterSeq: number,
    listener: (event: AgentEvent) => void,
  ): Subscription | undefined {
    const ar = this.runs.get(runId);
    if (!ar) {
      return undefined;
    }
    this.flushDeltas(ar);
    const replay = ar.events.filter((e) => e.seq > afterSeq);
    if (ar.done) {
      return { replay, finished: true, unsubscribe: () => {} };
    }
    ar.listeners.add(listener);
    return {
      replay,
      finished: false,
      unsubscribe: () => {
        ar.listeners.delete(listener);
      },
    };
  }

  cancel(runId: string): Run {
    const ar = this.runs.get(runId);
    if (!ar) {
      throw notFound(`Run not found: ${runId}`);
    }
    if (!ar.done && !ar.controller.signal.aborted) {
      ar.controller.abort();
      for (const approval of [...ar.approvals.values()]) {
        approval.finish({ approved: false, reason: "cancelled" }, "cancelled");
      }
    }
    return structuredClone(ar.run);
  }

  async cancelConversation(conversationId: string) {
    const ar = this.activeByConversation.get(conversationId);
    if (!ar) {
      return;
    }
    this.cancel(ar.run.id);
    await Promise.race([
      ar.donePromise,
      new Promise((resolve) => setTimeout(resolve, 10000).unref()),
    ]);
  }

  forgetConversation(conversationId: string) {
    this.resourceCaches.delete(conversationId);
  }

  decideApproval(
    runId: string,
    approvalId: string,
    decision: ApprovalDecision,
    reason?: string,
  ): ApprovalBlock {
    const ar = this.runs.get(runId);
    if (!ar) {
      throw notFound(`Run not found: ${runId}`);
    }
    const approval = ar.approvals.get(approvalId);
    if (!approval) {
      throw notFound(`Pending approval not found: ${approvalId}`);
    }
    if (decision !== "approve" && decision !== "reject") {
      throw badRequest('The decision must be either "approve" or "reject"');
    }
    approval.finish(
      { approved: decision === "approve", reason },
      decision === "approve" ? "approved" : "rejected",
    );
    return structuredClone(
      ar.builder.getBlock<ApprovalBlock>(approval.blockId)!,
    );
  }

  private requestApproval(
    ar: ActiveRun,
    req: ApprovalRequest,
    signal?: AbortSignal,
  ): Promise<ApprovalResult> {
    if (ar.done || ar.controller.signal.aborted) {
      return Promise.resolve({ approved: false, reason: "cancelled" });
    }

    const { builder } = ar;
    const approvalId = newID();
    const toolBlock = builder.getToolBlock(req.toolCallId);
    if (toolBlock) {
      builder.updateBlock<ToolBlock>(toolBlock.id, {
        status: "awaiting_approval",
      });
    }

    const block = builder.addBlock<ApprovalBlock>({
      type: "approval",
      approvalId,
      toolCallId: req.toolCallId,
      title: req.title,
      description: req.description,
      risk: req.risk,
      status: "pending",
      preview: req.preview,
    });
    this.setStatus(ar, "awaiting_approval");

    return new Promise<ApprovalResult>((resolve) => {
      let settled = false;
      const onAbort = () =>
        finish({ approved: false, reason: "cancelled" }, "cancelled");

      const finish = (
        result: ApprovalResult,
        status: ApprovalBlock["status"],
      ) => {
        if (settled) {
          return;
        }
        settled = true;
        signal?.removeEventListener("abort", onAbort);
        ar.approvals.delete(approvalId);

        builder.updateBlock<ApprovalBlock>(block.id, {
          status,
          reason: result.reason,
          decidedAt: new Date().toISOString(),
        });

        const tool = builder.getToolBlock(req.toolCallId);
        if (tool && status !== "cancelled") {
          builder.updateBlock<ToolBlock>(tool.id, {
            status: result.approved ? "running" : "rejected",
          });
        }

        if (ar.approvals.size === 0 && !ar.done) {
          this.setStatus(ar, "running");
        }
        resolve(result);
      };

      ar.approvals.set(approvalId, {
        blockId: block.id,
        toolCallId: req.toolCallId,
        finish,
      });
      signal?.addEventListener("abort", onAbort, { once: true });
    });
  }

  private resourceCache(conversationId: string): ResourceCache {
    let cache = this.resourceCaches.get(conversationId);
    if (!cache) {
      cache = new ResourceCache();
      this.resourceCaches.set(conversationId, cache);
    }
    return cache;
  }

  current(conversationId: string): ToolRunContext | undefined {
    const ar = this.activeByConversation.get(conversationId);
    if (!ar || ar.done) {
      return undefined;
    }
    return {
      runId: ar.run.id,
      conversationId,
      builder: ar.builder,
      resources: this.resourceCache(conversationId),
      resultsDir: this.opts.store.resultsDir(conversationId),
      requestApproval: (req, signal) => this.requestApproval(ar, req, signal),
    };
  }

  async shutdown(timeoutMs = 10000) {
    const pending = [...this.activeByConversation.values()];
    for (const ar of pending) {
      this.cancel(ar.run.id);
    }
    await Promise.race([
      Promise.all(pending.map((ar) => ar.donePromise)),
      new Promise((resolve) => setTimeout(resolve, timeoutMs).unref()),
    ]);
  }
}
