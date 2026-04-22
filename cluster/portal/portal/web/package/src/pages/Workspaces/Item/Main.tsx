import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { GetOptions } from "@/apis/metav1/metav1";
import CopyText from "@/components/CopyText";
import InfoItem from "@/components/InfoItem";
import LinkWrap from "@/components/LinkWrap";
import PageWrap from "@/components/PageWrap";
import Repository from "@/components/Repository";
import ResourceYAML from "@/components/ResourceYAML";
import SpaceName from "@/components/SpaceName";
import TimeAgo from "@/components/TimeAgo";
import WorkspaceStatus from "@/components/WorkspaceStatus";
import { clearTerminalGroup } from "@/features/terminalgroup/slice";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppDispatch } from "@/utils/hooks";
import {
  getPathSpace,
  getPathTemplate,
  invalidateResource,
} from "@/utils/octelium";
import { canStopWorkspace, getResourceRef, getShortName } from "@/utils/pb";
import {
  Anchor,
  Badge,
  Button,
  Group,
  Modal,
  Stack,
  Text,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconBrandGit,
  IconCpu,
  IconDatabase,
  IconExternalLink,
  IconPlayerPlay,
  IconPlayerStop,
  IconServer,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import axios from "axios";
import * as React from "react";
import { useContextWorkspace } from "../utils";

interface AuthBegin {
  loginURL: string;
}

const LoginGitProvider = (props: { item: WsPB.Workspace }) => {
  const { item } = props;

  if (
    !item.status?.templateRef ||
    item.status.spaceType !== WsPB.Space_Status_Type.ORGANIZATION ||
    item.status.state !== WsPB.Workspace_Status_State.STOPPED
  )
    return null;

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", item.status.templateRef.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: item.status!.templateRef!.uid }),
      );
      return response;
    },
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

  if (!qryTemplate.isSuccess || !qryTemplate.data.status?.gitProviderRef)
    return null;

  return (
    <Button
      fullWidth
      variant="default"
      leftSection={<IconBrandGit size={15} />}
      loading={mutation.isPending}
      onClick={() => mutation.mutate()}
    >
      Login to Git provider
    </Button>
  );
};

