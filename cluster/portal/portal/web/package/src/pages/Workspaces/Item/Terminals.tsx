import ConsoleShell from "@/components/ConsoleShell";
import Empty from "@/components/Empty";
import Terminal from "@/components/Terminal";
import {
  addTerminal,
  initTerminalGroup,
  setActiveTerminal,
} from "@/features/terminalgroup/slice";
import { isDev, onError } from "@/utils";
import { getClientWorkspaceSvc } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import { Alert, Button, Tooltip } from "@mantine/core";
import { useHotkeys, useOs } from "@mantine/hooks";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconPlus, IconTerminal2 } from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { twMerge } from "tailwind-merge";
import { useContextWorkspace } from "../utils";
import { canUseTerminals } from "./utils";
import TerminalTabs from "./TerminalTabs";
import { restoreTerminalPreferences } from "@/features/terminalgroup/preferences";
import QueryBoundary from "@/components/QueryBoundary";

const TerminalGroup = (props: { workspace: WsPB.Workspace }) => {
  const item = props.workspace;
  const workspaceID = item.metadata!.uid;
  const wsC = getClientWorkspaceSvc(item.status?.regionRef);
  const tg = useAppSelector((state) => state.terminalGroup);
  const fullscreen = useAppSelector((s) => s.settings.terminalFullscreen);
  const dispatch = useAppDispatch();
  const connection = useAppSelector((state) => state.conn.status);
  const ready = canUseTerminals(item);
  const devMode = isDev();
  const isMac = useOs() === "macos";
  const newTerminalHint = isMac ? "⌘+⌥+T" : "Ctrl+Alt+T";
  const newTerminalKeys = isMac ? "Meta+Alt+T" : "Control+Alt+T";

  const qryListTerm = useQuery({
    queryKey: ["workspace/ws/listTerminal", workspaceID],
    gcTime: 0,
    queryFn: async () => {
      if (devMode) {
        const response = WsPB.ListTerminalResponse.create({
          items: [{ id: "dev-terminal-1" }],
        });
        return response;
      }

      const { response } = await wsC.listTerminal(
        WsPB.ListTerminalRequest.create({ workspaceRef: getResourceRef(item) }),
      );
      return response;
    },
    enabled: ready || devMode,
  });

  React.useEffect(() => {
    if (!qryListTerm.data) return;
    dispatch(
      initTerminalGroup({
        workspaceID,
        termList: restoreTerminalPreferences(
          workspaceID,
          qryListTerm.data.items.map((terminal) => ({
            id: terminal.id,
            title: devMode ? "Dev terminal" : "Terminal",
          })),
        ),
      }),
    );
  }, [dispatch, qryListTerm.data, workspaceID, devMode]);

  const refetchTerminals = qryListTerm.refetch;
  React.useEffect(() => {
    if (connection === "connected" && ready) void refetchTerminals();
  }, [connection, ready, refetchTerminals]);

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

  if (!qryListTerm.data || tg.workspaceID !== workspaceID) {
    return (
      <QueryBoundary
        query={{
          ...qryListTerm,
          isPending: qryListTerm.isPending || tg.workspaceID !== workspaceID,
        }}
      />
    );
  }

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
    <>
      {qryListTerm.isError && (
        <Alert
          color="yellow"
          title="Could not refresh terminal sessions"
          className="mb-3"
        >
          Existing sessions remain open.
          <Button
            size="compact-xs"
            variant="subtle"
            color="yellow"
            ml="sm"
            onClick={() => qryListTerm.refetch()}
          >
            Retry
          </Button>
        </Alert>
      )}
      <ConsoleShell
        height={fullscreen ? undefined : 560}
        tabs={
          <TerminalTabs
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
              role="tabpanel"
              id={`terminal-panel-${x.id}`}
              aria-labelledby={`terminal-tab-${x.id}`}
              aria-hidden={x.id !== tg.activeTerminal}
              inert={x.id !== tg.activeTerminal}
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
    </>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return ctx.workspace.data ? (
    <TerminalGroup workspace={ctx.workspace.data} />
  ) : null;
};

export default Page;
