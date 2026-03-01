import { RouteObject } from "react-router-dom";
import Create from "./Create";
import Root from "./index";
import List from "./List";

export default (): RouteObject => {
  return {
    path: "gitproviders",
    element: <Root />,
    children: [
      {
        path: "",
        element: <List />,
      },
      {
        path: "create",
        element: <Create />,
      },
    ],
  };
};