const StartStopButton = (props: { item: WsPB.Workspace }) => {
  const dispatch = useAppDispatch();
  const client = getClientWorkspace();
  const { item } = props;
  const canStop = canStopWorkspace(item);
  const isStopped = item.status?.state === WsPB.Workspace_Status_State.STOPPED;
  const [opened, { open, close }] = useDisclosure(false);

  const mutationStop = useMutation({
    mutationFn: async () => {
      const { response } = await client.stopWorkspace(
        WsPB.StopWorkspaceRequest.create({
          workspaceRef: getResourceRef(item),
        }),
      );
      return response;
    },
    onSuccess: () => {
      close();
      dispatch(clearTerminalGroup({}));
      invalidateResource(item);
    },
    onError: () => close(),
  });

  const mutationStart = useMutation({
    mutationFn: async () => {
      const { response } = await client.startWorkspace(
        WsPB.StartWorkspaceRequest.create({
          workspaceRef: getResourceRef(item),
        }),
      );
      return response;
    },
    onSuccess: () => invalidateResource(item),
  });

  return (
    <>
      {canStop && (
        <Button
          fullWidth
          variant="outline"
          color="red"
          leftSection={<IconPlayerStop size={15} />}
          onClick={open}
        >
          Stop workspace
        </Button>
      )}

      {isStopped && (
        <Button
          fullWidth
          leftSection={<IconPlayerPlay size={15} />}
          loading={mutationStart.isPending}
          onClick={() => mutationStart.mutate()}
        >
          Start workspace
        </Button>
      )}

      <Modal
        opened={opened}
        onClose={close}
        centered
        title={
          <Text fw={600} size="sm">
            Stop workspace?
          </Text>
        }
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            This will stop the running workspace and terminate all active
            sessions.
          </Text>

          <div
            style={{
              background: "#f8fafc",
              borderRadius: 8,
              border: "1px solid #e2e8f0",
              padding: "8px 14px",
            }}
          >
            <InfoItem title="Name">{item.metadata!.name}</InfoItem>
            <InfoItem title="UID">{item.metadata!.uid}</InfoItem>
          </div>

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              color="red"
              loading={mutationStop.isPending}
              onClick={() => mutationStop.mutate()}
            >
              Stop workspace
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

const ResourceLimits = (props: { item: WsPB.Workspace }) => {
  const { item } = props;
  const limit = item.status?.limit;
  if (!limit || (!limit.cpu && !limit.memory && !limit.storage)) return null;

  const chips: { icon: React.ReactNode; label: string }[] = [];

  if (limit.cpu?.millicores) {
    chips.push({
      icon: <IconCpu size={12} />,
      label: `${limit.cpu.millicores / 1000} CPU`,
    });
  }
  if (limit.memory?.megabytes) {
    const mem =
      limit.memory.megabytes >= 1000
        ? `${limit.memory.megabytes / 1000}GB RAM`
        : `${limit.memory.megabytes}MB RAM`;
    chips.push({ icon: <IconServer size={12} />, label: mem });
  }
  if (limit.storage?.megabytes) {
    const stor =
      limit.storage.megabytes >= 1000
        ? `${limit.storage.megabytes / 1000}GB Storage`
        : `${limit.storage.megabytes}MB Storage`;
    chips.push({ icon: <IconDatabase size={12} />, label: stor });
  }

  return (
    <InfoItem title="Resource limits">
      <Group gap={6}>
        {chips.map((c) => (
          <Badge key={c.label} size="sm" variant="light" leftSection={c.icon}>
            {c.label}
          </Badge>
        ))}
      </Group>
    </InfoItem>
  );
};

const AppItem = (props: {
  app: WsPB.Workspace_Spec_Application;
  item: WsPB.Workspace;
}) => {
  const { item, app } = props;
  const href = item.status?.hostname
    ? app.isDefault
      ? `https://${item.status.hostname}`
      : `https://${app.name}_${item.status.hostname}`
    : undefined;

  return (
    <Anchor
      href={href}
      target="_blank"
      underline="never"
      style={{ display: "inline-flex", alignItems: "center", gap: 6 }}
    >
      <Badge
        size="sm"
        variant="outline"
        rightSection={<IconExternalLink size={10} />}
        style={{ cursor: "pointer" }}
      >
        {app.displayName || app.name}
        {app.port > 0 && ` :${app.port}`}
        {app.isDefault && " · default"}
      </Badge>
    </Anchor>
  );
};

const InfoBar = (props: { item: WsPB.Workspace }) => {
  const { item } = props;
  const isActive = item.status?.state !== WsPB.Workspace_Status_State.STOPPED;

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", item.status!.templateRef!.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: item.status!.templateRef!.uid }),
      );
      return response;
    },
  });

  const qrySpace = useQuery({
    queryKey: ["workspace/getSpace", item.status!.spaceRef!.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getSpace(
        GetOptions.create({ uid: item.status!.spaceRef!.uid }),
      );
      return response;
    },
  });

  const apps = item.spec?.applications ?? [];

  return (
    <div style={{ display: "flex", gap: 20, alignItems: "flex-start" }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            background: "white",
            border: "1px solid #e2e8f0",
            borderRadius: 12,
            overflow: "hidden",
          }}
        >
          <div
            style={{
              padding: "14px 20px",
              borderBottom: "1px solid #e2e8f0",
              background: "#f8fafc",
            }}
          >
            <Text
              size="xs"
              fw={700}
              tt="uppercase"
              style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
            >
              Workspace details
            </Text>
          </div>

          <div style={{ padding: "4px 20px 8px" }}>
            <InfoItem title="Name">
              <CopyText value={item.metadata!.name} />
            </InfoItem>

            {item.metadata?.displayName && (
              <InfoItem title="Display name">
                {item.metadata.displayName}
              </InfoItem>
            )}

            <InfoItem title="State">
              <WorkspaceStatus status={item.status!.state} />
            </InfoItem>

            {qrySpace.isSuccess && (
              <InfoItem title="Space">
                <LinkWrap to={getPathSpace(qrySpace.data!)}>
                  <SpaceName spaceRef={getResourceRef(qrySpace.data!)} />
                </LinkWrap>
              </InfoItem>
            )}

            {qryTemplate.isSuccess && (
              <InfoItem title="Template">
                <LinkWrap to={getPathTemplate(qryTemplate.data)}>
                  {getShortName(qryTemplate.data)}
                </LinkWrap>
              </InfoItem>
            )}

            {isActive && item.status?.hostname && (
              <InfoItem title="URL">
                <Anchor
                  href={`https://${item.status.hostname}`}
                  target="_blank"
                  size="sm"
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 4,
                  }}
                >
                  {`https://${item.status.hostname}`}
                  <IconExternalLink size={12} />
                </Anchor>
              </InfoItem>
            )}

            <InfoItem title="Created">
              <TimeAgo rfc3339={item.metadata?.createdAt} />
            </InfoItem>

            {item.status?.isEphemeral && (
              <InfoItem title="Storage">
                <Badge size="sm" color="orange" variant="light">
                  Ephemeral
                </Badge>
              </InfoItem>
            )}

            {item.status?.lastInitializedAt && (
              <InfoItem title="Last initialized">
                <TimeAgo rfc3339={item.status.lastInitializedAt} />
              </InfoItem>
            )}

            {item.status?.lastStoppedAt && (
              <InfoItem title="Last stopped">
                <TimeAgo rfc3339={item.status.lastStoppedAt} />
              </InfoItem>
            )}

            {item.status?.state === WsPB.Workspace_Status_State.RUNNING &&
              item.status?.lastActivityAt && (
                <InfoItem title="Last activity">
                  <TimeAgo rfc3339={item.status.lastActivityAt} />
                </InfoItem>
              )}

            <ResourceLimits item={item} />

            <InfoItem title="Config">
              <ResourceYAML item={item} size="xs" />
            </InfoItem>
          </div>
        </div>

        {apps.length > 0 && (
          <div
            style={{
              marginTop: 12,
              background: "white",
              border: "1px solid #e2e8f0",
              borderRadius: 12,
              overflow: "hidden",
            }}
          >
            <div
              style={{
                padding: "14px 20px",
                borderBottom: "1px solid #e2e8f0",
                background: "#f8fafc",
              }}
            >
              <Text
                size="xs"
                fw={500}
                tt="uppercase"
                style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
              >
                Applications
              </Text>
            </div>
            <div
              style={{
                padding: "12px 20px",
                display: "flex",
                flexWrap: "wrap",
                gap: 6,
              }}
            >
              {apps.map((app) => (
                <AppItem key={app.name} app={app} item={item} />
              ))}
            </div>
          </div>
        )}

        <div style={{ marginTop: 12 }}>
          <Repository item={item} />
        </div>
      </div>

      <div style={{ width: 220, flexShrink: 0 }}>
        <Stack gap="sm">
          <StartStopButton item={item} />
          <LoginGitProvider item={item} />
        </Stack>
      </div>
    </div>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <InfoBar item={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Page;
