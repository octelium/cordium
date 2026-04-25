import {
  ListTemplateOptions,
  Template,
  TemplateList,
} from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";
import * as React from "react";

import EmptyList from "@/components/EmptyList";
import Meta from "@/components/Meta";
import PageWrap from "@/components/PageWrap";
import Paginator from "@/components/Paginator";
import {
  ResourceListCreateItem,
  ResourceListItem,
  ResourceListItemMetadata,
  ResourceListWrapper,
} from "@/components/ResourceList";
import { useContextSpace } from "@/pages/Spaces/utils";
import { useAppSelector } from "@/utils/hooks";
import { getPathTemplate } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { useQuery } from "@tanstack/react-query";

const Item = (props: { item: Template }) => {
  return (
    <div className="font-semibold w-full">
      <div className="flex items-center">
        <div className="flex flex-col flex-1">
          <ResourceListItemMetadata resource={props.item} />
        </div>
      </div>
    </div>
  );
};

const TemplateListC = (props: { itemsList: TemplateList }) => {
  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.length === 0 && (
          <EmptyList title="No Templates found"></EmptyList>
        )}
        {props.itemsList.items.map((item) => (
          <ResourceListItem
            key={item.metadata!.uid}
            path={getPathTemplate(item)}
          >
            <Item item={item} />
          </ResourceListItem>
        ))}
      </ResourceListWrapper>
    </div>
  );
};

const Page = () => {
  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;

  let [page, setPage] = React.useState(0);

  const ctx = useContextSpace();

  if (!ctx.space.isSuccess) {
    return <></>;
  }

  const qry = useQuery({
    queryKey: ["workspace/listTemplate", ctx.space.data.metadata?.uid, page],
    queryFn: () => {
      const { response } = getClientWorkspace().listTemplate(
        ListTemplateOptions.create({
          spaceRef: getResourceRef(ctx.space.data!),
        }),
      );
      return response;
    },
    enabled: ctx.space.isSuccess,
  });

  return (
    <PageWrap qry={qry} title="Templates">
      <Meta title="Space Templates" />
      {qry.data && (
        <div>
          <ResourceListCreateItem title="Create a Template" path={`./create`} />
          <TemplateListC itemsList={qry.data} />

          <div className="mt-4">
            <Paginator meta={qry.data.listResponseMeta!} path="/spaces" />
          </div>
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
