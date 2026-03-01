import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import {
  UserConfig,
  UserConfig_Spec_Dotfiles,
  UserConfig_Spec_EnvVar,
} from "@/apis/cordiumv1/cordiumv1";
import EditItem from "@/components/EditItem";
import Editor from "@/components/Editor";
import Field from "@/components/Field";
import ItemContainer from "@/components/ItemContainer";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

import Switch from "@/components/Switch";
import { Button, Group, Select } from "@mantine/core";

const Edit = (props: { userConfig: UserConfig }) => {
  const client = getClientWorkspace();
  const queryClient = useQueryClient();

  let [req, setReq] = React.useState(props.userConfig);

  const updateReq = () => {
    const clone = UserConfig.clone(req);
    setReq(clone);
  };

  const navigate = useNavigate();

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.updateUserConfig(req);

      return response;
    },
    onSuccess: (data) => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listSpace", 0],
      });

      queryClient.refetchQueries({
        queryKey: ["workspace/getUserConfig"],
      });

      toast.success("Settings updated");
      navigate("/");
    },
    onError: onError,
  });

  return (
    <div>
      <div>
        <EditItem
          title="Dotfiles"
          description="Set your dotfiles repository"
          onUnset={() => {
            req.spec!.dotfiles = undefined;
            updateReq();
          }}
          obj={req.spec!.dotfiles ? {} : undefined}
          onSet={() => {
            req!.spec!.dotfiles = UserConfig_Spec_Dotfiles.create({});
            updateReq();
          }}
        >
          {req.spec!.dotfiles && (
            <Field
              val={req!.spec!.dotfiles!.url}
              label="Repo URL"
              placeholder="https://github.com/torvalds/dotfiles"
              onChange={(v) => {
                req!.spec!.dotfiles!.url = v as string;

                updateReq();
              }}
            />
          )}
        </EditItem>

        <EditItem
          title="Environment Variables"
          isList
          obj={req.spec!.envVars}
          onSet={() => {
            req.spec!.envVars.push(
              UserConfig_Spec_EnvVar.create({
                key: "",
                type: {
                  oneofKind: "value",
                  value: "",
                },
              }),
            );
            updateReq();
          }}
          onAddListItem={() => {
            req.spec!.envVars.push(
              UserConfig_Spec_EnvVar.create({
                key: "",
                type: {
                  oneofKind: "value",
                  value: "",
                },
              }),
            );
            updateReq();
          }}
          onUnset={() => {
            req.spec!.envVars = [];
            updateReq();
          }}
        >
          {req.spec!.envVars.map((envVar, idxEnvVar, envVarsArray) => (
            <EditItem
              obj={envVarsArray[idxEnvVar]}
              onUnset={() => {
                envVarsArray.splice(idxEnvVar, 1);
                updateReq();
              }}
            >
              <Group grow>
                <Field
                  val={envVar.key}
                  label="Key"
                  placeholder="KEY"
                  isRequired
                  maxRows={7}
                  onChange={(v) => {
                    envVarsArray[idxEnvVar].key = v as string;
                    updateReq();
                  }}
                />

                {envVar.type.oneofKind === `value` && (
                  <Field
                    val={envVar.type.value}
                    isRequired
                    label="Value"
                    placeholder="VALUE"
                    onChange={(v) => {
                      envVarsArray[idxEnvVar].type = {
                        oneofKind: "value",
                        value: v as string,
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
          title="Tasks"
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
          {req.spec!.tasks.map((command, idxCommand, commandsArray) => (
            <EditItem
              obj={commandsArray[idxCommand]}
              onUnset={() => {
                commandsArray.splice(idxCommand, 1);
                updateReq();
              }}
            >
              <Group grow>
                <Field
                  val={commandsArray[idxCommand].name}
                  label="Name"
                  isRequired
                  placeholder="task-1"
                  onChange={(v) => {
                    commandsArray[idxCommand].name = v as string;
                    updateReq();
                  }}
                />

                <Field
                  val={commandsArray[idxCommand].workingDir}
                  label="Working Directory"
                  placeholder="/usr/bin"
                  onChange={(v) => {
                    commandsArray[idxCommand].workingDir = v as string;
                    updateReq();
                  }}
                />

                <Select
                  required
                  label="Command Type"
                  data={[
                    {
                      label: "On Creation (i.e. First Run)",
                      value:
                        WsPB.Workspace_Spec_Runtime_Task_Type[
                          WsPB.Workspace_Spec_Runtime_Task_Type.ON_CREATE
                        ],
                    },
                    {
                      label: "Post Start (i.e. On Every Run)",
                      value:
                        WsPB.Workspace_Spec_Runtime_Task_Type[
                          WsPB.Workspace_Spec_Runtime_Task_Type.POST_START
                        ],
                    },
                    {
                      label: "Pre Stop",
                      value:
                        WsPB.Workspace_Spec_Runtime_Task_Type[
                          WsPB.Workspace_Spec_Runtime_Task_Type.PRE_STOP
                        ],
                    },
                  ]}
                  defaultValue={
                    WsPB.Workspace_Spec_Runtime_Task_Type[
                      commandsArray[idxCommand].type
                    ]
                  }
                  onChange={(val) => {
                    if (!val) {
                      return;
                    }

                    commandsArray[idxCommand].type =
                      WsPB.Workspace_Spec_Runtime_Task_Type[val as "ON_CREATE"];
                    updateReq();
                  }}
                />

                <Switch
                  label="Run in background"
                  val={commandsArray[idxCommand].isBackground}
                  onChange={(v) => {
                    commandsArray[idxCommand].isBackground = v;
                    updateReq();
                  }}
                />

                <Switch
                  label="Run as root"
                  val={commandsArray[idxCommand].runAsRoot}
                  onChange={(v) => {
                    commandsArray[idxCommand].runAsRoot = v;
                    updateReq();
                  }}
                />
              </Group>

              <ItemContainer title="Run Command">
                <Editor
                  mode="shell"
                  value={commandsArray[idxCommand].run}
                  onChange={(v) => {
                    commandsArray[idxCommand].run = v as string;
                    updateReq();
                  }}
                />
              </ItemContainer>
            </EditItem>
          ))}
        </EditItem>
      </div>

      <div className="mt-4 flex items-center justify-end">
        <Button
          size="lg"
          loading={mutation.isPending}
          onClick={() => {
            mutation.mutate();
          }}
        >
          Update
        </Button>
      </div>
    </div>
  );
};

export default () => {
  const client = getClientWorkspace();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { isLoading, isSuccess, data } = useQuery({
    queryKey: ["workspace/getUserConfig"],
    queryFn: () => {
      const { response } = client.getUserConfig({});
      return response;
    },
  });

  if (!isSuccess) {
    return <></>;
  }

  return (
    <div>
      <Edit userConfig={data} />
    </div>
  );
};
