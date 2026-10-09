import {
  isTerminalRunStatus,
  type AgentEvent,
  type AgentInfo,
  type ApprovalDecision,
  type Conversation,
  type ConversationDetail,
  type FileInfo,
  type ListConversationsResponse,
  type Message,
  type RunInput,
} from "@/apis/agent/protocol";
import {
  ActionIcon,
  Button,
  Drawer,
  Menu,
  Modal,
  Textarea,
  TextInput,
  Tooltip,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconArrowDown,
  IconArrowUp,
  IconCopy,
  IconCube,
  IconDots,
  IconFileUpload,
  IconLayoutSidebarLeftExpand,
  IconMessagePlus,
  IconPaperclip,
  IconPencil,
  IconPlayerStopFilled,
  IconRefresh,
  IconSearch,
  IconShieldCheck,
  IconSparkles,
  IconTerminal2,
  IconTestPipe,
  IconTrash,
  IconWorld,
  IconX,
} from "@tabler/icons-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import dayjs from "dayjs";
import { motion } from "framer-motion";
import * as React from "react";
import { toast } from "react-hot-toast";
import { useSearchParams } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import { BlockView, ToolGroup, type BlockContext } from "./Blocks";
import { AgentClient } from "./client";
import SettingsDrawer from "./Models";
import { applyEvent, emptyChatState, type ChatState } from "./reducer";
import {
  approvalModes,
  copyText,
  formatSize,
  groupBlocks,
  modelLabel,
} from "./utils";

const Typing = () => (
  <span className="inline-flex items-center gap-1 py-1.5 text-ink-subtle">
    <i className="h-1.5 w-1.5 animate-bounce rounded-full bg-current [animation-delay:-0.3s]" />
    <i className="h-1.5 w-1.5 animate-bounce rounded-full bg-current [animation-delay:-0.15s]" />
    <i className="h-1.5 w-1.5 animate-bounce rounded-full bg-current" />
  </span>
);

const AgentAvatar = (props: { className?: string }) => (
  <span
    className={twMerge(
      "flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-inverted text-on-inverted shadow-panel",
      props.className,
    )}
  >
    <IconSparkles size={16} />
  </span>
);

const messageStatusLabel: Partial<Record<Message["status"], string>> = {
  failed: "Failed",
  cancelled: "Stopped",
  interrupted: "Interrupted",
};

const toChatState = (detail: ConversationDetail): ChatState => {
  const messages = [...detail.messages];
  const active = detail.activeRun;
  if (active?.message && !messages.some((m) => m.id === active.message!.id)) {
    messages.push(active.message);
  }
  return {
    messages,
    run: active?.run,
    lastSeq: active?.run.lastSeq ?? 0,
  };
};

const formatTokens = (n: number) =>
  n >= 1000 ? `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}k` : String(n);

