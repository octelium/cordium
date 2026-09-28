import OptionalBlock from "@/components/OptionalBlock";
import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import {
  SegmentedControl,
  Select,
  Stack,
  TagsInput,
  Text,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconWorld } from "@tabler/icons-react";
import { SectionProps } from "./types";

const Action = WsPB.Workspace_Spec_Runtime_Network_Rule_Action;
const DefaultAction = WsPB.Workspace_Spec_Runtime_Network_Egress_DefaultAction;

const defaultActionData = [
  {
    value: DefaultAction[DefaultAction.ALLOW_PUBLIC],
    label: "Allow public — deny private networks",
  },
  { value: DefaultAction[DefaultAction.DENY], label: "Deny everything" },
];

const NetworkSection = (props: SectionProps) => {
  const { spec, patch } = props;
  const egress = spec.runtime?.network?.egress;

  return (
    <Stack gap="md">
      <Text size="xs" c="dimmed">
        DENY rules always win over ALLOW rules. The Cluster's own protected
        networks are denied regardless of this configuration.
      </Text>

      <OptionalBlock
        icon={<IconWorld size={16} />}
        title="Egress policy"
        description="Control which destinations the Workspace can reach."
        enabled={!!egress}
        onEnable={() =>
          patch((d) => {
            if (!d.runtime) {
              d.runtime = WsPB.Workspace_Spec_Runtime.create();
            }
            d.runtime.network = WsPB.Workspace_Spec_Runtime_Network.create({
              egress: {
                defaultAction: DefaultAction.ALLOW_PUBLIC,
                rules: [],
              },
            });
          })
        }
        onDisable={() =>
          patch((d) => {
            d.runtime!.network = undefined;
          })
        }
      >
        {egress && (
          <Stack gap="lg">
            <Select
              label="Default action"
              description="Applied to destinations that no rule matches."
              allowDeselect={false}
              data={defaultActionData}
              value={
                DefaultAction[
                  egress.defaultAction === DefaultAction.DEFAULT_ACTION_UNSET
                    ? DefaultAction.ALLOW_PUBLIC
                    : egress.defaultAction
                ]
              }
              onChange={(val) => {
                if (!val) return;
                patch((d) => {
                  d.runtime!.network!.egress!.defaultAction = DefaultAction[
                    val as keyof typeof DefaultAction
                  ] as WsPB.Workspace_Spec_Runtime_Network_Egress_DefaultAction;
                });
              }}
            />

            <RepeatBlock
              title="Rules"
              description="Match destination networks and, optionally, ports."
              addLabel="Add rule"
              emptyHint="No rules defined. The default action applies to all traffic."
              count={egress.rules.length}
              onAdd={() =>
                patch((d) => {
                  d.runtime!.network!.egress!.rules.push(
                    WsPB.Workspace_Spec_Runtime_Network_Rule.create({
                      action: Action.ALLOW,
                    }),
                  );
                })
              }
            >
              {egress.rules.map((rule, idx) => (
                <RepeatItem
                  key={idx}
                  index={idx}
                  label={`${rule.action === Action.DENY ? "Deny" : "Allow"}${
                    rule.cidrs.length > 0 ? ` ${rule.cidrs.join(", ")}` : ""
                  }`}
                  onRemove={() =>
                    patch((d) => {
                      d.runtime!.network!.egress!.rules.splice(idx, 1);
                    })
                  }
                >
                  <Stack gap="md">
                    <div>
                      <Text size="sm" fw={500} mb={6}>
                        Action
                      </Text>
                      <SegmentedControl
                        size="xs"
                        value={rule.action === Action.DENY ? "deny" : "allow"}
                        onChange={(v) =>
                          patch((d) => {
                            d.runtime!.network!.egress!.rules[idx].action =
                              v === "deny" ? Action.DENY : Action.ALLOW;
                          })
                        }
                        data={[
                          { label: "Allow", value: "allow" },
                          { label: "Deny", value: "deny" },
                        ]}
                      />
                    </div>
                    <div className="grid gap-4 md:grid-cols-2">
                      <TagsInput
                        label="CIDRs"
                        description="Destination networks. Press Enter after each."
                        placeholder="10.0.0.0/8"
                        required
                        value={rule.cidrs}
                        onChange={(v) =>
                          patch((d) => {
                            d.runtime!.network!.egress!.rules[idx].cidrs = v;
                          })
                        }
                      />
                      <TagsInput
                        label="Ports"
                        description="Destination ports. Leave empty to match every port."
                        placeholder="443"
                        value={rule.ports.map((x) => `${x}`)}
                        onChange={(v) =>
                          patch((d) => {
                            d.runtime!.network!.egress!.rules[idx].ports = v
                              .map((x) => parseInt(x, 10))
                              .filter((x) => x > 0 && x <= 65535);
                          })
                        }
                      />
                    </div>
                  </Stack>
                </RepeatItem>
              ))}
            </RepeatBlock>
          </Stack>
        )}
      </OptionalBlock>
    </Stack>
  );
};

export default NetworkSection;
