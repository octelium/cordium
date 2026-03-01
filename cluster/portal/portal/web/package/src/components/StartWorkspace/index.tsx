import * as MetaPB from "@/apis/metav1/metav1";
import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getResourceRef, getShortName } from "@/utils/pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import { Group, Tabs } from "@mantine/core";

import {
  getPathWorkspace,
  invalidateResource,
  invalidateWorkspace,
} from "@/utils/octelium";

import { Select } from "@mantine/core";

import Field from "../Field";
import ItemContainer from "../ItemContainer";
import Label from "../Label";
import Switch from "../Switch";
import TimeAgo from "../TimeAgo";
import WorkspaceEdit from "../WorkspaceEdit";
import { Button } from "@mantine/core";

const StartWorkspaceTemplate = (props: {
  item: WsPB.Template;
  workspace: WsPB.Workspace;
  onUpdateWorkspace: (item: WsPB.Workspace) => void;
}) => {
  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { item } = props;

  let [doStart, setDoStart] = React.useState(true);

  let [req, setReq] = React.useState(WsPB.Workspace.clone(props.workspace));

  const updateReq = (arg: WsPB.Workspace) => {
    const c = WsPB.Workspace.clone(arg);
    setReq(c);
    props.onUpdateWorkspace(c);
  };

  React.useEffect(() => {
    setReq(WsPB.Workspace.clone(props.workspace));
  }, [props.workspace]);

  const mutationStartWorkspaceDefault = useMutation({
    mutationFn: async () => {
      const { response } = await client.createWorkspace(req);

      const uid = response.metadata!.uid;
      if (doStart) {
        await client.startWorkspace(WsPB.StartWorkspaceRequest.create({ uid }));
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
    <div>
      <div>
        {/**
         <Group grow>
          <Switch
            label="Ephemeral Storage"
            val={(req as WsPB.Workspace).status?.isEphemeral}
            onChange={(v) => {
              (req as WsPB.Workspace).status!.isEphemeral = v;
              updateReq(req);
            }}
          />

          <Switch
            label="Start after Creation"
            val={doStart}
            onChange={(v) => {
              setDoStart(v);
            }}
          />
        </Group>
         **/}

        <Group grow>
          <Field
            val={req.spec!.repository?.url ?? ""}
            label="Repo URL"
            placeholder="https://github.com/torvalds/linux"
            onChange={(v) => {
              if (!req.spec!.repository) {
                req.spec!.repository = WsPB.Workspace_Spec_Repository.create();
              }

              req.spec!.repository!.url = v as string;

              updateReq(req);
            }}
          />

          <Field
            val={
              req.spec!.image?.type.oneofKind === `registry` &&
              req.spec!.image?.type.registry.url
                ? req.spec!.image?.type.registry.url
                : ""
            }
            label="Workspace Docker Image"
            placeholder="ubuntu:jammy"
            onChange={(v) => {
              if (!req.spec!.image) {
                req.spec!.image = WsPB.Workspace_Spec_Image.create();
              }

              if (req.spec!.image.type.oneofKind !== `registry`) {
                req.spec!.image.type = {
                  oneofKind: "registry",
                  registry: WsPB.Workspace_Spec_Image_Registry.create(),
                };
              }

              req.spec!.image.type.registry.url = v as string;

              updateReq(req);
            }}
          />
        </Group>
      </div>
      <div>
        <ChooseBuild
          template={item}
          onSet={(i) => {
            updateReq(req);
          }}
        />
      </div>
    </div>
  );
};

const WrapC = (props: {
  item: WsPB.Template;
  disableChooseTemplate?: boolean;
  disableChooseEnvironment?: boolean;
}) => {
  let [template, setTemplate] = React.useState(WsPB.Template.clone(props.item));
  const client = getClientWorkspace();
  const navigate = useNavigate();
  let [doStart, setDoStart] = React.useState(true);
  let [req, setReq] = React.useState(
    WsPB.Workspace.create({
      apiVersion: "workspace/v1",
      kind: "Workspace",
      metadata: {},
      spec: {},
      status: {
        templateRef: getResourceRef(template),
      },
    }),
  );

  const mutationStartWorkspaceDefault = useMutation({
    mutationFn: async () => {
      const { response } = await client.createWorkspace(req);

      const uid = response.metadata!.uid;
      if (doStart) {
        await client.startWorkspace(WsPB.StartWorkspaceRequest.create({ uid }));
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
    <div className="w-full p-4 mt-4 mb-4 rounded-xl shadow-md border-[1px] border-gray-300">
      <div className="w-full font-bold text-xl flex items-center justify-center mb-4 text-shadow-2xs">
        Create and Start a Workspace
      </div>

      <Group className="mb-4" grow>
        {!props.disableChooseTemplate && (
          <div>
            <ChooseTemplate
              cur={template}
              spaceRef={template.status!.spaceRef!}
              onSet={(i) => {
                setTemplate(WsPB.Template.clone(i));
                req.status!.templateRef = getResourceRef(i);
                setReq(WsPB.Workspace.clone(req));
              }}
            />
          </div>
        )}

        <Switch
          label="Ephemeral Storage"
          description="Ephemeral Workspaces Storage is reset on every run"
          val={(req as WsPB.Workspace).status?.isEphemeral}
          onChange={(v) => {
            (req as WsPB.Workspace).status!.isEphemeral = v;
            setReq(WsPB.Workspace.clone(req));
          }}
        />

        <Switch
          label="Start after Creation"
          description="Immediately start Workspace after creation"
          val={doStart}
          onChange={(v) => {
            setDoStart(v);
          }}
        />
      </Group>

      <div>
        <Tabs defaultValue="main" className="font-bold text-xl">
          <Tabs.List className="mb-2">
            <Tabs.Tab value="main" onClick={() => {}}>
              Quick Mode
            </Tabs.Tab>
            <Tabs.Tab value="custom" onClick={() => {}}>
              Customize
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="main" className="mt-2">
            <StartWorkspaceTemplate
              item={template}
              workspace={req}
              onUpdateWorkspace={(i) => {
                setReq(WsPB.Workspace.clone(i));
              }}
            />
          </Tabs.Panel>
          <Tabs.Panel value="custom" className="mt-2">
            <WorkspaceEdit
              spaceRef={template!.status!.spaceRef!}
              item={req}
              onUpdate={(item) => {
                let v = item as WsPB.Workspace;
                let reqClone = WsPB.Workspace.clone(req);
                reqClone.spec = v.spec;
                setReq(reqClone);
              }}
            />
          </Tabs.Panel>
        </Tabs>

        <div className="w-full flex items-center justify-end mt-8">
          <Button
            size="lg"
            loading={mutationStartWorkspaceDefault.isPending}
            onClick={() => {
              mutationStartWorkspaceDefault.mutate(undefined);
            }}
          >
            Submit
          </Button>
        </div>
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
  let [page, setPage] = React.useState(0);

  const qry = useQuery({
    queryKey: ["workspace/listTemplate", spaceRef.uid, page],
    queryFn: () => {
      const { response } = client.listTemplate(
        WsPB.ListTemplateOptions.create({
          spaceRef,
        }),
      );
      return response;
    },
  });

  if (!qry.isSuccess) {
    return <></>;
  }

  return (
    <div>
      <Select
        label="Choose Template"
        value={
          props.cur ? props.cur.metadata!.uid : qry.data.items[0].metadata!.uid
        }
        onChange={(val) => {
          const tmpl = qry.data.items.find((x) => x.metadata!.uid === val)!;
          props.onSet(tmpl);
        }}
        data={qry.data.items.map((x) => ({
          label: `${x.metadata!.name.split(".").at(0)}${
            x.metadata?.displayName ? "(" + x.metadata?.displayName + ")" : ""
          }`,
          value: x.metadata!.uid,
        }))}
      />
    </div>
  );
};

const ChooseBuild = (props: {
  template: WsPB.Template;
  onSet: (id: string) => void;
}) => {
  const { template } = props;

  if (
    !template.status ||
    !template.status.buildInfo ||
    !template.status.buildInfo.builds ||
    template.status.buildInfo.builds.length < 1 ||
    template.status.buildInfo.currentReadyBuildID.length < 1
  ) {
    return <></>;
  }

  return (
    <div>
      <Select
        value={template.status.buildInfo.currentReadyBuildID}
        onChange={(val) => {
          if (!val) {
            return;
          }
          props.onSet(val);
        }}
      >
        {template.status.buildInfo.builds
          .filter(
            (x) => x.state === WsPB.Template_Status_BuildInfo_Build_State.READY,
          )
          .map((x) => (
            <div key={x.id}>
              {`${x.id}`}{" "}
              {x.tags.map((tag) => (
                <Label>{tag}</Label>
              ))}{" "}
              <TimeAgo rfc3339={x.doneAt} />
            </div>
          ))}
      </Select>
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
        MetaPB.GetOptions.create({
          uid: templateRef?.uid,
        }),
      );
      return response;
    },
    enabled: !!templateRef?.uid,
  });

  const queryTemplateDefault = useQuery({
    queryKey: ["workspace/getTemplate", `default.default.${spaceRef?.name}`],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        MetaPB.GetOptions.create({
          name: `default.${spaceRef?.name}`,
        }),
      );
      return response;
    },
    enabled: !templateRef?.uid || !!spaceRef?.name,
  });

  if (queryTemplateRef.isSuccess) {
    return (
      <WrapC
        item={queryTemplateRef.data}
        disableChooseTemplate={props.disableChooseTemplate}
        disableChooseEnvironment={props.disableChooseEnvironment}
      />
    );
  }

  if (queryTemplateDefault.isSuccess) {
    return (
      <WrapC
        item={queryTemplateDefault.data}
        disableChooseTemplate={props.disableChooseTemplate}
        disableChooseEnvironment={props.disableChooseEnvironment}
      />
    );
  }

  return <div></div>;
};

export default StartWorkspace;
