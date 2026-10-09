import {
  API_PREFIX,
  TERMINAL_RUN_EVENT_TYPES,
  type AgentEvent,
  type AgentEventType,
  type AgentInfo,
  type AgentSettings,
  type ApprovalDecision,
  type ArtifactInfo,
  type Conversation,
  type ConversationDetail,
  type CreateConversationResponse,
  type FileInfo,
  type ListAuthProvidersResponse,
  type ListConversationsResponse,
  type LoginSession,
  type ModelsResponse,
  type Run,
  type RunInput,
  type SetModelRequest,
  type UpdateSettingsRequest,
} from "@/apis/agent/protocol";

export class AgentRequestError extends Error {
  status: number;
  code?: string;

  constructor(status: number, message: string, code?: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export const AGENT_EVENT_TYPES: AgentEventType[] = [
  "run.started",
  "run.updated",
  "run.completed",
  "run.failed",
  "run.cancelled",
  "message.created",
  "message.completed",
  "block.created",
  "block.delta",
  "block.updated",
  "conversation.updated",
];

export class AgentClient {
  readonly baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl.replace(/\/+$/, "");
  }

  url(path: string): string {
    return `${this.baseUrl}${API_PREFIX}${path}`;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    init?: Parameters<typeof fetch>[1],
  ): Promise<T> {
    const res = await fetch(this.url(path), {
      method,
      credentials: "include",
      headers:
        body === undefined ? undefined : { "content-type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
      ...init,
    });

    if (!res.ok) {
      let message = `${res.status} ${res.statusText}`;
      let code: string | undefined;
      try {
        const err = (await res.json()) as {
          error?: { code?: string; message?: string };
        };
        message = err.error?.message ?? message;
        code = err.error?.code;
      } catch {
        code = undefined;
      }
      throw new AgentRequestError(res.status, message, code);
    }

    if (res.status === 204) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }

  info(signal?: AbortSignal) {
    return this.request<AgentInfo>("GET", "/info", undefined, { signal });
  }

  listConversations() {
    return this.request<ListConversationsResponse>("GET", "/conversations");
  }

  getConversation(id: string) {
    return this.request<ConversationDetail>("GET", `/conversations/${id}`);
  }

  createConversation(input: RunInput) {
    return this.request<CreateConversationResponse>("POST", "/conversations", {
      input,
    });
  }

  renameConversation(id: string, title: string) {
    return this.request<Conversation>("PATCH", `/conversations/${id}`, {
      title,
    });
  }

  deleteConversation(id: string) {
    return this.request<void>("DELETE", `/conversations/${id}`);
  }

  startRun(conversationId: string, input: RunInput) {
    return this.request<Run>("POST", `/conversations/${conversationId}/runs`, {
      input,
    });
  }

  cancelRun(runId: string) {
    return this.request<Run>("POST", `/runs/${runId}/cancel`, {});
  }

  decideApproval(
    runId: string,
    approvalId: string,
    decision: ApprovalDecision,
    reason?: string,
  ) {
    return this.request<Run>("POST", `/runs/${runId}/approvals/${approvalId}`, {
      decision,
      reason,
    });
  }

  uploadFile(file: File) {
    return this.request<FileInfo>(
      "POST",
      `/files?name=${encodeURIComponent(file.name)}`,
      undefined,
      {
        headers: {
          "content-type": file.type || "application/octet-stream",
        },
        body: file,
      },
    );
  }

  async downloadArtifact(artifact: ArtifactInfo): Promise<Blob> {
    const res = await fetch(`${this.baseUrl}${artifact.url}`, {
      credentials: "include",
    });
    if (!res.ok) {
      throw new AgentRequestError(
        res.status,
        `Could not download ${artifact.name}`,
      );
    }
    return res.blob();
  }

  listModels() {
    return this.request<ModelsResponse>("GET", "/models");
  }

  setModel(req: SetModelRequest) {
    return this.request<ModelsResponse>("PUT", "/model", req);
  }

  getSettings() {
    return this.request<AgentSettings>("GET", "/settings");
  }

  updateSettings(req: UpdateSettingsRequest) {
    return this.request<AgentSettings>("PATCH", "/settings", req);
  }

  listAuthProviders() {
    return this.request<ListAuthProvidersResponse>("GET", "/auth/providers");
  }

  logout(provider: string) {
    return this.request<void>("DELETE", `/auth/providers/${provider}`);
  }

  startLogin(provider: string) {
    return this.request<LoginSession>("POST", "/auth/logins", {
      provider,
      selectModel: true,
    });
  }

  getLogin(id: string) {
    return this.request<LoginSession>("GET", `/auth/logins/${id}`);
  }

  answerLogin(id: string, promptId: string, value: string) {
    return this.request<LoginSession>(
      "POST",
      `/auth/logins/${id}/prompts/${promptId}`,
      { value },
    );
  }

  cancelLogin(id: string) {
    return this.request<LoginSession>("DELETE", `/auth/logins/${id}`);
  }

  subscribe(
    runId: string,
    afterSeq: number,
    onEvent: (event: AgentEvent) => void,
    onClose: () => void,
  ): () => void {
    const source = new EventSource(
      this.url(`/runs/${runId}/events?after=${afterSeq}`),
      { withCredentials: true },
    );

    let closed = false;
    const close = () => {
      if (closed) return;
      closed = true;
      source.close();
      onClose();
    };

    const handler = (msg: MessageEvent<string>) => {
      let event: AgentEvent;
      try {
        event = JSON.parse(msg.data) as AgentEvent;
      } catch {
        return;
      }
      onEvent(event);
      if (TERMINAL_RUN_EVENT_TYPES.includes(event.type)) {
        close();
      }
    };

    for (const type of AGENT_EVENT_TYPES) {
      source.addEventListener(type, handler as (event: Event) => void);
    }

    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        close();
      }
    };

    return () => {
      closed = true;
      source.close();
    };
  }
}
