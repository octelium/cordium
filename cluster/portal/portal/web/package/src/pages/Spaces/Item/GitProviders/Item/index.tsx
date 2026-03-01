import { DeleteOptions, GetOptions } from "@/apis/metav1/metav1";
import { getClientWorkspace } from "@/utils/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";

import { onError } from "@/utils";
import { getShortName } from "@/utils/pb";

import DeleteResource from "@/components/DeleteResource";
import InfoItem from "@/components/InfoItem";
import TimeAgo from "@/components/TimeAgo";

const Page = () => {
  let { uid } = useParams();

  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getGitProvider", uid],
    queryFn: () => {
      const { response } = client.getGitProvider(GetOptions.create({ uid }));
      return response;
    },
  });

  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteGitProvider(
        DeleteOptions.create({
          uid: data?.metadata?.uid,
        }),
      );

      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listGitProvider", data?.status?.spaceRef?.uid, 0],
      });
      navigate(`/spaces/uid/${data?.status?.spaceRef?.uid}`);
    },
    onError: onError,
  });

  if (!isSuccess) {
    return <></>;
  }

  return (
    <>
      <div>
        <div>
          <div className="w-full">
            <InfoItem title="Name">{getShortName(data)}</InfoItem>
            {data.metadata?.displayName && (
              <InfoItem title="Display Name">
                {data.metadata?.displayName}
              </InfoItem>
            )}
            <InfoItem title="Created">
              <TimeAgo rfc3339={data.metadata?.createdAt} />
            </InfoItem>

            {data.spec?.type.oneofKind === `github` && (
              <>
                <InfoItem title="Type">GitHub</InfoItem>
                <InfoItem title="Client ID">
                  {data.spec.type.github.clientID}
                </InfoItem>
              </>
            )}

            {data.spec?.type.oneofKind === `gitlab` && (
              <>
                <InfoItem title="Type">Gitlab</InfoItem>
                <InfoItem title="Client ID">
                  {data.spec.type.gitlab.clientID}
                </InfoItem>
              </>
            )}

            {data.spec?.type.oneofKind === `oauth2` && (
              <>
                <InfoItem title="Type">Generic OAuth2 </InfoItem>
                <InfoItem title="Client ID">
                  {data.spec.type.oauth2.clientID}
                </InfoItem>
              </>
            )}
          </div>
          <div className="flex">
            {
              <DeleteResource
                onDelete={() => {
                  mutationDelete.mutate();
                }}
              />
            }
          </div>
        </div>
      </div>
    </>
  );
};

export default Page;
