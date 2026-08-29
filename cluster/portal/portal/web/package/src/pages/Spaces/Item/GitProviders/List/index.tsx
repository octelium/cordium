import ConfirmAction from "@/components/ConfirmAction";
import Empty from "@/components/Empty";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, CardTitle, ClickableCard } from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateGitProviders } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Alert, Button, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import {
  IconBrandGithub,
  IconBrandGitlab,
  IconGitBranch,
  IconPlus,
  IconTrash,
  IconWorld,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const providerInfo = (item: WsPB.GitProvider) => {
  switch (item.spec?.type.oneofKind) {
    case "github":
      return {
        label: "GitHub",
        icon: <IconBrandGithub size={11} />,
        clientID: item.spec.type.github.clientID,
      };
    case "gitlab":
      return {
        label: "GitLab",
        icon: <IconBrandGitlab size={11} />,
        clientID: item.spec.type.gitlab.clientID,
      };
    case "oauth2":
      return {
        label: "OAuth2",
        icon: <IconWorld size={11} />,
        clientID: item.spec.type.oauth2.clientID,
      };
    default:
      return { label: "Unknown", icon: null, clientID: "" };
  }
};

const ProviderRow = (props: { item: WsPB.GitProvider; canManage: boolean }) => {
  const { item } = props;
  const info = providerInfo(item);
  const client = getClientWorkspace();

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteGitProvider(
        MetaPB.DeleteOptions.create({ uid: item.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateGitProviders();
      toast.success("Git provider deleted");
    },
    onError,
  });

  return (
    <ClickableCard>
      <div className="flex items-center gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
          <IconGitBranch size={17} />
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
          <div className="mt-2 flex flex-wrap gap-1.5">
            <Tag tone="info" icon={info.icon}>
              {info.label}
            </Tag>
            {info.clientID && (
              <Tag label="Client ID" mono>
                {info.clientID}
              </Tag>
            )}
          </div>
        </div>
        {props.canManage && (
          <ConfirmAction
            triggerLabel="Delete"
            triggerIcon={<IconTrash size={13} />}
            title="Delete this Git provider?"
            confirmLabel="Delete provider"
            description="Templates using it will stop prompting users to sign in, and private clones may start failing."
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
    queryKey: [
      "workspace/listGitProvider",
      space?.metadata?.uid,
      page,
      itemsPerPage,
    ],
    queryFn: () => {
      const { response } = getClientWorkspace().listGitProvider(
        WsPB.ListGitProviderOptions.create({
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
            Git providers in {getShortName(space)}
          </Text>
          <Text size="xs" c="dimmed">
            OAuth apps that let members sign in so their Workspaces clone
            private repositories with their own credentials.
          </Text>
        </div>
        {ctx.isAdmin && (
          <Button
            size="xs"
            leftSection={<IconPlus size={14} />}
            onClick={() =>
              navigate(`${getPathSpace(space)}/gitproviders/create`)
            }
          >
            New provider
          </Button>
        )}
      </div>

      {!ctx.isAdmin && (
        <Alert color="gray" variant="light">
          Only Space admins can manage Git providers.
        </Alert>
      )}

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconGitBranch size={22} />}
                title="No Git providers"
                description="Add one to let members authenticate against GitHub, GitLab or any OAuth2 provider."
                action={
                  ctx.isAdmin ? (
                    <Button
                      leftSection={<IconPlus size={15} />}
                      onClick={() =>
                        navigate(`${getPathSpace(space)}/gitproviders/create`)
                      }
                    >
                      New provider
                    </Button>
                  ) : undefined
                }
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <ProviderRow
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
