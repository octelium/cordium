import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import { onError } from "@/utils";

import * as WsPB from "@octelium/apis/main/cordiumv1";

import { useParams } from "react-router-dom";

import { useNavigate } from "react-router-dom";

import { Button } from "@mantine/core";
import { GetOptions } from "@octelium/apis/main/metav1";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";

const Edit = () => {
  let { uid } = useParams();

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getGitProvider", uid],

    queryFn: () => {
      const { response } = getClientWorkspace().getGitProvider(
        GetOptions.create({ uid }),
      );
      return response;
    },
  });

  if (!isSuccess) {
    return <></>;
  }

  let [req, setReq] = React.useState(WsPB.GitProvider.clone(data));

  const item = data;

  const client = getClientWorkspace();

  const queryClient = useQueryClient();

  const navigate = useNavigate();

  const mutationUpdate = useMutation({
    mutationFn: async (req: WsPB.GitProvider) => {
      const { response } = await client.updateGitProvider(req);
    },

    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/getGitProvider", item.metadata?.uid],
      });

      navigate(`/gitproviders/uid/${item.metadata?.uid}`);
      toast.success("Git Provider updated");
    },
    onError,
  });

  return (
    <>
      <div>
        <div></div>

        <div>
          <div className="flex flex-row justify-end items-center">
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
