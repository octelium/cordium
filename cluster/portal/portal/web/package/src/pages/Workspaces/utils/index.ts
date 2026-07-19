import { getClientWorkspace } from "@/utils/client";
import * as MetaPB from "@octelium/apis/main/metav1";
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
