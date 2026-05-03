import { useContextSpace } from "@/pages/Spaces/utils";
import { getShortName } from "@/utils/pb";
import { Tabs, Text } from "@mantine/core";
import {
  IconBolt,
  IconDeviceDesktop,
  IconLayoutGrid,
  IconSettings,
} from "@tabler/icons-react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { match } from "ts-pattern";

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const loc = useLocation();

  if (!ctx.template.isSuccess) return null;

  const data = ctx.template.data;

  const activeTab = match(loc.pathname.split("/").reverse().at(0))
    .with("edit", (v) => v)
    .with("workspaces", (v) => v)
    .with("actions", (v) => v)
    .otherwise(() => "main");

  const tabs = [
    {
      value: "main",
      label: "Overview",
      icon: <IconLayoutGrid size={14} />,
      path: "./",
    },
    {
      value: "edit",
      label: "Config",
      icon: <IconSettings size={14} />,
      path: "./edit",
    },
    {
      value: "workspaces",
      label: "Your workspaces",
      icon: <IconDeviceDesktop size={14} />,
      path: "./workspaces",
    },
    {
      value: "actions",
      label: "Actions",
      icon: <IconBolt size={14} />,
      path: "./actions",
    },
  ];

  return (
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
            }}
          >
            <Text size="xs" c="dimmed" fw={`bold`}>
              {getShortName(data)}
            </Text>
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
  );
};

export default Page;
