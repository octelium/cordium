import * as React from "react";

import { getClientWorkspace } from "@/utils/client";
import { cloneResource, getResourceRef } from "@/utils/pb";
import {
  GitProvider,
  GitProviderList,
  Membership_Spec_Role,
} from "@octelium/apis/main/cordiumv1";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import EditItem from "@/components/EditItem";
import Editor from "@/components/Editor";
import Field from "@/components/Field";
import ItemContainer from "@/components/ItemContainer";
import Meta from "@/components/Meta";
import PageWrap from "@/components/PageWrap";
import Switch from "@/components/Switch";
import { onError } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateSpace } from "@/utils/octelium";
import { Button, Group, Select } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import axios from "axios";
import { toast } from "react-hot-toast";
import { twMerge } from "tailwind-merge";
import { useContextSpace } from "../../utils";

interface AuthBegin {
  loginURL: string;
}

const LoginGitProvider = (props: { item: GitProvider }) => {
  const { item } = props;

  const mutation = useMutation({
    mutationFn: async () => {
      const resp = await axios.post<AuthBegin>(
        `/auth/v1/begin/${item.metadata!.uid}`,
      );
      return resp.data;
    },
    onSuccess: (data) => {
      window.location.href = data.loginURL;
    },
    onError: onError,
  });

  return (
    <div>
      <button className={twMerge("w-full flex items-center justify-center")}>
        {item.metadata!.name}{" "}
        {item.metadata?.displayName && `(${item.metadata.displayName})`}
      </button>
    </div>
  );
};

const LoginGitProviderList = (props: { itemList: GitProviderList }) => {
  const { itemList } = props;

  return (
    <div className="flex flex-col items-center justify-center">
      {itemList.items.map((x) => (
        <LoginGitProvider key={x.metadata!.uid} item={x} />
      ))}
    </div>
  );
};

