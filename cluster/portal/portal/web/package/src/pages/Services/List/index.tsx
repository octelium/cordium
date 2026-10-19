import CopyText from "@/components/CopyText";
import Empty from "@/components/Empty";
import Facts, { Fact } from "@/components/Facts";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, ClickableCard } from "@/components/ResourceCards";
import SearchField from "@/components/SearchField";
import Tag from "@/components/Tag";
import { getDomain } from "@/utils";
import { getClientUser } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import {
  getServiceHostname,
  getServicePrivateFQDN,
  getServicePublicFQDN,
} from "@/utils/octelium";
import {
  ActionIcon,
  Alert,
  Button,
  Collapse,
  Select,
  Stack,
  Tooltip,
} from "@mantine/core";
import {
  ListNamespaceOptions,
  ListServiceOptions,
  Service,
  Service_Spec_Type,
} from "@octelium/apis/main/userv1";
import {
  IconChevronDown,
  IconChevronUp,
  IconExternalLink,
  IconServer2,
  IconRefresh,
  IconSearchOff,
  IconX,
} from "@tabler/icons-react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useSearchParams } from "react-router-dom";
import {
  CommonListOptions_OrderBy_Mode,
  CommonListOptions_OrderBy_Type,
} from "@octelium/apis/main/metav1";
import { getServiceTypeLabel, searchServices, SERVICE_TYPES } from "./utils";

const SEARCH_FETCH_LIMIT = 500;
const ORDER_BY_NAME = {
  type: CommonListOptions_OrderBy_Type.NAME,
  mode: CommonListOptions_OrderBy_Mode.ASC,
};

const ServiceRow = (props: { item: Service; domain: string }) => {
  const { item, domain } = props;
  const [expanded, setExpanded] = React.useState(false);
  const detailsId = React.useId();
  const isWeb = item.spec?.isPublic && item.spec.type === Service_Spec_Type.WEB;

  return (
    <ClickableCard>
      <div className="flex flex-col gap-3 md:flex-row md:items-start">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className="truncate text-sm font-bold text-ink-strong">
              {getServiceHostname(item)}
            </span>
            {item.metadata?.displayName && (
              <span className="truncate text-[0.78rem] font-medium text-ink-muted">
                {item.metadata.displayName}
              </span>
            )}
          </div>

          {item.metadata?.description && (
            <p className="mt-0.5 text-[0.78rem] font-medium text-ink-muted">
              {item.metadata.description}
            </p>
          )}

          <div className="mt-2 flex flex-wrap gap-1.5">
            <Tag label="Type">{getServiceTypeLabel(item)}</Tag>
            <Tag label="Namespace">{item.status?.namespace}</Tag>
            {item.spec?.port ? <Tag label="Port">{item.spec.port}</Tag> : null}
            {item.spec?.isTLS && <Tag tone="success">TLS</Tag>}
            {item.spec?.isPublic && <Tag tone="info">Public</Tag>}
          </div>

          <Collapse id={detailsId} expanded={expanded}>
            <div className="mt-3 border-t border-line-subtle pt-1">
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
              rel="noopener noreferrer"
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
            aria-expanded={expanded}
            aria-controls={detailsId}
            aria-label={`${expanded ? "Hide" : "Show"} details of ${getServiceHostname(item)}`}
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? "Less" : "Details"}
          </Button>
        </div>
      </div>
    </ClickableCard>
  );
};

const FilterChip = (props: {
  label: string;
  value: string;
  onClear: () => void;
}) => (
  <span className="inline-flex max-w-full items-center gap-1 rounded-md border border-line bg-surface py-0.5 pl-2 pr-0.5 text-xs text-ink-soft">
    <span className="shrink-0 font-semibold">{props.label}:</span>
    <span className="truncate">{props.value}</span>
    <button
      type="button"
      aria-label={`Remove ${props.label.toLowerCase()} filter`}
      onClick={props.onClear}
      className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-ink-muted outline-none hover:bg-surface-strong hover:text-ink focus-visible:ring-2 focus-visible:ring-focus"
    >
      <IconX size={12} aria-hidden />
    </button>
  </span>
);

