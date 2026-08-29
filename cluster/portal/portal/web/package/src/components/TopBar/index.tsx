/// <reference types="vite-plugin-svgr/client" />

import { useAppSelector } from "@/utils/hooks";
import { Avatar, Button, Menu, Text } from "@mantine/core";
import {
  IconKey,
  IconPlus,
  IconSettings,
  IconStack2,
} from "@tabler/icons-react";
import { Link, useNavigate } from "react-router-dom";

import Logo from "@/assets/main.svg?react";

const TopBar = () => {
  const navigate = useNavigate();
  const status = useAppSelector((state) => state.settings.status);

  const picURL =
    status?.session?.metadata?.picURL ?? status?.user?.metadata?.picURL;

  const displayName =
    status?.user?.metadata?.displayName ?? status?.user?.metadata?.name ?? "";

  const shortName = displayName.split(".").at(0) ?? displayName;
  const initials = shortName.slice(0, 2).toUpperCase();
  const email = status?.user?.spec?.email;

  return (
    <div className="flex h-full w-full items-center gap-3 bg-slate-100 px-4">
      <Link
        to="/"
        aria-label="Cordium home"
        className="flex items-center rounded-md px-1 py-1 transition-opacity duration-150 hover:opacity-80"
      >
        <Logo className="h-auto w-[112px] md:w-[140px]" />
      </Link>

      <div className="flex-1" />

      <Button
        size="xs"
        visibleFrom="xs"
        leftSection={<IconPlus size={14} />}
        onClick={() => navigate("/workspaces/create")}
      >
        New workspace
      </Button>

      <Menu position="bottom-end" offset={8} withArrow arrowPosition="center">
        <Menu.Target>
          <button
            aria-label="Account menu"
            className="flex items-center rounded-full transition-opacity duration-150 hover:opacity-85"
          >
            <Avatar
              src={picURL || undefined}
              radius="xl"
              size={34}
              color="dark"
              className="border-2 border-slate-200"
            >
              {!picURL && initials}
            </Avatar>
          </button>
        </Menu.Target>

        <Menu.Dropdown>
          <div className="px-3 py-2">
            <Text size="sm" fw={700} truncate>
              {shortName || "Account"}
            </Text>
            {email && (
              <Text size="xs" c="dimmed" truncate>
                {email}
              </Text>
            )}
          </div>
          <Menu.Divider />
          <Menu.Item
            leftSection={<IconStack2 size={14} />}
            onClick={() => navigate("/spaces")}
          >
            Your Spaces
          </Menu.Item>
          <Menu.Item
            leftSection={<IconKey size={14} />}
            onClick={() => navigate("/usersecrets")}
          >
            Your Secrets
          </Menu.Item>
          <Menu.Divider />
          <Menu.Item
            leftSection={<IconSettings size={14} />}
            onClick={() => navigate("/settings")}
          >
            Settings
          </Menu.Item>
        </Menu.Dropdown>
      </Menu>
    </div>
  );
};

export default TopBar;
