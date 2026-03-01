import {
  ListUserSecretOptions,
  UserSecret,
  UserSecretList,
  UserSecret_Spec_Type,
} from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";

import EmptyList from "@/components/EmptyList";
import Label from "@/components/Label";
import Meta from "@/components/Meta";
import {
  ResourceListCreateItem,
  ResourceListItem,
  ResourceListItemMetadata,
  ResourceListWrapper,
} from "@/components/ResourceList";
import { toNumOrZero } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";

const Item = (props: { item: UserSecret }) => {
  return (
    <div className="font-semibold w-full">
      <div className="flex items-center">
        <div className="flex flex-col flex-1">
          <ResourceListItemMetadata resource={props.item} />
          <div>
            {props.item.spec?.type === UserSecret_Spec_Type.SSH_KEY && (
              <Label>SSH Key</Label>
            )}
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
          <ResourceListItem
            key={item.metadata!.uid}
            path={`/usersecrets/uid/${item.metadata!.uid}`}
          >
            <Item item={item} />
          </ResourceListItem>
        ))}
      </ResourceListWrapper>
    </div>
  );
};

const Page = () => {
  let [searchParams, _] = useSearchParams();

  const page = toNumOrZero(searchParams.get("page"));
  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;

  const { isLoading, isSuccess, data } = useQuery({
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
      <Meta title="Your Secrets" />
      <ResourceListCreateItem
        title="Create a User Secret"
        path={`/usersecrets/create`}
      />
      {isSuccess && <SecretListC itemsList={data} />}
    </>
  );
};

export default Page;
