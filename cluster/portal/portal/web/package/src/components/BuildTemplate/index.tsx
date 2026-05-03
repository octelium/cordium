import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { invalidateResource } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { useMutation } from "@tanstack/react-query";
import { Hammer } from "lucide-react";
import { toast } from "react-hot-toast";

const BuildTemplate = (props: { item: WsPB.Template }) => {
  const client = getClientWorkspace();
  const { item } = props;
  const [opened, { open, close }] = useDisclosure(false);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.buildTemplate(
        WsPB.BuildTemplateRequest.create({ templateRef: getResourceRef(item) }),
      );
      return response;
    },
    onSuccess: () => {
      close();
      invalidateResource(item);
      toast.success("Build started");
    },
    onError,
  });

  return (
    <>
      <Button size="sm" leftSection={<Hammer size={14} />} onClick={open}>
        Build template
      </Button>

      <Modal
        opened={opened}
        onClose={close}
        centered
        size="sm"
        title={
          <Text fw={600} size="sm">
            Build template
          </Text>
        }
      >
        <Stack gap="lg">
          <div
            style={{
              background: "#f8fafc",
              border: "1px solid #e2e8f0",
              borderRadius: 8,
              padding: "10px 14px",
            }}
          >
            <Text size="xs" c="dimmed" mb={2}>
              Template
            </Text>
            <Text size="sm" fw={500} style={{ fontFamily: "monospace" }}>
              {getShortName(item)}
            </Text>
          </div>

          <Text size="sm" c="dimmed">
            This will start a new build for this Template.
          </Text>

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              leftSection={<Hammer size={13} />}
              loading={mutation.isPending}
              onClick={() => mutation.mutate()}
              autoFocus
            >
              Start build
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

export default BuildTemplate;
