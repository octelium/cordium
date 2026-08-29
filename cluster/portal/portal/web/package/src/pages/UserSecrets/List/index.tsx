import ConfirmAction from "@/components/ConfirmAction";
import CopyText from "@/components/CopyText";
import Empty from "@/components/Empty";
import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, CardTitle, ClickableCard } from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { invalidateUserSecrets } from "@/utils/octelium";
import { getShortName } from "@/utils/pb";
import { Button, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { IconKey, IconPlus, IconTrash, IconTerminal2 } from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const SecretRow = (props: { item: WsPB.UserSecret }) => {
  const { item } = props;
  const client = getClientWorkspace();
  const isSSH = item.spec?.type === WsPB.UserSecret_Spec_Type.SSH_KEY;

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteUserSecret(
        MetaPB.DeleteOptions.create({ uid: item.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateUserSecrets();
      toast.success("Secret deleted");
    },
    onError,
  });

  return (
    <ClickableCard>
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
          {isSSH ? <IconTerminal2 size={17} /> : <IconKey size={17} />}
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
          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            <Tag tone={isSSH ? "info" : "neutral"}>
              {isSSH ? "SSH key" : "Value"}
            </Tag>
          </div>
          {item.status?.details.oneofKind === "sshKey" && (
            <div className="mt-2 text-[0.75rem] text-slate-500">
              <span className="mr-1 font-semibold">Public key</span>
              <CopyText
                value={item.status.details.sshKey.publicKey}
                truncate={56}
              />
            </div>
          )}
        </div>

        <ConfirmAction
          triggerLabel="Delete"
          triggerIcon={<IconTrash size={13} />}
          title="Delete this User Secret?"
          confirmLabel="Delete Secret"
          description="Workspaces or settings referencing it will fail until they are updated."
          loading={mutationDelete.isPending}
          onConfirm={() => mutationDelete.mutate()}
        />
      </div>
    </ClickableCard>
  );
};

const Page = () => {
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);
  const navigate = useNavigate();
  const [page, setPage] = React.useState(0);

  const qry = useQuery({
    queryKey: ["workspace/listUserSecret", page, itemsPerPage],
    queryFn: () => {
      const { response } = getClientWorkspace().listUserSecret(
        WsPB.ListUserSecretOptions.create({ common: { page, itemsPerPage } }),
      );
      return response;
    },
  });

  return (
    <>
      <Meta title="Your Secrets" />
      <PageHeader
        title="Your Secrets"
        description="Personal Secrets available in every Workspace you own — dotfiles credentials, SSH keys and environment values."
        actions={
          <Button
            leftSection={<IconPlus size={15} />}
            onClick={() => navigate("/usersecrets/create")}
          >
            New Secret
          </Button>
        }
      />

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconKey size={22} />}
                title="No personal Secrets"
                description="Add one to authenticate to private dotfiles repositories or inject values into your Workspaces."
                action={
                  <Button
                    leftSection={<IconPlus size={15} />}
                    onClick={() => navigate("/usersecrets/create")}
                  >
                    New Secret
                  </Button>
                }
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <SecretRow key={x.metadata?.uid} item={x} />
                ))}
              </CardList>
            )}
            <Paginator meta={qry.data.listResponseMeta!} onPageChange={setPage} />
          </Stack>
        )}
      </QueryBoundary>
    </>
  );
};

export default Page;
