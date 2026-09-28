import QueryBoundary from "@/components/QueryBoundary";
import { useContextSpace } from "@/pages/Spaces/utils";
import { getClientWorkspace } from "@/utils/client";
import * as MetaPB from "@octelium/apis/main/metav1";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { GitProviderForm } from "../Create";

const EditGitProvider = () => {
  const ctx = useContextSpace();
  const { gitProviderName } = useParams();
  const space = ctx.space.data;
  const name = `${gitProviderName}.${space?.metadata?.name}`;

  const qry = useQuery({
    queryKey: ["workspace/getGitProvider", name],
    queryFn: () => {
      const { response } = getClientWorkspace().getGitProvider(
        MetaPB.GetOptions.create({ name }),
      );
      return response;
    },
    enabled: !!gitProviderName && !!space,
  });

  return (
    <QueryBoundary query={qry}>
      {qry.data && (
        <GitProviderForm
          key={`${qry.data.metadata!.uid}:${qry.data.metadata?.updatedAt?.seconds ?? 0}`}
          item={qry.data}
        />
      )}
    </QueryBoundary>
  );
};

export default EditGitProvider;
