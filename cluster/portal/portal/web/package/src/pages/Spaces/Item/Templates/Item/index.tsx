import { useContextSpace } from "@/pages/Spaces/utils";
import { Outlet, useNavigate } from "react-router-dom";

import { Tabs } from "@mantine/core";

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  if (!ctx.template.isSuccess) {
    return <></>;
  }

  return (
    <div className="ml-8">
      <div>
        <Tabs defaultValue="main" className="font-bold">
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
              value="edit"
              onClick={() => {
                navigate("./edit");
              }}
            >
              Config
            </Tabs.Tab>
            {/**
             <Tabs.Tab
              value="builds"
              onClick={() => {
                navigate("./builds");
              }}
            >
              Builds
            </Tabs.Tab>
             **/}

            <Tabs.Tab
              value="workspaces"
              onClick={() => {
                navigate("./workspaces");
              }}
            >
              Your Workspaces
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
    </div>
  );
};

export default Page;
