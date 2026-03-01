import * as MetaPB from "@/apis/metav1/metav1";
import { getClientWorkspace } from "@/utils/client";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";

export const useContextWorkspace = () => {
  let { name } = useParams();
  const client = getClientWorkspace();
  const workspace = useQuery({
    queryKey: ["workspace/getWorkspace", name],
    queryFn: () => {
      const { response } = client.getWorkspace(
        MetaPB.GetOptions.create({ name }),
      );
      return response;
    },
    enabled: !!name,
  });

  return {
    workspace,
  };
};
