import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import EmptyList from "../EmptyList";
import Paginator from "../Paginator";
import { ResourceListWrapper } from "../ResourceList";
import ItemWorkspace from "../ResourceList/ItemWorkspace";

const WorkspaceListC = (props: {
  itemsList: WsPB.WorkspaceList;
  showEnvironment?: boolean;
  showTemplate?: boolean;
  showSpace?: boolean;
}) => {
  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.length === 0 && (
          <EmptyList title="No Workspaces found"></EmptyList>
        )}
        {props.itemsList.items.map((item) => (
          <ItemWorkspace
            key={item.metadata?.uid}
            item={item}
            showSpace={props.showSpace}
            showEnvironment={props.showEnvironment}
            showTemplate={props.showTemplate}
          />
        ))}
      </ResourceListWrapper>
    </div>
  );
};

export const ListWorkspaceTemplate = (props: { item: WsPB.Template }) => {
  const { item } = props;

  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;
  let [page, setPage] = React.useState(0);
  const client = getClientWorkspace();

  const qryWorkspace = useQuery({
    queryKey: ["workspace/listWorkspace", item.metadata?.uid],
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

  if (!qryWorkspace.isSuccess) {
    return <></>;
  }

  return (
    <>
      <WorkspaceListC itemsList={qryWorkspace.data} showTemplate />

      <div className="mt-4">
        <Paginator
          meta={qryWorkspace.data.listResponseMeta!}
          onPageChange={(val) => {
            setPage(val);
          }}
        />
      </div>
    </>
  );
};

export const ListWorkspaceSpace = (props: { item: WsPB.Space }) => {
  const { item } = props;

  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;
  let [page, setPage] = React.useState(0);
  const client = getClientWorkspace();

  const qryWorkspace = useQuery({
    queryKey: ["workspace/listWorkspace", item.metadata?.uid],
    queryFn: () => {
      const { response } = client.listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: {
            oneofKind: "spaceRef",
            spaceRef: getResourceRef(item),
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

  if (!qryWorkspace.isSuccess) {
    return <></>;
  }

  return (
    <>
      <WorkspaceListC itemsList={qryWorkspace.data} showTemplate />

      <div className="mt-4">
        <Paginator
          meta={qryWorkspace.data.listResponseMeta!}
          onPageChange={(val) => {
            setPage(val);
          }}
        />
      </div>
    </>
  );
};
