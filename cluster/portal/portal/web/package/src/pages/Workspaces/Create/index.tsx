import { getClientWorkspace } from "@/utils/client";
import * as React from "react";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import Field from "@/components/Field";
import ItemContainer from "@/components/ItemContainer";

import { useNavigate, useSearchParams } from "react-router-dom";

import { GetOptions } from "@/apis/metav1/metav1";
import WorkspaceEdit from "@/components/WorkspaceEdit";
import { onError } from "@/utils";
import { getResourceRef } from "@/utils/pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import Switch from "@/components/Switch";
import { getPathWorkspace, invalidateWorkspace } from "@/utils/octelium";
import { Button } from "@mantine/core";

const CreateWorkspace = () => {
  let [searchParams, _] = useSearchParams();

  let [doStart, setDoStart] = React.useState(true);

  let [req, setReq] = React.useState(
    WsPB.Workspace.create({
      apiVersion: "cordium/v1",
      kind: "Workspace",
      metadata: {},
      spec: {},
      status: {},
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const uid = searchParams.get("templateUID")!;
  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid }),
      );
      return response;
    },
    enabled: !!searchParams.get("templateUID"),
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

      if (doStart) {
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

      <ItemContainer title="Ephemeral Storage" isHorizontal>
        <Switch
          val={(req as WsPB.Workspace).spec?.isEphemeral}
          onChange={(v) => {
            (req as WsPB.Workspace).spec!.isEphemeral = v;
            setReq(WsPB.Workspace.clone(req));
          }}
        />
      </ItemContainer>

      <div className="mt-4">
        <ItemContainer title="Start after Creation" isHorizontal>
          <Switch
            val={doStart}
            onChange={(v) => {
              setDoStart(v);
            }}
          />
        </ItemContainer>
      </div>

      <div>
        <div className="flex flex-row items-center justify-end">
          <Button
            variant="outline"
            onClick={() => {
              navigate("/workspaces");
            }}
          >
            Cancel
          </Button>
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

const Page = () => {
  return (
    <>
      <div>
        <CreateWorkspace />
      </div>
    </>
  );
};

export default Page;
