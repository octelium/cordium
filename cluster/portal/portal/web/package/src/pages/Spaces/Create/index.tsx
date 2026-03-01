import * as React from "react";

import * as WsPB from "../../../apis/cordiumv1/cordiumv1";

import { getClientWorkspace } from "../../../utils/client";

import { onError } from "@/utils";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import MetadataEdit from "@/components/MetadataEdit";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateSpace } from "@/utils/octelium";
import { Button } from "@mantine/core";

const CreateSpace = () => {
  let [req, setReq] = React.useState(
    WsPB.Space.create({
      metadata: {},
      spec: {},
      status: {
        type: WsPB.Space_Status_Type.USER,
      },
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const user = useAppSelector((a) => a.settings.status?.user);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createSpace(req);

      return response;
    },
    onSuccess: (data) => {
      invalidateSpace(data);
      navigate(getPathSpace(data));
    },
    onError: onError,
  });

  return (
    <div>
      <div>
        <MetadataEdit
          metadata={req.metadata!}
          onUpdate={(itm) => {
            req.metadata = itm;
            setReq(req);
          }}
          parentName={
            req.status?.type === WsPB.Space_Status_Type.USER
              ? user?.metadata?.name
              : "cordium"
          }
        />
      </div>

      {/**
       <div>
        <Select
          label="Type"
          data={[
            {
              label: "User",
              value: WsPB.Space_Status_Type[WsPB.Space_Status_Type.USER],
            },
            {
              label: "Organization",
              value:
                WsPB.Space_Status_Type[WsPB.Space_Status_Type.ORGANIZATION],
            },
          ]}
          value={WsPB.Space_Status_Type[req.status!.type]}
          onChange={(val) => {
            if (!val) {
              return;
            }
            req.status!.type =
              WsPB.Space_Status_Type[val as "ORGANIZATION" | "PERSONAL"];
            setReq(WsPB.Space.clone(req));
          }}
        />
      </div>
       **/}

      <div className="flex items-center justify-end mt-4">
        <Button
          variant="outline"
          size="lg"
          onClick={() => {
            navigate(-1);
          }}
        >
          Cancel
        </Button>
        <Button
          loading={mutation.isPending}
          size="lg"
          className="ml-2"
          onClick={() => {
            mutation.mutate();
          }}
        >
          Create Space
        </Button>
      </div>
    </div>
  );
};

export default CreateSpace;
