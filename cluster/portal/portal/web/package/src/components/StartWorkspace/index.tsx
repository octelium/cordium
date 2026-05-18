import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as MetaPB from "@/apis/metav1/metav1";
import * as React from "react";

import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getResourceRef } from "@/utils/pb";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import {
  Badge,
  Button,
  Divider,
  Group,
  Select,
  Stack,
  Switch,
  Tabs,
  Text,
  TextInput,
  ThemeIcon,
  Tooltip,
} from "@mantine/core";

import {
  getPathWorkspace,
  invalidateResource,
  invalidateWorkspace,
} from "@/utils/octelium";

import TimeAgo from "../TimeAgo";
import WorkspaceEdit from "../WorkspaceEdit";

import {
  IconBox,
  IconBrandGit,
  IconDatabase,
  IconInfoCircle,
  IconLayoutGrid,
  IconPlayerPlay,
  IconSettings,
} from "@tabler/icons-react";

const StartWorkspaceTemplate = (props: {
  item: WsPB.Template;
  workspace: WsPB.Workspace;
  onUpdateWorkspace: (item: WsPB.Workspace) => void;
}) => {
  const { item, workspace } = props;

  const updateReq = (arg: WsPB.Workspace) => {
    const c = WsPB.Workspace.clone(arg);
    props.onUpdateWorkspace(c);
  };

  const req = workspace;

  return (
    <Stack gap="md">
      <Group grow align="flex-start">
        <TextInput
          label="Repository URL"
          description="Git repository to clone on start"
          placeholder="https://github.com/org/repo"
          leftSection={<IconBrandGit size={15} />}
          value={req.spec?.repository?.url ?? ""}
          onChange={(e) => {
            if (!req.spec!.repository) {
              req.spec!.repository = WsPB.Workspace_Spec_Repository.create();
            }
            req.spec!.repository!.url = e.currentTarget.value;
            updateReq(req);
          }}
        />

        <TextInput
          label="Docker Image"
          description="Base container image for this workspace"
          placeholder="ubuntu:jammy"
          leftSection={<IconBox size={15} />}
          value={
            req.spec?.image?.type.oneofKind === "registry" &&
            req.spec?.image?.type.registry.url
              ? req.spec.image.type.registry.url
              : ""
          }
          onChange={(e) => {
            if (!req.spec!.image) {
              req.spec!.image = WsPB.Workspace_Spec_Image.create();
            }
            if (req.spec!.image.type.oneofKind !== "registry") {
              req.spec!.image.type = {
                oneofKind: "registry",
                registry: WsPB.Workspace_Spec_Image_Registry.create(),
              };
            }
            req.spec!.image.type.registry.url = e.currentTarget.value;
            updateReq(req);
          }}
        />
      </Group>

      <ChooseBuild
        template={item}
        onSet={() => {
          updateReq(req);
        }}
      />
    </Stack>
  );
};

