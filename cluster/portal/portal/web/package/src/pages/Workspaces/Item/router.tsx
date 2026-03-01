import { RouteObject } from "react-router-dom";
import Actions from "./Actions";
import Edit from "./Edit";
import Logs from "./Logs";
import Main from "./Main";
import Terminals from "./Terminals";
import Root from "./index";

export default (): RouteObject => {
  return {
    path: ":name",
    element: <Root />,
    children: [
      {
        path: "",
        element: <Main />,
      },
      {
        path: "terminals",
        element: <Terminals />,
      },
      {
        path: "logs",
        element: <Logs />,
      },
      {
        path: "edit",
        element: <Edit />,
      },
      {
        path: "actions",
        element: <Actions />,
      },
    ],
  };
};
