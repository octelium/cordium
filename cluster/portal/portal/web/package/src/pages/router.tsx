import { RouteObject } from "react-router-dom";
import Home from "./Home";
import routerServices from "./Services/router";
import routerSettings from "./Settings/router";
import routerSpaces from "./Spaces/router";
import routerUserSecret from "./UserSecrets/router";
import routerWorkspaces from "./Workspaces/router";
import Root from "./index";

const routerRoot = (): RouteObject => {
  return {
    path: "/",
    element: <Root />,
    children: [
      { path: "", element: <Home /> },
      routerWorkspaces(),
      routerSpaces(),
      routerServices(),
      routerUserSecret(),
      routerSettings(),
    ],
  };
};

export default routerRoot;
