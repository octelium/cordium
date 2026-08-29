import Empty from "@/components/Empty";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import {
  CardGrid,
  CardTitle,
  ClickableCard,
} from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace } from "@/utils/octelium";
import { getShortName, isOrgSpace } from "@/utils/pb";
import { Button, Stack } from "@mantine/core";
import { ListSpaceOptions, Space } from "@octelium/apis/main/cordiumv1";
import {
  IconBuilding,
  IconPlus,
  IconStack2,
  IconUser,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useNavigate } from "react-router-dom";

const SpaceCard = (props: { item: Space }) => {
  const { item } = props;
  const org = isOrgSpace(item);

  return (
    <ClickableCard to={getPathSpace(item)}>
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
          <IconStack2 size={18} />
        </span>
        <div className="min-w-0 flex-1">
          <CardTitle
            name={getShortName(item)}
            displayName={item.metadata?.displayName}
            meta={
              <>
                Created <TimeAgo rfc3339={item.metadata?.createdAt} />
              </>
            }
          />
          <div className="mt-2 flex flex-wrap gap-1.5">
            <Tag
              tone={org ? "info" : "neutral"}
              icon={
                org ? <IconBuilding size={11} /> : <IconUser size={11} />
              }
            >
              {org ? "Organization" : "Personal"}
            </Tag>
          </div>
        </div>
      </div>
    </ClickableCard>
  );
};

const Page = () => {
  const itemsPerPage = useAppSelector((state) => state.settings.itemsPerPage);
  const navigate = useNavigate();
  const [page, setPage] = React.useState(0);

  const qry = useQuery({
    queryKey: ["workspace/listSpace", page, itemsPerPage],
    queryFn: () => {
      const { response } = getClientWorkspace().listSpace(
        ListSpaceOptions.create({ common: { page, itemsPerPage } }),
      );
      return response;
    },
  });

  return (
    <>
      <Meta title="Spaces" />
      <PageHeader
        title="Spaces"
        description="A Space groups the Templates, Secrets and Git providers that its Workspaces are built from."
        actions={
          <Button
            leftSection={<IconPlus size={15} />}
            onClick={() => navigate("create")}
          >
            New Space
          </Button>
        }
      />

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconStack2 size={22} />}
                title="No Spaces yet"
                description="Create a Space to organise your Templates, Secrets and Workspaces."
                action={
                  <Button
                    leftSection={<IconPlus size={15} />}
                    onClick={() => navigate("create")}
                  >
                    New Space
                  </Button>
                }
              />
            ) : (
              <CardGrid columns={3}>
                {qry.data.items.map((x) => (
                  <SpaceCard key={x.metadata?.uid} item={x} />
                ))}
              </CardGrid>
            )}

            <Paginator
              meta={qry.data.listResponseMeta!}
              onPageChange={setPage}
            />
          </Stack>
        )}
      </QueryBoundary>
    </>
  );
};

export default Page;
