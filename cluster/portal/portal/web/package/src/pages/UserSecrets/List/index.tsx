import { getClientWorkspace } from "@/utils/client";
import {
  ListUserSecretOptions,
  UserSecret,
  UserSecretList,
  UserSecret_Spec_Type,
} from "@octelium/apis/main/cordiumv1";

import CopyText from "@/components/CopyText";
import DeleteResource from "@/components/DeleteResource";
import EmptyList from "@/components/EmptyList";
import InfoItem from "@/components/InfoItem";
import Label from "@/components/Label";
import PageWrap from "@/components/PageWrap";
import Paginator from "@/components/Paginator";
import {
  ResourceListCreateItem,
  ResourceListItem,
  ResourceListItemMetadata,
  ResourceListWrapper,
} from "@/components/ResourceList";
import { onError, toNumOrZero } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import * as MetaPB from "@octelium/apis/main/metav1";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useSearchParams } from "react-router-dom";

const Item = (props: { item: UserSecret }) => {
  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const mutationDelete = useMutation({
    mutationFn: async (spaceRef: MetaPB.ObjectReference) => {
      const { response } = await client.deleteUserSecret(
        MetaPB.DeleteOptions.create({
          uid: spaceRef.uid,
          name: spaceRef.name,
        }),
      );

      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listUserSecret", 0],
      });
    },
    onError: onError,
  });

  return (
    <div className="font-semibold w-full">
      <div className="flex items-center">
        <div className="flex flex-col flex-1">
          <ResourceListItemMetadata resource={props.item} />
          <div>
            {props.item.spec?.type === UserSecret_Spec_Type.SSH_KEY && (
              <Label>SSH Key</Label>
            )}
            {props.item.status?.details.oneofKind === `sshKey` && (
              <InfoItem title="Public Key">
                <div>
                  <CopyText
                    value={props.item.status.details.sshKey.publicKey}
                    truncate={42}
                  />
                </div>
              </InfoItem>
            )}
          </div>
          <div className="flex justify-end">
            <DeleteResource
              btnSize="xs"
              onDelete={() => {
                mutationDelete.mutate(getResourceRef(props.item));
              }}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

const SecretListC = (props: { itemsList: UserSecretList }) => {
  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.length === 0 && (
          <EmptyList title="No Secrets found"></EmptyList>
        )}
        {props.itemsList.items.map((item) => (
          <ResourceListItem key={item.metadata!.uid}>
            <Item item={item} />
          </ResourceListItem>
        ))}
      </ResourceListWrapper>
    </div>
  );
};

const Page = () => {
  let [searchParams, _] = useSearchParams();

  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;
  const [page, setPage] = useState(toNumOrZero(searchParams.get("page")));

  const qry = useQuery({
    queryKey: ["workspace/listUserSecret", page],
    queryFn: () => {
      const { response } = getClientWorkspace().listUserSecret(
        ListUserSecretOptions.create({
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
    <>
      <PageWrap qry={qry} title="User Secrets">
        {qry.data && (
          <div className="w-full flex flex-col gap-4">
            <ResourceListCreateItem
              title="Create a User Secret"
              path={`/usersecrets/create`}
            />
            <SecretListC itemsList={qry.data} />
            <Paginator
              meta={qry.data.listResponseMeta!}
              onPageChange={setPage}
            />
          </div>
        )}
      </PageWrap>
    </>
  );
};

export default Page;
