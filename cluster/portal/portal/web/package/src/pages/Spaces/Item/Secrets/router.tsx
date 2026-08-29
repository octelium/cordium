import { RouteObject } from "react-router-dom";
import Create from "./Create";
import List from "./List";
import Root from "./index";

const routerSpacesItemSecrets = (): RouteObject => {
  return {
    path: "secrets",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
    ],
  };
};

export default routerSpacesItemSecrets;
