import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import {
  ListSecretOptions,
  Secret,
  SecretList,
} from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";
import * as React from "react";

import * as MetaPB from "@/apis/metav1/metav1";
import DeleteResource from "@/components/DeleteResource";
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
import { onError } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef, isMemberAdmin } from "@/utils/pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

const Item = (props: { item: Secret }) => {
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

const SecretListC = (props: {
  itemsList: SecretList;
  space: WsPB.Space;
  isAdmin?: boolean;
}) => {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const client = getClientWorkspace();

  const mutationDelete = useMutation({
    mutationFn: async (spaceRef: MetaPB.ObjectReference) => {
      const { response } = await client.deleteSecret(
        MetaPB.DeleteOptions.create({
          uid: spaceRef.uid,
          name: spaceRef.name,
        }),
      );

      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: [
          "workspace/listSecret",
          props.space?.metadata?.uid,
          props.itemsList.listResponseMeta?.page,
        ],
      });
    },
    onError: onError,
  });

  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.length === 0 && (
          <EmptyList title="No Secrets found"></EmptyList>
        )}
        {props.itemsList.items.map((item) => (
          <ResourceListItem key={item.metadata!.uid}>
            <Item item={item} />

            {props.isAdmin && (
              <div className="flex justify-end">
                <DeleteResource
                  btnSize="xs"
                  onDelete={() => {
                    mutationDelete.mutate(getResourceRef(item));
                  }}
                />
              </div>
            )}
          </ResourceListItem>
        ))}
      </ResourceListWrapper>
    </div>
  );
};

export const ListSecret = (props: { item: WsPB.Space }) => {
  const { item } = props;
  let [page, setPage] = React.useState(0);
  const client = getClientWorkspace();

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/listSecret", item?.metadata?.uid, page],
    queryFn: () => {
      const { response } = client.listSecret(
        ListSecretOptions.create({
          spaceRef: getResourceRef(item),
        }),
      );
      return response;
    },
  });

  const ctx = useContextSpace();

  if (!isSuccess || !data || !ctx.membership.isSuccess) {
    return <></>;
  }

  const isAdmin = isMemberAdmin(ctx.membership.data);

  return (
    <>
      <Meta title="Secrets" />
      <ResourceListCreateItem
        title="Create a Secret"
        path={`${getPathSpace(item)}/secrets/create`}
      />
      <SecretListC itemsList={data} space={item} isAdmin={isAdmin} />

      <div className="mt-4">
        <Paginator
          meta={data.listResponseMeta!}
          onPageChange={(i) => {
            setPage(i);
          }}
        />
      </div>
    </>
  );
};

const Page = () => {
  const ctx = useContextSpace();

  return (
    <PageWrap qry={ctx.space}>
      {ctx.space.data && <ListSecret item={ctx.space.data} />}
    </PageWrap>
  );
};

export default Page;
