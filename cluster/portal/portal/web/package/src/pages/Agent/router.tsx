import { RouteObject } from "react-router-dom";

import Main from "./index";

const routerAgent = (): RouteObject => {
  return {
    path: "agent",
    element: <Main />,
  };
};

export default routerAgent;
