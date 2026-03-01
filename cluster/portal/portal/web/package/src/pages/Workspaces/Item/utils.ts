import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { match } from "ts-pattern";

export const canUseWorkspaceService = (item: WsPB.Workspace) => {
  return match(item.status!.state)
    .with(WsPB.Workspace_Status_State.BUILDING_IMAGE, () => true)
    .with(WsPB.Workspace_Status_State.PREPARING, () => true)
    .with(WsPB.Workspace_Status_State.PULLING_IMAGE, () => true)
    .with(WsPB.Workspace_Status_State.STARTING_RUNTIME, () => true)
    .with(WsPB.Workspace_Status_State.RUNNING, () => true)
    .otherwise(() => false);
};

export const canUseTerminals = (item: WsPB.Workspace) => {
  return (
    item.status!.state === WsPB.Workspace_Status_State.RUNNING ||
    item.status!.state === WsPB.Workspace_Status_State.PREPARING
  );
};
