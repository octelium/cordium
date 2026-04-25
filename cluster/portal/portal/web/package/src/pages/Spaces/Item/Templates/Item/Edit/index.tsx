import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import { onError } from "@/utils";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";

import { useNavigate } from "react-router-dom";

import Meta from "@/components/Meta";
import WorkspaceEdit from "@/components/WorkspaceEdit";
import { useContextSpace } from "@/pages/Spaces/utils";
import { getPathTemplate } from "@/utils/octelium";
import { Button } from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";

const Edit = () => {
  const ctx = useContextSpace();
  if (!ctx.template.isSuccess) {
    return <></>;
  }

  const data = ctx.template.data;

  let [req, setReq] = React.useState(WsPB.Template.clone(data));

  const item = data;

  const client = getClientWorkspace();

  const queryClient = useQueryClient();

  const navigate = useNavigate();

  const mutationUpdate = useMutation({
    mutationFn: async (req: WsPB.Template) => {
      const { response } = await client.updateTemplate(req);
    },

    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/getTemplate", item.metadata?.uid],
      });

      navigate(getPathTemplate(item));
      toast.success("Template updated");
    },
    onError,
  });

  return (
    <>
      <div>
        <Meta title="Template Config" />
        <div>
          <WorkspaceEdit
            spaceRef={data.status!.spaceRef!}
            item={req}
            onUpdate={(item) => {
              let v = item as WsPB.Template;
              let reqClone = WsPB.Template.clone(req);
              reqClone.spec = v.spec;
              setReq(reqClone);
            }}
          />
        </div>

        <div>
          <div className="flex flex-row justify-end items-center mt-8">
            <Button
              size="lg"
              variant="outline"
              className="mr-2"
              onClick={() => {
                navigate(-1);
              }}
            >
              Cancel
            </Button>

            <Button
              size="lg"
              onClick={() => {
                mutationUpdate.mutate(req);
              }}
            >
              Update
            </Button>
          </div>
        </div>
      </div>
    </>
  );
};

export default Edit;
