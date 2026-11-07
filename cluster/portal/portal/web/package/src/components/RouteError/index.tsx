import { Button } from "@mantine/core";
import {
  IconAlertTriangle,
  IconHome,
  IconRefresh,
  IconZoomExclamation,
} from "@tabler/icons-react";
import { Link, useRouteError } from "react-router-dom";

import Empty from "@/components/Empty";
import Meta from "@/components/Meta";
import { Panel } from "@/components/Panel";

const Title = (props: { children: string }) => (
  <h1 className="text-inherit font-inherit">{props.children}</h1>
);

const OverviewButton = (props: { label: string }) => (
  <Button
    component={Link}
    to="/"
    variant="default"
    leftSection={<IconHome size={15} />}
  >
    {props.label}
  </Button>
);

export const RouteError = () => {
  const error = useRouteError();
  const message = error instanceof Error ? error.message : String(error);

  return (
    <Panel>
      <Meta title="Something went wrong" />
      <div role="alert">
        <Empty
          title={<Title>This page hit an unexpected error</Title>}
          description={
            <span className="font-mono text-xs [overflow-wrap:anywhere]">
              {message}
            </span>
          }
          icon={<IconAlertTriangle size={22} />}
          action={
            <>
              <Button
                variant="default"
                leftSection={<IconRefresh size={15} />}
                onClick={() => window.location.reload()}
              >
                Reload
              </Button>
              <OverviewButton label="Overview" />
            </>
          }
        />
      </div>
    </Panel>
  );
};

export const NotFoundPage = () => (
  <Panel>
    <Meta title="Page not found" />
    <Empty
      title={<Title>Page not found</Title>}
      description="This address does not match any page of the portal."
      icon={<IconZoomExclamation size={22} />}
      action={<OverviewButton label="Go to Overview" />}
    />
  </Panel>
);
