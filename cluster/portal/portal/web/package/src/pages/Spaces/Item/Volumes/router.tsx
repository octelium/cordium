import { RouteObject } from "react-router-dom";
import Create from "./Create";
import List from "./List";
import Root from "./index";

const routerSpacesItemVolumes = (): RouteObject => {
  return {
    path: "volumes",
    element: <Root />,
    children: [
      { path: "", element: <List /> },
      { path: "create", element: <Create /> },
    ],
  };
};

export default routerSpacesItemVolumes;
