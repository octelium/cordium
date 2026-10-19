import {
  consoleToolbarButtonClass,
  consoleToolbarButtonVars,
} from "@/components/ConsoleShell/toolbar";
import { saveTerminalPreferences } from "@/features/terminalgroup/preferences";
import {
  moveTerminal,
  removeTerminal,
  setActiveTerminal,
  setTerminalPinned,
} from "@/features/terminalgroup/slice";
import { onError } from "@/utils";
import { getClientWorkspaceSvc } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { ActionIcon, Menu, Tooltip } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { useMutation } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  Ellipsis,
  Pin,
  PinOff,
  Plus,
  SquareTerminal,
  X,
} from "lucide-react";
import * as React from "react";
import { twMerge } from "tailwind-merge";

const WHEEL_STEP_DELTA = 40;
const WHEEL_STEP_COOLDOWN_MS = 90;

const TerminalTabs = (props: {
  workspace: WsPB.Workspace;
  onCreate: () => void;
  creating: boolean;
  devMode: boolean;
  newTerminalHint: string;
  newTerminalKeys: string;
}) => {
  const dispatch = useAppDispatch();
  const tg = useAppSelector((state) => state.terminalGroup);
  const connection = useAppSelector((state) => state.conn.status);
  const fullscreen = useAppSelector(
    (state) => state.settings.terminalFullscreen,
  );
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const groupRef = React.useRef(tg);
  const wheelDelta = React.useRef(0);
  const lastStepAt = React.useRef(0);
  const dragId = React.useRef<string | null>(null);
  const [dragging, setDragging] = React.useState<string | null>(null);
  const [drop, setDrop] = React.useState<{
    id: string;
    position: "before" | "after";
  } | null>(null);
  const [menuID, setMenuID] = React.useState<string | null>(null);

  React.useEffect(() => {
    groupRef.current = tg;
  }, [tg]);

  React.useEffect(() => {
    const container = scrollRef.current;
    if (!container) return;

    const onWheel = (event: WheelEvent) => {
      event.preventDefault();

      const raw =
        Math.abs(event.deltaX) > Math.abs(event.deltaY)
          ? event.deltaX
          : event.deltaY;
      if (!raw) return;

      const delta =
        event.deltaMode === 1
          ? raw * 16
          : event.deltaMode === 2
            ? raw * container.clientWidth
            : raw;
      if (Math.sign(delta) !== Math.sign(wheelDelta.current)) {
        wheelDelta.current = 0;
      }
      wheelDelta.current += delta;
      if (Math.abs(wheelDelta.current) < WHEEL_STEP_DELTA) return;

      const direction = Math.sign(wheelDelta.current);
      wheelDelta.current = 0;
      if (event.timeStamp - lastStepAt.current < WHEEL_STEP_COOLDOWN_MS) return;
      lastStepAt.current = event.timeStamp;

      const { terminals, activeTerminal } = groupRef.current;
      const index = terminals.findIndex(
        (terminal) => terminal.id === activeTerminal,
      );
      if (index < 0) return;
      const next = Math.min(
        terminals.length - 1,
        Math.max(0, index + direction),
      );
      if (next !== index)
        dispatch(setActiveTerminal({ id: terminals[next].id }));
    };

    container.addEventListener("wheel", onWheel, { passive: false });
    return () => container.removeEventListener("wheel", onWheel);
  }, [dispatch]);

  React.useEffect(() => {
    saveTerminalPreferences(props.workspace.metadata!.uid, tg.terminals);
  }, [props.workspace.metadata, tg.terminals]);

  React.useEffect(() => {
    const tab = scrollRef.current?.querySelector<HTMLElement>(
      `[data-terminal-tab="${CSS.escape(tg.activeTerminal ?? "")}"]`,
    );
    tab?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [tg.activeTerminal, tg.terminals]);

  const close = useMutation({
    mutationFn: async (id: string) => {
      if (!props.devMode) {
        await getClientWorkspaceSvc(
          props.workspace.status?.regionRef,
        ).removeTerminal(WsPB.RemoveTerminalRequest.create({ id }));
      }
    },
    onSuccess: (_response, id) => dispatch(removeTerminal({ id })),
    onError,
  });

  const select = (index: number) => {
    const terminal = tg.terminals[index];
    if (!terminal) return;
    dispatch(setActiveTerminal({ id: terminal.id }));
    document.getElementById(`terminal-tab-${terminal.id}`)?.focus();
  };
  const move = (index: number, direction: number) => {
    const terminal = tg.terminals[index];
    const target = tg.terminals[index + direction];
    if (!terminal || !target || !!terminal.pinned !== !!target.pinned) return;
    dispatch(
      moveTerminal({
        id: terminal.id,
        targetId: target.id,
        position: direction < 0 ? "before" : "after",
      }),
    );
  };
  const resetDrag = () => {
    dragId.current = null;
    setDragging(null);
    setDrop(null);
  };

  return (
    <div className="flex min-w-0 flex-1 items-center gap-1.5">
      <div
        ref={scrollRef}
        role="tablist"
        aria-label="Terminal sessions"
        className="scrollbar-none flex min-w-0 flex-1 items-center gap-1 overflow-x-auto overscroll-contain"
      >
        {tg.terminals.map((terminal, index) => {
          const isActive = tg.activeTerminal === terminal.id;
          const label = `${terminal.title || "Terminal"}${terminal.pinned ? " (pinned)" : ""}`;
          return (
            <div
              key={terminal.id}
              role="presentation"
              draggable
              data-terminal-tab={terminal.id}
              data-pinned={!!terminal.pinned}
              onContextMenu={(event) => {
                event.preventDefault();
                setMenuID(terminal.id);
              }}
              onDragStart={(event) => {
                dragId.current = terminal.id;
                event.dataTransfer.setData("text/plain", terminal.id);
                event.dataTransfer.effectAllowed = "move";
                setDragging(terminal.id);
              }}
              onDragEnd={resetDrag}
              onDragOver={(event) => {
                if (!dragId.current || dragId.current === terminal.id) return;
                event.preventDefault();
                event.dataTransfer.dropEffect = "move";
                const bounds = event.currentTarget.getBoundingClientRect();
                setDrop({
                  id: terminal.id,
                  position:
                    event.clientX < bounds.left + bounds.width / 2
                      ? "before"
                      : "after",
                });
                const container = scrollRef.current;
                if (container) {
                  const area = container.getBoundingClientRect();
                  if (event.clientX < area.left + 30)
                    container.scrollLeft -= 12;
                  else if (event.clientX > area.right - 30)
                    container.scrollLeft += 12;
                }
              }}
              onDrop={(event) => {
                event.preventDefault();
                if (dragId.current && drop?.id === terminal.id) {
                  dispatch(
                    moveTerminal({
                      id: dragId.current,
                      targetId: terminal.id,
                      position: drop.position,
                    }),
                  );
                }
                resetDrag();
              }}
              className={twMerge(
                "group relative flex h-9 shrink-0 items-center rounded-lg border transition-colors",
                terminal.pinned ? "w-11 justify-center" : "w-44 pr-1",
                isActive
                  ? "border-line bg-surface text-ink shadow-panel"
                  : "border-transparent text-ink-muted hover:bg-surface-strong/50 hover:text-ink-soft",
                dragging === terminal.id && "opacity-50",
                drop?.id === terminal.id &&
                  (drop.position === "before"
                    ? "border-l-ink-muted"
                    : "border-r-ink-muted"),
              )}
            >
              <button
                type="button"
                role="tab"
                id={`terminal-tab-${terminal.id}`}
                aria-controls={`terminal-panel-${terminal.id}`}
                aria-selected={isActive}
                aria-label={label}
                tabIndex={isActive ? 0 : -1}
                title={label}
                className={twMerge(
                  "flex h-full min-w-0 flex-1 items-center gap-2 rounded-lg px-2.5 text-left text-[0.8rem] outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-focus",
                  isActive ? "font-medium" : "font-normal",
                  terminal.pinned && "justify-center px-0",
                )}
                onClick={() => dispatch(setActiveTerminal({ id: terminal.id }))}
                onKeyDown={(event) => {
                  if (
                    event.altKey &&
                    event.shiftKey &&
                    ["ArrowLeft", "ArrowRight"].includes(event.key)
                  ) {
                    event.preventDefault();
                    move(index, event.key === "ArrowLeft" ? -1 : 1);
                  } else if (
                    event.key === "ArrowRight" ||
                    event.key === "ArrowLeft"
                  ) {
                    event.preventDefault();
                    select(
                      (index +
                        (event.key === "ArrowRight" ? 1 : -1) +
                        tg.terminals.length) %
                        tg.terminals.length,
                    );
                  } else if (event.key === "Home" || event.key === "End") {
                    event.preventDefault();
                    select(event.key === "Home" ? 0 : tg.terminals.length - 1);
                  } else if (
                    event.key === "Delete" &&
                    !terminal.pinned &&
                    !close.isPending
                  ) {
                    event.preventDefault();
                    close.mutate(terminal.id);
                  } else if (
                    event.key === "ContextMenu" ||
                    (event.shiftKey && event.key === "F10")
                  ) {
                    event.preventDefault();
                    setMenuID(terminal.id);
                  }
                }}
              >
                {terminal.pinned ? (
                  <Pin size={15} aria-hidden />
                ) : (
                  <SquareTerminal size={14} className="shrink-0" aria-hidden />
                )}
                {!terminal.pinned && (
                  <span className="truncate">
                    {terminal.title || "Terminal"}
                  </span>
                )}
              </button>
              <Menu
                opened={menuID === terminal.id}
                onChange={(opened) => setMenuID(opened ? terminal.id : null)}
                position="bottom-start"
                withinPortal
                zIndex={fullscreen ? 410 : undefined}
              >
                <Menu.Target>
                  <button
                    type="button"
                    aria-label={`Actions for ${label}`}
                    tabIndex={isActive ? 0 : -1}
                    className={twMerge(
                      "flex h-6 w-6 shrink-0 items-center justify-center rounded text-ink-muted outline-none hover:bg-surface-strong hover:text-ink focus-visible:ring-2 focus-visible:ring-focus",
                      terminal.pinned &&
                        "absolute bottom-0 right-0 h-4 w-4 bg-surface opacity-0 group-hover:opacity-100 focus-visible:opacity-100",
                    )}
                  >
                    <Ellipsis size={14} aria-hidden />
                  </button>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Label>{terminal.title || "Terminal"}</Menu.Label>
                  <Menu.Item
                    leftSection={
                      terminal.pinned ? <PinOff size={14} /> : <Pin size={14} />
                    }
                    onClick={() =>
                      dispatch(
                        setTerminalPinned({
                          id: terminal.id,
                          pinned: !terminal.pinned,
                        }),
                      )
                    }
                  >
                    {terminal.pinned ? "Unpin tab" : "Pin tab"}
                  </Menu.Item>
                  <Menu.Item
                    leftSection={<ArrowLeft size={14} />}
                    disabled={
                      !index ||
                      !!tg.terminals[index - 1].pinned !== !!terminal.pinned
                    }
                    onClick={() => move(index, -1)}
                  >
                    Move left
                  </Menu.Item>
                  <Menu.Item
                    leftSection={<ArrowRight size={14} />}
                    disabled={
                      index === tg.terminals.length - 1 ||
                      !!tg.terminals[index + 1].pinned !== !!terminal.pinned
                    }
                    onClick={() => move(index, 1)}
                  >
                    Move right
                  </Menu.Item>
                  <Menu.Divider />
                  <Menu.Item
                    color="red"
                    leftSection={<X size={14} />}
                    disabled={close.isPending}
                    onClick={() => close.mutate(terminal.id)}
                  >
                    Close terminal
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
              {!terminal.pinned && (
                <button
                  type="button"
                  aria-label={`Close ${terminal.title || "terminal"}`}
                  tabIndex={-1}
                  disabled={close.isPending}
                  onClick={() => close.mutate(terminal.id)}
                  className="flex h-6 w-6 shrink-0 items-center justify-center rounded text-ink-muted outline-none hover:bg-surface-strong hover:text-hue-rose focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-40"
                >
                  <X size={13} aria-hidden />
                </button>
              )}
            </div>
          );
        })}
      </div>
      <Tooltip label={`New terminal (${props.newTerminalHint})`}>
        <ActionIcon
          size={28}
          variant="transparent"
          aria-label="New terminal"
          aria-keyshortcuts={props.newTerminalKeys}
          className={consoleToolbarButtonClass}
          vars={consoleToolbarButtonVars}
          loading={props.creating}
          onClick={props.onCreate}
        >
          <Plus size={17} />
        </ActionIcon>
      </Tooltip>
      {!props.devMode && connection !== "connected" && (
        <span role="status" className="shrink-0 px-1 text-xs text-ink-muted">
          {connection === "connecting" ? "Connecting…" : "Reconnecting…"}
        </span>
      )}
    </div>
  );
};

export default TerminalTabs;
