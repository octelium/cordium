import * as React from "react";

import { Timestamp } from "@octelium/apis/google/protobuf/timestamp";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import utc from "dayjs/plugin/utc";

dayjs.extend(relativeTime);
dayjs.extend(utc);

import { Tooltip } from "@mantine/core";

const TimeAgo = (props: { rfc3339?: Timestamp }) => {
  const at = props.rfc3339;
  const millis = at ? Timestamp.toDate(at).getTime() : 0;

  const [, setTick] = React.useState(0);

  React.useEffect(() => {
    if (!millis) return;
    const interval = setInterval(() => setTick((t) => t + 1), 10000);
    return () => clearInterval(interval);
  }, [millis]);

  if (!millis) return null;

  const label = dayjs(millis).fromNow();

  return (
    <Tooltip label={dayjs(millis).local().format("HH:mm:ss, ddd MMM D, YYYY")}>
      <span className="whitespace-nowrap">{label}</span>
    </Tooltip>
  );
};

export default TimeAgo;
