import { getClientWorkspace } from "@/utils/client";
import * as React from "react";

import Field from "@/components/Field";
import ItemContainer from "@/components/ItemContainer";
import * as WsPB from "@octelium/apis/main/cordiumv1";

import { useNavigate } from "react-router-dom";

import WorkspaceEdit from "@/components/WorkspaceEdit";
import { onError } from "@/utils";
import { getResourceRef } from "@/utils/pb";
import { GetOptions } from "@octelium/apis/main/metav1";
import { useMutation, useQuery } from "@tanstack/react-query";

import { getPathWorkspace, invalidateWorkspace } from "@/utils/octelium";
import { Button } from "@mantine/core";

const CreateWorkspace = (props: {
  item: WsPB.Workspace;
  doStart?: boolean;
}) => {
  let [req, setReq] = React.useState(WsPB.Workspace.clone(props.item));

  const client = getClientWorkspace();
  const navigate = useNavigate();

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", req.status?.templateRef?.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: req.status?.templateRef?.uid }),
      );
      return response;
    },
    enabled: !!req.status?.templateRef?.uid,
  });

  React.useEffect(() => {
    if (qryTemplate.data) {
      req.status!.templateRef = getResourceRef(qryTemplate.data!);
      setReq(WsPB.Workspace.clone(req));
    }
  }, [qryTemplate.data]);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createWorkspace(req);

      const uid = response.metadata!.uid;

      if (props.doStart) {
        await client.startWorkspace(
          WsPB.StartWorkspaceRequest.create({
            workspaceRef: getResourceRef(response),
          }),
        );
      }

      return { response };
    },
    onSuccess: ({ response }) => {
      invalidateWorkspace(response);
      navigate(getPathWorkspace(response));
    },
    onError,
  });

  if (!qryTemplate.isSuccess) {
    return <></>;
  }

  return (
    <div>
      <div>
        <ItemContainer title="Display Name">
          <Field
            val={req.metadata!.displayName}
            label="Display Name"
            onChange={(v) => {
              req.metadata!.displayName = v as string;
              setReq(WsPB.Workspace.clone(req));
            }}
          />
        </ItemContainer>
      </div>
      <div>
        <WorkspaceEdit
          spaceRef={qryTemplate.data!.status!.spaceRef!}
          item={req}
          onUpdate={(item) => {
            let v = item as WsPB.Workspace;
            let reqClone = WsPB.Workspace.clone(req);
            reqClone.spec = v.spec;
            setReq(reqClone);
          }}
        />
      </div>

      <div>
        <div className="flex flex-row items-center justify-end">
          <Button
            onClick={() => {
              mutation.mutate();
            }}
          >
            Create Workspace
          </Button>
        </div>
      </div>
    </div>
  );
};

export default CreateWorkspace;
