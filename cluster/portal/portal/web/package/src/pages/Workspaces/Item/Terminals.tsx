import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import EmptyList from "@/components/EmptyList";
import PageWrap from "@/components/PageWrap";
import Terminal from "@/components/Terminal";
import {
  addTerminal,
  initTerminalGroup,
  removeTerminal,
  setActiveTerminal,
} from "@/features/terminalgroup/slice";
import { truncateUtf8 } from "@/utils";
import { getClientWorkspaceSvc } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import {
  default as TerminalI,
  default as TerminalT,
} from "@/utils/types/terminal";
import {
  ActionIcon,
  Anchor,
  Button,
  Group,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import {
  IconExternalLink,
  IconPlus,
  IconTerminal2,
  IconX,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useContextWorkspace } from "../utils";
import { canUseTerminals } from "./utils";

const TAB_SCROLLBAR_STYLE = `
  .term-tabbar::-webkit-scrollbar {
    height: 3px;
  }
  .term-tabbar::-webkit-scrollbar-track {
    background: transparent;
  }
  .term-tabbar::-webkit-scrollbar-thumb {
    background: #334155;
    border-radius: 2px;
  }
  .term-tabbar::-webkit-scrollbar-thumb:hover {
    background: #475569;
  }
  .term-tabbar {
    scrollbar-width: thin;
    scrollbar-color: #334155 transparent;
  }
`;

const TabGroup = (props: { workspace: WsPB.Workspace }) => {
  const { workspace } = props;
  const dispatch = useAppDispatch();
  const tg = useAppSelector((state) => state.terminalGroup);
  const wsC = getClientWorkspaceSvc(workspace.status?.regionRef);
  const scrollRef = React.useRef<HTMLDivElement>(null);

  const handleCreate = async () => {
    const { response } = await wsC.createTerminal(
      WsPB.CreateTerminalRequest.create({
        workspaceRef: getResourceRef(workspace),
      }),
    );
    dispatch(addTerminal({ id: response.id } as TerminalT));
    dispatch(setActiveTerminal({ id: response.id }));
  };

  const handleRemove = async (id: string) => {
    await wsC.removeTerminal(WsPB.RemoveTerminalRequest.create({ id }));
    dispatch(removeTerminal({ id }));
  };

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const terminals = tg.terminals;
    if (terminals.length < 2) return;

    const currentIdx = terminals.findIndex((t) => t.id === tg.activeTerminal);
    if (currentIdx === -1) return;

    const delta = e.deltaY > 0 ? 1 : -1;
    const nextIdx = Math.max(
      0,
      Math.min(terminals.length - 1, currentIdx + delta),
    );

    if (nextIdx !== currentIdx) {
      dispatch(setActiveTerminal({ id: terminals[nextIdx].id }));

      const tabEl = scrollRef.current?.children[0]?.children[
        nextIdx
      ] as HTMLElement;
      tabEl?.scrollIntoView({
        behavior: "smooth",
        block: "nearest",
        inline: "nearest",
      });
    }
  };

  return (
    <>
      <style>{TAB_SCROLLBAR_STYLE}</style>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 4,
          background: "#1e293b",
          borderRadius: "8px 8px 0 0",
          padding: "6px 8px",
          borderBottom: "1px solid #0f172a",
        }}
        onWheel={handleWheel}
      >
        <div
          ref={scrollRef}
          className="term-tabbar"
          style={{
            display: "flex",
            flex: 1,
            gap: 2,
            alignItems: "center",
            minWidth: 0,
            overflowX: "auto",
            paddingBottom: 2,
          }}
        >
          <div style={{ display: "flex", gap: 2, alignItems: "center" }}>
            {tg.terminals.map((t) => {
              const isActive = tg.activeTerminal === t.id;
              return (
                <div
                  key={t.id}
                  onClick={() => dispatch(setActiveTerminal({ id: t.id }))}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 6,
                    padding: "5px 10px",
                    borderRadius: 6,
                    cursor: "pointer",
                    background: isActive ? "#334155" : "transparent",
                    border: isActive
                      ? "1px solid #475569"
                      : "1px solid transparent",
                    transition: "all 150ms ease",
                    flexShrink: 0,
                    maxWidth: 180,
                    userSelect: "none",
                  }}
                >
                  <IconTerminal2
                    size={13}
                    style={{
                      color: isActive ? "#94d2bd" : "#64748b",
                      flexShrink: 0,
                    }}
                  />
                  <Text
                    size="xs"
                    style={{
                      color: isActive ? "#e2e8f0" : "#94a3b8",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      maxWidth: 110,
                      fontFamily: "Ubuntu Mono, monospace",
                    }}
                  >
                    {truncateUtf8(t.title, 22, { suffix: "…" })}
                  </Text>
                  <ActionIcon
                    size={16}
                    variant="transparent"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleRemove(t.id);
                    }}
                    style={{ color: "#64748b", flexShrink: 0 }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = "#f87171";
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = "#64748b";
                    }}
                  >
                    <IconX size={11} />
                  </ActionIcon>
                </div>
              );
            })}
          </div>
        </div>

        <div
          style={{
            width: 1,
            height: 16,
            background: "#334155",
            flexShrink: 0,
            margin: "0 2px",
          }}
        />

        <Tooltip label="New terminal" withArrow position="bottom">
          <ActionIcon
            size={28}
            variant="subtle"
            onClick={handleCreate}
            style={{
              color: "#94a3b8",
              background: "transparent",
              border: "1px solid #334155",
              borderRadius: 6,
              flexShrink: 0,
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = "#334155";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = "transparent";
            }}
          >
            <IconPlus size={14} />
          </ActionIcon>
        </Tooltip>
      </div>
    </>
  );
};

