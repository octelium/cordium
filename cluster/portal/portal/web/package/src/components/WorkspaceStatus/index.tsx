import * as WsPB from "../../apis/cordiumv1/cordiumv1";

import ClipLoader from "react-spinners/ClipLoader";
import { twMerge } from "tailwind-merge";

const getStateText = (arg: WsPB.Workspace_Status_State): string => {
  switch (arg) {
    case WsPB.Workspace_Status_State.RUNNING:
      return "Running";
    case WsPB.Workspace_Status_State.STARTING_RUNTIME:
      return "Starting Runtime";
    case WsPB.Workspace_Status_State.PULLING_IMAGE:
      return "Pulling Image";
    case WsPB.Workspace_Status_State.BUILDING_IMAGE:
      return "Building Image";
    case WsPB.Workspace_Status_State.PREPARING:
      return "Preparing";
    case WsPB.Workspace_Status_State.STOPPING_REQUEST:
      return "Stopping Request";
    case WsPB.Workspace_Status_State.STOPPING:
      return "Stopping";
    case WsPB.Workspace_Status_State.STOPPED:
      return "Stopped";
    case WsPB.Workspace_Status_State.INITIALIZING:
      return "Initializing";
    case WsPB.Workspace_Status_State.INIT_REQUEST:
      return "Initialization Request";
    case WsPB.Workspace_Status_State.UNKNOWN:
      return "Unknown";
    default:
      return "Unknown";
  }
};

const getStateColor = (arg: WsPB.Workspace_Status_State): string => {
  switch (arg) {
    case WsPB.Workspace_Status_State.RUNNING:
      return "#1cc02a";
    case WsPB.Workspace_Status_State.STARTING_RUNTIME:
      return "#f031b0";
    case WsPB.Workspace_Status_State.PULLING_IMAGE:
      return "#5138e0";
    case WsPB.Workspace_Status_State.BUILDING_IMAGE:
      return "#2075d6";
    case WsPB.Workspace_Status_State.PREPARING:
      return "#aff07a";
    case WsPB.Workspace_Status_State.STOPPING_REQUEST:
    case WsPB.Workspace_Status_State.STOPPING:
      return "#444";
    case WsPB.Workspace_Status_State.STOPPED:
      return "#ccc";
    case WsPB.Workspace_Status_State.INIT_REQUEST:
    case WsPB.Workspace_Status_State.INITIALIZING:
      return "#999";
    case WsPB.Workspace_Status_State.UNKNOWN:
      return "#000";
    default:
      return "#aaa";
  }
};

const needsLoadingFn = (arg: WsPB.Workspace_Status_State): boolean => {
  switch (arg) {
    case WsPB.Workspace_Status_State.RUNNING:
      return false;
    case WsPB.Workspace_Status_State.STOPPED:
      return false;
    case WsPB.Workspace_Status_State.UNKNOWN:
      return false;
    default:
      return true;
  }
};

const WorkspaceStatus = (props: { status: WsPB.Workspace_Status_State }) => {
  const needsLoading = needsLoadingFn(props.status);
  return (
    <div>
      <div className="w-full flex items-center">
        {!needsLoading && (
          <div
            style={{
              backgroundColor: getStateColor(props.status),
            }}
            className={twMerge(`rounded-full w-[20px] h-[20px]`)}
          ></div>
        )}
        {needsLoading && (
          <ClipLoader
            color={getStateColor(props.status)}
            loading={true}
            size={20}
          />
        )}

        <div className="ml-1 text-sm">{getStateText(props.status)}</div>
      </div>
    </div>
  );
};

export default WorkspaceStatus;