const MessageView = (props: {
  message: Message;
  ctx: Omit<BlockContext, "runId" | "streaming">;
  streaming: boolean;
}) => {
  const { message } = props;

  if (message.role === "user") {
    const text = message.blocks
      .map((b) => (b.type === "markdown" ? b.text : ""))
      .join("\n");
    return (
      <motion.div
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.2, ease: "easeOut" }}
        className="flex w-full justify-end"
      >
        <div className="flex max-w-[85%] flex-col items-end gap-1.5">
          {message.attachments && message.attachments.length > 0 && (
            <div className="flex flex-wrap justify-end gap-1">
              {message.attachments.map((file) => (
                <span
                  key={file.id}
                  className="inline-flex items-center gap-1.5 rounded-lg border border-line bg-surface px-2 py-1 text-[0.75rem] font-medium text-ink-soft"
                >
                  <IconPaperclip size={12} />
                  {file.name}
                  <span className="text-ink-subtle">
                    {formatSize(file.size)}
                  </span>
                </span>
              ))}
            </div>
          )}
          {text && (
            <div className="rounded-2xl rounded-br-md bg-inverted px-4 py-2.5 text-on-inverted shadow-panel">
              <p className="whitespace-pre-wrap break-words text-[0.92rem] leading-6">
                {text}
              </p>
            </div>
          )}
        </div>
      </motion.div>
    );
  }

  const ctx: BlockContext = {
    ...props.ctx,
    runId: message.runId,
    streaming: props.streaming,
  };
  const status = messageStatusLabel[message.status];
  const groups = groupBlocks(message.blocks);
  const answer = message.blocks
    .filter((b) => b.type === "markdown")
    .map((b) => (b.type === "markdown" ? b.text : ""))
    .join("\n\n");

  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className="group/message flex w-full gap-3"
    >
      <AgentAvatar className="mt-0.5 hidden sm:flex" />
      <div className="min-w-0 flex-1">
        {groups.length === 0 && props.streaming ? (
          <Typing />
        ) : (
          groups.map((item, idx) =>
            item.type === "tools" ? (
              <ToolGroup key={item.id} blocks={item.blocks} />
            ) : (
              <BlockView
                key={item.id}
                block={item}
                ctx={ctx}
                isLast={idx === groups.length - 1}
              />
            ),
          )
        )}
        {props.streaming && groups.length > 0 && <Typing />}
        {(status || message.error) && (
          <p className="mt-2 flex items-center gap-1.5 text-[0.75rem] font-semibold text-ink-muted">
            <IconAlertCircle size={13} />
            {status}
            {message.error && !message.blocks.some((b) => b.type === "error")
              ? `: ${message.error.message}`
              : ""}
          </p>
        )}
        {!props.streaming && (
          <div className="mt-1.5 flex h-6 items-center gap-2 text-[0.72rem] font-medium text-ink-subtle opacity-0 transition-opacity duration-150 group-hover/message:opacity-100">
            {answer && (
              <Tooltip label="Copy the answer">
                <ActionIcon
                  size="sm"
                  variant="subtle"
                  color="gray"
                  aria-label="Copy the answer"
                  onClick={async () => {
                    if (await copyText(answer)) toast.success("Copied");
                  }}
                >
                  <IconCopy size={13} />
                </ActionIcon>
              </Tooltip>
            )}
            {message.model && <span>{modelLabel(message.model)}</span>}
            {message.usage && message.usage.totalTokens > 0 && (
              <span>
                · {formatTokens(message.usage.totalTokens)} tokens
                {message.usage.cost > 0
                  ? ` · $${message.usage.cost.toFixed(3)}`
                  : ""}
              </span>
            )}
          </div>
        )}
      </div>
    </motion.div>
  );
};

const groupConversations = (conversations: Conversation[]) => {
  const today = dayjs().startOf("day");
  const groups: { label: string; items: Conversation[] }[] = [
    { label: "Today", items: [] },
    { label: "Yesterday", items: [] },
    { label: "Previous 7 days", items: [] },
    { label: "Older", items: [] },
  ];
  for (const conversation of conversations) {
    const at = dayjs(conversation.updatedAt);
    if (!at.isBefore(today)) groups[0].items.push(conversation);
    else if (!at.isBefore(today.subtract(1, "day")))
      groups[1].items.push(conversation);
    else if (!at.isBefore(today.subtract(7, "day")))
      groups[2].items.push(conversation);
    else groups[3].items.push(conversation);
  }
  return groups.filter((g) => g.items.length > 0);
};

