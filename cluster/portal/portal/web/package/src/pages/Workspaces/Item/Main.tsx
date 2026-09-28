import CopyText from "@/components/CopyText";
import CreateSnapshot from "@/components/CreateSnapshot";
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
  invalidateResource,
} from "@/utils/octelium";
import {
  getResourceRef,
  getShortNameFromRef,
  isWorkspaceStopped,
} from "@/utils/pb";
import { Alert, Anchor, Button, Select, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { GetOptions } from "@octelium/apis/main/metav1";
import {
  IconAlertTriangle,
  IconBrandGit,
  IconExternalLink,
  IconShare,
  IconWorldWww,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import axios from "axios";
import toast from "react-hot-toast";
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
    case "networkPolicy":
      return "Network policy could not be enforced";
    case "volume":
      return `Volume "${failure.type.volume.name}" could not be mounted`;
    default:
      return "Workspace failed";
  }
};

const stoppingReasonLabel = (
  reason: WsPB.Workspace_Status_StoppingReason,
): string | undefined => {
  switch (reason) {
    case WsPB.Workspace_Status_StoppingReason.API:
      return "Stopped on request";
    case WsPB.Workspace_Status_StoppingReason.ERROR:
      return "Stopped after a failure";
    case WsPB.Workspace_Status_StoppingReason.CLUSTER:
      return "Stopped by the Cluster";
    default:
      return undefined;
  }
};

const ShareMode = WsPB.Workspace_Status_SharedPort_Mode;

const ApplicationShare = (props: {
  item: WsPB.Workspace;
  app: WsPB.Workspace_Spec_Application;
}) => {
  const { item, app } = props;
  const client = getClientWorkspace();
  const sharedPort = item.status?.sharedPorts.find(
    (x) => x.applicationName === app.name,
  );
  const isOrg = item.status?.spaceType === WsPB.Space_Status_Type.ORGANIZATION;

  const mutation = useMutation({
    mutationFn: async (val: string) => {
      if (val === "private") {
        await client.unshareWorkspacePort(
          WsPB.UnshareWorkspacePortRequest.create({
            workspaceRef: getResourceRef(item),
            applicationName: app.name,
          }),
        );
        return;
      }

      await client.shareWorkspacePort(
        WsPB.ShareWorkspacePortRequest.create({
          workspaceRef: getResourceRef(item),
          applicationName: app.name,
          mode:
            val === "all"
              ? WsPB.ShareWorkspacePortRequest_Mode.ALL
              : WsPB.ShareWorkspacePortRequest_Mode.MEMBERS,
        }),
      );
    },
    onSuccess: (_, val) => {
      invalidateResource(item);
      toast.success(
        val === "private" ? "Application unshared" : "Application shared",
      );
    },
    onError,
  });

  const value =
    sharedPort?.mode === ShareMode.ALL
      ? "all"
      : sharedPort?.mode === ShareMode.MEMBERS
        ? "members"
        : "private";

  return (
    <Select
      size="xs"
      w={170}
      aria-label={`Sharing of ${app.name}`}
      allowDeselect={false}
      leftSection={<IconShare size={13} />}
      disabled={mutation.isPending}
      data={[
        { value: "private", label: "Only you" },
        ...(isOrg || value === "members"
          ? [{ value: "members", label: "Space members" }]
          : []),
        { value: "all", label: "All users" },
      ]}
      value={value}
      onChange={(val) => {
        if (val && val !== value) mutation.mutate(val);
      }}
    />
  );
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
  const volumeMounts = item.spec?.runtime?.volumeMounts ?? [];
  const stoppingReason = stoppingReasonLabel(
    item.status?.stoppingReason ?? WsPB.Workspace_Status_StoppingReason.UNSET,
  );
  const canSnapshot =
    !(item.spec?.isEphemeral && !active) &&
    (!!item.status?.regionRef || !!item.status?.lastRegionRef);

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
                {item.status?.workspaceSnapshotRef && (
                  <Fact label="Restored from">
                    <Anchor component={Link} to="/snapshots" size="sm" fw={600}>
                      {getShortNameFromRef(item.status.workspaceSnapshotRef)}
                    </Anchor>
                  </Fact>
                )}
                {volumeMounts.length > 0 && (
                  <Fact label="Volumes">
                    <div className="flex flex-wrap gap-1.5">
                      {volumeMounts.map((m, idx) => (
                        <Tag key={idx} mono>
                          {m.volumeRef ? getShortNameFromRef(m.volumeRef) : "?"}
                          {` → ${m.mountPath}`}
                          {m.readOnly && " · ro"}
                        </Tag>
                      ))}
                    </div>
                  </Fact>
                )}
                {item.status?.regionRef?.name && (
                  <Fact label="Region">{item.status.regionRef.name}</Fact>
                )}
                {!item.status?.regionRef?.name &&
                  !item.spec?.isEphemeral &&
                  item.status?.lastRegionRef?.name && (
                    <Fact label="Storage region">
                      {item.status.lastRegionRef.name}
                    </Fact>
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
                    {!active && stoppingReason && ` · ${stoppingReason}`}
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
                    ? "Ports exposed by this workspace over HTTPS. Share them with other users when needed."
                    : "Available once the workspace is running."
                }
              />
              <PanelBody className="px-5 py-1">
                <div className="divide-y divide-line-subtle">
                  {apps.map((app) => {
                    const href = getApplicationURL(item, app);
                    const label = `${app.displayName || app.name}${
                      app.port ? ` :${app.port}` : ""
                    }`;
                    return (
                      <div
                        key={app.name}
                        className="flex flex-wrap items-center justify-between gap-3 py-2.5"
                      >
                        {!href || !active ? (
                          <Tag>
                            {label}
                            {app.isDefault && " · default"}
                          </Tag>
                        ) : (
                          <Anchor
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
                        )}
                        <ApplicationShare item={item} app={app} />
                      </div>
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
              {canSnapshot && (
                <CreateSnapshot item={item} size="sm" fullWidth />
              )}
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
