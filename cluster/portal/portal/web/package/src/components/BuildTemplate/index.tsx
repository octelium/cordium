import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { invalidateTemplate } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import {
  Button,
  Group,
  Modal,
  Stack,
  TagsInput,
  Text,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconHammer } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";

const BuildTemplate = (props: {
  item: WsPB.Template;
  size?: string;
  variant?: string;
}) => {
  const client = getClientWorkspace();
  const { item } = props;
  const [opened, { open, close }] = useDisclosure(false);
  const [tags, setTags] = React.useState<string[]>([]);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.buildTemplate(
        WsPB.BuildTemplateRequest.create({
          templateRef: getResourceRef(item),
          tags,
        }),
      );
      return response;
    },
    onSuccess: () => {
      close();
      setTags([]);
      invalidateTemplate(item);
      toast.success("Build started");
    },
    onError,
  });

  return (
    <>
      <Button
        size={props.size ?? "xs"}
        variant={props.variant ?? "default"}
        leftSection={<IconHammer size={14} />}
        onClick={open}
      >
        Build
      </Button>

      <Modal opened={opened} onClose={close} size="md" title="Build template">
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            Builds the image defined by this Template so Workspaces start from a
            prebuilt layer instead of building on first run.
          </Text>

          <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-2.5">
            <Text size="xs" c="dimmed">
              Template
            </Text>
            <Text size="sm" fw={600} className="font-mono">
              {getShortName(item)}
            </Text>
          </div>

          <TagsInput
            label="Tags"
            description="Optional labels to identify this build later."
            placeholder="nightly"
            value={tags}
            onChange={setTags}
          />

          <Group justify="flex-end" gap="sm">
            <Button variant="default" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button
              size="sm"
              leftSection={<IconHammer size={14} />}
              loading={mutation.isPending}
              onClick={() => mutation.mutate()}
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
