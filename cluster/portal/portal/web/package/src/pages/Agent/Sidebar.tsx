import type { Conversation } from "@/apis/agent/protocol";
import { ActionIcon, Menu, Tooltip } from "@mantine/core";
import {
  IconDots,
  IconEdit,
  IconLayoutSidebarLeftCollapse,
  IconMessages,
  IconPencil,
  IconSearch,
  IconSearchOff,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import { AnimatePresence, motion } from "framer-motion";
import * as React from "react";
import { twMerge } from "tailwind-merge";
import { Highlight, Kbd, StatusDot } from "./ui";
import {
  groupConversations,
  matchesTerms,
  modKey,
  searchTerms,
} from "./utils";

const ConversationItem = React.memo(
  (props: {
    conversation: Conversation;
    selected: boolean;
    terms: string[];
    onSelect: (id: string) => void;
    onRename: (conversation: Conversation, title: string) => Promise<void>;
    onDelete: (conversation: Conversation) => void;
  }) => {
    const { conversation, selected } = props;
    const [editing, setEditing] = React.useState(false);
    const [menuOpened, setMenuOpened] = React.useState(false);
    const [title, setTitle] = React.useState(conversation.title);
    const inputRef = React.useRef<HTMLInputElement>(null);

    const startEditing = () => {
      setTitle(conversation.title);
      setEditing(true);
      requestAnimationFrame(() => inputRef.current?.select());
    };

    const commit = async () => {
      setEditing(false);
      const value = title.trim();
      if (value === "" || value === conversation.title) return;
      await props.onRename(conversation, value);
    };

    return (
      <li className="relative">
        {selected && (
          <motion.span
            layoutId="agent-conversation-selected"
            transition={{ type: "spring", bounce: 0.12, duration: 0.35 }}
            className="absolute inset-0 rounded-lg bg-surface shadow-panel ring-1 ring-line"
          />
        )}
        <div
          className={twMerge(
            "group relative flex h-9 items-center rounded-lg text-[0.82rem]/5 transition-colors duration-150",
            !selected && "hover:bg-surface-strong/50",
          )}
        >
          {editing ? (
            <input
              ref={inputRef}
              aria-label="Conversation title"
              value={title}
              maxLength={200}
              onChange={(event) => setTitle(event.currentTarget.value)}
              onBlur={() => void commit()}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void commit();
                } else if (event.key === "Escape") {
                  event.preventDefault();
                  setEditing(false);
                }
              }}
              className="mx-1 h-7 min-w-0 flex-1 rounded-md border border-line-strong bg-surface px-1.5 text-ink outline-none ring-2 ring-ink/10"
            />
          ) : (
            <button
              type="button"
              onClick={() => props.onSelect(conversation.id)}
              onDoubleClick={startEditing}
              aria-current={selected ? "page" : undefined}
              title={conversation.title}
              className={twMerge(
                "flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-lg pl-2.5 pr-8 text-left outline-none focus-visible:ring-2 focus-visible:ring-focus",
                selected
                  ? "text-ink"
                  : "text-ink-soft group-hover:text-ink",
              )}
            >
              <span
                className={twMerge(
                  "min-w-0 flex-1 truncate",
                  selected ? "font-semibold" : "font-medium",
                )}
              >
                <Highlight text={conversation.title} terms={props.terms} />
              </span>
              {conversation.activeRunId && (
                <Tooltip label="Working">
                  <span className="flex shrink-0 items-center">
                    <StatusDot tone="blue" pulse />
                  </span>
                </Tooltip>
              )}
            </button>
          )}
          {!editing && (
            <span
              className={twMerge(
                "absolute right-1 flex transition-opacity duration-150 focus-within:opacity-100 [@media(hover:none)]:opacity-100",
                selected || menuOpened
                  ? "opacity-100"
                  : "opacity-0 group-hover:opacity-100",
              )}
            >
              <Menu
                position="bottom-end"
                withinPortal
                width={160}
                opened={menuOpened}
                onChange={setMenuOpened}
              >
                <Menu.Target>
                  <ActionIcon
                    size="sm"
                    variant="subtle"
                    color="gray"
                    aria-label="Conversation actions"
                  >
                    <IconDots size={15} />
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item
                    leftSection={<IconPencil size={13} />}
                    onClick={startEditing}
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
            </span>
          )}
        </div>
      </li>
    );
  },
);

const SectionLabel = (props: { children: React.ReactNode }) => (
  <p className="px-2.5 pb-1 pt-4 text-[0.68rem] font-bold uppercase tracking-[0.08em] text-ink-subtle first:pt-1">
    {props.children}
  </p>
);

const ListSkeleton = () => (
  <div className="space-y-1.5 px-1 pt-2 motion-safe:animate-pulse">
    {[72, 56, 84, 64, 48].map((width) => (
      <div key={width} className="flex h-9 items-center px-2.5">
        <div
          className="h-2.5 rounded-full bg-surface-strong"
          style={{ width: `${width}%` }}
        />
      </div>
    ))}
  </div>
);

