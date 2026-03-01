import { GetOptions } from "@/apis/metav1/metav1";
import { getClientWorkspace } from "@/utils/client";
import { useQuery } from "@tanstack/react-query";
import React from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";

import { useAppSelector } from "@/utils/hooks";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { MdOutlineArrowForwardIos } from "react-icons/md";
import { twMerge } from "tailwind-merge";
import * as WsPB from "../../apis/cordiumv1/cordiumv1";
import SpaceName from "../SpaceName";

const Block = (props: { children?: React.ReactNode }) => {
  return (
    <span className="transition-all duration-300 shadow-sm mr-4 mb-2 py-2 px-4 overflow-hidden border-[2px] border-gray-400 rounded-full font-extrabold text-xl flex flex-nowrap items-center justify-center">
      {props.children}
    </span>
  );
};

const getUIDFromPath = (pth: string): string => {
  return (
    pth
      .split("/")
      .reverse()
      .find((x) => x.length === 36) ?? ""
  );
};

export const TitleItemSpace = (props: {
  uid?: string;
  linkList?: boolean;
  linkItem?: boolean;
}) => {
  const client = getClientWorkspace();
  const { uid } = props;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getSpace", uid!],
    queryFn: () => {
      const { response } = client.getSpace(GetOptions.create({ uid }));
      return response;
    },
    enabled: !!uid,
  });

  return (
    <Wrapper>
      <DoTitleItemSpace item={data} />
    </Wrapper>
  );
};

const LinkType = (props: { link?: string; children?: React.ReactNode }) => {
  const navigate = useNavigate();
  const { link } = props;
  return (
    <a
      onClick={() => {
        if (link) {
          navigate(link);
        }
      }}
      className={twMerge(
        `transition-all duration-300`,
        link
          ? `text-sky-500 hover:text-sky-700 cursor-pointer`
          : `text-gray-600`,
      )}
    >
      {props.children}
    </a>
  );
};

const DoTitleItemSpace = (props: { item?: WsPB.Space; linkItem?: boolean }) => {
  const { item } = props;

  const status = useAppSelector((state) => state.settings.status);

  return (
    <Block>
      <LinkType link={item?.metadata?.uid ? "/spaces" : undefined}>
        Spaces
      </LinkType>

      {item && (
        <span>
          <MdOutlineArrowForwardIos />
        </span>
      )}
      {item && (
        <LinkType link={props.linkItem ? getPathSpace(item) : undefined}>
          <SpaceName spaceRef={getResourceRef(item)} />
        </LinkType>
      )}
    </Block>
  );
};

const Wrapper = (props: { children?: React.ReactNode }) => {
  return <div className="w-full flex flex-wrap">{props.children}</div>;
};

export const TitleItemTemplate = (props: {
  uid?: string;
  parentUID?: string;
  linkList?: boolean;
  linkItem?: boolean;
}) => {
  const client = getClientWorkspace();
  const { uid, parentUID } = props;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getTemplate", uid!],
    queryFn: () => {
      const { response } = client.getTemplate(GetOptions.create({ uid }));
      return response;
    },
    enabled: !!uid,
  });

  const querySpace = useQuery({
    queryKey: ["workspace/getSpace", data?.status?.spaceRef?.uid],
    queryFn: () => {
      const { response } = client.getSpace(
        GetOptions.create({
          uid: data?.status?.spaceRef?.uid,
        }),
      );
      return response;
    },
    enabled: isSuccess,
  });

  return (
    <Wrapper>
      <DoTitleItemTemplate item={data} space={querySpace.data} />
    </Wrapper>
  );
};

