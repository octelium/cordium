import ConfirmAction from "@/components/ConfirmAction";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import QueryBoundary from "@/components/QueryBoundary";
import StateBadge from "@/components/StateBadge";
import TabNav from "@/components/TabNav";
import Tag from "@/components/Tag";
import YamlDrawer from "@/components/YamlDrawer";
import { clearTerminalGroup } from "@/features/terminalgroup/slice";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppDispatch } from "@/utils/hooks";
import {
  getPathSpaceRef,
  getPathWorkspace,
  getWorkspaceURL,
  invalidateResource,
} from "@/utils/octelium";
import { canStopWorkspace, getResourceRef, getShortNameFromRef, isWorkspaceStopped } from "@/utils/pb";
import { Button } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import {
  IconActivity,
  IconBolt,
  IconExternalLink,
  IconLayoutGrid,
  IconPlayerPlay,
  IconPlayerStop,
  IconSettings,
  IconTerminal2,
} from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import { Outlet } from "react-router-dom";
import { useContextWorkspace } from "../utils";

export const StartStopButtons = (props: {
  item: WsPB.Workspace;
  fullWidth?: boolean;
}) => {
  const { item } = props;
  const dispatch = useAppDispatch();
  const client = getClientWorkspace();

  const mutationStart = useMutation({
    mutationFn: async () => {
      await client.startWorkspace(
        WsPB.StartWorkspaceRequest.create({
          workspaceRef: getResourceRef(item),
        }),
      );
    },
    onSuccess: () => invalidateResource(item),
    onError,
  });

  const mutationStop = useMutation({
    mutationFn: async () => {
      await client.stopWorkspace(
        WsPB.StopWorkspaceRequest.create({
          workspaceRef: getResourceRef(item),
        }),
      );
    },
    onSuccess: () => {
      dispatch(clearTerminalGroup({}));
      invalidateResource(item);
    },
    onError,
  });

  if (isWorkspaceStopped(item)) {
    return (
      <Button
        leftSection={<IconPlayerPlay size={15} />}
        fullWidth={props.fullWidth}
        loading={mutationStart.isPending}
        onClick={() => mutationStart.mutate()}
      >
        Start
      </Button>
    );
  }

  if (canStopWorkspace(item)) {
    return (
      <ConfirmAction
        triggerLabel="Stop"
        triggerIcon={<IconPlayerStop size={14} />}
        size="sm"
        fullWidth={props.fullWidth}
        title="Stop this workspace?"
        confirmLabel="Stop workspace"
        description={
          item.spec?.isEphemeral
            ? "This workspace uses ephemeral storage — everything outside the image is discarded when it stops."
            : "Running processes and terminal sessions are terminated. Persistent storage is kept."
        }
        loading={mutationStop.isPending}
        onConfirm={() => mutationStop.mutate()}
      />
    );
  }

  return null;
};

const Page = () => {
  const dispatch = useAppDispatch();
  const ctx = useContextWorkspace();
  const data = ctx.workspace.data;

  React.useEffect(() => {
    dispatch(clearTerminalGroup({}));
    return () => {
      dispatch(clearTerminalGroup({}));
    };
  }, [dispatch]);

  const url = data ? getWorkspaceURL(data) : undefined;
  const base = data ? getPathWorkspace(data) : "";

  return (
    <QueryBoundary query={ctx.workspace}>
      {data && (
        <>
          <Meta title={`${data.metadata!.name} · Workspace`} />
          <PageHeader
            title={data.metadata?.displayName || data.metadata!.name}
            crumbs={[
              { label: "Workspaces", to: "/workspaces" },
              ...(data.status?.spaceRef
                ? [
                    {
                      label: getShortNameFromRef(data.status.spaceRef),
                      to: getPathSpaceRef(data.status.spaceRef),
                    },
                  ]
                : []),
              { label: data.metadata!.name },
            ]}
            badges={
              <>
                <StateBadge state={data.status!.state} size="md" />
                {data.spec?.isEphemeral && (
                  <Tag tone="warning" icon={<IconBolt size={11} />}>
                    Ephemeral
                  </Tag>
                )}
              </>
            }
            actions={
              <>
                <YamlDrawer item={data} />
                {url && !isWorkspaceStopped(data) && (
                  <Button
                    variant="default"
                    component="a"
                    href={url}
                    target="_blank"
                    rel="noreferrer"
                    leftSection={<IconExternalLink size={15} />}
                  >
                    Open
                  </Button>
                )}
                <StartStopButtons item={data} />
              </>
            }
          />

          <TabNav
            items={[
              {
                label: "Overview",
                to: base,
                end: true,
                icon: <IconLayoutGrid size={14} />,
              },
              {
                label: "Terminals",
                to: `${base}/terminals`,
                icon: <IconTerminal2 size={14} />,
              },
              {
                label: "Logs",
                to: `${base}/logs`,
                icon: <IconActivity size={14} />,
              },
              {
                label: "Config",
                to: `${base}/settings`,
                icon: <IconSettings size={14} />,
              },
            ]}
          />

          <Outlet />
        </>
      )}
    </QueryBoundary>
  );
};

export default Page;
