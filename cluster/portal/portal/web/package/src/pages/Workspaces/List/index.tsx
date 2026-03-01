import {
  ListWorkspaceOptions,
  WorkspaceList,
} from "@/apis/cordiumv1/cordiumv1";
import Meta from "@/components/Meta";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";

import { GetOptions } from "@/apis/metav1/metav1";
import EmptyList from "@/components/EmptyList";
import Paginator from "@/components/Paginator";
import { ResourceListWrapper } from "@/components/ResourceList";
import ItemWorkspace from "@/components/ResourceList/ItemWorkspace";
import { toNumOrZero } from "@/utils";
import { getResourceRef } from "@/utils/pb";

const WorkspaceListC = (props: {
  itemsList: WorkspaceList;
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

const WorkspacesAll = (props: { page?: number; itemsPerPage?: number }) => {
  const { page, itemsPerPage } = props;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/listWorkspace", page],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        ListWorkspaceOptions.create({
          common: {
            page,
            itemsPerPage,
          },
        }),
      );
      return response;
    },
  });

  if (!isSuccess) {
    return <></>;
  }

  return (
    <>
      <Meta title="Workspaces" />

      <div className="w-full">
        <WorkspaceListC
          itemsList={data}
          showSpace
          showEnvironment
          showTemplate
        />

        <div className="mt-4">
          <Paginator meta={data.listResponseMeta!} path="/workspaces" />
        </div>
      </div>
    </>
  );
};

const WorkspacesBySpace = (props: {
  uid: string;
  page?: number;
  itemsPerPage?: number;
}) => {
  const { uid, page, itemsPerPage } = props;
  const parentQuery = useQuery({
    queryKey: ["workspace/getSpace", uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getSpace(
        GetOptions.create({ uid }),
      );
      return response;
    },
  });

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/listWorkspace", uid, page],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        ListWorkspaceOptions.create({
          filter: {
            oneofKind: "spaceRef",
            spaceRef: getResourceRef(parentQuery.data!),
          },

          common: {
            page,
            itemsPerPage,
          },
        }),
      );
      return response;
    },
    enabled: parentQuery.isSuccess,
  });

  if (!isSuccess) {
    return <></>;
  }

  return (
    <>
      <Meta title="Workspaces" />

      <div className="w-full">
        <WorkspaceListC itemsList={data} showEnvironment />

        <div className="mt-4">
          <Paginator
            meta={data.listResponseMeta!}
            path={`/workspaces?spaceUID=${uid}`}
          />
        </div>
      </div>
    </>
  );
};

const WorkspacesByTemplate = (props: {
  uid: string;
  page?: number;
  itemsPerPage?: number;
}) => {
  const { uid, page } = props;
  const parentQuery = useQuery({
    queryKey: ["workspace/getTemplate", uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid }),
      );
      return response;
    },
  });

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/listWorkspace", uid, page],
    queryFn: () => {
      const { response } = getClientWorkspace().listWorkspace(
        ListWorkspaceOptions.create({
          filter: {
            oneofKind: "templateRef",
            templateRef: getResourceRef(parentQuery.data!),
          },
          common: {
            page,
          },
        }),
      );
      return response;
    },
    enabled: parentQuery.isSuccess,
  });

  if (!isSuccess) {
    return <></>;
  }

  return (
    <>
      <Meta title="Workspaces" />

      <div className="w-full">
        <WorkspaceListC itemsList={data} />
        <div className="mt-4">
          <Paginator
            meta={data.listResponseMeta!}
            path={`/workspaces?templateUID=${uid}`}
          />
        </div>
      </div>
    </>
  );
};

const Workspaces = () => {
  let [searchParams, _] = useSearchParams();

  const page = toNumOrZero(searchParams.get("page"));
  const settings = useAppSelector((state) => state.settings);

  const itemsPerPage = settings.itemsPerPage;

  return (
    <>
      {!!searchParams.get("spaceUID") && (
        <WorkspacesBySpace
          uid={searchParams.get("spaceUID")!}
          page={page}
          itemsPerPage={itemsPerPage}
        />
      )}
      {!!searchParams.get("templateUID") && (
        <WorkspacesByTemplate
          uid={searchParams.get("templateUID")!}
          page={page}
          itemsPerPage={itemsPerPage}
        />
      )}

      {!searchParams.get("spaceUID") &&
        !searchParams.get("environmentUID") &&
        !searchParams.get("templateUID") && (
          <WorkspacesAll page={page} itemsPerPage={itemsPerPage} />
        )}
    </>
  );
};

export default Workspaces;
