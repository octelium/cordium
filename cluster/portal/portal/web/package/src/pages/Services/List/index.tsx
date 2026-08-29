import CopyText from "@/components/CopyText";
import Empty from "@/components/Empty";
import Facts, { Fact } from "@/components/Facts";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, ClickableCard } from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import { getDomain } from "@/utils";
import { getClientUser } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import {
  getServiceHostname,
  getServicePrivateFQDN,
  getServicePublicFQDN,
} from "@/utils/octelium";
import { Button, Collapse, Stack } from "@mantine/core";
import {
  ListServiceOptions,
  Service,
  Service_Spec_Type,
} from "@octelium/apis/main/userv1";
import {
  IconChevronDown,
  IconChevronUp,
  IconExternalLink,
  IconServer2,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { match } from "ts-pattern";

const getType = (svc: Service): string =>
  match(svc.spec?.type)
    .with(Service_Spec_Type.GRPC, () => "gRPC")
    .with(Service_Spec_Type.HTTP, () => "HTTP")
    .with(Service_Spec_Type.KUBERNETES, () => "Kubernetes")
    .with(Service_Spec_Type.MYSQL, () => "MySQL")
    .with(Service_Spec_Type.POSTGRES, () => "PostgreSQL")
    .with(Service_Spec_Type.SSH, () => "SSH")
    .with(Service_Spec_Type.TCP, () => "TCP")
    .with(Service_Spec_Type.UDP, () => "UDP")
    .with(Service_Spec_Type.WEB, () => "Web app")
    .otherwise(() => "Service");

const ServiceRow = (props: { item: Service; domain: string }) => {
  const { item, domain } = props;
  const [expanded, setExpanded] = React.useState(false);
  const isWeb =
    item.spec?.isPublic && item.spec.type === Service_Spec_Type.WEB;

  return (
    <ClickableCard>
      <div className="flex flex-col gap-3 md:flex-row md:items-start">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className="truncate text-sm font-bold text-slate-800">
              {getServiceHostname(item)}
            </span>
            {item.metadata?.displayName && (
              <span className="truncate text-[0.78rem] font-medium text-slate-500">
                {item.metadata.displayName}
              </span>
            )}
          </div>

          {item.metadata?.description && (
            <p className="mt-0.5 text-[0.78rem] font-medium text-slate-500">
              {item.metadata.description}
            </p>
          )}

          <div className="mt-2 flex flex-wrap gap-1.5">
            <Tag label="Type">{getType(item)}</Tag>
            <Tag label="Namespace">{item.status?.namespace}</Tag>
            {item.spec?.port ? <Tag label="Port">{item.spec.port}</Tag> : null}
            {item.spec?.isTLS && <Tag tone="success">TLS</Tag>}
            {item.spec?.isPublic && <Tag tone="info">Public</Tag>}
          </div>

          <Collapse expanded={expanded}>
            <div className="mt-3 border-t border-slate-100 pt-1">
              <Facts>
                <Fact label="Private FQDN">
                  <CopyText value={getServicePrivateFQDN(item, domain)} />
                </Fact>
                {item.spec?.isPublic && (
                  <Fact label="Public FQDN">
                    <CopyText value={getServicePublicFQDN(item, domain)} />
                  </Fact>
                )}
                {(item.status?.addresses?.length ?? 0) > 0 && (
                  <Fact label="Addresses">
                    <span className="flex flex-col gap-0.5">
                      {item.status!.addresses.map((a) => (
                        <CopyText key={a} value={a} />
                      ))}
                    </span>
                  </Fact>
                )}
              </Facts>
            </div>
          </Collapse>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {isWeb && (
            <Button
              size="xs"
              variant="default"
              component="a"
              href={`https://${getServicePublicFQDN(item, domain)}`}
              target="_blank"
              rel="noreferrer"
              leftSection={<IconExternalLink size={13} />}
            >
              Visit
            </Button>
          )}
          <Button
            size="xs"
            variant="subtle"
            color="gray"
            rightSection={
              expanded ? (
                <IconChevronUp size={13} />
              ) : (
                <IconChevronDown size={13} />
              )
            }
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? "Less" : "Details"}
          </Button>
        </div>
      </div>
    </ClickableCard>
  );
};

const Page = () => {
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);
  const domain = getDomain();

  const qry = useQuery({
    queryKey: ["user/listService", page, itemsPerPage],
    queryFn: async () => {
      const { response } = await getClientUser().listService(
        ListServiceOptions.create({ common: { page, itemsPerPage } }),
      );
      return response;
    },
  });

  return (
    <>
      <Meta title="Services" />
      <PageHeader
        title="Services"
        description="Octelium Services assigned to you. Workspaces can reach them privately, and serve them with the runtime's Octelium integration."
      />

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconServer2 size={22} />}
                title="No Services assigned"
                description="Ask a Cluster administrator to grant you access to a Service."
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <ServiceRow
                    key={x.metadata?.uid}
                    item={x}
                    domain={domain}
                  />
                ))}
              </CardList>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>
    </>
  );
};

export default Page;
