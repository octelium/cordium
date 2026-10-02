import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";
import svgr from "vite-plugin-svgr";
import { visualizer } from "rollup-plugin-visualizer";

import { createRequire } from "module";
import type { RollupCommonJSOptions } from "@rollup/plugin-commonjs";

const require = createRequire(import.meta.url);

const __dirname = path.resolve();

const shadowRootPlugin = {
  name: "mantine-shadow-root",
  resolveId(id: string, importer?: string) {
    if (
      importer?.includes("/node_modules/@mantine/core/") &&
      id.endsWith("/find-element-in-shadow-dom.mjs")
    ) {
      return path.resolve(__dirname, "src/utils/dom/shadowRoot.ts");
    }
  },
};

export default defineConfig({
  plugins: [
    { ...shadowRootPlugin, enforce: "pre" },
    react(),
    svgr(),
    visualizer({
      emitFile: true,
      filename: "tmp/stats.html",
    }),
  ],
  optimizeDeps: {
    rolldownOptions: {
      plugins: [shadowRootPlugin],
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    manifest: true,
    sourcemap: true,
    commonjsOptions: {
      defaultIsModuleExports(id) {
        try {
          const module = require(id);
          if (module?.default) {
            return false;
          }
          return "auto";
        } catch {
          return "auto";
        }
      },
      transformMixedEsModules: true,
    } as RollupCommonJSOptions,
  },

  server: {
    proxy: {
      "/connect": {
        target: "https://workspaces.octelium.org",
        changeOrigin: true,
        ws: true,
        secure: false,
        proxyTimeout: 5000,

        headers: {
          "x-octelium": "octelium",
        },
      },

      "/octelium.api": {
        target: "http://127.0.0.1:10003",
        // changeOrigin: true,
        // secure: false,
        // proxyTimeout: 5000,
        headers: {
          "x-octelium": "octelium",
          "content-type": "application/grpc-web-text+proto",
        },
      },
    },
  },
});
