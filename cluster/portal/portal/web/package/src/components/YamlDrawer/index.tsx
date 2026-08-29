import { Resource, resourceSpecToYAML, resourceToYAML } from "@/utils/pb";
import { Button, Drawer, SegmentedControl, Stack } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconFileCode } from "@tabler/icons-react";
import * as React from "react";
import CodeEditor from "../CodeEditor";

const YamlDrawer = (props: { item: Resource; label?: string }) => {
  const [opened, { open, close }] = useDisclosure(false);
  const [scope, setScope] = React.useState("full");

  return (
    <>
      <Button
        size="compact-xs"
        variant="default"
        leftSection={<IconFileCode size={13} />}
        onClick={open}
      >
        {props.label ?? "View YAML"}
      </Button>

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
