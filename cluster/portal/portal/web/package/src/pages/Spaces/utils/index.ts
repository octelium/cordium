import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getSpaceResourceName } from "@/utils/octelium";
import { getResourceRef, isMemberAdmin, isMemberOwner } from "@/utils/pb";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";

export const useContextSpace = () => {
  const { spaceName, templateName } = useParams();
  const client = getClientWorkspace();
  const userName = useAppSelector(
    (state) => state.settings.status?.user?.metadata?.name,
  );
  const resourceSpaceName = spaceName
    ? getSpaceResourceName(spaceName, userName)
    : undefined;

  const space = useQuery({
    queryKey: ["workspace/getSpace", resourceSpaceName],
    queryFn: () => {
      const { response } = client.getSpace(
        MetaPB.GetOptions.create({ name: resourceSpaceName }),
      );
      return response;
    },
    enabled: !!resourceSpaceName,
  });

  const template = useQuery({
    queryKey: ["workspace/getTemplate", `${templateName}.${resourceSpaceName}`],
    queryFn: () => {
      const { response } = client.getTemplate(
        MetaPB.GetOptions.create({
          name: `${templateName}.${resourceSpaceName}`,
        }),
      );
      return response;
    },
    enabled: !!templateName && !!resourceSpaceName,
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
    template,
    membership,
    isAdmin: membership.isSuccess && isMemberAdmin(membership.data),
    isOwner: membership.isSuccess && isMemberOwner(membership.data),
  };
};

export const useSpaceCounts = (spaceRef?: MetaPB.ObjectReference) => {
  const client = getClientWorkspace();
  const enabled = !!spaceRef?.uid;
  const common = { page: 0, itemsPerPage: 1 };

  const workspaces = useQuery({
    queryKey: ["workspace/listWorkspace", spaceRef?.uid, "count"],
    queryFn: () => {
      const { response } = client.listWorkspace(
        WsPB.ListWorkspaceOptions.create({
          filter: { oneofKind: "spaceRef", spaceRef: spaceRef! },
          common,
        }),
      );
      return response;
    },
    enabled,
  });

  const templates = useQuery({
    queryKey: ["workspace/listTemplate", spaceRef?.uid, "count"],
    queryFn: () => {
      const { response } = client.listTemplate(
        WsPB.ListTemplateOptions.create({ spaceRef: spaceRef!, common }),
      );
      return response;
    },
    enabled,
  });

  const secrets = useQuery({
    queryKey: ["workspace/listSecret", spaceRef?.uid, "count"],
    queryFn: () => {
      const { response } = client.listSecret(
        WsPB.ListSecretOptions.create({ spaceRef: spaceRef!, common }),
      );
      return response;
    },
    enabled,
  });

  const members = useQuery({
    queryKey: ["workspace/listMembership", spaceRef?.uid, "count"],
    queryFn: () => {
      const { response } = client.listMembership(
        WsPB.ListMembershipOptions.create({ spaceRef: spaceRef!, common }),
      );
      return response;
    },
    enabled,
  });

  return { workspaces, templates, secrets, members };
};
