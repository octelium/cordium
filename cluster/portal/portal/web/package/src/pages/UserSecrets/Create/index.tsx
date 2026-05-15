import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import Field from "@/components/Field";
import ItemContainer from "@/components/ItemContainer";
import MetadataEdit from "@/components/MetadataEdit";
import { onError } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { Button, Select } from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateSecret = () => {
  let [req, setReq] = React.useState(
    WsPB.UserSecret.create({
      apiVersion: "cordium/v1",
      kind: "UserSecret",
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
      const { response } = await client.createUserSecret(req);

      return response;
    },
    onSuccess: (data) => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listUserSecret/"],
      });
      toast.success("Your Secret created");
      navigate(`/usersecrets/uid/${data.metadata?.uid}`);
    },
    onError: onError,
  });

  const user = useAppSelector((a) => a.settings.status?.user);

  return (
    <div>
      <div className="w-full">
        <MetadataEdit
          metadata={req.metadata!}
          onUpdate={(itm) => {
            req.metadata = itm;
            setReq(req);
          }}
          parentName={user?.metadata?.name}
          skipDisplayName
        />

        <ItemContainer title="Type" isHorizontal>
          <Select
            data={[
              {
                label: "Default",
                value:
                  WsPB.UserSecret_Spec_Type[WsPB.UserSecret_Spec_Type.DEFAULT],
              },
              {
                label: "SSH Key",
                value:
                  WsPB.UserSecret_Spec_Type[WsPB.UserSecret_Spec_Type.SSH_KEY],
              },
            ]}
            defaultValue={WsPB.UserSecret_Spec_Type[req.spec!.type]}
            onChange={(val) => {
              req.spec!.type = WsPB.UserSecret_Spec_Type[val as "DEFAULT"];
              setReq(WsPB.UserSecret.clone(req));
            }}
          />
        </ItemContainer>

        {req.spec?.type === WsPB.UserSecret_Spec_Type.DEFAULT &&
          req.data!.type.oneofKind === `value` && (
            <Field
              val={req.data!.type.value}
              label="Value"
              placeholder="TOP SECRET"
              isRequired
              multiLine
              rows={3}
              maxRows={3}
              onChange={(v) => {
                req.data!.type = {
                  oneofKind: "value",
                  value: v as string,
                };

                setReq(WsPB.UserSecret.clone(req));
              }}
            />
          )}
      </div>
      <div className="flex items-center justify-end mt-4">
        <Button
          variant="outline"
          onClick={() => {
            navigate(-1);
          }}
        >
          Cancel
        </Button>
        <Button
          onClick={() => {
            mutation.mutate();
          }}
        >
          Create
        </Button>
      </div>
    </div>
  );
};

export default CreateSecret;
