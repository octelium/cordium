import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import Field from "@/components/Field";
import MetadataEdit from "@/components/MetadataEdit";
import PageWrap from "@/components/PageWrap";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { Button } from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateSecret = () => {
  const ctx = useContextSpace();

  if (!ctx.space.isSuccess) {
    return <></>;
  }

  const data = ctx.space.data;

  let [req, setReq] = React.useState(
    WsPB.Secret.create({
      apiVersion: "workspace/v1",
      kind: "Secret",
      metadata: {},
      spec: {},
      status: {},
      data: {
        type: {
          oneofKind: "value",
          value: "",
        },
      },
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: async () => {
      req.status!.spaceRef = getResourceRef(data!);
      const { response } = await client.createSecret(req);

      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listSecret", data?.metadata?.uid, 0],
      });

      toast.success("Secret created");
      navigate(getPathSpace(data));
    },
    onError: onError,
  });

  return (
    <PageWrap qry={ctx.space} title="Create a Secret">
      <div className="w-full">
        <MetadataEdit
          metadata={req.metadata!}
          onUpdate={(itm) => {
            req.metadata = itm;
            setReq(req);
          }}
          parentName={data.metadata?.name}
        />

        {req.data!.type.oneofKind === `value` && (
          <Field
            val={req.data!.type.value}
            label="Value"
            placeholder="TOP SECRET"
            isRequired
            rows={3}
            onChange={(v) => {
              req.data!.type = {
                oneofKind: "value",
                value: v as string,
              };

              setReq(WsPB.Secret.clone(req));
            }}
          />
        )}
      </div>
      <div className="flex items-center justify-end mt-4">
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
          loading={mutation.isPending}
          onClick={() => {
            mutation.mutate();
          }}
        >
          Create
        </Button>
      </div>
    </PageWrap>
  );
};

export default CreateSecret;
