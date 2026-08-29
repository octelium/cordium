import CopyText from "@/components/CopyText";
import Facts, { Fact } from "@/components/Facts";
import Panel, { PanelBody, PanelHeader } from "@/components/Panel";
import RepoLink from "@/components/RepoLink";
import StateBadge from "@/components/StateBadge";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { formatMegabytes, formatMillicores, onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import {
  getApplicationURL,
  getPathSpaceRef,
  getPathTemplateRef,
  getWorkspaceURL,
} from "@/utils/octelium";
import { getShortNameFromRef, isWorkspaceStopped } from "@/utils/pb";
import { Alert, Anchor, Button, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { GetOptions } from "@octelium/apis/main/metav1";
import {
  IconAlertTriangle,
  IconBrandGit,
  IconExternalLink,
  IconWorldWww,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import axios from "axios";
import { Link } from "react-router-dom";
import { useContextWorkspace } from "../utils";
import { StartStopButtons } from "./index";

interface AuthBegin {
  loginURL: string;
}

const GitProviderLogin = (props: { item: WsPB.Workspace }) => {
  const { item } = props;

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", item.status?.templateRef?.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: item.status!.templateRef!.uid }),
      );
      return response;
    },
    enabled: !!item.status?.templateRef?.uid,
  });

  const mutation = useMutation({
    mutationFn: async () => {
      const resp = await axios.post<AuthBegin>(
        `/auth/v1/begin/${item.metadata!.uid}`,
      );
      return resp.data;
    },
    onSuccess: (data) => {
      window.location.href = data.loginURL;
    },
    onError,
  });

  if (
    !isWorkspaceStopped(item) ||
    !qryTemplate.isSuccess ||
    !qryTemplate.data.status?.gitProviderRef
  ) {
    return null;
  }

  return (
    <Button
      fullWidth
      variant="default"
      leftSection={<IconBrandGit size={15} />}
      loading={mutation.isPending}
      onClick={() => mutation.mutate()}
    >
      Sign in to Git provider
    </Button>
  );
};

const failureLabel = (failure: WsPB.Workspace_Status_Failure): string => {
  switch (failure.type.oneofKind) {
    case "imageBuild":
      return "Image build failed";
    case "imagePull":
      return "Image pull failed";
    case "repoClone":
      return "Repository clone failed";
    case "additionalRepoClone":
      return `Additional repository "${failure.type.additionalRepoClone.name}" failed to clone`;
    case "buildTimeoutExceeded":
      return "Build timed out";
    case "task":
      return `Task "${failure.type.task.name}" exited with code ${failure.type.task.exitCode}`;
    case "startupTimeoutExceeded":
      return "Startup timed out";
    case "startupUnknown":
      return "Startup failed";
    case "loadStorage":
      return "Loading persistent storage failed";
    case "saveStorage":
      return "Saving persistent storage failed";
    case "stoppageTimeoutExceeded":
      return "Shutdown timed out";
    case "runContainer":
      return "Container failed to run";
    case "healthCheck":
      return "Health check failed";
    default:
      return "Workspace failed";
  }
};

