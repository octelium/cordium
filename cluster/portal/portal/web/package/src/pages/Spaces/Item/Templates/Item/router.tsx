import { RouteObject } from "react-router-dom";

import Builds from "./Builds";
import Main from "./Main";
import Settings from "./Settings";
import Workspaces from "./Workspaces";
import Root from "./index";

const routerSpacesItemTemplatesItem = (): RouteObject => {
  return {
    path: ":templateName",
    element: <Root />,
    children: [
      { path: "", element: <Main /> },
      { path: "workspaces", element: <Workspaces /> },
      { path: "builds", element: <Builds /> },
      { path: "settings", element: <Settings /> },
    ],
  };
};

export default routerSpacesItemTemplatesItem;
