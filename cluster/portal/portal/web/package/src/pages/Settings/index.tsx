import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import {
  UserConfig,
  UserConfig_Spec_Dotfiles,
  UserConfig_Spec_EnvVar,
} from "@/apis/cordiumv1/cordiumv1";
import EditItem from "@/components/EditItem";
import Editor from "@/components/Editor";
import ItemContainer from "@/components/ItemContainer";
import Meta from "@/components/Meta";
import Switch from "@/components/Switch";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import {
  Button,
  Divider,
  Group,
  Select,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const Edit = (props: { userConfig: UserConfig }) => {
  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const [req, setReq] = React.useState(UserConfig.clone(props.userConfig));

  const updateReq = () => setReq(UserConfig.clone(req));

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.updateUserConfig(req);
      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({ queryKey: ["workspace/listSpace", 0] });
      queryClient.refetchQueries({ queryKey: ["workspace/getUserConfig"] });
      toast.success("Settings updated");
      navigate("/");
    },
    onError,
  });

  return (
    <div
      style={{
        background: "white",
        border: "1px solid #e2e8f0",
        borderRadius: 10,
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: "14px 20px",
          borderBottom: "1px solid #e2e8f0",
          background: "#f8fafc",
        }}
      >
        <Text
          size="xs"
          fw={600}
          tt="uppercase"
          style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
        >
          User settings
        </Text>
      </div>

      <div style={{ padding: "16px 20px" }}>
        <Stack gap={0}>
          <EditItem
            title="Dotfiles"
            description="Clone and apply your dotfiles on workspace start"
            onUnset={() => {
              req.spec!.dotfiles = undefined;
              updateReq();
            }}
            obj={req.spec!.dotfiles ? {} : undefined}
            onSet={() => {
              req.spec!.dotfiles = UserConfig_Spec_Dotfiles.create({});
              updateReq();
            }}
          >
            {req.spec!.dotfiles && (
              <TextInput
                label="Repository URL"
                placeholder="https://github.com/you/dotfiles"
                value={req.spec!.dotfiles.url}
                onChange={(e) => {
                  req.spec!.dotfiles!.url = e.currentTarget.value;
                  updateReq();
                }}
              />
            )}
          </EditItem>

          <EditItem
            title="Environment variables"
            description="Injected into every workspace at start"
            isList
            obj={req.spec!.envVars}
            onSet={() => {
              req.spec!.envVars.push(
                UserConfig_Spec_EnvVar.create({
                  key: "",
                  type: { oneofKind: "value", value: "" },
                }),
              );
              updateReq();
            }}
            onAddListItem={() => {
              req.spec!.envVars.push(
                UserConfig_Spec_EnvVar.create({
                  key: "",
                  type: { oneofKind: "value", value: "" },
                }),
              );
              updateReq();
            }}
            onUnset={() => {
              req.spec!.envVars = [];
              updateReq();
            }}
          >
            {req.spec!.envVars.map((envVar, idx, arr) => (
              <EditItem
                key={idx}
                obj={arr[idx]}
                onUnset={() => {
                  arr.splice(idx, 1);
                  updateReq();
                }}
              >
                <Group grow align="flex-start">
                  <TextInput
                    label="Key"
                    placeholder="MY_VAR"
                    description="Set the environment variable key"
                    required
                    value={envVar.key}
                    onChange={(e) => {
                      arr[idx].key = e.currentTarget.value;
                      updateReq();
                    }}
                  />
                  {envVar.type.oneofKind === "value" && (
                    <TextInput
                      label="Value"
                      placeholder="value"
                      description="Set the environment variable value"
                      required
                      value={envVar.type.value}
                      onChange={(e) => {
                        arr[idx].type = {
                          oneofKind: "value",
                          value: e.currentTarget.value,
                        };
                        updateReq();
                      }}
                    />
                  )}
                </Group>
              </EditItem>
            ))}
          </EditItem>

          <EditItem
            title="Startup tasks"
            description="Scripts executed at workspace lifecycle events"
            isList
            obj={req.spec!.tasks}
            onSet={() => {
              req.spec!.tasks.push(WsPB.Workspace_Spec_Runtime_Task.create());
              updateReq();
            }}
            onAddListItem={() => {
              req.spec!.tasks.push(WsPB.Workspace_Spec_Runtime_Task.create());
              updateReq();
            }}
            onUnset={() => {
              req.spec!.tasks = [];
              updateReq();
            }}
          >
            {req.spec!.tasks.map((task, idx, arr) => (
              <EditItem
                key={idx}
                obj={arr[idx]}
                onUnset={() => {
                  arr.splice(idx, 1);
                  updateReq();
                }}
              >
                <Stack gap="md">
                  <Group grow align="flex-start">
                    <TextInput
                      label="Name"
                      placeholder="task-1"
                      required
                      value={arr[idx].name}
                      onChange={(e) => {
                        arr[idx].name = e.currentTarget.value;
                        updateReq();
                      }}
                    />
                    <TextInput
                      label="Working directory"
                      placeholder="/workspace"
                      value={arr[idx].workingDir}
                      onChange={(e) => {
                        arr[idx].workingDir = e.currentTarget.value;
                        updateReq();
                      }}
                    />
                    <Select
                      label="Trigger"
                      required
                      data={[
                        {
                          label: "On creation (first run)",
                          value:
                            WsPB.Workspace_Spec_Runtime_Task_Type[
                              WsPB.Workspace_Spec_Runtime_Task_Type.ON_CREATE
                            ],
                        },
                        {
                          label: "Post start (every run)",
                          value:
                            WsPB.Workspace_Spec_Runtime_Task_Type[
                              WsPB.Workspace_Spec_Runtime_Task_Type.POST_START
                            ],
                        },
                        {
                          label: "Pre stop",
                          value:
                            WsPB.Workspace_Spec_Runtime_Task_Type[
                              WsPB.Workspace_Spec_Runtime_Task_Type.PRE_STOP
                            ],
                        },
                      ]}
                      defaultValue={
                        WsPB.Workspace_Spec_Runtime_Task_Type[arr[idx].type]
                      }
                      onChange={(val) => {
                        if (!val) return;
                        arr[idx].type =
                          WsPB.Workspace_Spec_Runtime_Task_Type[
                            val as "ON_CREATE"
                          ];
                        updateReq();
                      }}
                    />
                  </Group>

                  <Group gap="xl">
                    <Switch
                      label="Run in background"
                      val={arr[idx].isBackground}
                      onChange={(v) => {
                        arr[idx].isBackground = v;
                        updateReq();
                      }}
                    />
                    <Switch
                      label="Run as root"
                      val={arr[idx].runAsRoot}
                      onChange={(v) => {
                        arr[idx].runAsRoot = v;
                        updateReq();
                      }}
                    />
                  </Group>

                  <ItemContainer title="Run command">
                    <Editor
                      mode="shell"
                      value={arr[idx].run}
                      onChange={(v) => {
                        arr[idx].run = v as string;
                        updateReq();
                      }}
                    />
                  </ItemContainer>
                </Stack>
              </EditItem>
            ))}
          </EditItem>

          <Divider mt="lg" mb="md" />

          <Group justify="flex-end">
            <Button
              size="sm"
              loading={mutation.isPending}
              onClick={() => mutation.mutate()}
            >
              Save settings
            </Button>
          </Group>
        </Stack>
      </div>
    </div>
  );
};

export default () => {
  const client = getClientWorkspace();

  const { isSuccess, data } = useQuery({
    queryKey: ["workspace/getUserConfig"],
    queryFn: () => {
      const { response } = client.getUserConfig({});
      return response;
    },
  });

  if (!isSuccess) return null;

  return (
    <div>
      <Meta title="Settings" />
      <Edit userConfig={data} />
    </div>
  );
};
