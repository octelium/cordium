import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { Trash2 } from "lucide-react";

const DeleteResource = (props: {
  onDelete: () => void;
  label?: string;
  btnSize?: "xs" | "sm";
}) => {
  const [opened, { open, close }] = useDisclosure(false);

  return (
    <>
      <Button
        size={`compact-sm`}
        variant="outline"
        color="red"
        leftSection={<Trash2 size={13} />}
        onClick={open}
      >
        {props.label ?? "Delete"}
      </Button>

      <Modal
        opened={opened}
        onClose={close}
        centered
        size="sm"
        title={
          <Text fw={600} size="sm">
            Confirm deletion
          </Text>
        }
      >
        <Stack gap="lg">
          <Text size="sm" c="dimmed">
            This action cannot be undone. Are you sure you want to delete this
            resource?
          </Text>

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              color="red"
              leftSection={<Trash2 size={13} />}
              onClick={() => {
                props.onDelete();
                close();
              }}
              autoFocus
            >
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

export default DeleteResource;
