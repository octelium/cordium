import ConsoleShell from "@/components/ConsoleShell";
import Empty from "@/components/Empty";
import Terminal from "@/components/Terminal";
import {
  addTerminal,
  initTerminalGroup,
  removeTerminal,
  setActiveTerminal,
} from "@/features/terminalgroup/slice";
import { onError, truncateUtf8 } from "@/utils";
import { getClientWorkspaceSvc } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import TerminalT from "@/utils/types/terminal";
import { ActionIcon, Button, Tooltip } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconPlus, IconTerminal2, IconX } from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { twMerge } from "tailwind-merge";
import { useContextWorkspace } from "../utils";
import { canUseTerminals } from "./utils";

const TabStrip = (props: {
  workspace: WsPB.Workspace;
  onCreate: () => void;
  creating: boolean;
}) => {
  const dispatch = useAppDispatch();
  const tg = useAppSelector((state) => state.terminalGroup);
  const wsC = getClientWorkspaceSvc(props.workspace.status?.regionRef);
  const scrollRef = React.useRef<HTMLDivElement>(null);

  const handleRemove = async (id: string) => {
    await wsC.removeTerminal(WsPB.RemoveTerminalRequest.create({ id }));
    dispatch(removeTerminal({ id }));
  };

  return (
    <div className="flex min-w-0 flex-1 items-center gap-1">
      <div
        ref={scrollRef}
        className="scrollbar-none flex min-w-0 flex-1 items-center gap-1 overflow-x-auto"
      >
        {tg.terminals.map((t) => {
          const isActive = tg.activeTerminal === t.id;
          return (
            <div
              key={t.id}
              role="tab"
              aria-selected={isActive}
              onClick={() => dispatch(setActiveTerminal({ id: t.id }))}
              className={twMerge(
                "flex max-w-[11rem] shrink-0 cursor-pointer select-none items-center gap-1.5",
                "rounded-md border px-2.5 py-1 transition-colors duration-150",
                isActive
                  ? "border-slate-600 bg-slate-700/70"
                  : "border-transparent hover:bg-slate-700/40",
              )}
            >
              <IconTerminal2
                size={12}
                className={twMerge(
                  "shrink-0",
                  isActive ? "text-emerald-300" : "text-slate-500",
                )}
              />
              <span
                className={twMerge(
                  "truncate font-mono text-[0.72rem]",
                  isActive ? "text-slate-100" : "text-slate-400",
                )}
              >
                {truncateUtf8(t.title, 24, { suffix: "…" })}
              </span>
              <ActionIcon
                size={16}
                variant="transparent"
                color="gray"
                aria-label="Close terminal"
                className="shrink-0 hover:text-rose-400"
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
      </div>

      <Tooltip label="New terminal">
        <ActionIcon
          size={26}
          variant="subtle"
          color="gray"
          aria-label="New terminal"
          loading={props.creating}
          onClick={props.onCreate}
        >
          <IconPlus size={14} />
        </ActionIcon>
      </Tooltip>
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

  const qryListTerm = useQuery({
    queryKey: ["workspace/ws/listTerminal", item.metadata!.uid],
    gcTime: 0,
    queryFn: async () => {
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
    enabled: ready,
  });

  const mutationCreate = useMutation({
    mutationFn: async () => {
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
          <Button
            leftSection={<IconPlus size={15} />}
            loading={mutationCreate.isPending}
            onClick={() => mutationCreate.mutate()}
          >
            New terminal
          </Button>
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