const WrapC = (props: {
  item: WsPB.Template;
  disableChooseTemplate?: boolean;
  disableChooseEnvironment?: boolean;
}) => {
  const [template, setTemplate] = React.useState(
    WsPB.Template.clone(props.item),
  );
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const [doStart, setDoStart] = React.useState(true);
  const [isEphemeral, setIsEphemeral] = React.useState(false);
  const [req, setReq] = React.useState(
    WsPB.Workspace.create({
      apiVersion: "cordium/v1",
      kind: "Workspace",
      metadata: {},
      spec: {},
      status: {
        templateRef: getResourceRef(template),
      },
    }),
  );

  const updateReq = (next: WsPB.Workspace) =>
    setReq(WsPB.Workspace.clone(next));

  const mutation = useMutation({
    mutationFn: async () => {
      const cloned = WsPB.Workspace.clone(req);
      cloned.spec!.isEphemeral = isEphemeral;
      const { response } = await client.createWorkspace(cloned);

      if (doStart) {
        await client.startWorkspace(
          WsPB.StartWorkspaceRequest.create({
            workspaceRef: getResourceRef(response),
          }),
        );
      }

      invalidateResource(response);
      return { response };
    },
    onSuccess: ({ response }) => {
      invalidateWorkspace(response);
      navigate(getPathWorkspace(response));
    },
    onError,
  });

  return (
    <div
      style={{
        background: "white",
        border: "1px solid #e2e8f0",
        borderRadius: 12,
        overflow: "hidden",
      }}
      className="my-8"
    >
      <div
        style={{
          padding: "20px 24px 16px",
          borderBottom: "1px solid #e2e8f0",
          background: "#f8fafc",
        }}
      >
        <Group justify="space-between" align="center">
          <Group gap="xs">
            <ThemeIcon variant="light" color="blue" size="md" radius="md">
              <IconPlayerPlay size={15} />
            </ThemeIcon>
            <div>
              <Text fw={600} size="sm">
                New Workspace/Sandbox
              </Text>
              <Text fw={700} size="xs" c="dimmed">
                Configure and launch a new Workspace/Sandbox
              </Text>
            </div>
          </Group>

          {!props.disableChooseTemplate && (
            <ChooseTemplate
              cur={template}
              spaceRef={template.status!.spaceRef!}
              onSet={(i) => {
                setTemplate(WsPB.Template.clone(i));
                req.status!.templateRef = getResourceRef(i);
                setReq(WsPB.Workspace.clone(req));
              }}
            />
          )}
        </Group>
      </div>

      <div
        style={{
          padding: "12px 24px",
          borderBottom: "1px solid #e2e8f0",
          background: "#fafafa",
        }}
      >
        <Group gap="xl">
          <Group gap="xs">
            <Switch
              size="sm"
              checked={isEphemeral}
              onChange={(e) => setIsEphemeral(e.currentTarget.checked)}
              label={
                <Group gap={4}>
                  <Text size="sm">Ephemeral storage</Text>
                  <Tooltip
                    label="Storage is wiped on every run — nothing persists between sessions"
                    withArrow
                    multiline
                    w={220}
                  >
                    <IconInfoCircle
                      size={13}
                      style={{ color: "var(--mantine-color-dimmed)" }}
                    />
                  </Tooltip>
                </Group>
              }
            />
          </Group>

          <Switch
            size="sm"
            checked={doStart}
            onChange={(e) => setDoStart(e.currentTarget.checked)}
            label={
              <Group gap={4}>
                <Text size="sm">Start immediately</Text>
                <Tooltip
                  label="Workspace will start automatically after creation"
                  withArrow
                  multiline
                  w={200}
                >
                  <IconInfoCircle
                    size={13}
                    style={{ color: "var(--mantine-color-dimmed)" }}
                  />
                </Tooltip>
              </Group>
            }
          />
        </Group>
      </div>

      <div style={{ padding: "0 24px 24px" }}>
        <Tabs defaultValue="main" mt="md">
          <Tabs.List mb="md">
            <Tabs.Tab value="main" leftSection={<IconLayoutGrid size={14} />}>
              Quick mode
            </Tabs.Tab>
            <Tabs.Tab value="custom" leftSection={<IconSettings size={14} />}>
              Customize
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="main">
            <StartWorkspaceTemplate
              item={template}
              workspace={req}
              onUpdateWorkspace={updateReq}
            />
          </Tabs.Panel>

          <Tabs.Panel value="custom">
            <WorkspaceEdit
              spaceRef={template!.status!.spaceRef!}
              item={req}
              onUpdate={(item) => {
                const v = item as WsPB.Workspace;
                const clone = WsPB.Workspace.clone(req);
                clone.spec = v.spec;
                setReq(clone);
              }}
            />
          </Tabs.Panel>
        </Tabs>

        <Divider my="lg" />

        <Group justify="flex-end" gap="sm">
          <Button
            size="sm"
            leftSection={<IconPlayerPlay size={14} />}
            loading={mutation.isPending}
            onClick={() => mutation.mutate(undefined)}
          >
            {doStart ? "Create & start" : "Create workspace"}
          </Button>
        </Group>
      </div>
    </div>
  );
};

