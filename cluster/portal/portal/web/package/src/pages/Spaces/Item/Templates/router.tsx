import { RouteObject } from "react-router-dom";
import Create from "./Create";
import routerItem from "./Item/router";
import List from "./List";
import Root from "./index";

const routerSpacesItemTemplates = (): RouteObject => {
  return {
    path: "templates",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
      routerItem(),
    ],
  };
};

export default routerSpacesItemTemplates;
