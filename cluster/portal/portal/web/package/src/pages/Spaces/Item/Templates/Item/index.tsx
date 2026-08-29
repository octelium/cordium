import BuildTemplate from "@/components/BuildTemplate";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import QueryBoundary from "@/components/QueryBoundary";
import TabNav from "@/components/TabNav";
import Tag from "@/components/Tag";
import YamlDrawer from "@/components/YamlDrawer";
import { getPathSpace, getPathTemplate } from "@/utils/octelium";
import { getShortName } from "@/utils/pb";
import {
  IconHammer,
  IconLayoutGrid,
  IconSettings,
  IconTerminal2,
} from "@tabler/icons-react";
import { Outlet } from "react-router-dom";
import { useContextSpace } from "@/pages/Spaces/utils";

const Page = () => {
  const ctx = useContextSpace();
  const data = ctx.template.data;
  const space = ctx.space.data;

  return (
    <QueryBoundary query={[ctx.space, ctx.template]}>
      {data && space && (
        <>
          <Meta title={`${getShortName(data)} · Template`} />
          <PageHeader
            title={data.metadata?.displayName || getShortName(data)}
            crumbs={[
              { label: "Spaces", to: "/spaces" },
              { label: getShortName(space), to: getPathSpace(space) },
              { label: "Templates", to: `${getPathSpace(space)}/templates` },
              { label: getShortName(data) },
            ]}
            description={data.metadata?.description || undefined}
            badges={
              data.status?.buildInfo?.currentReadyBuildID ? (
                <Tag tone="success">Prebuilt image ready</Tag>
              ) : undefined
            }
            actions={
              <>
                <YamlDrawer item={data} />
                <BuildTemplate item={data} size="sm" />
              </>
            }
          />

          <TabNav
            items={[
              {
                label: "Overview",
                to: getPathTemplate(data),
                end: true,
                icon: <IconLayoutGrid size={14} />,
              },
              {
                label: "Workspaces",
                to: `${getPathTemplate(data)}/workspaces`,
                icon: <IconTerminal2 size={14} />,
              },
              {
                label: "Builds",
                to: `${getPathTemplate(data)}/builds`,
                icon: <IconHammer size={14} />,
                count: data.status?.buildInfo?.builds.length,
              },
              {
                label: "Config",
                to: `${getPathTemplate(data)}/settings`,
                icon: <IconSettings size={14} />,
              },
            ]}
          />

          <Outlet />
        </>
      )}
    </QueryBoundary>
  );
};

export default Page;
