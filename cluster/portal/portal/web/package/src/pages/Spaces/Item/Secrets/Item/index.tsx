import DeleteResource from "@/components/DeleteResource";
import InfoItem from "@/components/InfoItem";
import TimeAgo from "@/components/TimeAgo";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getShortName } from "@/utils/pb";
import {
  GetSpaceMembershipRequest,
  Membership_Spec_Role,
} from "@octelium/apis/main/cordiumv1";
import { DeleteOptions, GetOptions } from "@octelium/apis/main/metav1";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";

const Page = () => {
  let { uid } = useParams();

  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getSecret", uid],
    queryFn: () => {
      const { response } = client.getSecret(GetOptions.create({ uid }));
      return response;
    },
  });

  const qryMem = useQuery({
    queryKey: ["workspace/getSpaceMembership", data?.status?.spaceRef?.uid],
    queryFn: () => {
      const { response } = client.getSpaceMembership(
        GetSpaceMembershipRequest.create({ spaceRef: data?.status?.spaceRef }),
      );
      return response;
    },
    enabled: !!data?.metadata?.uid,
  });

  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteSecret(
        DeleteOptions.create({
          uid: data?.metadata?.uid,
        }),
      );

      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listSecret", data?.status?.spaceRef?.uid, 0],
      });
      queryClient.refetchQueries({
        queryKey: ["workspace/listSecret/"],
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
          </div>
          <div className="flex">
            {qryMem.isSuccess &&
              (qryMem.data.spec?.role === Membership_Spec_Role.ADMIN ||
                qryMem.data.spec?.role === Membership_Spec_Role.OWNER) && (
                <DeleteResource
                  onDelete={() => {
                    mutationDelete.mutate();
                  }}
                />
              )}
          </div>
        </div>
      </div>
    </>
  );
};

export default Page;
