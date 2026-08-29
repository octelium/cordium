import { RouteObject } from "react-router-dom";
import Create from "./Create";
import routerItem from "./Item/router";
import List from "./List";
import Root from "./index";

const routerSpaces = (): RouteObject => {
  return {
    path: "spaces",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
      routerItem(),
    ],
  };
};

export default routerSpaces;
