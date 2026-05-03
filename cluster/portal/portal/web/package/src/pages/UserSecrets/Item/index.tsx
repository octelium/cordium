import { DeleteOptions, GetOptions } from "@/apis/metav1/metav1";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { getClientWorkspace } from "../../../utils/client";

import { UserSecret_Spec_Type } from "@/apis/cordiumv1/cordiumv1";
import CopyText from "@/components/CopyText";
import DeleteResource from "@/components/DeleteResource";
import InfoItem from "@/components/InfoItem";
import Label from "@/components/Label";
import TimeAgo from "@/components/TimeAgo";
import { onError } from "@/utils";
import { getShortName } from "@/utils/pb";

const Page = () => {
  let { uid } = useParams();

  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getUserSecret", uid],
    queryFn: () => {
      const { response } = client.getUserSecret(GetOptions.create({ uid }));
      return response;
    },
  });

  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteUserSecret(
        DeleteOptions.create({
          uid: data?.metadata?.uid,
        }),
      );

      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listUserSecret", 0],
      });
      queryClient.refetchQueries({
        queryKey: ["workspace/listUserSecret/"],
      });
      navigate(`/usersecrets`);
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
            {data.spec?.type === UserSecret_Spec_Type.SSH_KEY && (
              <InfoItem title="Type">
                <Label>SSH Key</Label>
              </InfoItem>
            )}
            {data.status?.details.oneofKind === `sshKey` && (
              <InfoItem title="Public Key">
                <div>
                  <CopyText
                    value={data.status.details.sshKey.publicKey}
                    truncate={64}
                  />
                </div>
              </InfoItem>
            )}
          </div>
          <div className="flex justify-end mt-6">
            <DeleteResource
              onDelete={() => {
                mutationDelete.mutate();
              }}
            />
          </div>
        </div>
      </div>
    </>
  );
};

export default Page;
