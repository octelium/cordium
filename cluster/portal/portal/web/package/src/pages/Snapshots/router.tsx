import { RouteObject } from "react-router-dom";
import List from "./List";
import Root from "./index";

const routerSnapshots = (): RouteObject => {
  return {
    path: "snapshots",
    element: <Root />,
    children: [{ path: "", element: <List /> }],
  };
};

export default routerSnapshots;