const Page = () => {
  const ctx = useContextWorkspace();
  const item = ctx.workspace.data;

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", item?.status?.templateRef?.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: item!.status!.templateRef!.uid }),
      );
      return response;
    },
    enabled: !!item?.status?.templateRef?.uid,
  });

  if (!item) return null;

  const url = getWorkspaceURL(item);
  const apps = item.spec?.applications ?? [];
  const limit = item.status?.limit;
  const failure = item.status?.failure;
  const active = !isWorkspaceStopped(item);

  return (
    <Stack gap="lg">
      {failure && (
        <Alert
          color="red"
          icon={<IconAlertTriangle size={16} />}
          title={failureLabel(failure)}
        >
          {failure.message || "Check the logs tab for details."}
        </Alert>
      )}

      <div className="grid gap-4 lg:grid-cols-[1fr_18rem]">
        <Stack gap="md">
          <Panel>
            <PanelHeader title="Details" />
            <PanelBody className="px-5 py-1">
              <Facts>
                <Fact label="Name">
                  <CopyText value={item.metadata!.name} />
                </Fact>
                {item.metadata?.displayName && (
                  <Fact label="Display name">{item.metadata.displayName}</Fact>
                )}
                <Fact label="State">
                  <StateBadge state={item.status!.state} />
                </Fact>
                {item.status?.spaceRef && (
                  <Fact label="Space">
                    <Anchor
                      component={Link}
                      to={getPathSpaceRef(item.status.spaceRef)}
                      size="sm"
                      fw={600}
                    >
                      {getShortNameFromRef(item.status.spaceRef)}
                    </Anchor>
                  </Fact>
                )}
                {item.status?.spaceRef && item.status?.templateRef && (
                  <Fact label="Template">
                    <Anchor
                      component={Link}
                      to={getPathTemplateRef(
                        item.status.spaceRef,
                        item.status.templateRef,
                      )}
                      size="sm"
                      fw={600}
                    >
                      {getShortNameFromRef(item.status.templateRef)}
                    </Anchor>
                  </Fact>
                )}
                {active && url && (
                  <Fact label="URL">
                    <Anchor
                      href={url}
                      target="_blank"
                      rel="noreferrer"
                      size="sm"
                      className="inline-flex items-center gap-1"
                    >
                      {url}
                      <IconExternalLink size={12} />
                    </Anchor>
                  </Fact>
                )}
                {item.spec?.repository?.url && (
                  <Fact label="Repository">
                    <RepoLink item={item} />
                  </Fact>
                )}
                <Fact label="Storage">
                  {item.spec?.isEphemeral
                    ? "Ephemeral — discarded on stop"
                    : "Persistent"}
                </Fact>
                {limit && (
                  <Fact label="Resources">
                    {[
                      limit.cpu?.millicores
                        ? formatMillicores(limit.cpu.millicores)
                        : null,
                      limit.memory?.megabytes
                        ? formatMegabytes(limit.memory.megabytes)
                        : null,
                      limit.storage?.megabytes
                        ? `${formatMegabytes(limit.storage.megabytes)} disk`
                        : null,
                    ]
                      .filter(Boolean)
                      .join(" · ") || "—"}
                  </Fact>
                )}
                {item.status?.regionRef?.name && (
                  <Fact label="Region">{item.status.regionRef.name}</Fact>
                )}
                <Fact label="Created">
                  <TimeAgo rfc3339={item.metadata?.createdAt} />
                </Fact>
                {item.status?.lastInitializedAt && (
                  <Fact label="Last started">
                    <TimeAgo rfc3339={item.status.lastInitializedAt} />
                  </Fact>
                )}
                {item.status?.lastStoppedAt && (
                  <Fact label="Last stopped">
                    <TimeAgo rfc3339={item.status.lastStoppedAt} />
                  </Fact>
                )}
                {active && item.status?.lastActivityAt && (
                  <Fact label="Last activity">
                    <TimeAgo rfc3339={item.status.lastActivityAt} />
                  </Fact>
                )}
                <Fact label="Successful runs">
                  {item.status?.successfulRuns ?? 0}
                </Fact>
              </Facts>
            </PanelBody>
          </Panel>

          {apps.length > 0 && (
            <Panel>
              <PanelHeader
                icon={<IconWorldWww size={16} />}
                title="Applications"
                description={
                  active
                    ? "Ports exposed by this workspace over HTTPS."
                    : "Available once the workspace is running."
                }
              />
              <PanelBody>
                <div className="flex flex-wrap gap-2">
                  {apps.map((app) => {
                    const href = getApplicationURL(item, app);
                    const label = `${app.displayName || app.name}${
                      app.port ? ` :${app.port}` : ""
                    }`;
                    if (!href || !active) {
                      return (
                        <Tag key={app.name}>
                          {label}
                          {app.isDefault && " · default"}
                        </Tag>
                      );
                    }
                    return (
                      <Anchor
                        key={app.name}
                        href={href}
                        target="_blank"
                        rel="noreferrer"
                        underline="never"
                      >
                        <Tag
                          tone="info"
                          icon={<IconExternalLink size={11} />}
                          className="cursor-pointer"
                        >
                          {label}
                          {app.isDefault && " · default"}
                        </Tag>
                      </Anchor>
                    );
                  })}
                </div>
              </PanelBody>
            </Panel>
          )}
        </Stack>

        <Panel>
          <PanelHeader title="Actions" />
          <PanelBody>
            <Stack gap="sm">
              <StartStopButtons item={item} fullWidth />
              <GitProviderLogin item={item} />
              {qryTemplate.isSuccess && item.status?.spaceRef && (
                <Button
                  fullWidth
                  variant="subtle"
                  color="gray"
                  component={Link}
                  to={getPathTemplateRef(
                    item.status.spaceRef,
                    item.status.templateRef!,
                  )}
                >
                  View Template
                </Button>
              )}
            </Stack>
          </PanelBody>
        </Panel>
      </div>
    </Stack>
  );
};

export default Page;
