import { LeaveSpaceRequest } from "@/apis/cordiumv1/cordiumv1";
import { DeleteOptions } from "@/apis/metav1/metav1";

import DeleteResource from "@/components/DeleteResource";
import PageWrap from "@/components/PageWrap";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { invalidateSpace } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { Button } from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useContextSpace } from "../utils";

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const client = getClientWorkspace();
  const data = ctx.space.data;
  const settings = useAppSelector((state) => state.settings);
  const queryClient = useQueryClient();
  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteSpace(
        DeleteOptions.create({ uid: data?.metadata?.uid }),
      );
      return data;
    },
    onSuccess: (data) => {
      if (data) {
        invalidateSpace(data);
      }

      navigate("/spaces");
    },
  });

  const mutationLeave = useMutation({
    mutationFn: async () => {
      const { response } = await client.leaveSpace(
        LeaveSpaceRequest.create({
          spaceRef: data ? getResourceRef(data) : undefined,
        }),
      );

      return response;
    },
    onSuccess: () => {
      if (data) {
        invalidateSpace(data);
      }
      navigate(`/spaces`);
    },
    onError: onError,
  });

  return (
    <PageWrap qry={ctx.space}>
      {data && (
        <div>
          <div className="flex items-center justify-end">
            <div className="flex flex-1 items-center justify-end">
              {data.status?.userRef?.uid !==
                settings.status?.user?.metadata?.uid && (
                <Button
                  size="small"
                  variant="outline"
                  onClick={() => {
                    mutationLeave.mutate();
                  }}
                >
                  Leave Space
                </Button>
              )}
              <div>
                <DeleteResource
                  onDelete={() => {
                    mutationDelete.mutate();
                  }}
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
