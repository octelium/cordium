import { RouteObject } from "react-router-dom";
import Connect from "./Connect";
import Root from "./index";
import List from "./List";

const routerServices = (): RouteObject => {
  return {
    path: "services",
    element: <Root />,
    children: [
      {
        path: "",
        element: <List />,
        children: [
          {
            path: ":name/connect",
            element: <Connect />,
          },
        ],
      },
    ],
  };
};

export default routerServices;
