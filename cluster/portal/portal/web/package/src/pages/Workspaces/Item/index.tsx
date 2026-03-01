import * as React from "react";

import { useAppDispatch } from "@/utils/hooks";

import { Outlet, useLocation } from "react-router-dom";

import { useNavigate } from "react-router-dom";

// import { sendListenEvent } from "@/features/conn/slice";

import { clearTerminalGroup } from "@/features/terminalgroup/slice";

import PageWrap from "@/components/PageWrap";
import { Tabs } from "@mantine/core";
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

  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && (
        <div className="w-full font-bold">
          <div>
            <Tabs
              defaultValue="main"
              value={match(loc.pathname.split("/").reverse().at(0))
                .with("edit", (v) => v)
                .with("actions", (v) => v)
                .with("terminals", (v) => v)
                .with("logs", (v) => v)
                .otherwise(() => "main")}
            >
              <Tabs.List className="mb-2">
                <Tabs.Tab
                  value="main"
                  onClick={() => {
                    navigate("./");
                  }}
                >
                  Main
                </Tabs.Tab>
                <Tabs.Tab
                  value="terminals"
                  onClick={() => {
                    navigate("./terminals");
                  }}
                >
                  Terminals
                </Tabs.Tab>
                <Tabs.Tab
                  value="edit"
                  onClick={() => {
                    navigate("./edit");
                  }}
                >
                  Edit
                </Tabs.Tab>
                <Tabs.Tab
                  value="logs"
                  onClick={() => {
                    navigate("./logs");
                  }}
                >
                  Activity Logs
                </Tabs.Tab>
                <Tabs.Tab
                  value="actions"
                  onClick={() => {
                    navigate("./actions");
                  }}
                >
                  Actions
                </Tabs.Tab>
              </Tabs.List>
            </Tabs>
          </div>

          <div>
            <Outlet />
          </div>

          {/*
      <div>
        <div>
          <InfoBar item={data} />
        </div>

        <div className="mt-6">
          <TerminalGroup workspace={data} />
        </div>

        <div>
          <LogsBar item={data} />
        </div>

        <div>
          <ActionsBar item={data} />
        </div>

        <div>
          <AppsBar item={data} />
        </div>

        <div className="mt-2 mb-12"></div>
      </div>
      */}
        </div>
      )}
    </PageWrap>
  );
};

export default Workspace;
