import { Loader } from "@mantine/core";
import * as React from "react";

const Agent = React.lazy(() => import("./Agent"));

const AgentPage = () => (
  <React.Suspense
    fallback={
      <div className="flex h-full items-center justify-center">
        <Loader size="sm" color="gray" />
      </div>
    }
  >
    <Agent />
  </React.Suspense>
);

export default AgentPage;
