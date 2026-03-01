import * as MetaPB from "@/apis/metav1/metav1";
import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { getClientWorkspace } from "@/utils/client";
import { getResourceRef } from "@/utils/pb";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";

export const useContextSpace = () => {
  let { spaceName, environmentName, templateName } = useParams();
  const client = getClientWorkspace();
  const space = useQuery({
    queryKey: ["workspace/getSpace", spaceName],
    queryFn: () => {
      const { response } = client.getSpace(
        MetaPB.GetOptions.create({ name: spaceName }),
      );
      return response;
    },
    enabled: !!spaceName,
  });

  const templateFullName = `${templateName}.${spaceName}`;

  const template = useQuery({
    queryKey: ["workspace/getTemplate", templateFullName],
    queryFn: () => {
      const { response } = client.getTemplate(
        MetaPB.GetOptions.create({ name: templateFullName }),
      );
      return response;
    },
    enabled: !!templateName,
  });

  const membership = useQuery({
    queryKey: ["workspace/getSpaceMembership", space.data?.metadata?.uid],
    queryFn: () => {
      const { response } = client.getSpaceMembership(
        WsPB.GetSpaceMembershipRequest.create({
          spaceRef: getResourceRef(space.data!),
        }),
      );
      return response;
    },
    enabled: !!space.data?.metadata?.uid,
  });
  return {
    space,
    membership,
    template,
  };
};
