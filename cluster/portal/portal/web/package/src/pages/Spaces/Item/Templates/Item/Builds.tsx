import BuildTemplate from "@/components/BuildTemplate";
import ConfirmAction from "@/components/ConfirmAction";
import Empty from "@/components/Empty";
import Panel, { PanelBody, PanelHeader } from "@/components/Panel";
import Paginator from "@/components/Paginator";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { invalidateTemplate } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { Loader, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import {
  IconCircleCheck,
  IconCircleX,
  IconHammer,
  IconPlayerStop,
} from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";

const BuildState = WsPB.Template_Status_BuildInfo_Build_State;

const BuildRow = (props: {
  item: WsPB.Template_Status_BuildInfo_Build;
  template: WsPB.Template;
  isCurrent: boolean;
}) => {
  const { item, template } = props;
  const client = getClientWorkspace();

  const mutationCancel = useMutation({
    mutationFn: async () => {
      const { response } = await client.cancelBuildTemplate(
        WsPB.CancelBuildTemplateRequest.create({
          templateRef: getResourceRef(template),
        }),
      );
      return response;
    },
    onSuccess: () => {
      invalidateTemplate(template);
      toast.success("Build canceled");
    },
    onError,
  });

  const running =
    !item.isCanceled && item.state === BuildState.RUNNING;

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white px-4 py-3 md:flex-row md:items-center">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-[0.8rem] font-semibold text-slate-700">
            {item.id}
          </span>
          {props.isCurrent && <Tag tone="success">Current</Tag>}
          {item.tags.map((t) => (
            <Tag key={t} mono>
              {t}
            </Tag>
          ))}
        </div>
        <div className="mt-1 text-[0.75rem] font-medium text-slate-400">
          {item.startedAt && (
            <>
              Started <TimeAgo rfc3339={item.startedAt} />
            </>
          )}
          {item.doneAt && (
            <>
              {" · Finished "}
              <TimeAgo rfc3339={item.doneAt} />
            </>
          )}
        </div>
        {item.failure?.message && (
          <p className="mt-1 text-[0.78rem] font-medium text-rose-600">
            {item.failure.message}
          </p>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-3">
        {item.isCanceled && <Tag tone="neutral">Canceled</Tag>}
        {!item.isCanceled && item.state === BuildState.READY && (
          <Tag tone="success" icon={<IconCircleCheck size={11} />}>
            Ready
          </Tag>
        )}
        {!item.isCanceled && item.state === BuildState.FAILED && (
          <Tag tone="danger" icon={<IconCircleX size={11} />}>
            Failed
          </Tag>
        )}
        {running && (
          <>
            <span className="inline-flex items-center gap-1.5 text-[0.75rem] font-semibold text-blue-700">
              <Loader size={12} color="blue" />
              Building
            </span>
            <ConfirmAction
              triggerLabel="Cancel"
              triggerIcon={<IconPlayerStop size={13} />}
              color="orange"
              title="Cancel this build?"
              confirmLabel="Cancel build"
              description="The running build is stopped. The previous ready image stays in use."
              loading={mutationCancel.isPending}
              onConfirm={() => mutationCancel.mutate()}
            />
          </>
        )}
      </div>
    </div>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);

  const template = ctx.template.data;
  if (!template) return null;

  const buildInfo = template.status?.buildInfo;
  const builds = [...(buildInfo?.builds ?? [])].reverse();

  return (
    <Panel>
      <PanelHeader
        icon={<IconHammer size={16} />}
        title="Builds"
        description="Prebuilding produces an image so Workspaces skip the build step on first run."
        actions={<BuildTemplate item={template} />}
      />
      <PanelBody className="p-3">
        {builds.length === 0 ? (
          <Empty
            compact
            icon={<IconHammer size={22} />}
            title="No builds yet"
            description="Start a build to produce a prebuilt image for this Template."
            action={<BuildTemplate item={template} size="sm" />}
          />
        ) : (
          <Stack gap="sm">
            {buildInfo?.currentRunningBuildID && (
              <Text size="xs" c="dimmed" px="xs">
                A build is currently running.
              </Text>
            )}
            {builds
              .slice(page * itemsPerPage, (page + 1) * itemsPerPage)
              .map((x) => (
                <BuildRow
                  key={x.id}
                  item={x}
                  template={template}
                  isCurrent={x.id === buildInfo?.currentReadyBuildID}
                />
              ))}
            <Paginator
              meta={MetaPB.ListResponseMeta.create({
                totalCount: builds.length,
                itemsPerPage,
                page,
              })}
              onPageChange={setPage}
            />
          </Stack>
        )}
      </PanelBody>
    </Panel>
  );
};

export default Page;