const TerminalGroupC = (props: { workspace: WsPB.Workspace }) => {
  const item = props.workspace;
  const wsC = getClientWorkspaceSvc(item.status?.regionRef);
  const tg = useAppSelector((state) => state.terminalGroup);
  const dispatch = useAppDispatch();
  const canUseTerminal = canUseTerminals(item);

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
            (x) => ({ id: x.id, title: "Terminal" }) as TerminalI,
          ),
        }),
      );
      return response;
    },
    enabled: canUseTerminal,
  });

  if (!canUseTerminal) {
    return <EmptyList title="Workspace needs to be ready to use terminals" />;
  }

  if (!qryListTerm.isSuccess) return null;

  if (tg.terminals.length < 1) {
    return (
      <EmptyList title="No terminals">
        <Stack align="center" gap="sm">
          <Text size="sm" c="dimmed">
            Start a terminal session to interact with your workspace.
          </Text>
          <Button
            leftSection={<IconPlus size={14} />}
            onClick={async () => {
              const { response } = await wsC.createTerminal(
                WsPB.CreateTerminalRequest.create({
                  workspaceRef: getResourceRef(item),
                }),
              );
              dispatch(addTerminal({ id: response.id } as TerminalT));
              dispatch(setActiveTerminal({ id: response.id }));
            }}
          >
            New terminal
          </Button>
        </Stack>
      </EmptyList>
    );
  }

  return (
    <Stack gap="md">
      {item.status?.hostname && (
        <Group gap="xs">
          <Text
            size="xs"
            fw={500}
            tt="uppercase"
            style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
          >
            URL
          </Text>
          <Anchor
            href={`https://${item.status.hostname}`}
            target="_blank"
            size="sm"
            style={{ display: "inline-flex", alignItems: "center", gap: 4 }}
          >
            {`https://${item.status.hostname}`}
            <IconExternalLink size={12} />
          </Anchor>
        </Group>
      )}

      <div
        style={{
          borderRadius: 10,
          overflow: "hidden",
          border: "1px solid #1e293b",
          boxShadow: "0 4px 16px rgba(0,0,0,0.12)",
        }}
      >
        <TabGroup workspace={props.workspace} />
        <div style={{ background: "#0f172a", minHeight: 500 }}>
          {tg.terminals.map((x) => (
            <div
              key={x.id}
              style={{ display: x.id !== tg.activeTerminal ? "none" : "block" }}
            >
              <Terminal item={x} />
            </div>
          ))}
        </div>
      </div>
    </Stack>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <TerminalGroupC workspace={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Page;
