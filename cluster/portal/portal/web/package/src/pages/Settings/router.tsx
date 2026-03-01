import { RouteObject } from "react-router-dom";
import Root from "./index";

export default (): RouteObject => {
  return {
    path: "settings",
    element: <Root />,
  };
};
