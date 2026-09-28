import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import VolumeSelect from "@/components/VolumeSelect";
import { getShortNameFromRef } from "@/utils/pb";
import { Stack, Switch, Text, TextInput } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { SectionProps } from "./types";

const VolumesSection = (props: SectionProps) => {
  const { kind, spec, patch } = props;
  const mounts = spec.runtime?.volumeMounts ?? [];

  return (
    <Stack gap="md">
      <Text size="xs" c="dimmed">
        {kind === "Template"
          ? "Mounted into every Workspace created from this Template. Workspaces can add their own mounts on top."
          : "Merged with the mounts defined by the Template. Volumes live in the Space and outlive the Workspace."}
      </Text>

      <RepeatBlock
        title="Volume mounts"
        description="Attach Volumes of this Space at absolute paths inside the container."
        addLabel="Add mount"
        emptyHint="No Volumes mounted."
        count={mounts.length}
        onAdd={() =>
          patch((d) => {
            if (!d.runtime) {
              d.runtime = WsPB.Workspace_Spec_Runtime.create();
            }
            d.runtime.volumeMounts.push(
              WsPB.Workspace_Spec_Runtime_VolumeMount.create({
                mountPath: "",
              }),
            );
          })
        }
      >
        {mounts.map((mount, idx) => (
          <RepeatItem
            key={idx}
            index={idx}
            label={
              mount.volumeRef
                ? `${getShortNameFromRef(mount.volumeRef)}${
                    mount.mountPath ? ` → ${mount.mountPath}` : ""
                  }`
                : mount.mountPath
            }
            onRemove={() =>
              patch((d) => {
                d.runtime!.volumeMounts.splice(idx, 1);
              })
            }
          >
            <Stack gap="md">
              <div className="grid gap-4 md:grid-cols-2">
                <VolumeSelect
                  spaceRef={props.spaceRef}
                  required
                  value={mount.volumeRef}
                  onChange={(ref) =>
                    patch((d) => {
                      d.runtime!.volumeMounts[idx].volumeRef = ref;
                    })
                  }
                />
                <TextInput
                  label="Mount path"
                  description="Absolute path inside the container."
                  placeholder="/data"
                  required
                  value={mount.mountPath}
                  onChange={(e) => {
                    const v = e.currentTarget.value;
                    patch((d) => {
                      d.runtime!.volumeMounts[idx].mountPath = v;
                    });
                  }}
                />
              </div>
              <Switch
                size="sm"
                label="Read-only"
                description="Mount the Volume read-only inside this Workspace."
                checked={mount.readOnly}
                onChange={(e) => {
                  const v = e.currentTarget.checked;
                  patch((d) => {
                    d.runtime!.volumeMounts[idx].readOnly = v;
                  });
                }}
              />
            </Stack>
          </RepeatItem>
        ))}
      </RepeatBlock>
    </Stack>
  );
};

export default VolumesSection;
