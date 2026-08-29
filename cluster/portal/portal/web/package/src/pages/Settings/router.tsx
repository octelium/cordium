import { RouteObject } from "react-router-dom";
import Root from "./index";

const routerSettings = (): RouteObject => {
  return {
    path: "settings",
    element: <Root />,
  };
};

export default routerSettings;
