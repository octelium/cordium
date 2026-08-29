import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import SpecEditor from "@/components/SpecEditor";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import {
  getPathWorkspace,
  invalidateWorkspace,
} from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import {
  Button,
  Collapse,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import {
  IconAdjustmentsHorizontal,
  IconBox,
  IconGitBranch,
  IconInfoCircle,
  IconPlayerPlay,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { useNavigate } from "react-router-dom";

const newWorkspace = (templateRef: MetaPB.ObjectReference) =>
  WsPB.Workspace.create({
    apiVersion: "cordium/v1",
    kind: "Workspace",
    metadata: {},
    spec: {},
    status: { templateRef },
  });

const RegionPicker = (props: {
  value: string;
  onChange: (val: string) => void;
}) => {
  const qry = useQuery({
    queryKey: ["workspace/listRegion"],
    queryFn: () => {
      const { response } = getClientWorkspace().listRegion(
        WsPB.ListRegionOptions.create({}),
      );
      return response;
    },
  });

  const items = qry.data?.items ?? [];
  if (items.length < 2) return null;

  return (
    <Select
      label="Region"
      description="Where the Workspace runs. Defaults to your preferred region."
      placeholder="Default"
      clearable
      data={items.map((x) => ({
        value: x.metadata!.name,
        label: [x.metadata!.name, x.status?.city, x.status?.country]
          .filter(Boolean)
          .join(" · "),
      }))}
      value={props.value || null}
      onChange={(val) => props.onChange(val ?? "")}
    />
  );
};

const LaunchForm = (props: {
  template: WsPB.Template;
  templates: WsPB.Template[];
  onTemplateChange?: (t: WsPB.Template) => void;
}) => {
  const { template } = props;
  const client = getClientWorkspace();
  const navigate = useNavigate();

  const [req, setReq] = React.useState(() =>
    newWorkspace(getResourceRef(template)),
  );
  const [advanced, setAdvanced] = React.useState(false);
  const [isEphemeral, setIsEphemeral] = React.useState(false);
  const [doStart, setDoStart] = React.useState(true);
  const [region, setRegion] = React.useState("");
  const [varOverrides, setVarOverrides] = React.useState<
    Record<string, string>
  >({});

  const templateVars = template.spec?.vars ?? [];

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = WsPB.Workspace.clone(req);
      payload.spec!.isEphemeral = isEphemeral;

      const overridden = templateVars
        .filter((v) => (varOverrides[v.name] ?? "") !== "")
        .map((v) =>
          WsPB.Workspace_Spec_Var.create({
            name: v.name,
            value: varOverrides[v.name],
          }),
        );
      if (overridden.length > 0) {
        payload.spec!.vars = overridden;
      }

      const { response } = await client.createWorkspace(payload);

      if (doStart) {
        await client.startWorkspace(
          WsPB.StartWorkspaceRequest.create({
            workspaceRef: getResourceRef(response),
            config: region
              ? { vars: [], regionRef: MetaPB.ObjectReference.create({ name: region }) }
              : undefined,
          }),
        );
      }

      return response;
    },
    onSuccess: (response) => {
      invalidateWorkspace(response);
      navigate(getPathWorkspace(response));
    },
    onError,
  });

  const repoUrl = req.spec?.repository?.url ?? "";
  const imageUrl =
    req.spec?.image?.type.oneofKind === "registry"
      ? req.spec.image.type.registry.url
      : "";

  return (
    <Panel>
      <PanelHeader
        icon={<IconPlayerPlay size={16} />}
        title="Launch a workspace"
        description={
          props.templates.length > 1
            ? "Pick a Template, tweak the essentials, and start coding."
            : `Created from the ${getShortName(template)} Template.`
        }
        actions={
          props.templates.length > 1 && props.onTemplateChange ? (
            <Select
              size="xs"
              w={220}
              aria-label="Template"
              allowDeselect={false}
              data={props.templates.map((x) => ({
                value: x.metadata!.uid,
                label: x.metadata!.displayName || getShortName(x),
              }))}
              value={template.metadata!.uid}
              onChange={(val) => {
                const found = props.templates.find(
                  (x) => x.metadata!.uid === val,
                );
                if (found) props.onTemplateChange!(found);
              }}
            />
          ) : undefined
        }
      />

      <PanelBody>
        <Stack gap="md">
          <div className="grid gap-4 md:grid-cols-2">
            <TextInput
              label="Repository"
              description="Overrides the Template repository for this Workspace."
              placeholder={
                template.spec?.repository?.url || "https://github.com/org/repo"
              }
              leftSection={<IconGitBranch size={15} />}
              value={repoUrl}
              onChange={(e) => {
                const v = e.currentTarget.value;
                const next = WsPB.Workspace.clone(req);
                if (!next.spec!.repository) {
                  next.spec!.repository =
                    WsPB.Workspace_Spec_Repository.create();
                }
                next.spec!.repository.url = v;
                setReq(next);
              }}
            />

            <TextInput
              label="Container image"
              description="Overrides the Template image for this Workspace."
              placeholder={
                template.spec?.image?.type.oneofKind === "registry"
                  ? template.spec.image.type.registry.url
                  : "ubuntu:24.04"
              }
              leftSection={<IconBox size={15} />}
              value={imageUrl}
              onChange={(e) => {
                const v = e.currentTarget.value;
                const next = WsPB.Workspace.clone(req);
                if (!next.spec!.image) {
                  next.spec!.image = WsPB.Workspace_Spec_Image.create();
                }
                if (next.spec!.image.type.oneofKind !== "registry") {
                  next.spec!.image.type = {
                    oneofKind: "registry",
                    registry: WsPB.Workspace_Spec_Image_Registry.create(),
                  };
                }
                next.spec!.image.type.registry.url = v;
                setReq(next);
              }}
            />
          </div>

          {templateVars.length > 0 && (
            <div>
              <Text size="sm" fw={700} mb={2}>
                Template variables
              </Text>
              <Text size="xs" c="dimmed" mb={10}>
                Leave a field empty to keep the Template default.
              </Text>
              <div className="grid gap-4 md:grid-cols-2">
                {templateVars.map((v) => (
                  <TextInput
                    key={v.name}
                    label={v.name}
                    placeholder={v.value || "(empty)"}
                    value={varOverrides[v.name] ?? ""}
                    onChange={(e) =>
                      setVarOverrides({
                        ...varOverrides,
                        [v.name]: e.currentTarget.value,
                      })
                    }
                  />
                ))}
              </div>
            </div>
          )}

          <RegionPicker value={region} onChange={setRegion} />

          <div className="flex flex-wrap gap-6 rounded-lg border border-slate-200 bg-slate-50/70 px-4 py-3">
            <Switch
              size="sm"
              checked={isEphemeral}
              onChange={(e) => setIsEphemeral(e.currentTarget.checked)}
              label={
                <span className="inline-flex items-center gap-1.5">
                  Ephemeral storage
                  <Tooltip
                    multiline
                    w={260}
                    label="Storage is discarded when the Workspace stops, and ON_CREATE tasks re-run on every start."
                  >
                    <IconInfoCircle size={13} className="text-slate-400" />
                  </Tooltip>
                </span>
              }
            />
            <Switch
              size="sm"
              checked={doStart}
              onChange={(e) => setDoStart(e.currentTarget.checked)}
              label="Start immediately"
            />
          </div>

          <div>
            <Button
              size="compact-sm"
              variant="subtle"
              color="gray"
              leftSection={<IconAdjustmentsHorizontal size={14} />}
              onClick={() => setAdvanced((v) => !v)}
            >
              {advanced ? "Hide full configuration" : "Full configuration"}
            </Button>

            <Collapse expanded={advanced}>
              <div className="mt-4 rounded-xl border border-slate-200 p-4">
                <SpecEditor
                  kind="Workspace"
                  item={req}
                  spaceRef={template.status!.spaceRef!}
                  onChange={(next) => setReq(next as WsPB.Workspace)}
                />
              </div>
            </Collapse>
          </div>
        </Stack>
      </PanelBody>

      <PanelFooter>
        <Button
          leftSection={<IconPlayerPlay size={15} />}
          loading={mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          {doStart ? "Create & start" : "Create workspace"}
        </Button>
      </PanelFooter>
    </Panel>
  );
};

const LaunchWorkspace = (props: {
  spaceRef: MetaPB.ObjectReference;
  templateRef?: MetaPB.ObjectReference;
}) => {
  const qry = useQuery({
    queryKey: ["workspace/listTemplate", props.spaceRef.uid, "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listTemplate(
        WsPB.ListTemplateOptions.create({
          spaceRef: props.spaceRef,
          common: { itemsPerPage: 500 },
        }),
      );
      return response;
    },
  });

  const templates = qry.data?.items ?? [];
  const [selectedUID, setSelectedUID] = React.useState<string | null>(null);

  const fixed = props.templateRef
    ? templates.find((x) => x.metadata!.uid === props.templateRef!.uid)
    : undefined;

  const selected =
    fixed ??
    templates.find((x) => x.metadata!.uid === selectedUID) ??
    templates.find((x) => x.metadata!.name.startsWith("default.")) ??
    templates.at(0);

  if (!selected) return null;

  return (
    <LaunchForm
      key={selected.metadata!.uid}
      template={selected}
      templates={fixed ? [selected] : templates}
      onTemplateChange={
        fixed ? undefined : (t) => setSelectedUID(t.metadata!.uid)
      }
    />
  );
};

export default LaunchWorkspace;
