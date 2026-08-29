import CodeEditor from "@/components/CodeEditor";
import { getClientWorkspace } from "@/utils/client";
import {
  cloneResource,
  getShortName,
  resourceFromYAML,
  resourceToYAML,
} from "@/utils/pb";
import { Alert, SegmentedControl, Select, Stack, Tabs } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import {
  IconAdjustments,
  IconBox,
  IconCpu,
  IconGitBranch,
  IconTerminal2,
  IconVariable,
  IconWorldWww,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import ApplicationsSection from "./Applications";
import ImageSection from "./Image";
import LimitsSection from "./Limits";
import RepositorySection from "./Repository";
import RuntimeSection from "./Runtime";
import { CommonSpec, SpecKind } from "./types";
import VarsSection from "./Vars";

type Editable = WsPB.Workspace | WsPB.Template;

const GitProviderPicker = (props: {
  spaceRef: MetaPB.ObjectReference;
  value: string;
  onChange: (val: string) => void;
}) => {
  const qry = useQuery({
    queryKey: ["workspace/listGitProvider", props.spaceRef.uid, "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listGitProvider(
        WsPB.ListGitProviderOptions.create({
          spaceRef: props.spaceRef,
          common: { itemsPerPage: 500 },
        }),
      );
      return response;
    },
  });

  const items = qry.data?.items ?? [];
  if (items.length === 0) return null;

  return (
    <Select
      label="Git provider"
      description="Workspaces from this Template ask the user to sign in to this provider, then clone with their own credentials."
      placeholder="None"
      clearable
      data={items.map((x) => ({
        value: x.metadata!.name,
        label: getShortName(x),
      }))}
      value={props.value || null}
      onChange={(val) => props.onChange(val ?? "")}
    />
  );
};

const SpecEditor = (props: {
  kind: SpecKind;
  item: Editable;
  spaceRef: MetaPB.ObjectReference;
  onChange: (item: Editable) => void;
}) => {
  const { item, kind } = props;
  const [mode, setMode] = React.useState("form");
  const [yamlDraft, setYamlDraft] = React.useState<string | null>(null);
  const [yamlError, setYamlError] = React.useState<string | null>(null);

  const spec = item.spec as CommonSpec;

  const patch = React.useCallback(
    (fn: (draft: CommonSpec) => void) => {
      const next = cloneResource(item);
      fn(next.spec as CommonSpec);
      props.onChange(next);
    },
    [item, props],
  );

  const applyYaml = (raw: string) => {
    try {
      const parsed = resourceFromYAML(raw);
      if (!parsed || parsed.kind !== kind) {
        setYamlError(`The document must be a ${kind} resource.`);
        return;
      }
      setYamlError(null);
      props.onChange(parsed as Editable);
      toast.success("YAML applied to the form");
      setYamlDraft(null);
      setMode("form");
    } catch (err) {
      setYamlError(err instanceof Error ? err.message : "Invalid YAML");
    }
  };

  const sections = [
    {
      value: "source",
      label: "Source",
      icon: <IconGitBranch size={14} />,
      body: <RepositorySection {...{ kind, spec, patch }} spaceRef={props.spaceRef} />,
    },
    {
      value: "image",
      label: "Image",
      icon: <IconBox size={14} />,
      body: <ImageSection {...{ kind, spec, patch }} spaceRef={props.spaceRef} />,
    },
    {
      value: "runtime",
      label: "Runtime",
      icon: <IconTerminal2 size={14} />,
      body: <RuntimeSection {...{ kind, spec, patch }} spaceRef={props.spaceRef} />,
    },
    ...(kind === "Workspace"
      ? [
          {
            value: "applications",
            label: "Applications",
            icon: <IconWorldWww size={14} />,
            body: (
              <ApplicationsSection
                {...{ kind, spec, patch }}
                spaceRef={props.spaceRef}
              />
            ),
          },
        ]
      : []),
    {
      value: "resources",
      label: "Resources",
      icon: <IconCpu size={14} />,
      body: <LimitsSection {...{ kind, spec, patch }} spaceRef={props.spaceRef} />,
    },
    {
      value: "variables",
      label: "Variables",
      icon: <IconVariable size={14} />,
      body: <VarsSection {...{ kind, spec, patch }} spaceRef={props.spaceRef} />,
    },
    ...(kind === "Template"
      ? [
          {
            value: "integrations",
            label: "Integrations",
            icon: <IconAdjustments size={14} />,
            body: (
              <GitProviderPicker
                spaceRef={props.spaceRef}
                value={(item as WsPB.Template).spec?.gitProvider ?? ""}
                onChange={(val) => {
                  const next = cloneResource(item as WsPB.Template);
                  next.spec!.gitProvider = val;
                  props.onChange(next);
                }}
              />
            ),
          },
        ]
      : []),
  ];

  return (
    <Stack gap="md">
      <SegmentedControl
        size="xs"
        className="w-fit"
        value={mode}
        onChange={(v) => {
          if (v === "yaml") {
            setYamlDraft(resourceToYAML(item));
            setYamlError(null);
          }
          setMode(v);
        }}
        data={[
          { label: "Form", value: "form" },
          { label: "YAML", value: "yaml" },
        ]}
      />

      {mode === "form" && (
        <Tabs defaultValue="source" orientation="vertical" variant="pills">
          <Tabs.List className="min-w-[11rem] pr-4">
            {sections.map((s) => (
              <Tabs.Tab key={s.value} value={s.value} leftSection={s.icon}>
                {s.label}
              </Tabs.Tab>
            ))}
          </Tabs.List>

          {sections.map((s) => (
            <Tabs.Panel key={s.value} value={s.value} className="min-w-0 flex-1">
              {s.body}
            </Tabs.Panel>
          ))}
        </Tabs>
      )}

      {mode === "yaml" && (
        <Stack gap="sm">
          {yamlError && (
            <Alert color="red" title="Could not parse the document">
              {yamlError}
            </Alert>
          )}
          <CodeEditor
            mode="yaml"
            value={yamlDraft ?? resourceToYAML(item)}
            minHeight="420px"
            onChange={(v) => setYamlDraft(v)}
          />
          <div className="flex justify-end">
            <button
              type="button"
              className="rounded-md bg-slate-800 px-3 py-1.5 text-[0.8rem] font-semibold text-white transition-colors duration-150 hover:bg-slate-900"
              onClick={() => applyYaml(yamlDraft ?? resourceToYAML(item))}
            >
              Apply to form
            </button>
          </div>
        </Stack>
      )}
    </Stack>
  );
};

export default SpecEditor;
