import Empty from "@/components/Empty";
import LaunchWorkspace from "@/components/LaunchWorkspace";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList } from "@/components/ResourceCards";
import WorkspaceRow from "@/components/WorkspaceRow";
import { useContextSpace } from "@/pages/Spaces/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconTerminal2 } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";

const Page = () => {
  const ctx = useContextSpace();
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);

  const template = ctx.template.data;
  const space = ctx.space.data;

  const qry = useQuery({
    queryKey: [
      "workspace/listWorkspace",
      template?.metadata?.uid,
      page,
      itemsPerPage,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: {
            oneofKind: "templateRef",
            templateRef: getResourceRef(template!),
          },
          common: { page, itemsPerPage },
        }),
      );
      return response;
    },
    enabled: !!template,
  });

  if (!template || !space) return null;

  return (
    <Stack gap="lg">
      <div>
        <Text size="sm" fw={700}>
          Your workspaces from {getShortName(template)}
        </Text>
        <Text size="xs" c="dimmed">
          Only Workspaces you own are listed.
        </Text>
      </div>

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconTerminal2 size={22} />}
                title="No workspaces from this Template"
                description="Launch one with the form below."
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <WorkspaceRow key={x.metadata?.uid} item={x} />
                ))}
              </CardList>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>

      <LaunchWorkspace
        spaceRef={getResourceRef(space)}
        templateRef={getResourceRef(template)}
      />
    </Stack>
  );
};

export default Page;
