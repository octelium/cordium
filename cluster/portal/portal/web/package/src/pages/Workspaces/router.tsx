import { RouteObject } from "react-router-dom";
import Create from "./Create";
import routerItem from "./Item/router";
import List from "./List";
import Root from "./index";

const routerWorkspaces = (): RouteObject => {
  return {
    path: "workspaces",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
      routerItem(),
    ],
  };
};

export default routerWorkspaces;
