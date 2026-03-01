import * as React from "react";

import { Metadata } from "@/apis/metav1/metav1";
import Field from "@/components/Field";
import { getShortNameFromStr } from "@/utils/pb";
import { Group } from "@mantine/core";

const MetadataEdit = (props: {
  metadata: Metadata;
  onUpdate: (md: Metadata) => void;
  parentName?: string;
  skipDisplayName?: boolean;
}) => {
  let [req, setReq] = React.useState(Metadata.clone(props.metadata));

  return (
    <div className="w-full">
      <Group grow>
        <Field
          val={getShortNameFromStr(req.name)}
          label="Name"
          placeholder="my-resource"
          isRequired
          onChange={(v) => {
            const arg = v as string;
            req!.name = props.parentName ? `${arg}.${props.parentName}` : arg;
            setReq(Metadata.clone(req));
            props.onUpdate(req);
          }}
        />

        {!props.skipDisplayName && (
          <Field
            val={req.displayName}
            label="Display Name"
            placeholder="My Resource"
            onChange={(v) => {
              req.displayName = v as string;
              setReq(Metadata.clone(req));
              props.onUpdate(req);
            }}
          />
        )}
      </Group>
    </div>
  );
};

export default MetadataEdit;
