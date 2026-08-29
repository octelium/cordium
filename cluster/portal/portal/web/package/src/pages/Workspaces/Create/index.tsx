import LaunchWorkspace from "@/components/LaunchWorkspace";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import QueryBoundary from "@/components/QueryBoundary";
import { getClientWorkspace } from "@/utils/client";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Select, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { GetOptions } from "@octelium/apis/main/metav1";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useSearchParams } from "react-router-dom";

const Page = () => {
  const [searchParams] = useSearchParams();
  const templateName = searchParams.get("template");
  const spaceParam = searchParams.get("space");

  const [spaceName, setSpaceName] = React.useState<string | null>(spaceParam);

  const qrySpaces = useQuery({
    queryKey: ["workspace/listSpace", "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listSpace(
        WsPB.ListSpaceOptions.create({ common: { itemsPerPage: 500 } }),
      );
      return response;
    },
  });

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", templateName],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ name: templateName! }),
      );
      return response;
    },
    enabled: !!templateName,
  });

  const spaces = qrySpaces.data?.items ?? [];
  const resolvedSpaceName =
    qryTemplate.data?.status?.spaceRef?.name ??
    spaceName ??
    spaces.find((x) => x.metadata!.name.startsWith("default."))?.metadata
      ?.name ??
    spaces.at(0)?.metadata?.name ??
    null;

  const space = spaces.find((x) => x.metadata!.name === resolvedSpaceName);

  return (
    <>
      <Meta title="New workspace" />
      <PageHeader
        title="New workspace"
        crumbs={[
          { label: "Workspaces", to: "/workspaces" },
          { label: "New" },
        ]}
        description="Workspaces are always created from a Template, which supplies the image, repository and runtime defaults."
        actions={
          !templateName && spaces.length > 1 ? (
            <Select
              w={240}
              label="Space"
              placeholder="Select a Space"
              searchable
              allowDeselect={false}
              data={spaces.map((x) => ({
                value: x.metadata!.name,
                label: x.metadata!.displayName || getShortName(x),
              }))}
              value={resolvedSpaceName}
              onChange={setSpaceName}
            />
          ) : undefined
        }
      />

      <QueryBoundary query={qrySpaces}>
        <Stack gap="lg">
          {space && (
            <LaunchWorkspace
              key={space.metadata!.uid}
              spaceRef={getResourceRef(space)}
              templateRef={
                qryTemplate.data ? getResourceRef(qryTemplate.data) : undefined
              }
            />
          )}
        </Stack>
      </QueryBoundary>
    </>
  );
};

export default Page;
