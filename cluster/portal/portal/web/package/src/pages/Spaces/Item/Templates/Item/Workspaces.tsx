import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import { useContextSpace } from "@/pages/Spaces/utils";
import { useAppSelector } from "@/utils/hooks";
import { useQuery } from "@tanstack/react-query";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import Meta from "@/components/Meta";
import PageWrap from "@/components/PageWrap";
import Paginator from "@/components/Paginator";
import WorkspaceListC from "@/components/ScopeResourceList/WorkspaceList";
import { getResourceRef } from "@/utils/pb";

const ListWorkspace = (props: { item: WsPB.Template }) => {
  const { item } = props;

  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;
  let [page, setPage] = React.useState(0);
  const client = getClientWorkspace();

  const qry = useQuery({
    queryKey: ["workspace/listWorkspace", item.metadata?.uid, page],
    queryFn: () => {
      const { response } = client.listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: {
            oneofKind: "templateRef",
            templateRef: getResourceRef(item),
          },
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
    <PageWrap qry={qry}>
      <Meta title="Template Workspaces" />
      {qry.data && (
        <div>
          <WorkspaceListC itemList={qry.data} />

          <div className="mt-4">
            <Paginator
              meta={qry.data.listResponseMeta!}
              onPageChange={(val) => {
                setPage(val);
              }}
            />
          </div>
        </div>
      )}
    </PageWrap>
  );
};

const Page = () => {
  const ctx = useContextSpace();

  return (
    <PageWrap qry={ctx.template}>
      {ctx.template.data && <ListWorkspace item={ctx.template.data} />}
    </PageWrap>
  );
};

export default Page;
