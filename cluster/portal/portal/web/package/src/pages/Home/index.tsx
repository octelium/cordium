import Empty from "@/components/Empty";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelHeader } from "@/components/Panel";
import QueryBoundary from "@/components/QueryBoundary";
import {
  CardList,
  CardTitle,
  ClickableCard,
} from "@/components/ResourceCards";
import StatTile from "@/components/StatTile";
import Tag from "@/components/Tag";
import WorkspaceRow from "@/components/WorkspaceRow";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace } from "@/utils/octelium";
import { getShortName, isOrgSpace } from "@/utils/pb";
import { Anchor, Button, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import {
  IconBuilding,
  IconPlayerPlay,
  IconPlus,
  IconStack2,
  IconTerminal2,
  IconUser,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";

const RUNNING = WsPB.Workspace_Status_State.RUNNING;

const Page = () => {
  const navigate = useNavigate();
  const user = useAppSelector((s) => s.settings.status?.user);

  const qryWorkspaces = useQuery({
    queryKey: ["workspace/listWorkspace", "overview"],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          common: { page: 0, itemsPerPage: 100 },
        }),
      );
      return response;
    },
  });

  const qrySpaces = useQuery({
    queryKey: ["workspace/listSpace", "overview"],
    queryFn: () => {
      const { response } = getClientWorkspace().listSpace(
        WsPB.ListSpaceOptions.create({ common: { page: 0, itemsPerPage: 6 } }),
      );
      return response;
    },
  });

  const workspaces = qryWorkspaces.data?.items ?? [];
  const running = workspaces.filter((x) => x.status?.state === RUNNING);
  const total = qryWorkspaces.data?.listResponseMeta?.totalCount ?? 0;
  const spaces = qrySpaces.data?.items ?? [];

  const greeting =
    user?.metadata?.displayName?.split(" ").at(0) ??
    user?.metadata?.name?.split(".").at(0) ??
    "";

  return (
    <>
      <Meta title="Overview" />
      <PageHeader
        title={greeting ? `Welcome back, ${greeting}` : "Overview"}
        description="Your sandboxes and the Spaces they belong to."
        actions={
          <Button
            leftSection={<IconPlus size={15} />}
            onClick={() => navigate("/workspaces/create")}
          >
            New workspace
          </Button>
        }
      />

      <Stack gap="lg">
        <div className="grid gap-3 sm:grid-cols-3">
          <StatTile
            label="Running"
            value={running.length}
            hint={running.length === 1 ? "workspace" : "workspaces"}
            icon={<IconPlayerPlay size={16} />}
            to="/workspaces"
          />
          <StatTile
            label="Workspaces"
            value={total}
            hint="across all Spaces"
            icon={<IconTerminal2 size={16} />}
            to="/workspaces"
          />
          <StatTile
            label="Spaces"
            value={qrySpaces.data?.listResponseMeta?.totalCount ?? "—"}
            icon={<IconStack2 size={16} />}
            to="/spaces"
          />
        </div>

        <div className="grid gap-4 lg:grid-cols-[1fr_22rem]">
          <Panel>
            <PanelHeader
              icon={<IconTerminal2 size={16} />}
              title={running.length > 0 ? "Running now" : "Recent workspaces"}
              actions={
                <Anchor component={Link} to="/workspaces" size="xs" fw={600}>
                  View all
                </Anchor>
              }
            />
            <PanelBody className="p-3">
              <QueryBoundary query={qryWorkspaces} minHeight={140}>
                {(running.length > 0 ? running : workspaces).length === 0 ? (
                  <Empty
                    compact
                    icon={<IconTerminal2 size={22} />}
                    title="No workspaces yet"
                    description="Create one to get a sandbox with a terminal in seconds."
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
                    {(running.length > 0 ? running : workspaces)
                      .slice(0, 6)
                      .map((x) => (
                        <WorkspaceRow key={x.metadata?.uid} item={x} showSpace />
                      ))}
                  </CardList>
                )}
              </QueryBoundary>
            </PanelBody>
          </Panel>

          <Panel>
            <PanelHeader
              icon={<IconStack2 size={16} />}
              title="Your Spaces"
              actions={
                <Anchor component={Link} to="/spaces" size="xs" fw={600}>
                  View all
                </Anchor>
              }
            />
            <PanelBody className="p-3">
              <QueryBoundary query={qrySpaces} minHeight={140}>
                {spaces.length === 0 ? (
                  <Empty compact title="No Spaces yet" />
                ) : (
                  <CardList>
                    {spaces.map((x) => (
                      <ClickableCard key={x.metadata?.uid} to={getPathSpace(x)}>
                        <div className="flex items-center gap-3">
                          <div className="min-w-0 flex-1">
                            <CardTitle
                              name={getShortName(x)}
                              displayName={x.metadata?.displayName}
                            />
                          </div>
                          <Tag
                            tone={isOrgSpace(x) ? "info" : "neutral"}
                            icon={
                              isOrgSpace(x) ? (
                                <IconBuilding size={11} />
                              ) : (
                                <IconUser size={11} />
                              )
                            }
                          >
                            {isOrgSpace(x) ? "Org" : "Personal"}
                          </Tag>
                        </div>
                      </ClickableCard>
                    ))}
                  </CardList>
                )}
              </QueryBoundary>
            </PanelBody>
          </Panel>
        </div>
      </Stack>
    </>
  );
};

export default Page;
