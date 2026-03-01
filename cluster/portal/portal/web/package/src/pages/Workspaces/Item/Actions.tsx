import { useAppDispatch } from "@/utils/hooks";

import { getClientWorkspace } from "@/utils/client";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";

import { useNavigate } from "react-router-dom";

import { DeleteOptions } from "@/apis/metav1/metav1";
import DeleteResource from "@/components/DeleteResource";
import { useMutation } from "@tanstack/react-query";
// import { sendListenEvent } from "@/features/conn/slice";

import { canStopWorkspace } from "@/utils/pb";

import PageWrap from "@/components/PageWrap";
import { invalidateWorkspace } from "@/utils/octelium";
import { useContextWorkspace } from "../utils";

const ActionsBar = (props: { item: WsPB.Workspace }) => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();

  const client = getClientWorkspace();
  const item = props.item;

  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteWorkspace(
        DeleteOptions.create({ uid: props.item.metadata?.uid }),
      );
    },
    onSuccess: () => {
      invalidateWorkspace(item);

      navigate("/workspaces");
    },
  });

  const canStop = canStopWorkspace(item);

  return (
    <div>
      <div className="flex items-center">
        <div className="flex flex-1 items-center"></div>
        <div className="flex flex-none">
          <DeleteResource
            onDelete={() => {
              mutationDelete.mutate();
            }}
          />
        </div>
      </div>
    </div>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <ActionsBar item={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Page;
