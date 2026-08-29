import { RouteObject } from "react-router-dom";
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
      },
    ],
  };
};

export default routerServices;
