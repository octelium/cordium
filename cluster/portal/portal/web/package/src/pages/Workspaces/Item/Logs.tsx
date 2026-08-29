import ConsoleShell from "@/components/ConsoleShell";
import Empty from "@/components/Empty";
import LogConsole from "@/components/LogConsole";
import { useAppSelector } from "@/utils/hooks";
import { ActionIcon, Text, Tooltip } from "@mantine/core";
import { IconActivity, IconEraser } from "@tabler/icons-react";
import * as React from "react";
import { useContextWorkspace } from "../utils";
import { canUseWorkspaceService } from "./utils";

const Page = () => {
  const ctx = useContextWorkspace();
  const fullscreen = useAppSelector((s) => s.settings.terminalFullscreen);
  const [clearToken, setClearToken] = React.useState(0);
  const item = ctx.workspace.data;

  if (!item) return null;

  if (!canUseWorkspaceService(item)) {
    return (
      <Empty
        icon={<IconActivity size={22} />}
        title="No logs to stream"
        description="Startup and task logs are streamed while the workspace is initialising or running."
      />
    );
  }

  return (
    <ConsoleShell
      height={fullscreen ? undefined : 560}
      tabs={
        <Text size="xs" fw={600} className="px-2 text-slate-400">
          Startup and task logs · live
        </Text>
      }
      actions={
        <Tooltip label="Clear">
          <ActionIcon
            size={26}
            variant="subtle"
            color="gray"
            aria-label="Clear logs"
            onClick={() => setClearToken((v) => v + 1)}
          >
            <IconEraser size={14} />
          </ActionIcon>
        </Tooltip>
      }
    >
      <LogConsole
        key={item.metadata!.uid}
        item={item}
        clearToken={clearToken}
      />
    </ConsoleShell>
  );
};

export default Page;
