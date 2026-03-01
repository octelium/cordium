import { RouteObject } from "react-router-dom";
import Root from "./index";
import CreateWorkspace from "./Create";
import List from "./List";
import routerItem from "./Item/router";

export default (): RouteObject => {
  return {
    path: "workspaces",
    element: <Root />,
    children: [
      {
        path: "",
        element: <List />,
      },
      {
        path: "create",
        element: <CreateWorkspace />,
      },
      routerItem(),
    ],
  };
};
