import ConfirmAction from "@/components/ConfirmAction";
import Empty from "@/components/Empty";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, ClickableCard } from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateMemberships } from "@/utils/octelium";
import { getResourceRef, getRoleLabel, getShortName } from "@/utils/pb";
import { Alert, Avatar, Button, Select, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { IconTrash, IconUserPlus, IconUsers } from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const Role = WsPB.Membership_Spec_Role;

const roleOptions = [
  { value: Role[Role.USER], label: "Member — can create Workspaces" },
  { value: Role[Role.ADMIN], label: "Admin — can manage Space resources" },
  { value: Role[Role.OWNER], label: "Owner — full control" },
];

const MemberRow = (props: {
  item: WsPB.Membership;
  canManage: boolean;
  isSelf: boolean;
}) => {
  const { item } = props;
  const client = getClientWorkspace();

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteMembership(
        MetaPB.DeleteOptions.create({ uid: item.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateMemberships();
      toast.success("Member removed");
    },
    onError,
  });

  const mutationUpdate = useMutation({
    mutationFn: async (role: WsPB.Membership_Spec_Role) => {
      const next = WsPB.Membership.clone(item);
      next.spec!.role = role;
      const { response } = await client.updateMembership(next);
      return response;
    },
    onSuccess: () => {
      invalidateMemberships();
      toast.success("Role updated");
    },
    onError,
  });

  const displayName = item.status?.userInfo?.displayName;
  const userName = item.status?.userRef?.name ?? "";

  return (
    <ClickableCard>
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <Avatar
          src={item.status?.userInfo?.picURL || undefined}
          radius="xl"
          size={36}
          color="dark"
        >
          {(displayName || userName).slice(0, 2).toUpperCase()}
        </Avatar>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className="truncate text-sm font-bold text-slate-800">
              {displayName || userName}
            </span>
            {displayName && (
              <span className="truncate font-mono text-[0.72rem] font-medium text-slate-400">
                {userName}
              </span>
            )}
            {props.isSelf && <Tag tone="neutral">You</Tag>}
          </div>
          <div className="mt-1">
            <Tag tone={item.spec!.role === Role.OWNER ? "accent" : "neutral"}>
              {getRoleLabel(item.spec!.role)}
            </Tag>
          </div>
        </div>

        {props.canManage && (
          <div className="flex shrink-0 items-center gap-2">
            <Select
              size="xs"
              w={190}
              aria-label="Role"
              allowDeselect={false}
              data={roleOptions}
              value={Role[item.spec!.role]}
              disabled={mutationUpdate.isPending}
              onChange={(val) => {
                if (!val) return;
                mutationUpdate.mutate(
                  Role[val as keyof typeof Role] as WsPB.Membership_Spec_Role,
                );
              }}
            />
            <ConfirmAction
              triggerLabel="Remove"
              triggerIcon={<IconTrash size={13} />}
              title="Remove this member?"
              confirmLabel="Remove member"
              description="They immediately lose access to this Space, its Templates and Secrets. Their existing Workspaces are not deleted."
              loading={mutationDelete.isPending}
              onConfirm={() => mutationDelete.mutate()}
            />
          </div>
        )}
      </div>
    </ClickableCard>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const userUID = useAppSelector((s) => s.settings.status?.user?.metadata?.uid);
  const [page, setPage] = React.useState(0);
  const space = ctx.space.data;

  const qry = useQuery({
    queryKey: [
      "workspace/listMembership",
      space?.metadata?.uid,
      page,
      itemsPerPage,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listMembership(
        WsPB.ListMembershipOptions.create({
          spaceRef: getResourceRef(space!),
          common: { page, itemsPerPage },
        }),
      );
      return response;
    },
    enabled: !!space,
  });

  if (!space) return null;

  return (
    <Stack gap="lg">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <Text size="sm" fw={700}>
            Members of {getShortName(space)}
          </Text>
          <Text size="xs" c="dimmed">
            Members can launch Workspaces from this Space's Templates. Admins
            can also manage Secrets, Templates and Git providers.
          </Text>
        </div>
        {ctx.isAdmin && (
          <Button
            size="xs"
            leftSection={<IconUserPlus size={14} />}
            onClick={() =>
              navigate(`${getPathSpace(space)}/memberships/create`)
            }
          >
            Add member
          </Button>
        )}
      </div>

      {!ctx.isAdmin && (
        <Alert color="gray" variant="light">
          Only Space admins can add or remove members.
        </Alert>
      )}

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconUsers size={22} />}
                title="No members yet"
                description="Invite teammates so they can launch Workspaces here."
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <MemberRow
                    key={x.metadata?.uid}
                    item={x}
                    canManage={ctx.isAdmin}
                    isSelf={x.status?.userRef?.uid === userUID}
                  />
                ))}
              </CardList>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>
    </Stack>
  );
};

export default Page;
