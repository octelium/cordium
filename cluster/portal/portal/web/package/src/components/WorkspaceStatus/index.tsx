import { Loader2 } from "lucide-react";
import { twMerge } from "tailwind-merge";
import { match } from "ts-pattern";
import * as WsPB from "../../apis/cordiumv1/cordiumv1";

type State = WsPB.Workspace_Status_State;
const S = WsPB.Workspace_Status_State;

interface StateMeta {
  label: string;
  dot: string;
  text: string;
  loading: boolean;
}

const getStateMeta = (state: State): StateMeta =>
  match(state)
    .with(S.RUNNING, () => ({
      label: "Running",
      dot: "bg-emerald-500",
      text: "text-emerald-700",
      loading: false,
    }))
    .with(S.STOPPED, () => ({
      label: "Stopped",
      dot: "bg-slate-300",
      text: "text-slate-500",
      loading: false,
    }))
    .with(S.STOPPING, S.STOPPING_REQUEST, () => ({
      label: state === S.STOPPING ? "Stopping" : "Stopping Request",
      dot: "bg-slate-500",
      text: "text-slate-600",
      loading: true,
    }))
    .with(S.STARTING_RUNTIME, () => ({
      label: "Starting Runtime",
      dot: "bg-pink-500",
      text: "text-pink-700",
      loading: true,
    }))
    .with(S.PULLING_IMAGE, () => ({
      label: "Pulling Image",
      dot: "bg-violet-500",
      text: "text-violet-700",
      loading: true,
    }))
    .with(S.BUILDING_IMAGE, () => ({
      label: "Building Image",
      dot: "bg-blue-500",
      text: "text-blue-700",
      loading: true,
    }))
    .with(S.PREPARING, () => ({
      label: "Preparing",
      dot: "bg-lime-400",
      text: "text-lime-700",
      loading: true,
    }))
    .with(S.INITIALIZING, S.INIT_REQUEST, () => ({
      label:
        state === S.INITIALIZING ? "Initializing" : "Initialization Request",
      dot: "bg-slate-400",
      text: "text-slate-600",
      loading: true,
    }))
    .otherwise(() => ({
      label: "Unknown",
      dot: "bg-slate-300",
      text: "text-slate-400",
      loading: false,
    }));

const WorkspaceStatus = (props: { status: State }) => {
  const meta = getStateMeta(props.status);

  return (
    <div className="inline-flex items-center gap-1.5">
      {meta.loading ? (
        <Loader2
          size={13}
          strokeWidth={2.5}
          className={twMerge("animate-spin shrink-0", meta.text)}
        />
      ) : (
        <span
          className={twMerge(
            "w-2 h-2 rounded-full shrink-0",
            meta.dot,
            props.status === S.RUNNING &&
              "shadow-[0_0_0_3px_rgba(16,185,129,0.15)]",
          )}
        />
      )}
      <span className={twMerge("text-[0.78rem] font-semibold", meta.text)}>
        {meta.label}
      </span>
    </div>
  );
};

export default WorkspaceStatus;
