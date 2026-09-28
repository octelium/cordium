import CreateSnapshot from "@/components/CreateSnapshot";
import Empty from "@/components/Empty";
import Panel, { PanelBody, PanelHeader } from "@/components/Panel";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList } from "@/components/ResourceCards";
import SnapshotRow from "@/components/SnapshotRow";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import { Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconCamera } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useContextWorkspace } from "../utils";

const Page = () => {
  const ctx = useContextWorkspace();
  const item = ctx.workspace.data;
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);

  const qry = useQuery({
    queryKey: [
      "workspace/listWorkspaceSnapshot",
      item?.metadata?.uid,
      page,
      itemsPerPage,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspaceSnapshot(
        WsPB.ListWorkspaceSnapshotOptions.create({
          filter: { oneofKind: "workspaceRef", workspaceRef: getResourceRef(item!) },
          common: { page, itemsPerPage },
        }),
      );
      return response;
    },
    enabled: !!item,
    refetchInterval: (q) =>
      q.state.data?.items.some(
        (x) =>
          x.status?.state === WsPB.WorkspaceSnapshot_Status_State.CREATING,
      )
        ? 5000
        : false,
  });

  if (!item) return null;

  return (
    <Panel>
      <PanelHeader
        icon={<IconCamera size={16} />}
        title="Snapshots"
        description="Point-in-time copies of this workspace's persistent storage. Restore one into a brand new workspace in the same Space."
        actions={<CreateSnapshot item={item} />}
      />
      <PanelBody className="p-3">
        <QueryBoundary query={qry} minHeight={120}>
          {qry.data && (
            <Stack gap="md">
              {qry.data.items.length === 0 ? (
                <Empty
                  compact
                  icon={<IconCamera size={22} />}
                  title="No snapshots yet"
                  description="Take a snapshot to checkpoint this workspace or to clone it into a new one."
                  action={<CreateSnapshot item={item} size="sm" />}
                />
              ) : (
                <CardList>
                  {qry.data.items.map((x) => (
                    <SnapshotRow key={x.metadata?.uid} item={x} />
                  ))}
                </CardList>
              )}
              <Paginator
                meta={qry.data.listResponseMeta!}
                onPageChange={setPage}
              />
            </Stack>
          )}
        </QueryBoundary>
      </PanelBody>
    </Panel>
  );
};

export default Page;
