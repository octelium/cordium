import "@mantine/core/styles.css";
import "@xterm/xterm/css/xterm.css";
import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";

import { MantineProvider } from "@mantine/core";

import { Provider } from "react-redux";
import { RouterProvider } from "react-router-dom";

import store from "@/store";

import router from "@/router";
import { queryClient } from "@/utils";
import {
  colorSchemeManager,
  DEFAULT_COLOR_SCHEME,
} from "@/utils/theme/colorScheme";
import themeMantine, { cssVariablesResolver } from "@/utils/theme/mantine";
import { QueryClientProvider } from "@tanstack/react-query";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Provider store={store}>
      <MantineProvider
        theme={themeMantine}
        cssVariablesResolver={cssVariablesResolver}
        colorSchemeManager={colorSchemeManager}
        defaultColorScheme={DEFAULT_COLOR_SCHEME}
      >
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router()} />
        </QueryClientProvider>
      </MantineProvider>
    </Provider>
  </React.StrictMode>,
);