const Sidebar = (props: {
  conversations: Conversation[];
  loading: boolean;
  selected?: string;
  onSelect: (id: string) => void;
  onNew: () => void;
  onRename: (conversation: Conversation, title: string) => Promise<void>;
  onDelete: (conversation: Conversation) => void;
  onClose: () => void;
  closeLabel: string;
  searchRef: React.RefObject<HTMLInputElement | null>;
  footer?: React.ReactNode;
}) => {
  const { searchRef } = props;
  const [query, setQuery] = React.useState("");
  const terms = React.useMemo(() => searchTerms(query), [query]);
  const searching = terms.length > 0;

  const groups = React.useMemo(
    () => groupConversations(props.conversations),
    [props.conversations],
  );

  const matches = React.useMemo(
    () =>
      searching
        ? props.conversations.filter((c) => matchesTerms(c.title, terms))
        : [],
    [props.conversations, searching, terms],
  );

  const itemProps = {
    terms,
    onSelect: props.onSelect,
    onRename: props.onRename,
    onDelete: props.onDelete,
  };

  return (
    <div className="flex h-full w-full flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 px-3">
        <span className="min-w-0 flex-1 truncate text-[0.7rem] font-bold uppercase tracking-[0.09em] text-ink-muted">
          Conversations
        </span>
        <Tooltip label={props.closeLabel}>
          <ActionIcon
            variant="subtle"
            color="gray"
            aria-label={props.closeLabel}
            onClick={props.onClose}
          >
            <IconLayoutSidebarLeftCollapse size={17} />
          </ActionIcon>
        </Tooltip>
      </div>

      <div className="space-y-2 px-3 pb-2">
        <button
          type="button"
          onClick={props.onNew}
          className="group flex h-9 w-full cursor-pointer items-center gap-2 rounded-lg border border-line bg-surface px-2.5 text-ink-strong shadow-panel outline-none transition-[border-color,box-shadow] duration-200 hover:border-line-strong hover:shadow-raised focus-visible:ring-2 focus-visible:ring-focus"
        >
          <IconEdit size={15} stroke={2} className="text-ink-muted" />
          <span className="flex-1 text-left text-[0.82rem]/5 font-semibold">
            New chat
          </span>
          <span className="hidden items-center gap-0.5 lg:flex">
            <Kbd>{modKey}</Kbd>
            <Kbd>⇧</Kbd>
            <Kbd>O</Kbd>
          </span>
        </button>

        <div className="relative text-base sm:text-[0.82rem]">
          <IconSearch
            size={14}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-subtle"
          />
          <input
            ref={searchRef}
            type="search"
            value={query}
            aria-label="Search conversations"
            placeholder="Search"
            onChange={(event) => setQuery(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                if (query) {
                  event.preventDefault();
                  setQuery("");
                } else {
                  event.currentTarget.blur();
                }
              } else if (event.key === "Enter" && matches[0]) {
                props.onSelect(matches[0].id);
              }
            }}
            className="h-9 w-full rounded-lg border border-transparent bg-surface-strong/50 pl-8 pr-14 text-ink outline-none transition-[background-color,border-color,box-shadow] duration-150 placeholder:text-ink-muted hover:bg-surface-strong/70 focus:border-line-strong focus:bg-surface focus:shadow-[0_0_0_3px_color-mix(in_oklab,var(--app-ink)_6%,transparent)] [&::-webkit-search-cancel-button]:hidden"
          />
          <span className="absolute right-1.5 top-1/2 flex -translate-y-1/2 items-center gap-1">
            {query ? (
              <ActionIcon
                size="sm"
                variant="subtle"
                color="gray"
                aria-label="Clear the search"
                onClick={() => {
                  setQuery("");
                  searchRef.current?.focus();
                }}
              >
                <IconX size={13} />
              </ActionIcon>
            ) : (
              <span className="hidden items-center gap-0.5 lg:flex">
                <Kbd>{modKey}</Kbd>
                <Kbd>K</Kbd>
              </span>
            )}
          </span>
        </div>
      </div>

      <nav
        aria-label="Conversations"
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 pb-3"
      >
        {props.loading ? (
          <ListSkeleton />
        ) : searching ? (
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key="search"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.12 }}
            >
              {matches.length > 0 ? (
                <>
                  <SectionLabel>Titles</SectionLabel>
                  <ul className="space-y-px">
                    {matches.map((conversation) => (
                      <ConversationItem
                        key={conversation.id}
                        conversation={conversation}
                        selected={conversation.id === props.selected}
                        {...itemProps}
                      />
                    ))}
                  </ul>
                </>
              ) : (
                <div className="flex flex-col items-center px-4 py-10 text-center">
                  <IconSearchOff size={20} className="text-ink-subtle" />
                  <p className="mt-2 text-xs font-semibold text-ink-soft">
                    No results
                  </p>
                  <p className="mt-0.5 text-[0.7rem]/4 text-ink-muted">
                    Nothing matches in the titles
                  </p>
                </div>
              )}
            </motion.div>
          </AnimatePresence>
        ) : props.conversations.length === 0 ? (
          <div className="flex flex-col items-center px-4 py-10 text-center">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl border border-line bg-surface text-ink-subtle shadow-panel">
              <IconMessages size={18} />
            </span>
            <p className="mt-3 text-xs font-semibold text-ink-soft">
              No conversations yet
            </p>
            <p className="mt-0.5 text-[0.7rem]/4 text-ink-muted">
              Your conversations with the agent appear here
            </p>
          </div>
        ) : (
          groups.map((group) => (
            <section key={group.label}>
              <SectionLabel>{group.label}</SectionLabel>
              <ul className="space-y-px">
                {group.items.map((conversation) => (
                  <ConversationItem
                    key={conversation.id}
                    conversation={conversation}
                    selected={conversation.id === props.selected}
                    {...itemProps}
                  />
                ))}
              </ul>
            </section>
          ))
        )}
      </nav>

      {props.footer && (
        <div className="shrink-0 border-t border-line p-2">{props.footer}</div>
      )}
    </div>
  );
};

export default React.memo(Sidebar);
