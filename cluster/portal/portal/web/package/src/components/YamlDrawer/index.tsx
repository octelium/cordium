import { Resource, resourceSpecToYAML, resourceToYAML } from "@/utils/pb";
import {
  ActionIcon,
  Button,
  Drawer,
  SegmentedControl,
  Stack,
  Tooltip,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconFileCode } from "@tabler/icons-react";
import * as React from "react";
import CodeEditor from "../CodeEditor";

const YamlDrawer = (props: {
  item: Resource;
  label?: string;
  iconOnly?: boolean;
}) => {
  const [opened, { open, close }] = useDisclosure(false);
  const [scope, setScope] = React.useState("full");

  return (
    <>
      {props.iconOnly ? (
        <Tooltip label="View YAML">
          <ActionIcon
            variant="default"
            size={36}
            aria-label="View YAML"
            onClick={open}
          >
            <IconFileCode size={16} />
          </ActionIcon>
        </Tooltip>
      ) : (
        <Button
          variant="default"
          leftSection={<IconFileCode size={15} />}
          onClick={open}
        >
          {props.label ?? "View YAML"}
        </Button>
      )}

      <Drawer
        opened={opened}
        onClose={close}
        size="xl"
        title={`${props.item.kind} · ${props.item.metadata?.name}`}
      >
        <Stack gap="md">
          <SegmentedControl
            size="xs"
            value={scope}
            onChange={setScope}
            data={[
              { label: "Full resource", value: "full" },
              { label: "Spec only", value: "spec" },
            ]}
            className="w-fit"
          />
          <CodeEditor
            mode="yaml"
            readOnly
            minHeight="420px"
            maxHeight="calc(100vh - 220px)"
            value={
              scope === "spec"
                ? resourceSpecToYAML(props.item)
                : resourceToYAML(props.item)
            }
          />
        </Stack>
      </Drawer>
    </>
  );
};

export default YamlDrawer;
