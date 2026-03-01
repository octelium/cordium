import { RouteObject } from "react-router-dom";

import Actions from "./Actions";
import Builds from "./Builds";
import Edit from "./Edit";
import Main from "./Main";
import Workspaces from "./Workspaces";
import Root from "./index";

export default (): RouteObject => {
  return {
    path: ":templateName",
    element: <Root />,
    children: [
      {
        path: "",
        element: <Main />,
      },
      {
        path: "workspaces",
        element: <Workspaces />,
      },
      {
        path: "builds",
        element: <Builds />,
      },
      {
        path: "actions",
        element: <Actions />,
      },
      {
        path: "edit",
        element: <Edit />,
      },
    ],
  };
};
