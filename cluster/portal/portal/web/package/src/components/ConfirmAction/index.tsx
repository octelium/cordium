import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import * as React from "react";

const ConfirmAction = (props: {
  onConfirm: () => void;
  title: string;
  description: React.ReactNode;
  confirmLabel: string;
  triggerLabel: string;
  triggerIcon?: React.ReactNode;
  color?: string;
  variant?: string;
  size?: string;
  fullWidth?: boolean;
  loading?: boolean;
  details?: React.ReactNode;
}) => {
  const [opened, { open, close }] = useDisclosure(false);

  return (
    <>
      <Button
        size={props.size ?? "xs"}
        variant={props.variant ?? "outline"}
        color={props.color ?? "red"}
        leftSection={props.triggerIcon}
        fullWidth={props.fullWidth}
        onClick={open}
      >
        {props.triggerLabel}
      </Button>

      <Modal opened={opened} onClose={close} size="md" title={props.title}>
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            {props.description}
          </Text>

          {props.details && (
            <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-2.5">
              {props.details}
            </div>
          )}

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              color={props.color ?? "red"}
              leftSection={props.triggerIcon}
              loading={props.loading}
              onClick={() => {
                props.onConfirm();
                close();
              }}
            >
              {props.confirmLabel}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

export default ConfirmAction;