const ChooseTemplate = (props: {
  cur?: WsPB.Template;
  spaceRef: MetaPB.ObjectReference;
  onSet: (item: WsPB.Template) => void;
}) => {
  const { spaceRef } = props;
  const client = getClientWorkspace();

  const qry = useQuery({
    queryKey: ["workspace/listTemplate", spaceRef.uid],
    queryFn: () => {
      const { response } = client.listTemplate(
        WsPB.ListTemplateOptions.create({ spaceRef }),
      );
      return response;
    },
  });

  if (!qry.isSuccess || qry.data.items.length === 0) return null;

  return (
    <Select
      label="Template"
      size="xs"
      w={200}
      value={
        props.cur ? props.cur.metadata!.uid : qry.data.items[0].metadata!.uid
      }
      onChange={(val) => {
        const tmpl = qry.data.items.find((x) => x.metadata!.uid === val);
        if (tmpl) props.onSet(tmpl);
      }}
      data={qry.data.items.map((x) => ({
        label: `${x.metadata!.name.split(".").at(0)}${
          x.metadata?.displayName ? " – " + x.metadata.displayName : ""
        }`,
        value: x.metadata!.uid,
      }))}
    />
  );
};

const ChooseBuild = (props: {
  template: WsPB.Template;
  onSet: (id: string) => void;
}) => {
  const { template } = props;
  const builds = template.status?.buildInfo?.builds ?? [];
  const readyBuilds = builds.filter(
    (x) => x.state === WsPB.Template_Status_BuildInfo_Build_State.READY,
  );

  if (
    !template.status?.buildInfo?.currentReadyBuildID ||
    readyBuilds.length === 0
  ) {
    return null;
  }

  return (
    <div>
      <Text size="sm" fw={500} mb={6}>
        Build
      </Text>
      <Select
        placeholder="Select build"
        value={template.status.buildInfo.currentReadyBuildID}
        onChange={(val) => val && props.onSet(val)}
        leftSection={<IconDatabase size={14} />}
        data={readyBuilds.map((x) => ({
          label: x.id,
          value: x.id,
        }))}
        renderOption={({ option }) => {
          const build = readyBuilds.find((b) => b.id === option.value);
          if (!build) return <Text size="sm">{option.label}</Text>;
          return (
            <Group gap="xs" wrap="nowrap">
              <Text size="sm" style={{ fontFamily: "monospace" }}>
                {build.id}
              </Text>
              <Group gap={4}>
                {build.tags.map((tag) => (
                  <Badge key={tag} size="xs" variant="light">
                    {tag}
                  </Badge>
                ))}
              </Group>
              <Text size="xs" c="dimmed" ml="auto">
                <TimeAgo rfc3339={build.doneAt} />
              </Text>
            </Group>
          );
        }}
      />
    </div>
  );
};

const StartWorkspace = (props: {
  templateRef?: MetaPB.ObjectReference;
  spaceRef?: MetaPB.ObjectReference;
  disableChooseTemplate?: boolean;
  disableChooseEnvironment?: boolean;
}) => {
  const { templateRef, spaceRef } = props;

  const queryTemplateRef = useQuery({
    queryKey: ["workspace/getTemplate", templateRef?.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        MetaPB.GetOptions.create({ uid: templateRef?.uid }),
      );
      return response;
    },
    enabled: !!templateRef?.uid,
  });

  const queryTemplateDefault = useQuery({
    queryKey: ["workspace/getTemplate", `default.${spaceRef?.name}`],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        MetaPB.GetOptions.create({ name: `default.${spaceRef?.name}` }),
      );
      return response;
    },
    enabled: !templateRef?.uid && !!spaceRef?.name,
  });

  const template =
    (queryTemplateRef.isSuccess && queryTemplateRef.data) ||
    (queryTemplateDefault.isSuccess && queryTemplateDefault.data);

  if (!template) return null;

  return (
    <WrapC
      item={template}
      disableChooseTemplate={props.disableChooseTemplate}
      disableChooseEnvironment={props.disableChooseEnvironment}
    />
  );
};

export default StartWorkspace;
