import ConfirmAction from "@/components/ConfirmAction";
import Empty from "@/components/Empty";
import Paginator from "@/components/Paginator";
import QueryBoundary from "@/components/QueryBoundary";
import { CardList, CardTitle, ClickableCard } from "@/components/ResourceCards";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { useContextSpace } from "@/pages/Spaces/utils";
import { formatMegabytes, onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateVolumes } from "@/utils/octelium";
import { getResourceRef, getShortName, isVolumeShared } from "@/utils/pb";
import {
  Alert,
  Button,
  Group,
  Loader,
  Modal,
  NumberInput,
  Stack,
  Text,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import {
  IconArrowsMaximize,
  IconCircleCheck,
  IconCircleX,
  IconDatabase,
  IconMapPin,
  IconPlus,
  IconTrash,
  IconUsers,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const VolumeState = WsPB.Volume_Status_State;

const VolumeStateTag = (props: { item: WsPB.Volume }) => {
  switch (props.item.status?.state) {
    case VolumeState.READY:
      return (
        <Tag tone="success" icon={<IconCircleCheck size={11} />}>
          Ready
        </Tag>
      );
    case VolumeState.FAILED:
      return (
        <Tag tone="danger" icon={<IconCircleX size={11} />}>
          Failed
        </Tag>
      );
    case VolumeState.PENDING:
      return (
        <span className="inline-flex items-center gap-1.5 text-[0.75rem] font-semibold text-hue-amber">
          <Loader size={12} color="orange" />
          Pending
        </span>
      );
    default:
      return <Tag tone="neutral">Unknown</Tag>;
  }
};

const ResizeVolume = (props: { item: WsPB.Volume }) => {
  const { item } = props;
  const client = getClientWorkspace();
  const [opened, { open, close }] = useDisclosure(false);
  const current = item.spec?.size?.megabytes ?? 0;
  const [megabytes, setMegabytes] = React.useState(current);

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = WsPB.Volume.clone(item);
      payload.spec!.size = WsPB.Volume_Spec_Size.create({ megabytes });
      const { response } = await client.updateVolume(payload);
      return response;
    },
    onSuccess: () => {
      close();
      invalidateVolumes();
      toast.success("Volume resized");
    },
    onError,
  });

  return (
    <>
      <Button
        size="xs"
        variant="default"
        leftSection={<IconArrowsMaximize size={13} />}
        onClick={() => {
          setMegabytes(current);
          open();
        }}
      >
        Resize
      </Button>

      <Modal opened={opened} onClose={close} size="md" title="Resize Volume">
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            Volumes can only be grown. Growing an already provisioned Volume
            requires the storage backend to support volume expansion.
          </Text>

          <NumberInput
            label="Size"
            description={`Megabytes. Currently ${formatMegabytes(current)}.`}
            min={current}
            max={10000000}
            step={1000}
            value={megabytes}
            onChange={(v) => setMegabytes(typeof v === "number" ? v : Number(v) || 0)}
          />

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              leftSection={<IconArrowsMaximize size={14} />}
              loading={mutation.isPending}
              disabled={megabytes <= current}
              onClick={() => mutation.mutate()}
            >
              Resize Volume
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

const VolumeRow = (props: { item: WsPB.Volume; canManage: boolean }) => {
  const { item } = props;
  const client = getClientWorkspace();
  const capacity = item.status?.capacity?.megabytes ?? 0;

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteVolume(
        MetaPB.DeleteOptions.create({ uid: item.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateVolumes();
      toast.success("Volume deleted");
    },
    onError,
  });

  return (
    <ClickableCard>
      <div className="flex flex-col gap-3 md:flex-row md:items-start">
        <div className="flex min-w-0 flex-1 items-start gap-3">
          <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-muted">
            <IconDatabase size={17} />
          </span>
          <div className="min-w-0 flex-1">
            <CardTitle
              name={getShortName(item)}
              displayName={item.metadata?.displayName}
              meta={
                <>
                  Created <TimeAgo rfc3339={item.metadata?.createdAt} />
                  {item.status?.readyAt && (
                    <>
                      {" · Provisioned "}
                      <TimeAgo rfc3339={item.status.readyAt} />
                    </>
                  )}
                </>
              }
            />
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              <VolumeStateTag item={item} />
              <Tag label="Size">
                {formatMegabytes(item.spec?.size?.megabytes ?? 0)}
              </Tag>
              {capacity > 0 && capacity !== item.spec?.size?.megabytes && (
                <Tag label="Capacity">{formatMegabytes(capacity)}</Tag>
              )}
              <Tag
                tone={isVolumeShared(item) ? "info" : "neutral"}
                icon={isVolumeShared(item) ? <IconUsers size={11} /> : undefined}
              >
                {isVolumeShared(item) ? "Shared" : "Exclusive"}
              </Tag>
              {item.status?.regionRef?.name && (
                <Tag icon={<IconMapPin size={11} />} label="Region">
                  {item.status.regionRef.name}
                </Tag>
              )}
            </div>
            {item.status?.failure?.message && (
              <p className="mt-1.5 text-[0.78rem] font-medium text-hue-rose">
                {item.status.failure.message}
              </p>
            )}
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {props.canManage && <ResizeVolume item={item} />}
          {props.canManage && (
            <ConfirmAction
              triggerLabel="Delete"
              triggerIcon={<IconTrash size={13} />}
              title="Delete this Volume?"
              confirmLabel="Delete Volume"
              description="The Volume and all of its data are permanently removed. It cannot be deleted while a Template or a Workspace still mounts it."
              loading={mutationDelete.isPending}
              onConfirm={() => mutationDelete.mutate()}
            />
          )}
        </div>
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
    queryKey: ["workspace/listVolume", space?.metadata?.uid, page, itemsPerPage],
    queryFn: () => {
      const { response } = getClientWorkspace().listVolume(
        WsPB.ListVolumeOptions.create({
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
            Volumes in {getShortName(space)}
          </Text>
          <Text size="xs" c="dimmed">
            Persistent storage shared across the Workspaces of this Space. Mount
            them from the Volumes tab of a Template or a Workspace config.
          </Text>
        </div>
        {ctx.isAdmin && (
          <Button
            size="xs"
            leftSection={<IconPlus size={14} />}
            onClick={() => navigate(`${getPathSpace(space)}/volumes/create`)}
          >
            New Volume
          </Button>
        )}
      </div>

      {!ctx.isAdmin && (
        <Alert color="gray" variant="light">
          Only Space admins can create, resize or delete Volumes.
        </Alert>
      )}

      <QueryBoundary query={qry}>
        {qry.data && (
          <Stack gap="md">
            {qry.data.items.length === 0 ? (
              <Empty
                icon={<IconDatabase size={22} />}
                title="No Volumes in this Space"
                description="Create a Volume to keep datasets, caches and build artifacts beyond the lifetime of a single Workspace."
                action={
                  ctx.isAdmin ? (
                    <Button
                      leftSection={<IconPlus size={15} />}
                      onClick={() =>
                        navigate(`${getPathSpace(space)}/volumes/create`)
                      }
                    >
                      New Volume
                    </Button>
                  ) : undefined
                }
              />
            ) : (
              <CardList>
                {qry.data.items.map((x) => (
                  <VolumeRow
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
