import { RouteObject } from "react-router-dom";
import Logs from "./Logs";
import Main from "./Main";
import Settings from "./Settings";
import Snapshots from "./Snapshots";
import Terminals from "./Terminals";
import Root from "./index";

const routerWorkspacesItem = (): RouteObject => {
  return {
    path: ":name",
    element: <Root />,
    children: [
      { path: "", element: <Main /> },
      { path: "terminals", element: <Terminals /> },
      { path: "logs", element: <Logs /> },
      { path: "snapshots", element: <Snapshots /> },
      { path: "settings", element: <Settings /> },
    ],
  };
};

export default routerWorkspacesItem;
