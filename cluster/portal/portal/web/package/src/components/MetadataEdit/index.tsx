import { Metadata } from "@/apis/metav1/metav1";
import { getShortNameFromStr } from "@/utils/pb";
import { Group, TextInput } from "@mantine/core";
import * as React from "react";

const MetadataEdit = (props: {
  metadata: Metadata;
  onUpdate: (md: Metadata) => void;
  parentName?: string;
  skipDisplayName?: boolean;
}) => {
  const [req, setReq] = React.useState(Metadata.clone(props.metadata));

  const update = (next: Metadata) => {
    const cloned = Metadata.clone(next);
    setReq(cloned);
    props.onUpdate(cloned);
  };

  const shortName = getShortNameFromStr(req.name);

  return (
    <Group grow align="flex-start">
      <TextInput
        label="Name"
        description={"Set a unique name for th resource"}
        placeholder="my-resource"
        required
        value={shortName}
        onChange={(e) => {
          const arg = e.currentTarget.value;
          req.name = props.parentName ? `${arg}.${props.parentName}` : arg;
          update(req);
        }}
      />

      {!props.skipDisplayName && (
        <TextInput
          label="Display name"
          description="Optional human-friendly label"
          placeholder="My Resource"
          value={req.displayName}
          onChange={(e) => {
            req.displayName = e.currentTarget.value;
            update(req);
          }}
        />
      )}
    </Group>
  );
};

export default MetadataEdit;
