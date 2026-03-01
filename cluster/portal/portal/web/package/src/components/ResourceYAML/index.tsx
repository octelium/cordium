import { Resource, resourceSpecToYAML, resourceToYAML } from "@/utils/pb";

import * as React from "react";

import { Button, Drawer } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import Editor from "../Editor";

const ResourceYAML = (props: {
  item: Resource;
  size?: "xs" | "small";
  btnItem?: boolean;
}) => {
  let [showSpec, setShowSpec] = React.useState(false);
  const [opened, { open, close }] = useDisclosure(false);

  return (
    <>
      <Button size="compact-xs" variant="outline" onClick={open}>
        YAML
      </Button>
      <Drawer opened={opened} onClose={close} size={"lg"}>
        <div>
          <Editor
            mode="yaml"
            value={
              showSpec
                ? resourceSpecToYAML(props.item)
                : resourceToYAML(props.item)
            }
            readOnly
            onChange={() => {}}
          />
        </div>
      </Drawer>
    </>
  );
};

export default ResourceYAML;