export const DoTitleItemTemplate = (props: {
  item?: WsPB.Template;
  space?: WsPB.Space;
  linkItem?: boolean;
}) => {
  const { item, space } = props;

  const navigate = useNavigate();
  return (
    <>
      <DoTitleItemSpace item={props.space} linkItem />
      <Block>
        <LinkType
          link={
            item?.metadata?.uid
              ? `/templates?spaceUID=${space?.metadata?.uid}`
              : undefined
          }
        >
          Templates
        </LinkType>

        {item && (
          <span>
            <MdOutlineArrowForwardIos />
          </span>
        )}
        {item && (
          <LinkType
            link={
              props.linkItem
                ? `/templates/uid/${item.metadata?.uid}`
                : undefined
            }
          >
            {getShortName(item)}
          </LinkType>
        )}
      </Block>
    </>
  );
};

export const TitleItemWorkspace = (props: {
  uid?: string;
  parentUID?: string;
  linkList?: boolean;
  linkItem?: boolean;
  parentType?: string;
}) => {
  const client = getClientWorkspace();
  const { uid, parentUID, parentType } = props;
  const pType = parentType?.replace("UID", "");

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getWorkspace", uid!],
    queryFn: () => {
      const { response } = client.getWorkspace(GetOptions.create({ uid }));
      return response;
    },
    enabled: !!uid,
  });
  const queryTemplate = useQuery({
    queryKey: [
      "workspace/getTemplate",
      data?.status?.templateRef?.uid ?? parentUID,
    ],
    queryFn: () => {
      const { response } = client.getTemplate(
        GetOptions.create({ uid: data?.status?.templateRef?.uid }),
      );
      return response;
    },
    enabled: isSuccess || pType === "template",
  });

  const querySpace = useQuery({
    queryKey: ["workspace/getSpace", data?.status?.spaceRef?.uid],
    queryFn: () => {
      const { response } = client.getSpace(
        GetOptions.create({
          uid:
            data?.status?.spaceRef?.uid ??
            queryTemplate.data?.status?.spaceRef?.uid ??
            parentUID,
        }),
      );
      return response;
    },
    enabled: isSuccess || queryTemplate.isSuccess || pType === "space",
  });

  return (
    <Wrapper>
      <DoTitleItemWorkspace
        item={data}
        template={queryTemplate.data}
        space={querySpace.data}
      />
    </Wrapper>
  );
};

export const DoTitleItemWorkspace = (props: {
  item?: WsPB.Workspace;
  template?: WsPB.Template;
  space?: WsPB.Space;
}) => {
  const { item } = props;

  return (
    <>
      <Block>
        <LinkType link={item?.metadata?.uid ? `/workspaces` : undefined}>
          Workspaces
        </LinkType>

        {item && (
          <span>
            <MdOutlineArrowForwardIos />
          </span>
        )}
        {item && <div className="text-gray-500">{item.metadata?.name}</div>}
      </Block>
    </>
  );
};

export const TitleItemSecret = (props: {
  uid?: string;
  parentUID?: string;
  linkList?: boolean;
  linkItem?: boolean;
}) => {
  const client = getClientWorkspace();
  const { uid, parentUID } = props;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getSecret", uid!],
    queryFn: () => {
      const { response } = client.getSecret(GetOptions.create({ uid }));
      return response;
    },
    enabled: !!uid,
  });

  const querySpace = useQuery({
    queryKey: ["workspace/getSpace", data?.status?.spaceRef?.uid ?? parentUID],
    queryFn: () => {
      const { response } = client.getSpace(
        GetOptions.create({ uid: data?.status?.spaceRef?.uid ?? parentUID }),
      );
      return response;
    },
    enabled: isSuccess || !!parentUID,
  });

  return (
    <Wrapper>
      <DoTitleItemSecret item={data} space={querySpace.data} />
    </Wrapper>
  );
};

