import { RouteObject } from "react-router-dom";
import Create from "./Create";
import List from "./List";
import Root from "./index";

const routerSpacesItemMemberships = (): RouteObject => {
  return {
    path: "memberships",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
    ],
  };
};

export default routerSpacesItemMemberships;
