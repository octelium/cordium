import EmptyList from "@/components/EmptyList";
import PageWrap from "@/components/PageWrap";
import Paginator from "@/components/Paginator";
import {
  ResourceListCreateItem,
  ResourceListWrapper,
} from "@/components/ResourceList";
import ItemSpace from "@/components/ResourceList/ItemSpace";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { ListSpaceOptions, SpaceList } from "@octelium/apis/main/cordiumv1";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { Navigate } from "react-router-dom";

const SpaceListC = (props: { itemsList: SpaceList }) => {
  if (!props.itemsList) return <Navigate to="/" />;

  return (
    <ResourceListWrapper>
      {props.itemsList.items.length === 0 && (
        <EmptyList title="No Spaces found" />
      )}
      {props.itemsList.items.map((x) => (
        <ItemSpace key={x.metadata?.uid} item={x} />
      ))}
    </ResourceListWrapper>
  );
};

const Page = () => {
  const settings = useAppSelector((state) => state.settings);
  const [page, setPage] = React.useState(0);

  const qry = useQuery({
    queryKey: ["workspace/listSpace", page],
    queryFn: () => {
      const { response } = getClientWorkspace().listSpace(
        ListSpaceOptions.create({
          common: { page, itemsPerPage: settings.itemsPerPage },
        }),
      );
      return response;
    },
  });

  return (
    <PageWrap qry={qry} title="Spaces">
      {qry.data && (
        <div className="w-full flex flex-col gap-4">
          <ResourceListCreateItem title="Create a Space" />
          <SpaceListC itemsList={qry.data} />
          <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
