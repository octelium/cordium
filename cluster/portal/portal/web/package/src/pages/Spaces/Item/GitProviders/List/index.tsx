import {
  GitProvider,
  GitProviderList,
  ListGitProviderOptions,
} from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";
import * as React from "react";

import EmptyList from "@/components/EmptyList";
import Label from "@/components/Label";
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
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { useQuery } from "@tanstack/react-query";
import { FaGithub } from "react-icons/fa6";
import { IoLogoGitlab } from "react-icons/io5";

const ClientID = (props: { clientID: string }) => {
  return <Label>{props.clientID}</Label>;
};

const Item = (props: { item: GitProvider }) => {
  return (
    <div className="font-semibold w-full">
      <div className="flex items-center">
        <div className="flex flex-col flex-1">
          <ResourceListItemMetadata resource={props.item} />
        </div>
      </div>
      <div className="mt-1 flex">
        {props.item.spec?.type.oneofKind === `github` && (
          <>
            <Label>
              <div className="flex items-center">
                <span className="mr-2">GitHub</span> <FaGithub />
              </div>
            </Label>
            <ClientID clientID={props.item.spec.type.github.clientID} />
          </>
        )}
        {props.item.spec?.type.oneofKind === `gitlab` && (
          <>
            <Label>
              <div className="flex items-center">
                <span className="mr-2">Gitlab</span> <IoLogoGitlab />
              </div>
            </Label>
            <ClientID clientID={props.item.spec.type.gitlab.clientID} />
          </>
        )}

        {props.item.spec?.type.oneofKind === `oauth2` && (
          <>
            <Label>
              <div className="flex items-center">
                <span className="mr-2">
                  OAuth2 {`(${props.item.spec.type.oauth2.authURL})`}
                </span>{" "}
                <IoLogoGitlab />
              </div>
            </Label>
            <ClientID clientID={props.item.spec.type.oauth2.clientID} />
          </>
        )}
      </div>
    </div>
  );
};

const SecretListC = (props: { itemsList: GitProviderList }) => {
  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.length === 0 && (
          <EmptyList title="No Git Providers found"></EmptyList>
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
  const ctx = useContextSpace();
  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage;
  let [page, setPage] = React.useState(0);

  if (!ctx.space.isSuccess) {
    return <></>;
  }
  const { isLoading, isSuccess, data } = useQuery({
    queryKey: [
      "workspace/listGitProvider",
      ctx.space.data?.metadata?.uid,
      page,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listGitProvider(
        ListGitProviderOptions.create({
          spaceRef: getResourceRef(ctx.space.data!),
          common: {
            page,
            itemsPerPage: settings.itemsPerPage,
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
    <PageWrap qry={ctx.space} title="Git Providers">
      <ResourceListCreateItem
        title="Create a Git Provider"
        path={`${getPathSpace(ctx.space.data)}/gitproviders/create`}
      />
      <SecretListC itemsList={data} />

      <div className="mt-4">
        <Paginator
          meta={data.listResponseMeta!}
          onPageChange={(i) => {
            setPage(i);
          }}
        />
      </div>
    </PageWrap>
  );
};

export default Page;
