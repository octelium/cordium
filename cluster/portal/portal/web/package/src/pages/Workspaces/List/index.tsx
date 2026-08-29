import Empty from "@/components/Empty";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList } from "@/components/ResourceCards";
import WorkspaceRow from "@/components/WorkspaceRow";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Button, Select, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconPlus, IconTerminal2 } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

const Page = () => {
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const navigate = useNavigate();
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
      "workspace/listWorkspace",
      selectedSpace?.metadata?.uid ?? "all",
      page,
      itemsPerPage,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: selectedSpace
            ? { oneofKind: "spaceRef", spaceRef: getResourceRef(selectedSpace) }
            : { oneofKind: undefined },
          common: { page, itemsPerPage },
        }),
      );
      return response;
    },
    enabled: !spaceName || !!selectedSpace,
  });

  return (
    <>
      <Meta title="Workspaces" />
      <PageHeader
        title="Workspaces"
        description="Your running and stopped sandboxes across every Space."
        actions={
          <>
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
            <Button
              leftSection={<IconPlus size={15} />}
              onClick={() => navigate("/workspaces/create")}
            >
              New workspace
            </Button>
          </>
        }
      />

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconTerminal2 size={22} />}
                title={
                  spaceName
                    ? "No workspaces in this Space"
                    : "No workspaces yet"
                }
                description="Create one from a Template to get a sandbox with a terminal."
                action={
                  <Button
                    leftSection={<IconPlus size={15} />}
                    onClick={() => navigate("/workspaces/create")}
                  >
                    New workspace
                  </Button>
                }
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <WorkspaceRow
                    key={x.metadata?.uid}
                    item={x}
                    showSpace={!selectedSpace}
                    showTemplate
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
