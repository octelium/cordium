import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import MetadataEdit from "@/components/MetadataEdit";
import PageWrap from "@/components/PageWrap";
import WorkspaceEdit from "@/components/WorkspaceEdit";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError, queryClient } from "@/utils";
import { getPathTemplate } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { Button } from "@mantine/core";
import { useMutation } from "@tanstack/react-query";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateTemplate = () => {
  const ctx = useContextSpace();

  if (!ctx.space.isSuccess) {
    return <></>;
  }

  let [req, setReq] = React.useState(
    WsPB.Template.create({
      apiVersion: "workspace/v1",
      kind: "Template",
      metadata: {},
      spec: {},
      status: {
        spaceRef: getResourceRef(ctx.space.data),
      },
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createTemplate(req);

      return response;
    },

    onSuccess: (data) => {
      navigate(getPathTemplate(data));
      queryClient.invalidateQueries({
        queryKey: ["workspace/listTemplate", data.status?.spaceRef?.uid, 0],
      });
      toast.success(`Template ${data.metadata?.name} created`);
    },
    onError: onError,
  });

  return (
    <PageWrap qry={ctx.space} title="Create a Template">
      {ctx.space.data && (
        <div>
          <div>
            <MetadataEdit
              metadata={req.metadata!}
              onUpdate={(itm) => {
                req.metadata = itm;
                setReq(WsPB.Template.clone(req));
              }}
              parentName={ctx.space.data.metadata?.name}
            />
          </div>
          <WorkspaceEdit
            spaceRef={getResourceRef(ctx.space.data)}
            item={req}
            onUpdate={(itm) => {
              const item = itm as WsPB.Template;
              req.spec = item.spec;
              setReq(WsPB.Template.clone(req));
            }}
          />

          <div className="flex items-center justify-end mt-8">
            <Button
              size="lg"
              variant="outline"
              onClick={() => {
                navigate(-1);
              }}
            >
              Cancel
            </Button>
            <Button
              className="ml-4"
              size="lg"
              onClick={() => {
                mutation.mutate();
              }}
            >
              Create Template
            </Button>
          </div>
        </div>
      )}
    </PageWrap>
  );
};

export default CreateTemplate;
