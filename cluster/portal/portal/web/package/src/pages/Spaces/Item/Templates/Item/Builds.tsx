import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import { useContextSpace } from "@/pages/Spaces/utils";
import { useAppSelector } from "@/utils/hooks";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as MetaPB from "@/apis/metav1/metav1";
import BuildTemplate from "@/components/BuildTemplate";

import EmptyList from "@/components/EmptyList";
import Label from "@/components/Label";
import PageWrap from "@/components/PageWrap";
import Paginator from "@/components/Paginator";
import { ResourceListItem } from "@/components/ResourceList";
import TimeAgo from "@/components/TimeAgo";
import { onError } from "@/utils";
import { invalidateTemplate } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { Button } from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";
import ClipLoader from "react-spinners/ClipLoader";
const Item = (props: {
  item: WsPB.Template_Status_BuildInfo_Build;
  template: WsPB.Template;
}) => {
  const { item, template } = props;
  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const mutationCancel = useMutation({
    mutationFn: async () => {
      const { response } = await client.cancelBuildTemplate(
        WsPB.CancelBuildTemplateRequest.create({
          templateRef: getResourceRef(template),
        }),
      );

      return { response };
    },
    onSuccess: ({ response }) => {
      invalidateTemplate(template);
      toast.success("Build canceled");
    },
    onError,
  });

  return (
    <ResourceListItem key={item.id}>
      <div className="flex flex-row items-center justify-center">
        <div className="flex-1">
          {/*
          <InfoItem title="ID">
            <div className="flex flex-row items-center">
              <div className="text-gray-500">{item.id}</div>
            </div>
          </InfoItem>
          */}

          <div className="font-bold text-sm">
            <div className="flex flex-row items-center">
              <div className="text-gray-600 mr-2">{item.id}</div>
              {item.isCanceled && (
                <div className="flex flex-row items-center">
                  <div
                    style={{
                      backgroundColor: `#999`,
                    }}
                    className={`rounded-full w-[20px] h-[20px]`}
                  ></div>
                  <span className="mx-2">Cancelled</span>

                  <span className="text-slate-500">
                    <TimeAgo rfc3339={item.doneAt} />
                  </span>
                </div>
              )}

              {item.state ===
                WsPB.Template_Status_BuildInfo_Build_State.READY && (
                <div className="flex flex-row items-center">
                  <div
                    style={{
                      backgroundColor: `#1cc02a`,
                    }}
                    className={`rounded-full w-[20px] h-[20px]`}
                  ></div>
                  <span className="mx-2">Ready</span>

                  <span className="text-slate-500">
                    <TimeAgo rfc3339={item.doneAt} />
                  </span>
                </div>
              )}

              {!item.isCanceled &&
                item.state ==
                  WsPB.Template_Status_BuildInfo_Build_State.RUNNING && (
                  <div className="flex flex-row items-center">
                    <ClipLoader color={`#777`} loading={true} size={20} />
                    <span className="mx-2">Running</span>

                    {item.startedAt && (
                      <span className="text-slate-500">
                        <TimeAgo rfc3339={item.startedAt} />
                      </span>
                    )}

                    <div className="ml-4">
                      <Button
                        size="xs"
                        variant="outline"
                        onClick={() => {
                          mutationCancel.mutate();
                        }}
                      >
                        Cancel Build
                      </Button>
                    </div>
                  </div>
                )}
            </div>
          </div>

          {item.tags.length > 0 && (
            <div>
              {item.tags.map((x) => (
                <Label>{x}</Label>
              ))}
            </div>
          )}
        </div>
      </div>
    </ResourceListItem>
  );
};

const ListBuild = (props: { item: WsPB.Template }) => {
  const { item } = props;

  const settings = useAppSelector((state) => state.settings);
  const itemsPerPage = settings.itemsPerPage ?? 10;
  let [page, setPage] = React.useState(0);
  const client = getClientWorkspace();

  const bldArr = item.status?.buildInfo?.builds;
  if (!bldArr || bldArr.length < 1) {
    return <EmptyList title="No Builds Found" />;
  }

  return (
    <div>
      {
        <div>
          <div className="font-bold text-lg mb-8 flex items-center justify-center">
            <span>Template Builds</span>
            <Label>Total: {bldArr.length}</Label>
          </div>

          <div className="mt-4">
            {bldArr
              .slice(page * itemsPerPage, (page + 1) * itemsPerPage)
              .map((x) => (
                <Item item={x} template={item} />
              ))}
          </div>

          <div>
            <Paginator
              meta={MetaPB.ListResponseMeta.create({
                totalCount: bldArr.length,
                itemsPerPage,
                page,
              })}
              onPageChange={(i) => {
                setPage(i);
              }}
            />
          </div>
        </div>
      }
    </div>
  );
};

const Page = () => {
  const ctx = useContextSpace();

  return (
    <PageWrap qry={ctx.template}>
      <div>
        {ctx.template.data && (
          <div>
            <div className="my-8 flex items-center justify-center">
              <BuildTemplate item={ctx.template.data} />
            </div>
            <ListBuild item={ctx.template.data} />
          </div>
        )}
      </div>
    </PageWrap>
  );
};

export default Page;
