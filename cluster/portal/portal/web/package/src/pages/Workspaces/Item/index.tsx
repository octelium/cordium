import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import PageWrap from "@/components/PageWrap";
import WorkspaceStatus from "@/components/WorkspaceStatus";
import { clearTerminalGroup } from "@/features/terminalgroup/slice";
import { useAppDispatch } from "@/utils/hooks";
import { getShortName } from "@/utils/pb";
import { Tabs, Text } from "@mantine/core";
import {
  IconActivity,
  IconBolt,
  IconLayoutGrid,
  IconPencil,
  IconTerminal2,
} from "@tabler/icons-react";
import * as React from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { match } from "ts-pattern";
import { useContextWorkspace } from "../utils";

const Workspace = () => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const ctx = useContextWorkspace();
  const loc = useLocation();

  React.useEffect(() => {
    dispatch(clearTerminalGroup({}));
    return () => {
      dispatch(clearTerminalGroup({}));
    };
  }, [dispatch]);

  const data = ctx.workspace.data;

  const activeTab = match(loc.pathname.split("/").reverse().at(0))
    .with("edit", (v) => v)
    .with("actions", (v) => v)
    .with("terminals", (v) => v)
    .with("logs", (v) => v)
    .otherwise(() => "main");

  const tabs = [
    {
      value: "main",
      label: "Overview",
      icon: <IconLayoutGrid size={14} />,
      path: "./",
    },
    {
      value: "terminals",
      label: "Terminals",
      icon: <IconTerminal2 size={14} />,
      path: "./terminals",
    },
    {
      value: "edit",
      label: "Edit",
      icon: <IconPencil size={14} />,
      path: "./edit",
    },
    {
      value: "logs",
      label: "Activity logs",
      icon: <IconActivity size={14} />,
      path: "./logs",
    },
    {
      value: "actions",
      label: "Actions",
      icon: <IconBolt size={14} />,
      path: "./actions",
    },
  ];

  const isRunning = data?.status?.state === WsPB.Workspace_Status_State.RUNNING;

  return (
    <PageWrap qry={ctx.workspace}>
      {data && (
        <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
          <Tabs value={activeTab}>
            <Tabs.List
              style={{
                background: "white",
                borderRadius: "10px 10px 0 0",
                border: "1px solid #e2e8f0",
                borderBottom: "none",
                padding: "0 8px",
              }}
            >
              {tabs.map((t) => (
                <Tabs.Tab
                  key={t.value}
                  value={t.value}
                  leftSection={t.icon}
                  onClick={() => navigate(t.path)}
                  style={{ fontSize: 13 }}
                >
                  {t.label}
                </Tabs.Tab>
              ))}

              <div
                style={{
                  marginLeft: "auto",
                  display: "flex",
                  alignItems: "center",
                  paddingRight: 12,
                  gap: 8,
                }}
              >
                <Text size="xs" c="dimmed" style={{ fontFamily: "monospace" }}>
                  {getShortName(data)}
                </Text>
                {data.status?.state !== undefined && (
                  <WorkspaceStatus status={data.status.state} />
                )}
              </div>
            </Tabs.List>

            <div
              style={{
                background: "white",
                border: "1px solid #e2e8f0",
                borderTop: "none",
                borderRadius: "0 0 10px 10px",
                padding: "20px",
                minHeight: 200,
              }}
            >
              <Outlet />
            </div>
          </Tabs>
        </div>
      )}
    </PageWrap>
  );
};

export default Workspace;