const ConversationList = (props: {
  conversations: Conversation[];
  loading: boolean;
  selected?: string;
  onSelect: (id?: string) => void;
  onRename: (conversation: Conversation) => void;
  onDelete: (conversation: Conversation) => void;
  footer?: React.ReactNode;
}) => {
  const [query, setQuery] = React.useState("");
  const filtered = query.trim()
    ? props.conversations.filter((c) =>
        c.title.toLowerCase().includes(query.trim().toLowerCase()),
      )
    : props.conversations;
  const groups = groupConversations(filtered);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-col gap-2.5 p-3">
        <Button
          fullWidth
          leftSection={<IconMessagePlus size={15} />}
          onClick={() => props.onSelect(undefined)}
        >
          New chat
        </Button>
        <TextInput
          size="xs"
          placeholder="Search conversations"
          leftSection={<IconSearch size={13} />}
          value={query}
          onChange={(event) => setQuery(event.currentTarget.value)}
        />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {groups.map((group) => (
          <div key={group.label} className="mb-2">
            <p className="px-2 pb-1 pt-2 text-[0.66rem] font-bold uppercase tracking-[0.09em] text-ink-subtle">
              {group.label}
            </p>
            <ul className="flex flex-col gap-0.5">
              {group.items.map((conversation) => {
                const isSelected = conversation.id === props.selected;
                return (
                  <li key={conversation.id}>
                    <div
                      className={twMerge(
                        "group flex items-center gap-1 rounded-lg pr-1 transition-colors duration-150",
                        isSelected
                          ? "bg-surface-strong text-ink"
                          : "text-ink-soft hover:bg-surface-muted hover:text-ink",
                      )}
                    >
                      <button
                        type="button"
                        onClick={() => props.onSelect(conversation.id)}
                        className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 px-2.5 py-2 text-left"
                      >
                        {conversation.activeRunId && (
                          <span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-hue-emerald-solid" />
                        )}
                        <span className="truncate text-[0.82rem] font-semibold">
                          {conversation.title}
                        </span>
                      </button>
                      <Menu position="bottom-end" withinPortal width={160}>
                        <Menu.Target>
                          <ActionIcon
                            size="sm"
                            variant="subtle"
                            color="gray"
                            aria-label="Conversation actions"
                            className="opacity-0 transition-opacity group-hover:opacity-100 focus:opacity-100"
                          >
                            <IconDots size={14} />
                          </ActionIcon>
                        </Menu.Target>
                        <Menu.Dropdown>
                          <Menu.Item
                            leftSection={<IconPencil size={13} />}
                            onClick={() => props.onRename(conversation)}
                          >
                            Rename
                          </Menu.Item>
                          <Menu.Item
                            color="red"
                            leftSection={<IconTrash size={13} />}
                            onClick={() => props.onDelete(conversation)}
                          >
                            Delete
                          </Menu.Item>
                        </Menu.Dropdown>
                      </Menu>
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
        {!props.loading && filtered.length === 0 && (
          <p className="px-3 py-8 text-center text-[0.78rem] text-ink-subtle">
            {query ? "No matching conversations" : "No conversations yet"}
          </p>
        )}
      </div>
      {props.footer && (
        <div className="border-t border-line p-3">{props.footer}</div>
      )}
    </div>
  );
};

const suggestions = [
  {
    icon: IconTestPipe,
    title: "Test two branches in parallel",
    text: "Create 2 Workspaces running the latest debian image with github.com/octelium/octelium as their repository, one on the main branch and the other on dev, and run all the unit tests in both",
  },
  {
    icon: IconWorld,
    title: "Call a Service from a sandbox",
    text: "Run a new Workspace in the my-space Space, curl the my-api Service at /v1/users, show me the output and then stop the Workspace",
  },
  {
    icon: IconCopy,
    title: "Move files between Workspaces",
    text: "Copy /workspace/repo/dist from my Workspace abc to def",
  },
  {
    icon: IconCube,
    title: "Review my Workspaces",
    text: "Which of my Workspaces are running, what are they for and which ones look idle?",
  },
];

const EmptyState = (props: {
  user?: string;
  hasModel: boolean;
  disabled: boolean;
  onPick: (text: string) => void;
  onSettings: () => void;
}) => (
  <div className="mx-auto flex w-full max-w-3xl flex-1 flex-col items-center justify-center px-1 py-8 text-center">
    <motion.div
      initial={{ opacity: 0, scale: 0.92 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.3, ease: "easeOut" }}
    >
      <AgentAvatar className="h-12 w-12 rounded-2xl [&_svg]:h-6 [&_svg]:w-6" />
    </motion.div>
    <h2 className="mt-4 text-xl font-bold tracking-tight text-ink">
      {props.user
        ? `What are we building today, ${props.user}?`
        : "What are we building today?"}
    </h2>
    <p className="mt-2 max-w-xl text-[0.86rem] leading-6 text-ink-muted">
      I run inside my own Workspace with your identity. I can create and run
      Workspaces, execute commands and test suites in them, move files between
      them and reach the Octelium Services that you can access.
    </p>
    {!props.hasModel && (
      <Button
        className="mt-4"
        variant="light"
        leftSection={<IconSparkles size={15} />}
        onClick={props.onSettings}
      >
        Sign in with Claude or ChatGPT to get started
      </Button>
    )}
    <div className="mt-7 grid w-full gap-2.5 sm:grid-cols-2">
      {suggestions.map((suggestion, idx) => (
        <motion.button
          key={suggestion.title}
          type="button"
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.05 * idx, ease: "easeOut" }}
          disabled={props.disabled}
          onClick={() => props.onPick(suggestion.text)}
          className="group flex cursor-pointer items-start gap-3 rounded-xl border border-line bg-surface px-3.5 py-3 text-left shadow-panel transition-all duration-150 hover:-translate-y-0.5 hover:border-line-strong disabled:cursor-not-allowed disabled:opacity-60"
        >
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-soft transition-colors group-hover:bg-inverted group-hover:text-on-inverted">
            <suggestion.icon size={16} />
          </span>
          <span className="min-w-0">
            <span className="block text-[0.84rem] font-bold text-ink">
              {suggestion.title}
            </span>
            <span className="mt-0.5 line-clamp-2 block text-[0.76rem] leading-5 text-ink-muted">
              {suggestion.text}
            </span>
          </span>
        </motion.button>
      ))}
    </div>
  </div>
);

const RenameForm = (props: {
  conversation: Conversation;
  onClose: () => void;
  onSubmit: (conversation: Conversation, title: string) => void;
}) => {
  const [title, setTitle] = React.useState(props.conversation.title);

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        if (title.trim()) {
          props.onSubmit(props.conversation, title.trim());
        }
      }}
      className="flex flex-col gap-4"
    >
      <TextInput
        data-autofocus
        value={title}
        maxLength={200}
        onChange={(event) => setTitle(event.currentTarget.value)}
      />
      <div className="flex justify-end gap-2">
        <Button variant="default" onClick={props.onClose}>
          Cancel
        </Button>
        <Button type="submit" disabled={!title.trim()}>
          Rename
        </Button>
      </div>
    </form>
  );
};

const RenameModal = (props: {
  conversation?: Conversation;
  onClose: () => void;
  onSubmit: (conversation: Conversation, title: string) => void;
}) => (
  <Modal
    opened={!!props.conversation}
    onClose={props.onClose}
    title="Rename the conversation"
  >
    {props.conversation && (
      <RenameForm
        key={props.conversation.id}
        conversation={props.conversation}
        onClose={props.onClose}
        onSubmit={props.onSubmit}
      />
    )}
  </Modal>
);

const DeleteModal = (props: {
  conversation?: Conversation;
  onClose: () => void;
  onConfirm: (conversation: Conversation) => void;
}) => (
  <Modal
    opened={!!props.conversation}
    onClose={props.onClose}
    title="Delete the conversation"
  >
    <p className="text-sm text-ink-muted">
      Delete “{props.conversation?.title}”? Its messages are deleted from the
      agent. The Workspaces and files that it created are kept.
    </p>
    <div className="mt-5 flex justify-end gap-2">
      <Button variant="default" onClick={props.onClose}>
        Cancel
      </Button>
      <Button
        color="red"
        leftSection={<IconTrash size={14} />}
        onClick={() =>
          props.conversation && props.onConfirm(props.conversation)
        }
      >
        Delete
      </Button>
    </div>
  </Modal>
);

const Chat = (props: {
  client: AgentClient;
  info: AgentInfo;
  workspace?: React.ReactNode;
  actions?: React.ReactNode;
}) => {
  const { client, info } = props;
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const selected = searchParams.get("c") ?? undefined;

  const [chat, setChat] = React.useState<ChatState>(emptyChatState);
  const [loading, setLoading] = React.useState(!!selected);
  const [input, setInput] = React.useState("");
  const [attachments, setAttachments] = React.useState<FileInfo[]>([]);
  const [uploading, setUploading] = React.useState(0);
  const [sending, setSending] = React.useState(false);
  const [settingsOpened, setSettingsOpened] = React.useState(false);
  const [listOpened, setListOpened] = React.useState(false);
  const [dragging, setDragging] = React.useState(false);
  const [atBottom, setAtBottom] = React.useState(true);
  const [renaming, setRenaming] = React.useState<Conversation>();
  const [deleting, setDeleting] = React.useState<Conversation>();
  const fileRef = React.useRef<HTMLInputElement>(null);
  const inputRef = React.useRef<HTMLTextAreaElement>(null);
  const transcriptRef = React.useRef<HTMLDivElement>(null);
  const stickToBottomRef = React.useRef(true);

  const conversationsKey = React.useMemo(
    () => ["agent", client.baseUrl, "conversations"],
    [client.baseUrl],
  );
  const conversationsQuery = useQuery({
    queryKey: conversationsKey,
    queryFn: () => client.listConversations(),
  });

  const modelsQuery = useQuery({
    queryKey: ["agent", client.baseUrl, "models"],
    queryFn: () => client.listModels(),
    enabled: info.capabilities.models,
  });
  const settingsQuery = useQuery({
    queryKey: ["agent", client.baseUrl, "settings"],
    queryFn: () => client.getSettings(),
    enabled: info.capabilities.settings,
    initialData: info.settings,
  });
  const currentModel = modelsQuery.data?.current ?? info.model;
  const approvalMode = approvalModes.find(
    (m) => m.value === settingsQuery.data?.approvals.api,
  );

  const setSelected = React.useCallback(
    (id?: string) => {
      setListOpened(false);
      setSearchParams(
        (params) => {
          const ret = new URLSearchParams(params);
          if (id) {
            ret.set("c", id);
          } else {
            ret.delete("c");
          }
          return ret;
        },
        { replace: false },
      );
    },
    [setSearchParams],
  );

  const onConversationUpdated = React.useCallback(
    (conversation: Conversation) => {
      queryClient.setQueryData<ListConversationsResponse>(
        conversationsKey,
        (data) => {
          const items = (data?.items ?? []).filter(
            (itm) => itm.id !== conversation.id,
          );
          return { items: [conversation, ...items] };
        },
      );
    },
    [queryClient, conversationsKey],
  );

  const [view, setView] = React.useState<{ id?: string; fresh: boolean }>({
    id: selected,
    fresh: false,
  });
  if (view.id !== selected) {
    setView({ id: selected, fresh: false });
    setChat(emptyChatState);
    setLoading(!!selected);
  }

  const viewRef = React.useRef(view.id);
  React.useEffect(() => {
    viewRef.current = view.id;
  }, [view.id]);

  const fetchConversation = React.useCallback(
    (id: string, isCurrent: () => boolean) =>
      client
        .getConversation(id)
        .then((detail) => {
          if (isCurrent()) setChat(toChatState(detail));
        })
        .catch((err: Error) => {
          if (isCurrent()) {
            toast.error(err.message);
            setChat(emptyChatState);
          }
        })
        .finally(() => {
          if (isCurrent()) setLoading(false);
        }),
    [client],
  );

  React.useEffect(() => {
    stickToBottomRef.current = true;
    if (!view.id) {
      inputRef.current?.focus();
      return;
    }
    if (view.fresh) {
      return;
    }
    let active = true;
    void fetchConversation(view.id, () => active);
    return () => {
      active = false;
    };
  }, [view, fetchConversation]);

  const run = chat.run;
  const isActive = !!run && !isTerminalRunStatus(run.status);
  const runId = run?.id;
  const conversationId = run?.conversationId;

  const lastSeqRef = React.useRef(chat.lastSeq);
  React.useEffect(() => {
    lastSeqRef.current = chat.lastSeq;
  }, [chat.lastSeq]);

  React.useEffect(() => {
    if (!isActive || !runId || !conversationId) return;
    const afterSeq = lastSeqRef.current;

    const unsubscribe = client.subscribe(
      runId,
      afterSeq,
      (event: AgentEvent) => {
        if (event.type === "conversation.updated") {
          onConversationUpdated(event.conversation);
        }
        setChat((state) =>
          viewRef.current === event.conversationId
            ? applyEvent(state, event)
            : state,
        );
      },
      () => {
        void queryClient.invalidateQueries({ queryKey: conversationsKey });
        if (viewRef.current === conversationId) {
          void fetchConversation(
            conversationId,
            () => viewRef.current === conversationId,
          );
        }
      },
    );

    return unsubscribe;
  }, [
    client,
    isActive,
    runId,
    conversationId,
    onConversationUpdated,
    queryClient,
    conversationsKey,
    fetchConversation,
  ]);

  React.useEffect(() => {
    const el = transcriptRef.current;
    if (el && stickToBottomRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [chat.messages]);

  const scrollToBottom = () => {
    const el = transcriptRef.current;
    if (!el) return;
    stickToBottomRef.current = true;
    el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  };

  const addFiles = async (files: FileList | File[] | null) => {
    if (!files) return;
    for (const file of Array.from(files)) {
      if (file.size > info.capabilities.uploads.maxBytes) {
        toast.error(
          `${file.name} exceeds the maximum size of ${formatSize(info.capabilities.uploads.maxBytes)}`,
        );
        continue;
      }
      setUploading((n) => n + 1);
      try {
        const uploaded = await client.uploadFile(file);
        setAttachments((current) => [...current, uploaded]);
      } catch (err) {
        toast.error(`Could not upload ${file.name}: ${(err as Error).message}`);
      } finally {
        setUploading((n) => n - 1);
      }
    }
  };

  const send = async (textArg?: string) => {
    const text = (textArg ?? input).trim();
    if ((!text && attachments.length === 0) || isActive || sending) return;

    const runInput: RunInput = {
      text,
      attachments:
        attachments.length > 0 ? attachments.map((a) => a.id) : undefined,
    };

    setSending(true);
    stickToBottomRef.current = true;
    try {
      if (selected) {
        const next = await client.startRun(selected, runInput);
        setChat((state) => ({ ...state, run: next, lastSeq: 0 }));
      } else {
        const res = await client.createConversation(runInput);
        setView({ id: res.conversation.id, fresh: true });
        setLoading(false);
        setChat({ messages: [], run: res.run, lastSeq: 0 });
        onConversationUpdated(res.conversation);
        setSelected(res.conversation.id);
      }
      setInput("");
      setAttachments([]);
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setSending(false);
    }
  };

  const stop = async () => {
    if (!run) return;
    try {
      await client.cancelRun(run.id);
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const onDecide = React.useCallback(
    async (
      decisionRunId: string,
      approvalId: string,
      decision: ApprovalDecision,
    ) => {
      try {
        await client.decideApproval(decisionRunId, approvalId, decision);
      } catch (err) {
        toast.error((err as Error).message);
      }
    },
    [client],
  );

  const rename = async (conversation: Conversation, title: string) => {
    setRenaming(undefined);
    if (title === conversation.title) return;
    try {
      onConversationUpdated(
        await client.renameConversation(conversation.id, title),
      );
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const remove = async (conversation: Conversation) => {
    setDeleting(undefined);
    try {
      await client.deleteConversation(conversation.id);
      queryClient.setQueryData<ListConversationsResponse>(
        conversationsKey,
        (data) => ({
          items: (data?.items ?? []).filter(
            (itm) => itm.id !== conversation.id,
          ),
        }),
      );
      if (selected === conversation.id) {
        setSelected(undefined);
      }
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const blockCtx = React.useMemo(
    () => ({ client, onDecide }),
    [client, onDecide],
  );
  const lastAssistantID = [...chat.messages]
    .reverse()
    .find((m) => m.role === "assistant")?.id;
  const conversations = conversationsQuery.data?.items ?? [];
  const current = conversations.find((c) => c.id === selected);
  const userName =
    info.cluster.user?.displayName?.split(" ")[0] ??
    info.cluster.user?.name?.split(".")[0];

  const list = (
    <ConversationList
      conversations={conversations}
      loading={conversationsQuery.isPending}
      selected={selected}
      onSelect={setSelected}
      onRename={setRenaming}
      onDelete={setDeleting}
      footer={props.workspace}
    />
  );

  return (
    <div className="flex h-full min-h-0 overflow-hidden">
      <aside className="hidden w-[272px] shrink-0 flex-col border-r border-line bg-canvas md:flex">
        {list}
      </aside>
      <Drawer
        opened={listOpened}
        onClose={() => setListOpened(false)}
        position="left"
        size={300}
        padding={0}
        withCloseButton={false}
      >
        <div className="h-dvh">{list}</div>
      </Drawer>

      <div
        className="relative flex min-w-0 flex-1 flex-col bg-surface"
        onDragOver={(event) => {
          if (event.dataTransfer.types.includes("Files")) {
            event.preventDefault();
            setDragging(true);
          }
        }}
        onDragLeave={(event) => {
          if (event.currentTarget === event.target) setDragging(false);
        }}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          void addFiles(event.dataTransfer.files);
        }}
      >
        <header className="flex h-[52px] shrink-0 items-center gap-2 border-b border-line px-3 md:px-4">
          <ActionIcon
            variant="subtle"
            color="gray"
            hiddenFrom="md"
            aria-label="Conversations"
            onClick={() => setListOpened(true)}
          >
            <IconLayoutSidebarLeftExpand size={18} />
          </ActionIcon>
          <button
            type="button"
            disabled={!current}
            onClick={() => current && setRenaming(current)}
            className="min-w-0 flex-1 cursor-pointer text-left disabled:cursor-default"
          >
            <span className="block truncate text-[0.9rem] font-bold text-ink">
              {current?.title ?? "New conversation"}
            </span>
          </button>
          <Tooltip label={approvalMode?.description ?? "Approvals"}>
            <Button
              size="compact-sm"
              variant="subtle"
              color="gray"
              visibleFrom="sm"
              leftSection={<IconShieldCheck size={14} />}
              onClick={() => setSettingsOpened(true)}
            >
              {approvalMode ? `Ask: ${approvalMode.label}` : "Approvals"}
            </Button>
          </Tooltip>
          <Button
            size="compact-sm"
            variant="default"
            leftSection={<IconSparkles size={14} />}
            onClick={() => setSettingsOpened(true)}
            className="max-w-[220px]"
          >
            <span className="truncate">{modelLabel(currentModel)}</span>
          </Button>
          {props.actions}
        </header>

        <div
          ref={transcriptRef}
          onScroll={(event) => {
            const el = event.currentTarget;
            const bottom =
              el.scrollHeight - el.scrollTop - el.clientHeight < 80;
            stickToBottomRef.current = bottom;
            setAtBottom(bottom);
          }}
          className="flex min-h-0 flex-1 flex-col overflow-y-auto px-3 py-6 md:px-6"
        >
          {loading && chat.messages.length === 0 ? (
            <div className="flex flex-1 items-center justify-center">
              <Typing />
            </div>
          ) : chat.messages.length === 0 ? (
            <EmptyState
              user={userName}
              hasModel={!!currentModel}
              disabled={sending}
              onPick={(text) => void send(text)}
              onSettings={() => setSettingsOpened(true)}
            />
          ) : (
            <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
              {chat.messages.map((message) => (
                <MessageView
                  key={message.id}
                  message={message}
                  ctx={blockCtx}
                  streaming={
                    isActive &&
                    message.id === lastAssistantID &&
                    message.status === "streaming"
                  }
                />
              ))}
            </div>
          )}
        </div>

        <div className="shrink-0 px-3 pb-3 md:px-6 md:pb-4">
          <div className="relative mx-auto w-full max-w-3xl">
            {!atBottom && chat.messages.length > 0 && (
              <div className="pointer-events-none absolute inset-x-0 -top-14 flex justify-center">
                <ActionIcon
                  size="lg"
                  radius="xl"
                  variant="default"
                  aria-label="Scroll to the bottom"
                  className="pointer-events-auto shadow-panel"
                  onClick={scrollToBottom}
                >
                  <IconArrowDown size={16} />
                </ActionIcon>
              </div>
            )}
            {run?.activity && isActive && (
              <p className="mb-2 flex items-center gap-1.5 text-[0.75rem] font-semibold text-ink-muted">
                <IconRefresh size={13} className="animate-spin" />
                {run.activity.message}
              </p>
            )}
            {run?.status === "awaiting_approval" && (
              <p className="mb-2 flex items-center gap-1.5 text-[0.75rem] font-semibold text-hue-amber">
                <IconShieldCheck size={13} />
                The agent is waiting for your approval
              </p>
            )}

            <div
              className={twMerge(
                "rounded-2xl border border-line bg-surface shadow-panel transition-colors duration-150 focus-within:border-line-strong",
                dragging && "border-dashed border-ink",
              )}
            >
              {(attachments.length > 0 || uploading > 0) && (
                <div className="flex flex-wrap gap-1.5 px-3 pt-3">
                  {attachments.map((file) => (
                    <span
                      key={file.id}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-line bg-surface-subtle py-1 pl-2 pr-1 text-[0.75rem] text-ink-soft"
                    >
                      <IconPaperclip size={12} />
                      <span className="max-w-[160px] truncate">
                        {file.name}
                      </span>
                      <span className="text-ink-subtle">
                        {formatSize(file.size)}
                      </span>
                      <button
                        type="button"
                        aria-label={`Remove ${file.name}`}
                        onClick={() =>
                          setAttachments((current) =>
                            current.filter((itm) => itm.id !== file.id),
                          )
                        }
                        className="flex h-4 w-4 cursor-pointer items-center justify-center rounded text-ink-subtle hover:bg-surface-strong hover:text-ink"
                      >
                        <IconX size={11} />
                      </button>
                    </span>
                  ))}
                  {uploading > 0 && (
                    <span className="rounded-lg border border-dashed border-line-strong px-2 py-1 text-[0.75rem] text-ink-muted">
                      Uploading…
                    </span>
                  )}
                </div>
              )}

              {dragging ? (
                <div className="flex min-h-[84px] items-center justify-center gap-2 text-[0.84rem] font-semibold text-ink-soft">
                  <IconFileUpload size={18} />
                  Drop the files to attach them
                </div>
              ) : (
                <Textarea
                  ref={inputRef}
                  aria-label="Message"
                  placeholder="Ask the agent to create Workspaces, run commands, move files…"
                  autosize
                  minRows={2}
                  maxRows={12}
                  variant="unstyled"
                  className="px-4 pt-3"
                  classNames={{
                    input:
                      "text-[0.92rem] leading-6 text-ink placeholder:text-ink-subtle",
                  }}
                  value={input}
                  onChange={(event) => setInput(event.currentTarget.value)}
                  onPaste={(event) => {
                    const files = Array.from(event.clipboardData.files);
                    if (files.length > 0) {
                      event.preventDefault();
                      void addFiles(files);
                    }
                  }}
                  onKeyDown={(event) => {
                    if (
                      event.key === "Enter" &&
                      !event.shiftKey &&
                      !event.nativeEvent.isComposing
                    ) {
                      event.preventDefault();
                      void send();
                    }
                  }}
                />
              )}

              <div className="flex items-center justify-between gap-2 px-2.5 pb-2.5 pt-1">
                <div className="flex items-center gap-1">
                  <input
                    ref={fileRef}
                    type="file"
                    multiple
                    hidden
                    onChange={(event) => {
                      void addFiles(event.currentTarget.files);
                      event.currentTarget.value = "";
                    }}
                  />
                  <Tooltip label="Attach files">
                    <ActionIcon
                      variant="subtle"
                      color="gray"
                      aria-label="Attach files"
                      onClick={() => fileRef.current?.click()}
                    >
                      <IconPaperclip size={17} />
                    </ActionIcon>
                  </Tooltip>
                  <span className="hidden items-center gap-1 pl-1 text-[0.72rem] font-medium text-ink-subtle sm:inline-flex">
                    <IconTerminal2 size={12} />
                    Runs in{" "}
                    <span className="font-mono">
                      {info.workspace?.name ?? "your Workspace"}
                    </span>
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="hidden text-[0.72rem] text-ink-subtle lg:inline">
                    Enter to send · Shift + Enter for a new line
                  </span>
                  {isActive ? (
                    <Tooltip label="Stop">
                      <ActionIcon
                        size="lg"
                        radius="xl"
                        color="red"
                        variant="filled"
                        aria-label="Stop"
                        onClick={() => void stop()}
                      >
                        <IconPlayerStopFilled size={15} />
                      </ActionIcon>
                    </Tooltip>
                  ) : (
                    <Tooltip label="Send">
                      <ActionIcon
                        size="lg"
                        radius="xl"
                        variant="filled"
                        aria-label="Send"
                        loading={sending}
                        disabled={
                          uploading > 0 ||
                          (input.trim() === "" && attachments.length === 0)
                        }
                        onClick={() => void send()}
                      >
                        <IconArrowUp size={17} />
                      </ActionIcon>
                    </Tooltip>
                  )}
                </div>
              </div>
            </div>
            <p className="mt-2 text-center text-[0.7rem] text-ink-subtle">
              The agent acts with your identity and can make mistakes. Review
              its actions on important resources.
            </p>
          </div>
        </div>
      </div>

      <SettingsDrawer
        client={client}
        opened={settingsOpened}
        onClose={() => setSettingsOpened(false)}
      />
      <RenameModal
        conversation={renaming}
        onClose={() => setRenaming(undefined)}
        onSubmit={(conversation, title) => void rename(conversation, title)}
      />
      <DeleteModal
        conversation={deleting}
        onClose={() => setDeleting(undefined)}
        onConfirm={(conversation) => void remove(conversation)}
      />
    </div>
  );
};

export default Chat;
