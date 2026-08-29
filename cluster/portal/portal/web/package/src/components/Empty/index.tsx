import { EmptyState } from "@mantine/core";
import { IconInbox } from "@tabler/icons-react";
import * as React from "react";

const Empty = (props: {
  title: string;
  description?: React.ReactNode;
  icon?: React.ReactNode;
  action?: React.ReactNode;
  compact?: boolean;
}) => (
  <EmptyState
    size={props.compact ? "sm" : "md"}
    withIndicatorBackground
    icon={props.icon ?? <IconInbox size={22} />}
    title={props.title}
    description={props.description}
    py={props.compact ? 28 : 56}
  >
    {props.action && <EmptyState.Actions>{props.action}</EmptyState.Actions>}
  </EmptyState>
);

export default Empty;
