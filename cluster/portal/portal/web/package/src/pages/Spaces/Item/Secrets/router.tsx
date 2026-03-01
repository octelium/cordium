import { RouteObject } from "react-router-dom";
import Create from "./Create";
import Root from "./index";
import Item from "./Item";
import List from "./List";

export default (): RouteObject => {
  return {
    path: "secrets",
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
      {
        path: "uid/:uid",
        element: <Item />,
      },
    ],
  };
};
