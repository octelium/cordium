import {
  GetSpaceMembershipRequest,
  Membership,
  MembershipList,
  Membership_Spec_Role,
} from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";
import * as React from "react";

import { DeleteOptions, GetOptions } from "@/apis/metav1/metav1";
import DeleteResource from "@/components/DeleteResource";
import Label from "@/components/Label";
import Meta from "@/components/Meta";
import Paginator from "@/components/Paginator";
import {
  ResourceListCreateItem,
  ResourceListItem,
  ResourceListWrapper,
} from "@/components/ResourceList";

import { onError, toNumOrZero } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { getResourceRef } from "@/utils/pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useSearchParams } from "react-router-dom";
import { match } from "ts-pattern";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { Select } from "@mantine/core";

const Item = (props: {
  item: Membership;
  page: number;
  spaceUID: string;
  isAdmin: boolean;
}) => {
  const client = getClientWorkspace();
  const queryClient = useQueryClient();

  const { item, page, spaceUID } = props;

  let [req, setReq] = React.useState(item);

  const mutation = useMutation({
    mutationFn: async () => {
      const uid = item.metadata!.uid;

      await client.deleteMembership(DeleteOptions.create({ uid }));
      return {};
    },
    onSuccess: ({}) => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listMembership", spaceUID, page],
      });
    },
    onError,
  });

  const mutationUpdate = useMutation({
    mutationFn: async () => {
      await client.updateMembership(req);
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listMembership", spaceUID, page],
      });
    },
    onError,
  });

  const role = match(item.spec!.role)
    .with(Membership_Spec_Role.OWNER, () => "Owner")
    .with(Membership_Spec_Role.ADMIN, () => "Admin")
    .otherwise(() => "");

  return (
    <div className="font-semibold w-full">
      <div className="flex items-center">
        <div className="flex flex-col flex-1">
          <div className="w-full">
            {`${item.status?.userRef?.name}`}{" "}
            {item.status?.userInfo?.displayName &&
              ` (${item.status?.userInfo?.displayName})`}
          </div>
          {role.length > 0 && <div>{<Label>{role}</Label>}</div>}
        </div>
      </div>
      {props.isAdmin && (
        <div className="flex items-center justify-end">
          <Select
            data={[
              {
                label: "Ordinary User",
                value:
                  WsPB.Membership_Spec_Role[WsPB.Membership_Spec_Role.USER],
              },
              {
                label: "Administrator",
                value:
                  WsPB.Membership_Spec_Role[WsPB.Membership_Spec_Role.ADMIN],
              },
              {
                label: "Owner",
                value:
                  WsPB.Membership_Spec_Role[WsPB.Membership_Spec_Role.OWNER],
              },
            ]}
            defaultValue={WsPB.Membership_Spec_Role[req.spec!.role]}
            onChange={(val) => {
              req.spec!.role = WsPB.Membership_Spec_Role[val as "OWNER"];
              setReq(WsPB.Membership.clone(req));
              mutationUpdate.mutate();
            }}
          />

          <DeleteResource
            btnSize="xs"
            onDelete={() => {
              mutation.mutate();
            }}
          />
        </div>
      )}
    </div>
  );
};

const MembershipListC = (props: {
  itemsList: MembershipList;
  spaceUID: string;
  isAdmin: boolean;
}) => {
  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.map((item) => (
          <ResourceListItem key={item.metadata!.uid}>
            <Item
              item={item}
              page={props.itemsList.listResponseMeta!.page}
              spaceUID={props.spaceUID}
              isAdmin={props.isAdmin}
            />
          </ResourceListItem>
        ))}
      </ResourceListWrapper>
    </div>
  );
};

const Page = () => {
  let [searchParams, _] = useSearchParams();
  const page = toNumOrZero(searchParams.get("page"));
  let { spaceUID } = useParams();
  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;
  const client = getClientWorkspace();

  if (!spaceUID) {
    return <></>;
  }

  const spaceQuery = useQuery({
    queryKey: ["workspace/getSpace", spaceUID],
    queryFn: () => {
      const { response } = client.getSpace(
        GetOptions.create({ uid: spaceUID }),
      );
      return response;
    },
  });

  const qryMem = useQuery({
    queryKey: ["workspace/getSpaceMembership", spaceUID],
    queryFn: () => {
      const { response } = client.getSpaceMembership(
        GetSpaceMembershipRequest.create({
          spaceRef: getResourceRef(spaceQuery.data!),
        }),
      );
      return response;
    },
    enabled: spaceQuery.isSuccess,
  });

  if (!spaceQuery.isSuccess) {
    return <></>;
  }

  const isAdmin =
    qryMem.data?.spec?.role === Membership_Spec_Role.ADMIN ||
    qryMem.data?.spec?.role === Membership_Spec_Role.OWNER;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/listMembership", spaceUID, page],
    queryFn: () => {
      const { response } = getClientWorkspace().listMembership(
        WsPB.ListMembershipOptions.create({
          spaceRef: getResourceRef(spaceQuery.data),
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
    <div className="w-full">
      <Meta title="Memberships" />
      <ResourceListCreateItem
        title="Add a Member"
        path={`/spaces/uid/${spaceUID}/memberships/create`}
      />
      <MembershipListC itemsList={data} spaceUID={spaceUID} isAdmin={isAdmin} />

      <div className="mt-4">
        <Paginator
          meta={data.listResponseMeta!}
          path={`/memberships?spaceUID=${spaceUID}`}
        />
      </div>
    </div>
  );
};

export default Page;
