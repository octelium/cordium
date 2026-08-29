import { getShortNameFromStr } from "@/utils/pb";
import { Stack, TextInput } from "@mantine/core";
import { Metadata } from "@octelium/apis/main/metav1";

const MetadataEdit = (props: {
  metadata: Metadata;
  onChange: (md: Metadata) => void;
  parentName?: string;
  nameDescription?: string;
  skipDisplayName?: boolean;
  withDescription?: boolean;
  disableName?: boolean;
}) => {
  const md = props.metadata;
  const shortName = getShortNameFromStr(md.name);

  const patch = (fn: (draft: Metadata) => void) => {
    const next = Metadata.clone(md);
    fn(next);
    props.onChange(next);
  };

  return (
    <Stack gap="md">
      <div className="grid gap-4 md:grid-cols-2">
        <TextInput
          label="Name"
          description={
            props.nameDescription ??
            "Unique identifier. Lowercase letters, digits and dashes."
          }
          placeholder="my-resource"
          required
          disabled={props.disableName}
          value={shortName}
          onChange={(e) => {
            const arg = e.currentTarget.value;
            patch((d) => {
              d.name = props.parentName ? `${arg}.${props.parentName}` : arg;
            });
          }}
        />

        {!props.skipDisplayName && (
          <TextInput
            label="Display name"
            description="Optional human-friendly label shown across the portal."
            placeholder="My Resource"
            value={md.displayName}
            onChange={(e) => {
              const v = e.currentTarget.value;
              patch((d) => {
                d.displayName = v;
              });
            }}
          />
        )}
      </div>

      {props.withDescription && (
        <TextInput
          label="Description"
          description="Optional short sentence describing what this is for."
          placeholder="Build environment for the payments service"
          value={md.description}
          onChange={(e) => {
            const v = e.currentTarget.value;
            patch((d) => {
              d.description = v;
            });
          }}
        />
      )}

      {props.parentName && shortName.length > 0 && (
        <p className="-mt-1 text-[0.75rem] font-medium text-slate-400">
          Full name:{" "}
          <span className="font-mono text-slate-500">
            {shortName}.{props.parentName}
          </span>
        </p>
      )}
    </Stack>
  );
};

export default MetadataEdit;
