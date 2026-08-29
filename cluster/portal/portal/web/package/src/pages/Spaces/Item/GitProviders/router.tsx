import { RouteObject } from "react-router-dom";
import Create from "./Create";
import List from "./List";
import Root from "./index";

const routerSpacesItemGitproviders = (): RouteObject => {
  return {
    path: "gitproviders",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
    ],
  };
};

export default routerSpacesItemGitproviders;
