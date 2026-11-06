import {
  agentUserConfigKey,
  agentWorkspaceKey,
  getWorkspaceFailure,
  isNotFound,
  isWorkspaceStarting,
  isWorkspaceStopping,
  startAgentWorkspace,
} from "@/pages/Agent/utils";
import { getClientWorkspace } from "@/utils/client";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { GetOptions, ObjectReference } from "@octelium/apis/main/metav1";
import type { RpcError } from "@protobuf-ts/runtime-rpc";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";

const S = WsPB.Workspace_Status_State;

export const useAgentWorkspace = () => {
  const queryClient = useQueryClient();

  const configQuery = useQuery({
    queryKey: agentUserConfigKey,
    queryFn: async () =>
      (
        await getClientWorkspace().getUserConfig(
          WsPB.GetUserConfigRequest.create({}),
        )
      ).response,
    retry: 1,
    refetchOnWindowFocus: false,
  });

  const uid = configQuery.data?.status?.agentWorkspaceRef?.uid ?? "";

  const workspaceQuery = useQuery({
    queryKey: agentWorkspaceKey(uid),
    queryFn: async () =>
      (await getClientWorkspace().getWorkspace(GetOptions.create({ uid })))
        .response,
    enabled: !!uid,
    retry: (count, err) => !isNotFound(err) && count < 2,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => {
      const state = query.state.data?.status?.state;
      if (query.state.error) return false;
      if (state === S.RUNNING) return 15000;
      if (state === S.STOPPED) return false;
      return 2000;
    },
  });

  const missing = isNotFound(workspaceQuery.error);
  const workspace = uid && !missing ? workspaceQuery.data : undefined;
  const state = workspace?.status?.state;
  const failure = getWorkspaceFailure(workspace);

  const start = useMutation({
    mutationFn: async (ws: WsPB.Workspace) =>
      startAgentWorkspace(
        ObjectReference.create({
          uid: ws.metadata?.uid,
          name: ws.metadata?.name,
        }),
        (config) => queryClient.setQueryData(agentUserConfigKey, config),
      ),
    onSuccess: (_data, ws) =>
      queryClient.setQueryData<WsPB.Workspace>(
        agentWorkspaceKey(ws.metadata?.uid ?? ""),
        (current) =>
          current?.status?.state === S.STOPPED
            ? {
                ...current,
                status: { ...current.status, state: S.INIT_REQUEST },
              }
            : current,
      ),
    onSettled: (_data, _err, ws) =>
      queryClient.invalidateQueries({
        queryKey: agentWorkspaceKey(ws.metadata?.uid ?? ""),
      }),
  });

  const initialize = useMutation({
    mutationFn: async () =>
      (
        await getClientWorkspace().initializeAgent(
          WsPB.InitializeAgentRequest.create({}),
        )
      ).response,
    onSuccess: (data) => queryClient.setQueryData(agentUserConfigKey, data),
  });

  const { mutate } = start;
  const startWorkspace = React.useCallback(() => {
    if (workspace) mutate(workspace);
  }, [workspace, mutate]);

  const attempted = !start.isIdle && start.variables?.metadata?.uid === uid;
  const shouldStart =
    !!workspace && state === S.STOPPED && !failure && !attempted;

  React.useEffect(() => {
    if (shouldStart) startWorkspace();
  }, [shouldStart, startWorkspace]);

  return {
    configQuery,
    workspaceQuery,
    workspace,
    state,
    failure,
    uid,
    missing,
    starting:
      isWorkspaceStarting(workspace) ||
      (state === S.STOPPED && (start.isPending || shouldStart)),
    stopping: isWorkspaceStopping(workspace),
    running: state === S.RUNNING,
    startWorkspace,
    startPending: start.isPending,
    startError: start.error as RpcError | null,
    initialize,
  };
};
