import ConfirmAction from "@/components/ConfirmAction";
import Empty from "@/components/Empty";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, CardTitle, ClickableCard } from "@/components/ResourceCards";
import TimeAgo from "@/components/TimeAgo";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateSecrets } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Alert, Button, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { IconKey, IconPlus, IconTrash } from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const SecretRow = (props: { item: WsPB.Secret; canManage: boolean }) => {
  const { item } = props;
  const client = getClientWorkspace();

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteSecret(
        MetaPB.DeleteOptions.create({ uid: item.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateSecrets();
      toast.success("Secret deleted");
    },
    onError,
  });

  return (
    <ClickableCard>
      <div className="flex items-center gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
          <IconKey size={17} />
        </span>
        <div className="min-w-0 flex-1">
          <CardTitle
            name={getShortName(item)}
            displayName={item.metadata?.displayName}
            meta={
              <>
                Created <TimeAgo rfc3339={item.metadata?.createdAt} />
              </>
            }
          />
        </div>
        {props.canManage && (
          <ConfirmAction
            triggerLabel="Delete"
            triggerIcon={<IconTrash size={13} />}
            title="Delete this Secret?"
            confirmLabel="Delete Secret"
            description="Templates or Workspaces referencing it will fail to start until they are updated."
            loading={mutationDelete.isPending}
            onConfirm={() => mutationDelete.mutate()}
          />
        )}
      </div>
    </ClickableCard>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const [page, setPage] = React.useState(0);
  const space = ctx.space.data;

  const qry = useQuery({
    queryKey: ["workspace/listSecret", space?.metadata?.uid, page, itemsPerPage],
    queryFn: () => {
      const { response } = getClientWorkspace().listSecret(
        WsPB.ListSecretOptions.create({
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
            Secrets in {getShortName(space)}
          </Text>
          <Text size="xs" c="dimmed">
            Referenced by name from Templates and Workspaces in this Space.
            Values are write-only and never returned by the API.
          </Text>
        </div>
        {ctx.isAdmin && (
          <Button
            size="xs"
            leftSection={<IconPlus size={14} />}
            onClick={() => navigate(`${getPathSpace(space)}/secrets/create`)}
          >
            New Secret
          </Button>
        )}
      </div>

      {!ctx.isAdmin && (
        <Alert color="gray" variant="light">
          Only Space admins can create or delete Secrets.
        </Alert>
      )}

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconKey size={22} />}
                title="No Secrets in this Space"
                description="Store registry credentials, tokens and other sensitive values here."
                action={
                  ctx.isAdmin ? (
                    <Button
                      leftSection={<IconPlus size={15} />}
                      onClick={() =>
                        navigate(`${getPathSpace(space)}/secrets/create`)
                      }
                    >
                      New Secret
                    </Button>
                  ) : undefined
                }
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <SecretRow
                    key={x.metadata?.uid}
                    item={x}
                    canManage={ctx.isAdmin}
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
