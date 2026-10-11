From the portal package directory, install Chromium and run the browser tests:

```sh
npx playwright install chromium
npm test
```

The tests start their own Vite server and mount the Agent chat against an in-page fake agent (`fetch` and `EventSource`) to check the conversation list, search, composer, streaming, approvals, Workspace tool output, renaming, deletion, the sidebar on wide and narrow screens and the approval settings. A running cluster is not required.
