import { RouteObject } from "react-router-dom";
import Root from "./index";
import Create from "./Create";
import Item from "./Item";
import List from "./List";
import Edit from "./Item/Edit";

import routerItem from "./Item/router";

export default (): RouteObject => {
  return {
    path: "spaces",
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

      
      routerItem(),
    ],
  };
};
