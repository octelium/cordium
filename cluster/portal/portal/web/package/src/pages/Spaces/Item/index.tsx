import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import QueryBoundary from "@/components/QueryBoundary";
import TabNav, { TabItem } from "@/components/TabNav";
import Tag from "@/components/Tag";
import YamlDrawer from "@/components/YamlDrawer";
import { getPathSpace } from "@/utils/octelium";
import {
  getResourceRef,
  getRoleLabel,
  getShortName,
  isOrgSpace,
} from "@/utils/pb";
import { Button } from "@mantine/core";
import {
  IconBuilding,
  IconGitBranch,
  IconKey,
  IconLayoutGrid,
  IconPlus,
  IconSettings,
  IconTemplate,
  IconTerminal2,
  IconUser,
  IconUsers,
} from "@tabler/icons-react";
import { Outlet, useNavigate } from "react-router-dom";
import { useContextSpace, useSpaceCounts } from "../utils";

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const data = ctx.space.data;
  const counts = useSpaceCounts(data ? getResourceRef(data) : undefined);

  return (
    <QueryBoundary query={ctx.space}>
      {data && (
        <>
          <Meta title={`${getShortName(data)} · Space`} />
          <PageHeader
            title={data.metadata?.displayName || getShortName(data)}
            crumbs={[
              { label: "Spaces", to: "/spaces" },
              { label: getShortName(data) },
            ]}
            description={data.metadata?.description || undefined}
            badges={
              <>
                <Tag
                  tone={isOrgSpace(data) ? "info" : "neutral"}
                  icon={
                    isOrgSpace(data) ? (
                      <IconBuilding size={11} />
                    ) : (
                      <IconUser size={11} />
                    )
                  }
                >
                  {isOrgSpace(data) ? "Organization" : "Personal"}
                </Tag>
                {ctx.membership.isSuccess && (
                  <Tag tone="neutral">
                    {getRoleLabel(ctx.membership.data.spec!.role)}
                  </Tag>
                )}
              </>
            }
            actions={
              <>
                <YamlDrawer item={data} />
                <Button
                  leftSection={<IconPlus size={15} />}
                  onClick={() => navigate(`${getPathSpace(data)}/workspaces`)}
                >
                  New workspace
                </Button>
              </>
            }
          />

          <TabNav
            items={
              [
                {
                  label: "Overview",
                  to: getPathSpace(data),
                  end: true,
                  icon: <IconLayoutGrid size={14} />,
                },
                {
                  label: "Workspaces",
                  to: `${getPathSpace(data)}/workspaces`,
                  icon: <IconTerminal2 size={14} />,
                  count: counts.workspaces.data?.listResponseMeta?.totalCount,
                },
                {
                  label: "Templates",
                  to: `${getPathSpace(data)}/templates`,
                  icon: <IconTemplate size={14} />,
                  count: counts.templates.data?.listResponseMeta?.totalCount,
                },
                {
                  label: "Secrets",
                  to: `${getPathSpace(data)}/secrets`,
                  icon: <IconKey size={14} />,
                  count: counts.secrets.data?.listResponseMeta?.totalCount,
                },
                {
                  label: "Git providers",
                  to: `${getPathSpace(data)}/gitproviders`,
                  icon: <IconGitBranch size={14} />,
                },
                {
                  label: "Members",
                  to: `${getPathSpace(data)}/memberships`,
                  icon: <IconUsers size={14} />,
                  count: counts.members.data?.listResponseMeta?.totalCount,
                  hidden: !isOrgSpace(data),
                },
                {
                  label: "Settings",
                  to: `${getPathSpace(data)}/settings`,
                  icon: <IconSettings size={14} />,
                },
              ] satisfies TabItem[]
            }
          />

          <Outlet />
        </>
      )}
    </QueryBoundary>
  );
};

export default Page;
