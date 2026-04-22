/// <reference types="vite-plugin-svgr/client" />

import { useAppSelector } from "@/utils/hooks";
import { Avatar, Menu, Text } from "@mantine/core";
import { User } from "lucide-react";
import { useNavigate } from "react-router-dom";

const TopBar = () => {
  const navigate = useNavigate();
  const settings = useAppSelector((state) => state.settings);

  const picURL =
    settings.status?.session?.metadata?.picURL ??
    settings.status?.user?.metadata?.picURL;

  const displayName =
    settings.status?.user?.metadata?.displayName ??
    settings.status?.user?.metadata?.name ??
    "";

  const shortName = displayName.split(".").at(0) ?? displayName;
  const initials = shortName.slice(0, 2).toUpperCase();

  return (
    <nav
      style={{
        width: "100%",
        height: 60,
        display: "flex",
        alignItems: "center",
        padding: "0 16px",
        borderBottom: "1px solid #e2e8f0",
      }}
    >
      <button
        aria-label="Go to home"
        onClick={() => navigate("/")}
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          background: "none",
          border: "none",
          cursor: "pointer",
          padding: "4px 8px",
          borderRadius: 6,
        }}
      >
        <div
          style={{
            width: 28,
            height: 28,
            borderRadius: 6,
            background: "#1e293b",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <div
            style={{
              width: 12,
              height: 12,
              borderRadius: 2,
              background: "white",
              opacity: 0.9,
            }}
          />
        </div>
        <Text
          fw={700}
          size="sm"
          style={{ color: "#1e293b", letterSpacing: "-0.01em" }}
        >
          Cordium
        </Text>
      </button>

      <div style={{ flex: 1 }} />

      <Menu position="bottom-end" offset={8} withArrow arrowPosition="center">
        <Menu.Target>
          <button
            aria-label="User menu"
            style={{
              background: "none",
              border: "none",
              cursor: "pointer",
              padding: 0,
              borderRadius: "50%",
              display: "flex",
              alignItems: "center",
            }}
          >
            <Avatar
              src={picURL ?? undefined}
              radius="xl"
              size={36}
              color="blue"
              style={{ border: "2px solid #e2e8f0" }}
            >
              {!picURL && initials}
            </Avatar>
          </button>
        </Menu.Target>

        <Menu.Dropdown>
          {shortName && (
            <>
              <Menu.Label>{shortName}</Menu.Label>
              <Menu.Divider />
            </>
          )}
          <Menu.Item
            leftSection={<User size={14} />}
            onClick={() => navigate("/settings")}
          >
            Settings
          </Menu.Item>
        </Menu.Dropdown>
      </Menu>
    </nav>
  );
};

export default TopBar;
