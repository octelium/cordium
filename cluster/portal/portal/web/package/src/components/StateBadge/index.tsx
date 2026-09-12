import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconLoader2 } from "@tabler/icons-react";
import { twMerge } from "tailwind-merge";
import { match } from "ts-pattern";

type State = WsPB.Workspace_Status_State;
const S = WsPB.Workspace_Status_State;

interface StateMeta {
  label: string;
  dot: string;
  text: string;
  busy: boolean;
}

const getStateMeta = (state: State): StateMeta =>
  match(state)
    .with(S.RUNNING, () => ({
      label: "Running",
      dot: "bg-hue-emerald-solid",
      text: "text-hue-emerald",
      busy: false,
    }))
    .with(S.STOPPED, () => ({
      label: "Stopped",
      dot: "bg-ink-faint",
      text: "text-ink-muted",
      busy: false,
    }))
    .with(S.STOPPING, () => ({
      label: "Stopping",
      dot: "bg-ink-subtle",
      text: "text-ink-soft",
      busy: true,
    }))
    .with(S.STOPPING_REQUEST, () => ({
      label: "Stop requested",
      dot: "bg-ink-subtle",
      text: "text-ink-soft",
      busy: true,
    }))
    .with(S.STARTING_RUNTIME, () => ({
      label: "Starting runtime",
      dot: "bg-hue-sky-solid",
      text: "text-hue-sky",
      busy: true,
    }))
    .with(S.PULLING_IMAGE, () => ({
      label: "Pulling image",
      dot: "bg-hue-violet-solid",
      text: "text-hue-violet",
      busy: true,
    }))
    .with(S.BUILDING_IMAGE, () => ({
      label: "Building image",
      dot: "bg-hue-blue-solid",
      text: "text-hue-blue",
      busy: true,
    }))
    .with(S.PREPARING, () => ({
      label: "Preparing",
      dot: "bg-hue-teal-solid",
      text: "text-hue-teal",
      busy: true,
    }))
    .with(S.INITIALIZING, () => ({
      label: "Initializing",
      dot: "bg-hue-amber-solid",
      text: "text-hue-amber",
      busy: true,
    }))
    .with(S.INIT_REQUEST, () => ({
      label: "Start requested",
      dot: "bg-hue-amber-solid",
      text: "text-hue-amber",
      busy: true,
    }))
    .otherwise(() => ({
      label: "Unknown",
      dot: "bg-ink-faint",
      text: "text-ink-subtle",
      busy: false,
    }));

const StateBadge = (props: { state: State; size?: "sm" | "md" }) => {
  const meta = getStateMeta(props.state);
  const big = props.size === "md";

  return (
    <span
      className={twMerge(
        "inline-flex items-center gap-1.5 rounded-full border border-line bg-surface",
        big ? "px-2.5 py-1" : "px-2 py-0.5",
      )}
    >
      {meta.busy ? (
        <IconLoader2
          size={big ? 14 : 12}
          className={twMerge("animate-spin shrink-0", meta.text)}
        />
      ) : (
        <span
          className={twMerge(
            "rounded-full shrink-0",
            big ? "w-2.5 h-2.5" : "w-2 h-2",
            meta.dot,
            props.state === S.RUNNING && "state-pulse",
          )}
        />
      )}
      <span
        className={twMerge(
          "font-semibold",
          big ? "text-[0.8rem]" : "text-[0.72rem]",
          meta.text,
        )}
      >
        {meta.label}
      </span>
    </span>
  );
};

export default StateBadge;
