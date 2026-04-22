import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import { onError } from "@/utils";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";

import { useNavigate } from "react-router-dom";

import PageWrap from "@/components/PageWrap";
import WorkspaceEditC from "@/components/WorkspaceEdit";
import { getPathWorkspace, invalidateResource } from "@/utils/octelium";
import { Button } from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";
import { useContextWorkspace } from "../../utils";

export const WorkspaceEdit = (props: { item: WsPB.Workspace }) => {
  let [req, setReq] = React.useState<WsPB.Workspace>(props.item);
  const data = props.item;

  React.useEffect(() => {
    if (data) {
      setReq(WsPB.Workspace.clone(data));
    }
  }, [data]);

  const client = getClientWorkspace();

  const queryClient = useQueryClient();

  const navigate = useNavigate();

  const mutationUpdate = useMutation({
    mutationFn: async (req: WsPB.Workspace) => {
      const { response } = await client.updateWorkspace(req);
      return response;
    },

    onSuccess: (response) => {
      invalidateResource(response);

      navigate(getPathWorkspace(response));
      toast.success("Workspace updated");
    },
    onError,
  });

  if (!req) {
    return <></>;
  }

  return (
    <>
      <div>
        <div>
          <WorkspaceEditC
            spaceRef={data.status!.spaceRef!}
            item={req}
            onUpdate={(item) => {
              let v = item as WsPB.Workspace;
              if (req) {
                let reqClone = WsPB.Workspace.clone(req);
                reqClone.spec = v.spec;
                setReq(reqClone);
              }
            }}
          />
        </div>

        <div className="mt-8">
          <div className="flex flex-row justify-end items-center">
            <Button
              variant="outline"
              className="mr-2"
              onClick={() => {
                navigate(-1);
              }}
            >
              Cancel
            </Button>

            <Button
              loading={mutationUpdate.isPending}
              onClick={() => {
                if (req) {
                  mutationUpdate.mutate(req);
                }
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

const Edit = () => {
  const ctx = useContextWorkspace();
  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <WorkspaceEdit item={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Edit;
