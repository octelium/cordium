import ConfirmAction from "@/components/ConfirmAction";
import { CardTitle, ClickableCard } from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { formatBytes, onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import {
  getPathWorkspace,
  invalidateWorkspace,
  invalidateWorkspaceSnapshots,
} from "@/utils/octelium";
import {
  getResourceRef,
  getShortName,
  getShortNameFromRef,
  isWorkspaceSnapshotReady,
} from "@/utils/pb";
import {
  Anchor,
  Button,
  Group,
  Loader,
  Modal,
  Stack,
  Switch,
  Text,
  TextInput,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import {
  IconCamera,
  IconCircleCheck,
  IconCircleX,
  IconMapPin,
  IconRestore,
  IconStack2,
  IconTerminal2,
  IconTrash,
} from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { Link, useNavigate } from "react-router-dom";

const SnapshotState = WsPB.WorkspaceSnapshot_Status_State;
const Consistency = WsPB.WorkspaceSnapshot_Status_Consistency;

const SnapshotStateTag = (props: { item: WsPB.WorkspaceSnapshot }) => {
  switch (props.item.status?.state) {
    case SnapshotState.READY:
      return (
        <Tag tone="success" icon={<IconCircleCheck size={11} />}>
          Ready
        </Tag>
      );
    case SnapshotState.FAILED:
      return (
        <Tag tone="danger" icon={<IconCircleX size={11} />}>
          Failed
        </Tag>
      );
    case SnapshotState.CREATING:
      return (
        <span className="inline-flex items-center gap-1.5 text-[0.75rem] font-semibold text-hue-blue">
          <Loader size={12} color="blue" />
          Creating
        </span>
      );
    default:
      return <Tag tone="neutral">Unknown</Tag>;
  }
};

const RestoreSnapshot = (props: { item: WsPB.WorkspaceSnapshot }) => {
  const { item } = props;
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const [opened, { open, close }] = useDisclosure(false);
  const [displayName, setDisplayName] = React.useState("");
  const [doStart, setDoStart] = React.useState(true);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createWorkspace(
        WsPB.Workspace.create({
          apiVersion: "cordium/v1",
          kind: "Workspace",
          metadata: { displayName },
          spec: {},
          status: { workspaceSnapshotRef: getResourceRef(item) },
        }),
      );

      if (doStart) {
        await client.startWorkspace(
          WsPB.StartWorkspaceRequest.create({
            workspaceRef: getResourceRef(response),
          }),
        );
      }

      return response;
    },
    onSuccess: (response) => {
      close();
      invalidateWorkspace(response);
      toast.success("Workspace created from snapshot");
      navigate(getPathWorkspace(response));
    },
    onError,
  });

  return (
    <>
      <Button
        size="xs"
        variant="default"
        leftSection={<IconRestore size={13} />}
        onClick={open}
      >
        Restore
      </Button>

      <Modal
        opened={opened}
        onClose={close}
        size="md"
        title="Restore to a new workspace"
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            A new workspace is created from the same Template and Space, with
            its persistent storage restored from this snapshot. The source
            workspace is left untouched.
          </Text>

          <div className="rounded-lg border border-line bg-surface-subtle px-4 py-2.5">
            <Text size="xs" c="dimmed">
              Snapshot
            </Text>
            <Text size="sm" fw={600} className="font-mono">
              {getShortName(item)}
            </Text>
          </div>

          <TextInput
            label="Display name"
            description="Optional human-friendly label for the new workspace."
            placeholder="Restored workspace"
            value={displayName}
            onChange={(e) => setDisplayName(e.currentTarget.value)}
          />

          <Switch
            size="sm"
            checked={doStart}
            onChange={(e) => setDoStart(e.currentTarget.checked)}
            label="Start immediately"
          />

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              leftSection={<IconRestore size={14} />}
              loading={mutation.isPending}
              onClick={() => mutation.mutate()}
            >
              {doStart ? "Restore & start" : "Restore"}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

const SnapshotRow = (props: {
  item: WsPB.WorkspaceSnapshot;
  showWorkspace?: boolean;
  showSpace?: boolean;
}) => {
  const { item } = props;
  const client = getClientWorkspace();
  const status = item.status;

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteWorkspaceSnapshot(
        MetaPB.DeleteOptions.create({ uid: item.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateWorkspaceSnapshots();
      toast.success("Snapshot deleted");
    },
    onError,
  });

  return (
    <ClickableCard>
      <div className="flex flex-col gap-3 md:flex-row md:items-start">
        <div className="flex min-w-0 flex-1 items-start gap-3">
          <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-muted">
            <IconCamera size={17} />
          </span>
          <div className="min-w-0 flex-1">
            <CardTitle
              name={getShortName(item)}
              displayName={item.metadata?.displayName}
              meta={
                <>
                  Created <TimeAgo rfc3339={item.metadata?.createdAt} />
                  {status?.readyAt && (
                    <>
                      {" · Ready "}
                      <TimeAgo rfc3339={status.readyAt} />
                    </>
                  )}
                </>
              }
            />
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              <SnapshotStateTag item={item} />
              {status?.consistency === Consistency.CLEAN && (
                <Tag tone="info">Clean</Tag>
              )}
              {status?.consistency === Consistency.CRASH && (
                <Tag tone="warning">Crash-consistent</Tag>
              )}
              {!!status?.restoreSizeBytes && (
                <Tag label="Size">{formatBytes(status.restoreSizeBytes)}</Tag>
              )}
              {props.showWorkspace && status?.workspaceRef && (
                <Tag icon={<IconTerminal2 size={11} />} label="Workspace">
                  <Anchor
                    component={Link}
                    to={`/workspaces/${status.workspaceRef.name}`}
                    size="xs"
                    fw={600}
                    onClick={(e) => e.stopPropagation()}
                  >
                    {status.workspaceRef.name}
                  </Anchor>
                </Tag>
              )}
              {props.showSpace && status?.spaceRef && (
                <Tag icon={<IconStack2 size={11} />} label="Space">
                  {getShortNameFromRef(status.spaceRef)}
                </Tag>
              )}
              {status?.regionRef?.name && (
                <Tag icon={<IconMapPin size={11} />} label="Region">
                  {status.regionRef.name}
                </Tag>
              )}
            </div>
            {status?.failure?.message && (
              <p className="mt-1.5 text-[0.78rem] font-medium text-hue-rose">
                {status.failure.message}
              </p>
            )}
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {isWorkspaceSnapshotReady(item) && <RestoreSnapshot item={item} />}
          <ConfirmAction
            triggerLabel="Delete"
            triggerIcon={<IconTrash size={13} />}
            title="Delete this snapshot?"
            confirmLabel="Delete snapshot"
            description="The snapshot and its underlying storage snapshot are permanently removed. Workspaces already restored from it are not affected."
            loading={mutationDelete.isPending}
            onConfirm={() => mutationDelete.mutate()}
          />
        </div>
      </div>
    </ClickableCard>
  );
};

export default SnapshotRow;
