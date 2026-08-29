import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import { Alert, Stack, TextInput } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconVariable } from "@tabler/icons-react";
import { SectionProps } from "./types";

const VarsSection = (props: SectionProps) => {
  const { spec, patch, kind } = props;

  return (
    <Stack gap="md">
      <Alert
        variant="light"
        color="gray"
        icon={<IconVariable size={16} />}
        title="How variables work"
      >
        Reference a variable anywhere in the spec as{" "}
        <code className="font-mono text-[0.8em]">{"${{ vars.NAME }}"}</code>. It
        is substituted when the Workspace starts.{" "}
        {kind === "Template"
          ? "Workspaces created from this Template can override any of these values."
          : "Values set here override the Template's defaults."}
      </Alert>

      <RepeatBlock
        title="Variables"
        description="Name/value pairs used for spec substitution."
        addLabel="Add variable"
        emptyHint="No variables defined."
        count={spec.vars.length}
        onAdd={() =>
          patch((d) => {
            d.vars.push(WsPB.Workspace_Spec_Var.create());
          })
        }
      >
        {spec.vars.map((v, idx) => (
          <RepeatItem
            key={idx}
            index={idx}
            label={v.name}
            onRemove={() =>
              patch((d) => {
                d.vars.splice(idx, 1);
              })
            }
          >
            <div className="grid gap-4 md:grid-cols-2">
              <TextInput
                label="Name"
                description="Referenced as ${{ vars.NAME }}."
                placeholder="BRANCH"
                required
                value={v.name}
                onChange={(e) => {
                  const val = e.currentTarget.value;
                  patch((d) => {
                    d.vars[idx].name = val;
                  });
                }}
              />
              <TextInput
                label="Value"
                description={
                  kind === "Template"
                    ? "Default value when nothing overrides it."
                    : "Value used by this Workspace."
                }
                placeholder="main"
                value={v.value}
                onChange={(e) => {
                  const val = e.currentTarget.value;
                  patch((d) => {
                    d.vars[idx].value = val;
                  });
                }}
              />
            </div>
          </RepeatItem>
        ))}
      </RepeatBlock>
    </Stack>
  );
};

export default VarsSection;
