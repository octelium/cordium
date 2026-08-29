import { RouteObject } from "react-router-dom";
import Main from "./Main";
import Settings from "./Settings";
import Workspaces from "./Workspaces";
import Root from "./index";

import routerGitProvider from "./GitProviders/router";
import routerMembership from "./Memberships/router";
import routerSecret from "./Secrets/router";
import routerTemplate from "./Templates/router";

const routerSpacesItem = (): RouteObject => {
  return {
    path: ":spaceName",
    element: <Root />,
    children: [
      { path: "", element: <Main /> },
      { path: "workspaces", element: <Workspaces /> },
      { path: "settings", element: <Settings /> },
      routerGitProvider(),
      routerSecret(),
      routerMembership(),
      routerTemplate(),
    ],
  };
};

export default routerSpacesItem;
