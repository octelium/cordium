import Empty from "@/components/Empty";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList } from "@/components/ResourceCards";
import SnapshotRow from "@/components/SnapshotRow";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Select, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconCamera } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useSearchParams } from "react-router-dom";

const Page = () => {
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = React.useState(0);

  const spaceName = searchParams.get("space") ?? "";

  const qrySpaces = useQuery({
    queryKey: ["workspace/listSpace", "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listSpace(
        WsPB.ListSpaceOptions.create({ common: { itemsPerPage: 500 } }),
      );
      return response;
    },
  });

  const selectedSpace = qrySpaces.data?.items.find(
    (x) => x.metadata!.name === spaceName,
  );

  const qry = useQuery({
    queryKey: [
      "workspace/listWorkspaceSnapshot",
      selectedSpace?.metadata?.uid ?? "all",
      page,
      itemsPerPage,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspaceSnapshot(
        WsPB.ListWorkspaceSnapshotOptions.create({
          filter: selectedSpace
            ? { oneofKind: "spaceRef", spaceRef: getResourceRef(selectedSpace) }
            : { oneofKind: undefined },
          common: { page, itemsPerPage },
        }),
      );
      return response;
    },
    enabled: !spaceName || !!selectedSpace,
    refetchInterval: (q) =>
      q.state.data?.items.some(
        (x) =>
          x.status?.state === WsPB.WorkspaceSnapshot_Status_State.CREATING,
      )
        ? 5000
        : false,
  });

  return (
    <>
      <Meta title="Snapshots" />
      <PageHeader
        title="Snapshots"
        description="Point-in-time copies of your workspaces' persistent storage. Restore one to clone a workspace inside its Space."
        actions={
          <Select
            size="sm"
            w={220}
            placeholder="All Spaces"
            clearable
            searchable
            aria-label="Filter by Space"
            data={(qrySpaces.data?.items ?? []).map((x) => ({
              value: x.metadata!.name,
              label: x.metadata!.displayName || getShortName(x),
            }))}
            value={spaceName || null}
            onChange={(val) => {
              setPage(0);
              setSearchParams(val ? { space: val } : {});
            }}
          />
        }
      />

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconCamera size={22} />}
                title={
                  spaceName ? "No snapshots in this Space" : "No snapshots yet"
                }
                description="Take a snapshot from the Snapshots tab of any workspace you own."
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <SnapshotRow
                    key={x.metadata?.uid}
                    item={x}
                    showWorkspace
                    showSpace={!selectedSpace}
                  />
                ))}
              </CardList>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>
    </>
  );
};

export default Page;
