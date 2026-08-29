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
      dot: "bg-emerald-500",
      text: "text-emerald-700",
      busy: false,
    }))
    .with(S.STOPPED, () => ({
      label: "Stopped",
      dot: "bg-slate-300",
      text: "text-slate-500",
      busy: false,
    }))
    .with(S.STOPPING, () => ({
      label: "Stopping",
      dot: "bg-slate-400",
      text: "text-slate-600",
      busy: true,
    }))
    .with(S.STOPPING_REQUEST, () => ({
      label: "Stop requested",
      dot: "bg-slate-400",
      text: "text-slate-600",
      busy: true,
    }))
    .with(S.STARTING_RUNTIME, () => ({
      label: "Starting runtime",
      dot: "bg-sky-500",
      text: "text-sky-700",
      busy: true,
    }))
    .with(S.PULLING_IMAGE, () => ({
      label: "Pulling image",
      dot: "bg-violet-500",
      text: "text-violet-700",
      busy: true,
    }))
    .with(S.BUILDING_IMAGE, () => ({
      label: "Building image",
      dot: "bg-blue-500",
      text: "text-blue-700",
      busy: true,
    }))
    .with(S.PREPARING, () => ({
      label: "Preparing",
      dot: "bg-teal-500",
      text: "text-teal-700",
      busy: true,
    }))
    .with(S.INITIALIZING, () => ({
      label: "Initializing",
      dot: "bg-amber-500",
      text: "text-amber-700",
      busy: true,
    }))
    .with(S.INIT_REQUEST, () => ({
      label: "Start requested",
      dot: "bg-amber-500",
      text: "text-amber-700",
      busy: true,
    }))
    .otherwise(() => ({
      label: "Unknown",
      dot: "bg-slate-300",
      text: "text-slate-400",
      busy: false,
    }));

const StateBadge = (props: { state: State; size?: "sm" | "md" }) => {
  const meta = getStateMeta(props.state);
  const big = props.size === "md";

  return (
    <span
      className={twMerge(
        "inline-flex items-center gap-1.5 rounded-full border border-slate-200 bg-white",
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
