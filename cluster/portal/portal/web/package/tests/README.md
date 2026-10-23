From the portal package directory, install Chromium and run the browser tests:

```sh
npx playwright install --with-deps chromium
npm test
```

The tests start their own Vite servers and use in-page fixtures; a running cluster is not required.

The Agent suite checks conversations, search, composing, streaming, approvals, Workspace tool output, renaming, deletion, responsive navigation and approval settings. The domain suite verifies production and development RPC origins.

The portal suite builds a production fixture with protobuf RPC responses and a simulated WebSocket. It checks desktop and mobile navigation, Services search and server filters, pagination resets, error recovery, workspace actions, terminal pinning and drag ordering, deep links to a terminal, keyboard navigation, saved preferences, reconnect replay, full-screen continuity, WebGL context-loss recovery and log-stream cleanup. It also checks layouts in light and dark themes at desktop and mobile widths.

To capture screenshots while running the portal suite:

```sh
CORDIUM_PORTAL_ARTIFACT_DIR=../../../../../tmp/portal-review node --test tests/portal.test.mjs
```

The WebGL recovery test uses Chromium's software renderer. It reports a skip if the browser cannot expose the context-loss extension.
