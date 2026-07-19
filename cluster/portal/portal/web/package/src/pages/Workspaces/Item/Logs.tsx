import * as React from "react";

import * as WsPB from "@octelium/apis/main/cordiumv1";

// import { sendListenEvent } from "@/features/conn/slice";
import TerminalEvent from "@/components/TerminalEvent";
import { twMerge } from "tailwind-merge";

import { canUseWorkspaceService } from "./utils";

import EmptyList from "@/components/EmptyList";
import PageWrap from "@/components/PageWrap";
import { Button } from "@mantine/core";
import { useContextWorkspace } from "../utils";

const LogsBar = (props: { item: WsPB.Workspace }) => {
  const { item } = props;
  let [showLogs, setShowLogs] = React.useState(true);
  if (!canUseWorkspaceService(item)) {
    return <EmptyList title="Workspace needs to be active to see Logs" />;
  }

  return (
    <div className="w-full flex flex-col items-center justify-center">
      <div className="w-full flex items-center justify-center my-6">
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            setShowLogs(!showLogs);
          }}
        >
          {showLogs ? "Hide Logs" : "Show Logs"}
        </Button>
      </div>

      <div className={twMerge(showLogs ? "flex w-full" : "hidden")}>
        <TerminalEvent item={item} />
      </div>
    </div>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <LogsBar item={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Page;
