import * as React from "react";

import { ListSpaceOptions, SpaceList } from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";

import { ResourceListCreateItem } from "@/components/ResourceList";
import { useQuery } from "@tanstack/react-query";

import EmptyList from "@/components/EmptyList";
import PageWrap from "@/components/PageWrap";
import Paginator from "@/components/Paginator";
import { ResourceListWrapper } from "@/components/ResourceList";
import ItemSpace from "@/components/ResourceList/ItemSpace";
import { useAppSelector } from "@/utils/hooks";
import { Navigate } from "react-router-dom";

const SpaceListC = (props: { itemsList: SpaceList }) => {
  if (!props.itemsList) {
    return <Navigate to="/" />;
  }
  return (
    <div>
      <div>
        <ResourceListWrapper>
          {props.itemsList.items.length === 0 && (
            <EmptyList title="No Spaces found"></EmptyList>
          )}

          {props.itemsList.items.map((x) => (
            <ItemSpace key={x.metadata?.uid} item={x} />
          ))}
        </ResourceListWrapper>
      </div>
    </div>
  );
};

const Page = () => {
  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;

  let [page, setPage] = React.useState(0);

  const qry = useQuery({
    queryKey: ["workspace/listSpace", page],
    queryFn: () => {
      const { response } = getClientWorkspace().listSpace(
        ListSpaceOptions.create({
          common: {
            page,
            itemsPerPage,
          },
        }),
      );
      return response;
    },
  });

  return (
    <PageWrap qry={qry} title="Spaces">
      {qry.data && (
        <div className="w-full">
          <ResourceListCreateItem title="Create a Space" />

          <SpaceListC itemsList={qry.data} />

          <div className="mt-4">
            <Paginator
              meta={qry.data.listResponseMeta!}
              path="/spaces"
              onPageChange={(i) => {
                setPage(i);
              }}
            />
          </div>
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
