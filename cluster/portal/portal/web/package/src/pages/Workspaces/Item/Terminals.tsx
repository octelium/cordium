import ConsoleShell, {
  consoleToolbarButtonClass,
  consoleToolbarButtonVars,
} from "@/components/ConsoleShell";
import Empty from "@/components/Empty";
import Terminal from "@/components/Terminal";
import {
  addTerminal,
  initTerminalGroup,
  removeTerminal,
  setActiveTerminal,
} from "@/features/terminalgroup/slice";
import { isDev, onError, truncateUtf8 } from "@/utils";
import { getClientWorkspaceSvc } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import TerminalT from "@/utils/types/terminal";
import { ActionIcon, Button, Tooltip } from "@mantine/core";
import { useHotkeys, useOs } from "@mantine/hooks";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import {
  IconPlus,
  IconSquareRoundedPlus,
  IconTerminal2,
  IconX,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { twMerge } from "tailwind-merge";
import { useContextWorkspace } from "../utils";
import { canUseTerminals } from "./utils";

const WHEEL_STEP_DELTA = 40;
const WHEEL_STEP_COOLDOWN_MS = 90;

const TabStrip = (props: {
  workspace: WsPB.Workspace;
  onCreate: () => void;
  creating: boolean;
  devMode: boolean;
  newTerminalHint: string;
  newTerminalKeys: string;
}) => {
  const dispatch = useAppDispatch();
  const tg = useAppSelector((state) => state.terminalGroup);
  const wsC = getClientWorkspaceSvc(props.workspace.status?.regionRef);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const wheelDelta = React.useRef(0);
  const lastStepAt = React.useRef(0);

  const groupRef = React.useRef(tg);
  React.useEffect(() => {
    groupRef.current = tg;
  }, [tg]);

  const handleRemove = async (id: string) => {
    if (!props.devMode) {
      await wsC.removeTerminal(WsPB.RemoveTerminalRequest.create({ id }));
    }
    dispatch(removeTerminal({ id }));
  };

  const stepTerminal = React.useCallback(
    (direction: number) => {
      const { terminals, activeTerminal } = groupRef.current;
      if (terminals.length < 2) return;

      const idx = terminals.findIndex((x) => x.id === activeTerminal);
      if (idx < 0) return;

      const next = Math.min(terminals.length - 1, Math.max(0, idx + direction));
      if (next === idx) return;

      dispatch(setActiveTerminal({ id: terminals[next].id }));
    },
    [dispatch],
  );

  const handleTabWheel = React.useCallback(
    (event: WheelEvent) => {
      event.preventDefault();

      const raw =
        Math.abs(event.deltaX) > Math.abs(event.deltaY)
          ? event.deltaX
          : event.deltaY;
      if (!raw) return;

      const delta = event.deltaMode === 0 ? raw : raw * 16;

      if (Math.sign(delta) !== Math.sign(wheelDelta.current)) {
        wheelDelta.current = 0;
      }
      wheelDelta.current += delta;

      if (Math.abs(wheelDelta.current) < WHEEL_STEP_DELTA) return;

      const direction = Math.sign(wheelDelta.current);
      wheelDelta.current = 0;

      if (event.timeStamp - lastStepAt.current < WHEEL_STEP_COOLDOWN_MS) return;
      lastStepAt.current = event.timeStamp;

      stepTerminal(direction);
    },
    [stepTerminal],
  );

  React.useEffect(() => {
    const container = scrollRef.current;
    if (!container) return;

    container.addEventListener("wheel", handleTabWheel, { passive: false });
    return () => container.removeEventListener("wheel", handleTabWheel);
  }, [handleTabWheel]);

  React.useEffect(() => {
    const container = scrollRef.current;
    if (!container || !tg.activeTerminal) return;

    const tab = container.querySelector<HTMLElement>(
      `[data-terminal-tab="${CSS.escape(tg.activeTerminal)}"]`,
    );
    tab?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [tg.activeTerminal, tg.terminals.length]);

  return (
    <div className="flex min-w-0 flex-1 items-center gap-1">
      <div
        ref={scrollRef}
        role="tablist"
        aria-label="Terminal sessions"
        className="scrollbar-none flex min-w-0 flex-1 items-center gap-1 overflow-x-auto overscroll-none"
      >
        {tg.terminals.map((t) => {
          const isActive = tg.activeTerminal === t.id;
          return (
            <div
              key={t.id}
              role="tab"
              data-terminal-tab={t.id}
              aria-selected={isActive}
              onClick={() => dispatch(setActiveTerminal({ id: t.id }))}
              className={twMerge(
                "flex max-w-[11rem] shrink-0 cursor-pointer select-none items-center gap-1.5",
                "rounded-md border px-2.5 py-1 transition-colors duration-150",
                isActive
                  ? "border-zinc-600 bg-zinc-700/70"
                  : "border-transparent hover:bg-zinc-700/40",
              )}
            >
              <IconTerminal2
                size={12}
                className={twMerge(
                  "shrink-0",
                  isActive ? "text-emerald-300" : "text-zinc-500",
                )}
              />
              <span
                className={twMerge(
                  "truncate font-mono text-[0.72rem] font-semibold",
                  isActive ? "text-zinc-100" : "text-zinc-400",
                )}
              >
                {truncateUtf8(t.title, 24, { suffix: "…" })}
              </span>
              <ActionIcon
                size={16}
                variant="transparent"
                aria-label="Close terminal"
                className="shrink-0 text-zinc-500 transition-colors hover:bg-zinc-600/70 hover:text-rose-300"
                onClick={(e) => {
                  e.stopPropagation();
                  handleRemove(t.id);
                }}
              >
                <IconX size={11} />
              </ActionIcon>
            </div>
          );
        })}
        <div
          className="min-w-4 shrink-0 self-stretch"
          aria-hidden="true"
          title="Double-click to open a new terminal"
          onDoubleClick={() => {
            if (!props.creating) props.onCreate();
          }}
        />

        <div
          className={twMerge(
            "sticky right-0 z-10 shrink-0 self-stretch bg-console-chrome pl-1",
            "before:pointer-events-none before:absolute before:right-full before:top-0",
            "before:h-full before:w-6 before:bg-gradient-to-l before:from-console-chrome before:to-transparent",
          )}
        >
          <Tooltip label={`New terminal (${props.newTerminalHint})`}>
            <ActionIcon
              size={27}
              variant="transparent"
              aria-label="New terminal"
              aria-keyshortcuts={props.newTerminalKeys}
              className={consoleToolbarButtonClass}
              vars={consoleToolbarButtonVars}
              loading={props.creating}
              onClick={props.onCreate}
            >
              <IconSquareRoundedPlus size={15} stroke={1.9} />
            </ActionIcon>
          </Tooltip>
        </div>
      </div>
    </div>
  );
};

const TerminalGroup = (props: { workspace: WsPB.Workspace }) => {
  const item = props.workspace;
  const wsC = getClientWorkspaceSvc(item.status?.regionRef);
  const tg = useAppSelector((state) => state.terminalGroup);
  const fullscreen = useAppSelector((s) => s.settings.terminalFullscreen);
  const dispatch = useAppDispatch();
  const ready = canUseTerminals(item);
  const devMode = isDev();
  const isMac = useOs() === "macos";
  const newTerminalHint = isMac ? "⌘+⌥+T" : "Ctrl+Alt+T";
  const newTerminalKeys = isMac ? "Meta+Alt+T" : "Control+Alt+T";

  const qryListTerm = useQuery({
    queryKey: ["workspace/ws/listTerminal", item.metadata!.uid],
    gcTime: 0,
    queryFn: async () => {
      if (devMode) {
        const response = WsPB.ListTerminalResponse.create({
          items: [{ id: "dev-terminal-1" }],
        });
        dispatch(
          initTerminalGroup({
            termList: response.items.map(
              (x) => ({ id: x.id, title: "Dev terminal" }) as TerminalT,
            ),
          }),
        );
        return response;
      }

      const { response } = await wsC.listTerminal(
        WsPB.ListTerminalRequest.create({ workspaceRef: getResourceRef(item) }),
      );
      dispatch(
        initTerminalGroup({
          termList: response.items.map(
            (x) => ({ id: x.id, title: "Terminal" }) as TerminalT,
          ),
        }),
      );
      return response;
    },
    enabled: ready || devMode,
  });

  const mutationCreate = useMutation({
    mutationFn: async () => {
      if (devMode) {
        return WsPB.CreateTerminalResponse.create({
          id: `dev-terminal-${Date.now()}`,
        });
      }

      const { response } = await wsC.createTerminal(
        WsPB.CreateTerminalRequest.create({
          workspaceRef: getResourceRef(item),
        }),
      );
      return response;
    },
    onSuccess: (response) => {
      dispatch(addTerminal({ id: response.id }));
      dispatch(setActiveTerminal({ id: response.id }));
    },
    onError,
  });

  useHotkeys(
    [
      [
        "mod+alt+T",
        () => {
          if (ready && !mutationCreate.isPending) mutationCreate.mutate();
        },
        { preventDefault: true, usePhysicalKeys: true },
      ],
    ],
    [],
  );

  if (!ready) {
    return (
      <Empty
        icon={<IconTerminal2 size={22} />}
        title="Workspace is not running"
        description="Start the workspace to open a terminal session."
      />
    );
  }

  if (!qryListTerm.isSuccess) return null;

  if (tg.terminals.length === 0) {
    return (
      <Empty
        icon={<IconTerminal2 size={22} />}
        title="No terminal sessions"
        description="Open a shell to interact with your workspace."
        action={
          <Tooltip label={`New terminal (${newTerminalHint})`}>
            <Button
              leftSection={<IconPlus size={15} />}
              loading={mutationCreate.isPending}
              onClick={() => mutationCreate.mutate()}
            >
              New terminal
            </Button>
          </Tooltip>
        }
      />
    );
  }

  return (
    <ConsoleShell
      height={fullscreen ? undefined : 560}
      tabs={
        <TabStrip
          workspace={item}
          creating={mutationCreate.isPending}
          onCreate={() => mutationCreate.mutate()}
          devMode={devMode}
          newTerminalHint={newTerminalHint}
          newTerminalKeys={newTerminalKeys}
        />
      }
    >
      <div className="relative h-full w-full">
        {tg.terminals.map((x) => (
          <div
            key={x.id}
            className={twMerge(
              "absolute inset-0",
              x.id !== tg.activeTerminal && "invisible",
            )}
          >
            <Terminal id={x.id} isActive={x.id === tg.activeTerminal} />
          </div>
        ))}
      </div>
    </ConsoleShell>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return ctx.workspace.data ? (
    <TerminalGroup workspace={ctx.workspace.data} />
  ) : null;
};

export default Page;
