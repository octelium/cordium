import OptionalBlock from "@/components/OptionalBlock";
import { NumberInput, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { SectionProps } from "./types";

const LimitsSection = (props: SectionProps) => {
  const { spec, patch } = props;
  const limit = spec.limit;

  return (
    <Stack gap="md">
      <Text size="xs" c="dimmed">
        Leave off to inherit the Space default. Values above the Space maximum
        are rejected by the Cluster.
      </Text>

      <OptionalBlock
        title="Resource limits"
        description="Caps applied to the Workspace container."
        enabled={!!limit}
        onEnable={() =>
          patch((d) => {
            d.limit = WsPB.Workspace_Spec_Limit.create({
              cpu: { millicores: 2000 },
              memory: { megabytes: 4096 },
              storage: { megabytes: 20000 },
            });
          })
        }
        onDisable={() =>
          patch((d) => {
            d.limit = undefined;
          })
        }
      >
        {limit && (
          <div className="grid gap-4 md:grid-cols-3">
            <NumberInput
              label="CPU"
              description="Millicores. 1000 = 1 vCPU."
              placeholder="2000"
              min={0}
              max={1000000}
              step={500}
              value={limit.cpu?.millicores ?? 0}
              onChange={(v) => {
                const n = typeof v === "number" ? v : Number(v) || 0;
                patch((d) => {
                  d.limit!.cpu = WsPB.Workspace_Spec_Limit_CPU.create({
                    millicores: n,
                  });
                });
              }}
            />
            <NumberInput
              label="Memory"
              description="Megabytes of RAM. 4096 = 4 GB."
              placeholder="4096"
              min={0}
              max={10000000}
              step={512}
              value={limit.memory?.megabytes ?? 0}
              onChange={(v) => {
                const n = typeof v === "number" ? v : Number(v) || 0;
                patch((d) => {
                  d.limit!.memory = WsPB.Workspace_Spec_Limit_Memory.create({
                    megabytes: n,
                  });
                });
              }}
            />
            <NumberInput
              label="Storage"
              description="Megabytes of persistent disk. 20000 = 20 GB."
              placeholder="20000"
              min={0}
              max={10000000}
              step={1000}
              value={limit.storage?.megabytes ?? 0}
              onChange={(v) => {
                const n = typeof v === "number" ? v : Number(v) || 0;
                patch((d) => {
                  d.limit!.storage = WsPB.Workspace_Spec_Limit_Storage.create({
                    megabytes: n,
                  });
                });
              }}
            />
          </div>
        )}
      </OptionalBlock>
    </Stack>
  );
};

export default LimitsSection;
