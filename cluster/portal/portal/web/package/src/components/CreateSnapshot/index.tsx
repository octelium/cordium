import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { invalidateWorkspaceSnapshots } from "@/utils/octelium";
import { getResourceRef, isWorkspaceStopped } from "@/utils/pb";
import {
  Button,
  Group,
  Modal,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconCameraPlus } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";

const CreateSnapshot = (props: {
  item: WsPB.Workspace;
  size?: string;
  variant?: string;
  fullWidth?: boolean;
}) => {
  const client = getClientWorkspace();
  const { item } = props;
  const [opened, { open, close }] = useDisclosure(false);
  const [name, setName] = React.useState("");
  const [displayName, setDisplayName] = React.useState("");
  const userName = useAppSelector(
    (s) => s.settings.status?.user?.metadata?.name,
  );

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createWorkspaceSnapshot(
        WsPB.WorkspaceSnapshot.create({
          apiVersion: "cordium/v1",
          kind: "WorkspaceSnapshot",
          metadata: {
            name: userName ? `${name}.${userName}` : name,
            displayName,
          },
          spec: {},
          status: { workspaceRef: getResourceRef(item) },
        }),
      );
      return response;
    },
    onSuccess: () => {
      close();
      setName("");
      setDisplayName("");
      invalidateWorkspaceSnapshots();
      toast.success("Snapshot started");
    },
    onError,
  });

  return (
    <>
      <Button
        size={props.size ?? "xs"}
        variant={props.variant ?? "default"}
        fullWidth={props.fullWidth}
        leftSection={<IconCameraPlus size={14} />}
        onClick={open}
      >
        Take snapshot
      </Button>

      <Modal opened={opened} onClose={close} size="md" title="Take snapshot">
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            {isWorkspaceStopped(item)
              ? "The workspace is stopped, so the snapshot is a clean copy of its persistent storage."
              : "The workspace keeps running while the snapshot is taken. The result is crash-consistent: data still buffered in memory is not included."}{" "}
            Mounted Volumes are never included.
          </Text>

          <div className="rounded-lg border border-line bg-surface-subtle px-4 py-2.5">
            <Text size="xs" c="dimmed">
              Workspace
            </Text>
            <Text size="sm" fw={600} className="font-mono">
              {item.metadata!.name}
            </Text>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <TextInput
              label="Name"
              description="Lowercase letters, digits and dashes."
              placeholder="before-upgrade"
              required
              value={name}
              onChange={(e) => setName(e.currentTarget.value)}
            />
            <TextInput
              label="Display name"
              description="Optional human-friendly label."
              placeholder="Before upgrade"
              value={displayName}
              onChange={(e) => setDisplayName(e.currentTarget.value)}
            />
          </div>

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              leftSection={<IconCameraPlus size={14} />}
              loading={mutation.isPending}
              disabled={!name}
              onClick={() => mutation.mutate()}
            >
              Take snapshot
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

export default CreateSnapshot;
