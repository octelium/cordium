import Meta from "@/components/Meta";
import { getClientUser } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import * as React from "react";

import {
  ListNamespaceOptions,
  ListServiceOptions,
  Service_Spec_Type,
} from "@octelium/apis/main/userv1";
import { useQuery } from "@tanstack/react-query";

import CopyText from "@/components/CopyText";
import EmptyList from "@/components/EmptyList";
import InfoItem from "@/components/InfoItem";
import Paginator from "@/components/Paginator";
import {
  ResourceListItem,
  ResourceListWrapper,
} from "@/components/ResourceList";
import { getDomain, toNumOrZero } from "@/utils";
import { Service, ServiceList } from "@octelium/apis/main/userv1";
import { BiLinkExternal } from "react-icons/bi";
import { useSearchParams } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import { match } from "ts-pattern";

import {
  getServiceHostname,
  getServicePrivateFQDN,
  getServicePublicFQDN,
} from "@/utils/octelium";
import { Collapse } from "@mantine/core";

const getType = (svc: Service): string => {
  return match(svc.spec?.type)
    .with(Service_Spec_Type.GRPC, () => "gRPC")
    .with(Service_Spec_Type.HTTP, () => "HTTP")
    .with(Service_Spec_Type.KUBERNETES, () => "Kubernetes")
    .with(Service_Spec_Type.MYSQL, () => "MySQL")
    .with(Service_Spec_Type.POSTGRES, () => "PostgreSQL")
    .with(Service_Spec_Type.SSH, () => "SSH")
    .with(Service_Spec_Type.TCP, () => "TCP")
    .with(Service_Spec_Type.UDP, () => "UDP")
    .with(Service_Spec_Type.WEB, () => "Web App")
    .otherwise(() => "");
};

const SvcLabel = (props: { children?: React.ReactNode; label?: string }) => {
  return (
    <span
      className={twMerge(
        "p-0 rounded-full font-bold text-xs flex-none mx-1 my-1 flex flex-row flex-shrink",
        "border-[1px] border-gray-400 shadow-md",
      )}
    >
      {props.label && (
        <span
          className={twMerge(
            `bg-gray-800 text-white shadow-lg px-2 py-1 rounded-s-full`,
          )}
        >
          {props.label}
        </span>
      )}
      <span className={twMerge(`px-2 py-1 flex-none flex`)}>
        {props.children}
      </span>
    </span>
  );
};

const ItemDetails = (props: { item: Service; domain: string }) => {
  const { item } = props;
  const md = item.metadata!;

  return (
    <div>
      {md.description && (
        <InfoItem title="Description">{md.description}</InfoItem>
      )}
      <InfoItem title="Private FQDN">
        <CopyText value={getServicePrivateFQDN(item, props.domain)} />
      </InfoItem>
      {item.spec?.isPublic && (
        <InfoItem title="Public FQDN">
          <CopyText value={getServicePublicFQDN(item, props.domain)} />
        </InfoItem>
      )}
      {item.status?.addresses && item.status.addresses.length > 0 && (
        <InfoItem title="Private Addresses">
          <div className="flex flex-col">
            {item.status?.addresses.map((x) => (
              <span className="w-full">
                <CopyText value={x} />
              </span>
            ))}
          </div>
        </InfoItem>
      )}
    </div>
  );
};

const Item = (props: { item: Service; domain: string }) => {
  const { item } = props;

  const md = item.metadata!;

  let [showDetails, setShowDetails] = React.useState(false);

  return (
    <div
      className="font-semibold w-full"
      onMouseEnter={() => {
        setShowDetails(true);
      }}
      onMouseLeave={() => {
        setShowDetails(false);
      }}
    >
      <div className="flex">
        <div className="flex flex-col flex-1">
          <div className="flex items-center font-bold">
            <span className="text-gray-800 mr-2 flex flex-row">
              <CopyText value={getServiceHostname(item)} />
            </span>
            {md.displayName && (
              <span className="text-gray-600">{md.displayName}</span>
            )}
          </div>
          <div className="w-full mt-1 flex flex-row">
            <SvcLabel label="Type">{getType(item)}</SvcLabel>
            <SvcLabel label="Namespace"> {item.status?.namespace}</SvcLabel>
            <SvcLabel label="Port">{item.spec?.port}</SvcLabel>
            {/*
            <SvcLabel label="Namespace"> {item.metadata?.namespace}</SvcLabel>
            <SvcLabel label="Hostname">{getHostName(item)}</SvcLabel>
            */}
            {item.spec?.isTLS && <SvcLabel>TLS</SvcLabel>}
          </div>

          <Collapse expanded={showDetails}>
            <ItemDetails item={item} domain={props.domain} />
          </Collapse>
        </div>
        <div className="flex items-start justify-center">
          {item.spec?.isPublic && item.spec.type === Service_Spec_Type.WEB && (
            <a
              className={twMerge(
                "bg-gray-800 text-white py-2 px-4 ml-2 font-bold shadow-lg text-sm rounded-lg",
                "hover:bg-black transition-all duration-200 shadow-xl",
                "flex flex-row items-center justify-center",
              )}
              href={`https://${getServicePublicFQDN(item, props.domain)}`}
              target="_blank"
            >
              <span className="px-1">Visit</span>
              <BiLinkExternal />
            </a>
          )}
        </div>
      </div>
    </div>
  );
};

const ServiceListC = (props: { itemsList: ServiceList }) => {
  const domain = getDomain();

  return (
    <div>
      <ResourceListWrapper>
        {props.itemsList.items.length === 0 && (
          <EmptyList title="No Services found"></EmptyList>
        )}
        {props.itemsList.items.map((item) => (
          <ResourceListItem key={item.metadata!.uid}>
            <Item item={item} domain={domain} />
          </ResourceListItem>
        ))}
      </ResourceListWrapper>
      <Paginator meta={props.itemsList.listResponseMeta!} path="/services" />
    </div>
  );
};

const Page = () => {
  const settings = useAppSelector((state) => state.settings);

  let [searchParams, _] = useSearchParams();
  const page = toNumOrZero(searchParams.get("page"));

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["user/main.listService", page],
    queryFn: async () => {
      const svcResp = await getClientUser().listService(
        ListServiceOptions.create({
          common: {
            page,
            itemsPerPage: settings.itemsPerPage,
          },
        }),
      );

      const nsResp = await getClientUser().listNamespace(
        ListNamespaceOptions.create(),
      );
      return {
        serviceList: svcResp.response,
        namespaceList: nsResp.response,
      };
    },
  });

  return (
    <>
      <Meta title="Services" />
      {isSuccess && <ServiceListC itemsList={data.serviceList} />}
    </>
  );
};

export default Page;
