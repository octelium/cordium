import { RouteObject } from "react-router-dom";
import Create from "./Create";
import Root from "./index";
import List from "./List";

const routerUsersecrets = (): RouteObject => {
  return {
    path: "usersecrets",
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

export default routerUsersecrets;