export const SpaceEdit = (props: { item: WsPB.Space }) => {
  const client = getClientWorkspace();
  const data = props.item;

  const settings = useAppSelector((state) => state.settings);
  const navigate = useNavigate();
  let [req, setReq] = React.useState(WsPB.Space.clone(data));

  const mutationUpdate = useMutation({
    mutationFn: async (req: WsPB.Space) => {
      const { response } = await client.updateSpace(req);
      return response;
    },

    onSuccess: (response) => {
      invalidateSpace(response);

      navigate(getPathSpace(response));
      toast.success("Space updated");
    },
    onError,
  });

  const qryMem = useQuery({
    queryKey: ["workspace/getSpaceMembership", data?.metadata?.uid],
    queryFn: () => {
      const { response } = client.getSpaceMembership(
        WsPB.GetSpaceMembershipRequest.create({
          spaceRef: getResourceRef(data),
        }),
      );
      return response;
    },
    enabled: !!data?.metadata?.uid,
  });

  const isAdmin =
    qryMem.isSuccess &&
    (qryMem.data.spec!.role == Membership_Spec_Role.OWNER ||
      qryMem.data.spec!.role == Membership_Spec_Role.ADMIN);

  const isOwner =
    qryMem.isSuccess && qryMem.data.spec!.role == Membership_Spec_Role.OWNER;

  const updateReq = () => {
    const clone = cloneResource(req) as WsPB.Space;

    setReq(clone);
  };

  const isPersonal = data.status?.type === WsPB.Space_Status_Type.USER;
  const isOrg = data.status?.type === WsPB.Space_Status_Type.ORGANIZATION;

  return (
    <>
      <Meta title={`Space Config`} />
      <div>
        <div>
          <EditItem
            title="Runtime"
            description="Set startup scripts, env vars, etc..."
            obj={req.spec!.runtime ? {} : undefined}
            onSet={() => {
              req.spec!.runtime = WsPB.Workspace_Spec_Runtime.create();
              updateReq();
            }}
            onUnset={() => {
              req.spec!.runtime = undefined;
              updateReq();
            }}
          >
            {req.spec!.runtime && (
              <>
                <EditItem
                  title="Environment Variables"
                  isList
                  obj={req.spec!.runtime.envVars}
                  onSet={() => {
                    req.spec!.runtime!.envVars.push(
                      WsPB.Workspace_Spec_Runtime_EnvVar.create({
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
                    req.spec!.runtime!.envVars.push(
                      WsPB.Workspace_Spec_Runtime_EnvVar.create({
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
                    req.spec!.runtime!.envVars = [];
                    updateReq();
                  }}
                >
                  {req.spec!.runtime.envVars.map(
                    (envVar, idxEnvVar, envVarsArray) => (
                      <EditItem
                        obj={envVarsArray[idxEnvVar]}
                        onUnset={() => {
                          envVarsArray.splice(idxEnvVar, 1);
                          updateReq();
                        }}
                      >
                        <div className="flex flex-row">
                          <div className="w-full basis-1/3 mr-1">
                            <Field
                              val={envVar.key}
                              label="Key"
                              isRequired
                              placeholder="KEY"
                              multiLine
                              maxRows={7}
                              onChange={(v) => {
                                envVarsArray[idxEnvVar].key = v as string;
                                updateReq();
                              }}
                            />
                          </div>

                          {envVar.type.oneofKind === `value` && (
                            <div className="w-full basis-2/3 ml-1">
                              <Field
                                val={envVar.type.value}
                                label="Value"
                                isRequired
                                placeholder="MY VALUE"
                                onChange={(v) => {
                                  envVarsArray[idxEnvVar].type = {
                                    oneofKind: "value",
                                    value: v as string,
                                  };
                                  updateReq();
                                }}
                              />
                            </div>
                          )}
                        </div>
                      </EditItem>
                    ),
                  )}
                </EditItem>

                <EditItem
                  title="Tasks"
                  isList
                  obj={req.spec!.runtime.tasks}
                  onSet={() => {
                    req.spec!.runtime!.tasks.push(
                      WsPB.Workspace_Spec_Runtime_Task.create(),
                    );
                    updateReq();
                  }}
                  onAddListItem={() => {
                    req.spec!.runtime!.tasks.push(
                      WsPB.Workspace_Spec_Runtime_Task.create(),
                    );
                    updateReq();
                  }}
                  onUnset={() => {
                    req.spec!.runtime!.tasks = [];
                    updateReq();
                  }}
                >
                  {req.spec!.runtime!.tasks.map(
                    (command, idxCommand, commandsArray) => (
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
                              commandsArray[idxCommand].workingDir =
                                v as string;
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
                                    WsPB.Workspace_Spec_Runtime_Task_Type
                                      .ON_CREATE
                                  ],
                              },
                              {
                                label: "Post Start (i.e. On Every Run)",
                                value:
                                  WsPB.Workspace_Spec_Runtime_Task_Type[
                                    WsPB.Workspace_Spec_Runtime_Task_Type
                                      .POST_START
                                  ],
                              },
                              {
                                label: "Pre Stop",
                                value:
                                  WsPB.Workspace_Spec_Runtime_Task_Type[
                                    WsPB.Workspace_Spec_Runtime_Task_Type
                                      .PRE_STOP
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
                                WsPB.Workspace_Spec_Runtime_Task_Type[
                                  val as "ON_CREATE"
                                ];
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
                    ),
                  )}
                </EditItem>
              </>
            )}
          </EditItem>
        </div>

        <EditItem
          title="Authorization"
          description="Authorization-specific configuration"
          obj={req.spec!.authorization ? {} : undefined}
          onSet={() => {
            req.spec!.authorization = WsPB.Space_Spec_Authorization.create();
            updateReq();
          }}
          onUnset={() => {
            req.spec!.authorization = undefined;
            updateReq();
          }}
        >
          {req.spec!.authorization && (
            <>
              <Switch
                label="Disable SSH"
                description="Disable SSH access to Workspaces of this Space"
                val={req.spec!.authorization!.disableSSH}
                onChange={(v) => {
                  req.spec!.authorization!.disableSSH = v;
                  updateReq();
                }}
              />
            </>
          )}
        </EditItem>

        {isOrg && (
          <EditItem
            title="Limit"
            description="Set Workspace resource limits"
            onUnset={() => {
              req.spec!.limit = undefined;
              updateReq();
            }}
            obj={req.spec!.limit ? {} : undefined}
            onSet={() => {
              req.spec!.limit = WsPB.Space_Spec_Limit.create();
              updateReq();
            }}
          >
            {req.spec!.limit && (
              <>
                <EditItem
                  title="Default Limit"
                  description="Set Workspace resource default limits"
                  onUnset={() => {
                    req.spec!.limit!.defaultLimit = undefined;
                    updateReq();
                  }}
                  obj={req.spec!.limit.defaultLimit ? {} : undefined}
                  onSet={() => {
                    req.spec!.limit!.defaultLimit =
                      WsPB.Workspace_Spec_Limit.create({
                        cpu: {
                          millicores: 0,
                        },
                        memory: {
                          megabytes: 0,
                        },
                        storage: {
                          megabytes: 0,
                        },
                      });
                    updateReq();
                  }}
                >
                  {req.spec!.limit!.defaultLimit && (
                    <Group grow>
                      {req.spec!.limit!.defaultLimit.cpu && (
                        <Field
                          val={req.spec!.limit!.defaultLimit!.cpu!.millicores}
                          label="CPU Millicores"
                          description="Set the CPU limit in Millicores"
                          placeholder="2000"
                          isNumber
                          onChange={(v) => {
                            req.spec!.limit!.defaultLimit!.cpu!.millicores =
                              v as number;
                            updateReq();
                          }}
                        />
                      )}

                      {req.spec!.limit!.defaultLimit!.memory && (
                        <Field
                          isNumber
                          val={req.spec!.limit!.defaultLimit!.memory!.megabytes}
                          label="Memory"
                          description="Set RAM limit in Megabytes"
                          placeholder="6000"
                          onChange={(v) => {
                            req.spec!.limit!.defaultLimit!.memory!.megabytes =
                              v as number;
                            updateReq();
                          }}
                        />
                      )}

                      {req.spec!.limit!.defaultLimit!.storage && (
                        <Field
                          isNumber
                          val={
                            req.spec!.limit!.defaultLimit!.storage!.megabytes
                          }
                          label="Storage"
                          description="Set disk limit in Megabytes"
                          placeholder="10000"
                          onChange={(v) => {
                            req.spec!.limit!.defaultLimit!.storage!.megabytes =
                              v as number;
                            updateReq();
                          }}
                        />
                      )}
                    </Group>
                  )}
                </EditItem>

                <EditItem
                  title="Max Limit"
                  description="Set Workspace resource Max limits"
                  onUnset={() => {
                    req.spec!.limit!.maxLimit = undefined;
                    updateReq();
                  }}
                  obj={req.spec!.limit.maxLimit ? {} : undefined}
                  onSet={() => {
                    req.spec!.limit!.maxLimit =
                      WsPB.Workspace_Spec_Limit.create({
                        cpu: {
                          millicores: 0,
                        },
                        memory: {
                          megabytes: 0,
                        },
                        storage: {
                          megabytes: 0,
                        },
                      });
                    updateReq();
                  }}
                >
                  {req.spec!.limit!.maxLimit && (
                    <Group grow>
                      {req.spec!.limit!.maxLimit.cpu && (
                        <Field
                          val={req.spec!.limit!.maxLimit!.cpu!.millicores}
                          label="CPU Millicores"
                          description="Set the CPU limit in Millicores"
                          placeholder="2000"
                          isNumber
                          onChange={(v) => {
                            req.spec!.limit!.maxLimit!.cpu!.millicores =
                              v as number;
                            updateReq();
                          }}
                        />
                      )}

                      {req.spec!.limit!.maxLimit!.memory && (
                        <Field
                          isNumber
                          val={req.spec!.limit!.maxLimit!.memory!.megabytes}
                          label="Memory"
                          description="Set RAM limit in Megabytes"
                          placeholder="6000"
                          onChange={(v) => {
                            req.spec!.limit!.maxLimit!.memory!.megabytes =
                              v as number;
                            updateReq();
                          }}
                        />
                      )}

                      {req.spec!.limit!.maxLimit!.storage && (
                        <Field
                          isNumber
                          val={req.spec!.limit!.maxLimit!.storage!.megabytes}
                          label="Storage"
                          description="Set disk limit in Megabytes"
                          placeholder="10000"
                          onChange={(v) => {
                            req.spec!.limit!.maxLimit!.storage!.megabytes =
                              v as number;
                            updateReq();
                          }}
                        />
                      )}
                    </Group>
                  )}
                </EditItem>
              </>
            )}
          </EditItem>
        )}

        <div>
          <div className="flex flex-row justify-end items-center">
            <Button
              variant="outline"
              className="mr-2"
              onClick={() => {
                navigate(-1);
              }}
            >
              Cancel
            </Button>

            <Button
              onClick={() => {
                mutationUpdate.mutate(req);
              }}
            >
              Update
            </Button>
          </div>
        </div>
      </div>
    </>
  );
};

const Page = () => {
  const ctx = useContextSpace();

  return (
    <PageWrap qry={ctx.space}>
      {ctx.space.data && <SpaceEdit item={ctx.space.data} />}
    </PageWrap>
  );
};

export default Page;
