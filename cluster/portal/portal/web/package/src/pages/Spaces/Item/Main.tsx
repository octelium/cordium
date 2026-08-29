import Empty from "@/components/Empty";
import Facts, { Fact } from "@/components/Facts";
import LaunchWorkspace from "@/components/LaunchWorkspace";
import Panel, { PanelBody, PanelHeader } from "@/components/Panel";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList } from "@/components/ResourceCards";
import StatTile from "@/components/StatTile";
import TimeAgo from "@/components/TimeAgo";
import WorkspaceRow from "@/components/WorkspaceRow";
import { getClientWorkspace } from "@/utils/client";
import { formatMegabytes, formatMillicores } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef, getShortName, isOrgSpace } from "@/utils/pb";
import { Anchor, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import {
  IconKey,
  IconTemplate,
  IconTerminal2,
  IconUsers,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useContextSpace, useSpaceCounts } from "../utils";

const RecentWorkspaces = (props: { space: WsPB.Space }) => {
  const qry = useQuery({
    queryKey: ["workspace/listWorkspace", props.space.metadata?.uid, "recent"],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: { oneofKind: "spaceRef", spaceRef: getResourceRef(props.space) },
          common: { page: 0, itemsPerPage: 5 },
        }),
      );
      return response;
    },
  });

  return (
    <Panel>
      <PanelHeader
        icon={<IconTerminal2 size={16} />}
        title="Recent workspaces"
        actions={
          <Anchor
            component={Link}
            to={`${getPathSpace(props.space)}/workspaces`}
            size="xs"
            fw={600}
          >
            View all
          </Anchor>
        }
      />
      <PanelBody className="p-3">
        <QueryBoundary query={qry} minHeight={120}>
          {qry.data &&
            (qry.data.items.length === 0 ? (
              <Empty
                compact
                title="No workspaces yet"
                description="Launch one below to get started."
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <WorkspaceRow key={x.metadata?.uid} item={x} showTemplate />
                ))}
              </CardList>
            ))}
        </QueryBoundary>
      </PanelBody>
    </Panel>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const data = ctx.space.data;
  const counts = useSpaceCounts(data ? getResourceRef(data) : undefined);

  if (!data) return null;

  const base = getPathSpace(data);
  const limit = data.spec?.limit;

  return (
    <Stack gap="lg">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatTile
          label="Workspaces"
          value={counts.workspaces.data?.listResponseMeta?.totalCount ?? "—"}
          icon={<IconTerminal2 size={16} />}
          to={`${base}/workspaces`}
        />
        <StatTile
          label="Templates"
          value={counts.templates.data?.listResponseMeta?.totalCount ?? "—"}
          icon={<IconTemplate size={16} />}
          to={`${base}/templates`}
        />
        <StatTile
          label="Secrets"
          value={counts.secrets.data?.listResponseMeta?.totalCount ?? "—"}
          icon={<IconKey size={16} />}
          to={`${base}/secrets`}
        />
        <StatTile
          label="Members"
          value={counts.members.data?.listResponseMeta?.totalCount ?? "—"}
          icon={<IconUsers size={16} />}
          to={isOrgSpace(data) ? `${base}/memberships` : undefined}
          hint={isOrgSpace(data) ? undefined : "Personal Space"}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
        <RecentWorkspaces space={data} />

        <Panel>
          <PanelHeader title="Details" />
          <PanelBody className="px-5 py-1">
            <Facts>
              <Fact label="Name">
                <span className="font-mono">{getShortName(data)}</span>
              </Fact>
              <Fact label="Full name">
                <span className="font-mono text-[0.82em]">
                  {data.metadata?.name}
                </span>
              </Fact>
              <Fact label="Type">
                {isOrgSpace(data) ? "Organization" : "Personal"}
              </Fact>
              <Fact label="Created">
                <TimeAgo rfc3339={data.metadata?.createdAt} />
              </Fact>
              {data.spec?.authorization?.disableSSH && (
                <Fact label="SSH">Disabled for this Space</Fact>
              )}
              {limit?.defaultLimit && (
                <Fact label="Default limits">
                  {[
                    limit.defaultLimit.cpu?.millicores
                      ? formatMillicores(limit.defaultLimit.cpu.millicores)
                      : null,
                    limit.defaultLimit.memory?.megabytes
                      ? formatMegabytes(limit.defaultLimit.memory.megabytes)
                      : null,
                    limit.defaultLimit.storage?.megabytes
                      ? `${formatMegabytes(limit.defaultLimit.storage.megabytes)} disk`
                      : null,
                  ]
                    .filter(Boolean)
                    .join(" · ") || "—"}
                </Fact>
              )}
            </Facts>
          </PanelBody>
        </Panel>
      </div>

      <LaunchWorkspace spaceRef={getResourceRef(data)} />
    </Stack>
  );
};

export default Page;