const Page = () => {
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [searchParams, setSearchParams] = useSearchParams();
  const domain = getDomain();
  const search = (searchParams.get("q") ?? "").trim();
  const namespace = searchParams.get("namespace") ?? "";
  const typeInfo = SERVICE_TYPES.find(
    (info) => info.value === searchParams.get("type"),
  );
  const rawPage = Number(searchParams.get("common.page") ?? 0);
  const page = Number.isSafeInteger(rawPage) && rawPage >= 0 ? rawPage : 0;
  const hasFilters = !!search || !!namespace || !!typeInfo;

  const setParams = React.useCallback(
    (values: Record<string, string | null>, replace = false) => {
      setSearchParams(
        (previous) => {
          const params = new URLSearchParams(previous);
          params.delete("common.page");
          for (const [key, value] of Object.entries(values)) {
            if (value) params.set(key, value);
            else params.delete(key);
          }
          return params;
        },
        { replace },
      );
    },
    [setSearchParams],
  );

  const onSearchChange = React.useCallback(
    (value: string) => setParams({ q: value }, true),
    [setParams],
  );
  const clearFilters = () =>
    setParams({ q: null, namespace: null, type: null });
  const options = ListServiceOptions.create({
    common: {
      page: search ? 0 : page,
      itemsPerPage: search ? SEARCH_FETCH_LIMIT : itemsPerPage,
      orderBy: ORDER_BY_NAME,
    },
    namespace,
    type: typeInfo?.type ?? Service_Spec_Type.UNSET,
  });

  const qry = useQuery({
    queryKey: ["user/listService", ListServiceOptions.toJsonString(options)],
    queryFn: async () => (await getClientUser().listService(options)).response,
    placeholderData: keepPreviousData,
  });
  const namespaces = useQuery({
    queryKey: ["user/listNamespace"],
    queryFn: async () =>
      (
        await getClientUser().listNamespace(
          ListNamespaceOptions.create({
            common: {
              itemsPerPage: SEARCH_FETCH_LIMIT,
              orderBy: ORDER_BY_NAME,
            },
          }),
        )
      ).response,
    staleTime: 5 * 60 * 1000,
  });
  const namespaceData = (namespaces.data?.items ?? [])
    .filter((item) => item.metadata?.name && item.metadata.name !== "octelium")
    .map((item) => ({
      value: item.metadata!.name,
      label: item.metadata!.displayName || item.metadata!.name,
    }));
  if (namespace && !namespaceData.some((item) => item.value === namespace)) {
    namespaceData.unshift({ value: namespace, label: namespace });
  }
  const items = qry.data?.items ?? [];
  const visibleItems = searchServices(items, search, domain);
  const meta = qry.data?.listResponseMeta;
  const totalCount = meta?.totalCount ?? items.length;

  return (
    <>
      <Meta title="Services" />
      <PageHeader
        title="Services"
        description="Discover Services available to you and connect from your workspaces."
        actions={
          <Tooltip label="Refresh Services">
            <ActionIcon
              variant="default"
              size={34}
              aria-label="Refresh Services"
              loading={qry.isFetching && !qry.isPending}
              onClick={() => qry.refetch()}
            >
              <IconRefresh size={16} />
            </ActionIcon>
          </Tooltip>
        }
      />

      <div className="mb-4 space-y-2.5 border-b border-line pb-4">
        <div className="flex flex-col gap-2 lg:flex-row">
          <SearchField
            className="min-w-0 flex-1"
            label="Search Services"
            placeholder="Search by name, type, port or address"
            value={search}
            onChange={onSearchChange}
          />
          <div className="grid grid-cols-2 gap-2 lg:flex">
            <Select
              className="min-w-0 lg:w-48"
              aria-label="Filter by Namespace"
              placeholder="All Namespaces"
              searchable
              clearable
              clearSectionMode="clear"
              nothingFoundMessage="No matching Namespaces"
              data={namespaceData}
              value={namespace || null}
              onChange={(value) => setParams({ namespace: value })}
            />
            <Select
              className="min-w-0 lg:w-40"
              aria-label="Filter by type"
              placeholder="All types"
              clearable
              clearSectionMode="clear"
              data={SERVICE_TYPES.map(({ value, label }) => ({ value, label }))}
              value={typeInfo?.value ?? null}
              onChange={(value) => setParams({ type: value })}
            />
          </div>
        </div>
        {hasFilters && (
          <div className="flex flex-wrap items-center gap-1.5">
            {search && (
              <FilterChip
                label="Search"
                value={search}
                onClear={() => setParams({ q: null })}
              />
            )}
            {namespace && (
              <FilterChip
                label="Namespace"
                value={namespace}
                onClear={() => setParams({ namespace: null })}
              />
            )}
            {typeInfo && (
              <FilterChip
                label="Type"
                value={typeInfo.label}
                onClear={() => setParams({ type: null })}
              />
            )}
            <button
              type="button"
              onClick={clearFilters}
              className="ml-1 rounded text-xs font-semibold text-ink-soft outline-none hover:text-ink focus-visible:ring-2 focus-visible:ring-focus"
            >
              Clear filters
            </button>
          </div>
        )}
      </div>

      {qry.isError && (
        <Alert color="red" title="Could not load Services" className="mb-3">
          {qry.data
            ? "Refreshing failed. Showing the last loaded Services."
            : "The request failed. Try refreshing the list."}
          <Button
            size="compact-xs"
            variant="subtle"
            color="red"
            ml="sm"
            onClick={() => qry.refetch()}
          >
            Retry
          </Button>
        </Alert>
      )}
      {namespaces.isError && (
        <Alert color="yellow" className="mb-3">
          Namespace filters could not be loaded. Search and type filters are
          still available.
        </Alert>
      )}
      {search && totalCount > items.length && (
        <Alert color="yellow" className="mb-3">
          Search covers the first {items.length.toLocaleString()} of{" "}
          {totalCount.toLocaleString()} Services. Narrow it down with the
          Namespace or type filters.
        </Alert>
      )}
      <QueryBoundary query={{ isPending: qry.isPending }}>
        {qry.data && (
          <Stack
            gap="md"
            aria-busy={qry.isFetching}
            className={qry.isPlaceholderData ? "opacity-60" : undefined}
          >
            {visibleItems.length === 0 ? (
              <Empty
                icon={
                  hasFilters ? (
                    <IconSearchOff size={22} />
                  ) : (
                    <IconServer2 size={22} />
                  )
                }
                title={
                  hasFilters ? "No matching Services" : "No Services assigned"
                }
                description={
                  hasFilters
                    ? "Try another search or change the filters."
                    : "Ask a Cluster administrator to grant you access to a Service."
                }
                action={
                  hasFilters && (
                    <Button variant="default" size="xs" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  )
                }
              />
            ) : (
              <CardList>
                {visibleItems.map((item) => (
                  <ServiceRow
                    key={item.metadata?.uid || item.metadata?.name}
                    item={item}
                    domain={domain}
                  />
                ))}
              </CardList>
            )}
            {search ? (
              <p role="status" className="text-xs tabular-nums text-ink-muted">
                {visibleItems.length.toLocaleString()} of{" "}
                {items.length.toLocaleString()} Services match
              </p>
            ) : (
              meta && (
                <Paginator
                  meta={meta}
                  onPageChange={(value) =>
                    setParams({ "common.page": value ? String(value) : null })
                  }
                />
              )
            )}
          </Stack>
        )}
      </QueryBoundary>
    </>
  );
};

export default Page;