export const DoTitleItemSecret = (props: {
  item?: WsPB.Secret;
  space?: WsPB.Space;
}) => {
  const { item, space } = props;

  return (
    <>
      <DoTitleItemSpace item={props.space} linkItem />
      <Block>
        <LinkType
          link={
            item?.metadata?.uid
              ? `/secrets?spaceUID=${space?.metadata?.uid}`
              : undefined
          }
        >
          Secrets
        </LinkType>

        {item && (
          <span>
            <MdOutlineArrowForwardIos />
          </span>
        )}
        {item && <div className="text-gray-500">{getShortName(item)}</div>}
      </Block>
    </>
  );
};

export const TitleItemUserSecret = (props: {
  uid?: string;
  parentUID?: string;
  linkList?: boolean;
  linkItem?: boolean;
}) => {
  const client = getClientWorkspace();
  const { uid, parentUID } = props;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getUserSecret", uid!],
    queryFn: () => {
      const { response } = client.getUserSecret(GetOptions.create({ uid }));
      return response;
    },
    enabled: !!uid,
  });

  return (
    <Wrapper>
      <DoTitleItemUserSecret item={data} />
    </Wrapper>
  );
};

export const DoTitleItemUserSecret = (props: { item?: WsPB.UserSecret }) => {
  const { item } = props;

  return (
    <>
      <Block>
        <LinkType link={item?.metadata?.uid ? `/usersecrets` : undefined}>
          Your Secrets
        </LinkType>

        {item && (
          <span>
            <MdOutlineArrowForwardIos />
          </span>
        )}
        {item && <div className="text-gray-500">{getShortName(item)}</div>}
      </Block>
    </>
  );
};

export const TitleItemGen = (props: {
  spaceUID: string;
  pathPrefix: string;
  namePlural: string;
}) => {
  const { spaceUID, pathPrefix } = props;
  const client = getClientWorkspace();

  const querySpace = useQuery({
    queryKey: ["workspace/getSpace", spaceUID],
    queryFn: () => {
      const { response } = client.getSpace(
        GetOptions.create({ uid: spaceUID }),
      );
      return response;
    },
  });

  if (!querySpace.isSuccess) {
    return <></>;
  }

  return (
    <Wrapper>
      <DoTitleItemSpace item={querySpace.data} linkItem />
      <Block>
        <LinkType>{props.namePlural}</LinkType>
      </Block>
    </Wrapper>
  );
};

export const TitleItemGitProvider = (props: {
  uid?: string;
  parentUID?: string;
  linkList?: boolean;
  linkItem?: boolean;
}) => {
  const client = getClientWorkspace();
  const { uid, parentUID } = props;

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getGitProvider", uid!],
    queryFn: () => {
      const { response } = client.getGitProvider(GetOptions.create({ uid }));
      return response;
    },
    enabled: !!uid,
  });

  const querySpace = useQuery({
    queryKey: ["workspace/getSpace", data?.status?.spaceRef?.uid ?? parentUID],
    queryFn: () => {
      const { response } = client.getSpace(
        GetOptions.create({ uid: data?.status?.spaceRef?.uid ?? parentUID }),
      );
      return response;
    },
    enabled: isSuccess || !!parentUID,
  });

  return (
    <Wrapper>
      <DoTitleItemGitProvider item={data} space={querySpace.data} />
    </Wrapper>
  );
};

export const DoTitleItemGitProvider = (props: {
  item?: WsPB.GitProvider;
  space?: WsPB.Space;
  linkItem?: boolean;
}) => {
  const { item, space } = props;

  return (
    <>
      <DoTitleItemSpace item={props.space} linkItem />
      <Block>
        <LinkType
          link={
            item?.metadata?.uid
              ? `/gitproviders?spaceUID=${space?.metadata?.uid}`
              : undefined
          }
        >
          Git Providers
        </LinkType>

        {item && (
          <span>
            <MdOutlineArrowForwardIos />
          </span>
        )}
        {item && (
          <LinkType
            link={
              props.linkItem
                ? `/gitproviders/uid/${item.metadata?.uid}`
                : undefined
            }
          >
            {getShortName(item)}
          </LinkType>
        )}
      </Block>
    </>
  );
};
