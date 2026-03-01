import { RouteObject } from "react-router-dom";
import Actions from "./Actions";
import Edit from "./Edit";
import Main from "./Main";
import Workspaces from "./Workspaces";
import Root from "./index";

import routerGitProvider from "./GitProviders/router";
import routerMembership from "./Memberships/router";
import routerSecret from "./Secrets/router";
import routerTemplate from "./Templates/router";

export default (): RouteObject => {
  return {
    path: ":spaceName",
    element: <Root />,
    children: [
      {
        path: "",
        element: <Main />,
      },
      {
        path: "workspaces",
        element: <Workspaces />,
      },
      {
        path: "edit",
        element: <Edit />,
      },
      {
        path: "actions",
        element: <Actions />,
      },

      routerGitProvider(),
      routerSecret(),
      routerMembership(),
      routerTemplate(),
    ],
  };
};
