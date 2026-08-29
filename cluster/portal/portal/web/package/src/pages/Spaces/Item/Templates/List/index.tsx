import Empty from "@/components/Empty";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import {
  CardGrid,
  CardTitle,
  ClickableCard,
} from "@/components/ResourceCards";
import RepoLink from "@/components/RepoLink";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, getPathTemplate } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Button, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import {
  IconCircleCheck,
  IconPlus,
  IconTemplate,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useNavigate } from "react-router-dom";
import { useContextSpace } from "@/pages/Spaces/utils";

const TemplateCard = (props: { item: WsPB.Template }) => {
  const { item } = props;
  const readyBuild = item.status?.buildInfo?.currentReadyBuildID;

  return (
    <ClickableCard to={getPathTemplate(item)}>
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
          <IconTemplate size={18} />
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
          {item.spec?.repository?.url && (
            <div className="mt-1.5">
              <RepoLink item={item} />
            </div>
          )}
          {readyBuild && (
            <div className="mt-2">
              <Tag tone="success" icon={<IconCircleCheck size={11} />}>
                Prebuilt image ready
              </Tag>
            </div>
          )}
        </div>
      </div>
    </ClickableCard>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);
  const space = ctx.space.data;

  const qry = useQuery({
    queryKey: ["workspace/listTemplate", space?.metadata?.uid, page, itemsPerPage],
    queryFn: () => {
      const { response } = getClientWorkspace().listTemplate(
        WsPB.ListTemplateOptions.create({
          spaceRef: getResourceRef(space!),
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
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <Text size="sm" fw={700}>
            Templates in {getShortName(space)}
          </Text>
          <Text size="xs" c="dimmed">
            A Template is the reusable blueprint — image, repository and runtime
            — that Workspaces in this Space are created from.
          </Text>
        </div>
        <Button
          size="xs"
          leftSection={<IconPlus size={14} />}
          onClick={() => navigate(`${getPathSpace(space)}/templates/create`)}
        >
          New Template
        </Button>
      </div>

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconTemplate size={22} />}
                title="No Templates in this Space"
                description="Every Space starts with a `default` Template. Create more to capture different stacks."
                action={
                  <Button
                    leftSection={<IconPlus size={15} />}
                    onClick={() =>
                      navigate(`${getPathSpace(space)}/templates/create`)
                    }
                  >
                    New Template
                  </Button>
                }
              />
            ) : (
              <CardGrid columns={2}>
                {qry.data.items.map((x) => (
                  <TemplateCard key={x.metadata?.uid} item={x} />
                ))}
              </CardGrid>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>
    </Stack>
  );
};

export default Page;
