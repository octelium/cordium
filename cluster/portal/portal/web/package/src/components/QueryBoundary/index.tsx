import { Alert, Loader } from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import * as React from "react";

export interface QueryLike {
  isPending?: boolean;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

const errorMessage = (err: unknown): string => {
  if (!err) return "The request failed.";
  if (err instanceof Error) return err.message;
  return String(err);
};

const QueryBoundary = (props: {
  query: QueryLike | QueryLike[];
  children?: React.ReactNode;
  minHeight?: number;
}) => {
  const queries = Array.isArray(props.query) ? props.query : [props.query];
  const isLoading = queries.some((q) => q.isPending ?? q.isLoading);
  const failed = queries.find((q) => q.isError);

  if (failed) {
    return (
      <Alert
        color="red"
        variant="light"
        icon={<IconAlertTriangle size={16} />}
        title="Something went wrong"
      >
        {errorMessage(failed.error)}
      </Alert>
    );
  }

  if (isLoading) {
    return (
      <div
        className="flex w-full items-center justify-center"
        style={{ minHeight: props.minHeight ?? 220 }}
      >
        <Loader size="sm" color="dark" />
      </div>
    );
  }

  return <>{props.children}</>;
};

export default QueryBoundary;
