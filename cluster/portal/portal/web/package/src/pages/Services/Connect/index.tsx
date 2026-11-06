import * as React from "react";

const Connect = React.lazy(() => import("./Connect"));

const ConnectPage = () => (
  <React.Suspense fallback={null}>
    <Connect />
  </React.Suspense>
);

export default ConnectPage;
