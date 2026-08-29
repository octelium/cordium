import Empty from "@/components/Empty";
import LaunchWorkspace from "@/components/LaunchWorkspace";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList } from "@/components/ResourceCards";
import WorkspaceRow from "@/components/WorkspaceRow";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconTerminal2 } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useContextSpace } from "../utils";

const Page = () => {
  const ctx = useContextSpace();
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);
  const space = ctx.space.data;

  const qry = useQuery({
    queryKey: ["workspace/listWorkspace", space?.metadata?.uid, page, itemsPerPage],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: { oneofKind: "spaceRef", spaceRef: getResourceRef(space!) },
          common: { page, itemsPerPage },
        }),
      );
      return response;
    },
    enabled: !!space,
  });

  if (!space) return null;

  return (
    <Stack gap="lg">
      <div>
        <Text size="sm" fw={700}>
          Workspaces in {getShortName(space)}
        </Text>
        <Text size="xs" c="dimmed">
          Every Workspace here is created from a Template in this Space and
          inherits its Secrets and limits.
        </Text>
      </div>

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconTerminal2 size={22} />}
                title="No workspaces in this Space"
                description="Launch one with the form below."
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <WorkspaceRow key={x.metadata?.uid} item={x} showTemplate />
                ))}
              </CardList>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>

      <LaunchWorkspace spaceRef={getResourceRef(space)} />
    </Stack>
  );
};

export default Page;
