import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import { NumberInput, Switch, TextInput } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { SectionProps } from "./types";

const ApplicationsSection = (props: SectionProps) => {
  const spec = props.spec as WsPB.Workspace_Spec;
  const patch = props.patch as (
    fn: (draft: WsPB.Workspace_Spec) => void,
  ) => void;

  const apps = spec.applications ?? [];

  return (
    <RepeatBlock
      title="Applications"
      description="Named ports exposed over HTTPS. The default one is served at the Workspace URL; others at <name>_<workspace-host>."
      addLabel="Add application"
      emptyHint="No applications exposed. Add one to reach a server running inside the Workspace from your browser."
      count={apps.length}
      onAdd={() =>
        patch((d) => {
          d.applications.push(WsPB.Workspace_Spec_Application.create());
        })
      }
    >
      {apps.map((app, idx) => (
        <RepeatItem
          key={idx}
          index={idx}
          label={app.name || (app.port ? `:${app.port}` : undefined)}
          onRemove={() =>
            patch((d) => {
              d.applications.splice(idx, 1);
            })
          }
        >
          <div className="grid gap-4 md:grid-cols-3">
            <TextInput
              label="Name"
              description="Used as the URL prefix. Lowercase, unique."
              placeholder="web"
              required
              value={app.name}
              onChange={(e) => {
                const v = e.currentTarget.value;
                patch((d) => {
                  d.applications[idx].name = v;
                });
              }}
            />
            <TextInput
              label="Display name"
              description="Optional label shown in the portal."
              placeholder="Web UI"
              value={app.displayName}
              onChange={(e) => {
                const v = e.currentTarget.value;
                patch((d) => {
                  d.applications[idx].displayName = v;
                });
              }}
            />
            <NumberInput
              label="Port"
              description="Port your server listens on inside the Workspace."
              placeholder="3000"
              required
              min={1}
              max={65535}
              value={app.port || ""}
              onChange={(v) => {
                const n = typeof v === "number" ? v : Number(v) || 0;
                patch((d) => {
                  d.applications[idx].port = n;
                });
              }}
            />
          </div>

          <div className="mt-4">
            <Switch
              size="sm"
              label="Default application"
              description="Served directly at the Workspace URL. Only one application can be the default."
              checked={app.isDefault}
              onChange={(e) => {
                const v = e.currentTarget.checked;
                patch((d) => {
                  if (v) {
                    d.applications.forEach((a, i) => {
                      a.isDefault = i === idx;
                    });
                  } else {
                    d.applications[idx].isDefault = false;
                  }
                });
              }}
            />
          </div>
        </RepeatItem>
      ))}
    </RepeatBlock>
  );
};

export default ApplicationsSection;
