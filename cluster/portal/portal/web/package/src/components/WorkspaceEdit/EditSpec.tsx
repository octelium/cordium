import * as React from "react";
import * as WsPB from "../../apis/cordiumv1/cordiumv1";
import Field from "../Field";
import ItemContainer from "../ItemContainer";

import { GetOptions, ObjectReference } from "@/apis/metav1/metav1";
import { getClientWorkspace } from "@/utils/client";
import { Group, Select } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { cloneResource } from "../../utils/pb";
import Divider from "../Divider";
import EditItem from "../EditItem";
import Editor from "../Editor";
import Switch from "../Switch";

const EditSpec = (props: {
  item: WsPB.Workspace | WsPB.Template;
  onUpdate: (item: WsPB.Workspace | WsPB.Template) => void;
  spaceRef: ObjectReference;
}) => {
  let [req, setReq] = React.useState(props.item);

  const client = getClientWorkspace();

  const updateReq = () => {
    const clone = cloneResource(req) as
      | WsPB.Workspace
      | WsPB.Template
      | WsPB.Template;
    setReq(clone);

    console.log("CLONED", clone, req);
    props.onUpdate(clone);
  };

  const isWorkspace = (): boolean => {
    return props.item.kind === `Workspace`;
  };

  const isTemplate = (): boolean => {
    return props.item.kind === `Template`;
  };

  const qrySpace = useQuery({
    queryKey: ["workspace/getSpace", props.spaceRef.uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getSpace(
        GetOptions.create({ uid: props.spaceRef.uid }),
      );
      return response;
    },
  });

  let qrySecret = useQuery({
    queryKey: ["workspace/listSecret/"],
    queryFn: () => {
      const { response } = client.listSecret(
        WsPB.ListSecretOptions.create({
          spaceRef: props.spaceRef,
          common: {
            itemsPerPage: 500,
          },
        }),
      );
      return response;
    },
  });

  /*
  let qryUserSecret = useQuery({
    queryKey: ["workspace/listUserSecret/"],
    queryFn: () => {
      const { response } = client.listUserSecret(
        WsPB.ListUserSecretOptions.create({
          itemsPerPage: 500,
        })
      );
      return response;
    },
  });
  */

  let qryGitProvider = useQuery({
    queryKey: ["workspace/listGitProvider/"],
    queryFn: () => {
      const { response } = client.listGitProvider(
        WsPB.ListGitProviderOptions.create({
          spaceRef: props.spaceRef,
          common: {
            itemsPerPage: 500,
          },
        }),
      );
      return response;
    },
    enabled: qrySpace.isSuccess && isTemplate(),
  });

  return (
    <div>
      {/*
      {isWorkspace() && (
        <ItemContainer title="Ephemeral Storage" isHorizontal>
          <Switch
            val={(req as WsPB.Workspace).status?.isEphemeral}
            onChange={(v) => {
              (req as WsPB.Workspace).status!.isEphemeral = v;
              updateReq();
            }}
          />
        </ItemContainer>
      )}
      
      */}

      <EditItem
        title="Git Repository"
        description="Set the git repo URL of your environment"
        onUnset={() => {
          req.spec!.repository = undefined;
          updateReq();
        }}
        obj={req.spec!.repository ? {} : undefined}
        onSet={() => {
          req.spec!.repository = WsPB.Workspace_Spec_Repository.create();
          updateReq();
        }}
      >
        {req.spec!.repository && (
          <>
            <Field
              val={req.spec!.repository!.url}
              label="Repo URL"
              placeholder="https://github.com/torvalds/linux"
              isRequired
              description="Set the git repository URL"
              onChange={(v) => {
                req.spec!.repository!.url = v as string;

                updateReq();
              }}
            />

            <EditItem
              title="Clone Options"
              description="Set the clone options for the Repository"
              obj={req.spec!.repository!.cloneOptions ? {} : undefined}
              onSet={() => {
                req.spec!.repository!.cloneOptions =
                  WsPB.Workspace_Spec_Repository_CloneOptions.create({});

                updateReq();
              }}
              onUnset={() => {
                req.spec!.repository!.cloneOptions = undefined;
                updateReq();
              }}
            >
              {req.spec!.repository!.cloneOptions && (
                <Group grow>
                  <Field
                    val={req.spec!.repository!.cloneOptions!.branch}
                    label="Branch"
                    placeholder="dev"
                    description="Set the repo branch"
                    onChange={(v) => {
                      req.spec!.repository!.cloneOptions!.branch = v as string;

                      updateReq();
                    }}
                  />

                  <Field
                    val={req.spec!.repository!.cloneOptions!.depth}
                    label="Depth"
                    description="Create a shallow clone with n commits"
                    placeholder="4"
                    isNumber
                    onChange={(v) => {
                      req.spec!.repository!.cloneOptions!.depth = v as number;

                      updateReq();
                    }}
                  />

                  <Field
                    val={req.spec!.repository!.cloneOptions!.checkout}
                    label="Checkout"
                    placeholder="dev"
                    description="Set the clone checkout"
                    onChange={(v) => {
                      req.spec!.repository!.cloneOptions!.checkout =
                        v as string;

                      updateReq();
                    }}
                  />

                  <Switch
                    label="Single Branch"
                    description="Clone only the history leading to the tip of a single branch"
                    val={req.spec!.repository!.cloneOptions!.singleBranch}
                    onChange={(v) => {
                      req.spec!.repository!.cloneOptions!.singleBranch = v;
                      updateReq();
                    }}
                  />

                  <Switch
                    label="Shallow Submodules"
                    description="All submodules which are cloned will be shallow with a depth of 1"
                    val={req.spec!.repository!.cloneOptions!.shallowSubmodules}
                    onChange={(v) => {
                      req.spec!.repository!.cloneOptions!.shallowSubmodules = v;
                      updateReq();
                    }}
                  />
                </Group>
              )}
            </EditItem>

            {qrySecret.isSuccess &&
              qrySecret.data.listResponseMeta &&
              qrySecret.data.listResponseMeta?.totalCount > 0 && (
                <EditItem
                  title="HTTP Authentication"
                  description="Use username/password to authenticate to the private repo"
                  obj={req.spec!.repository!.authentication ? {} : undefined}
                  onSet={() => {
                    {
                      req.spec!.repository!.authentication =
                        WsPB.Workspace_Spec_Repository_Authentication.create({
                          type: {
                            oneofKind: "http",
                            http: {
                              password: {
                                type: {
                                  oneofKind: "fromSecret",
                                  fromSecret: "",
                                },
                              },
                            } as WsPB.Workspace_Spec_Repository_Authentication_HTTP,
                          },
                        });
                    }

                    updateReq();
                  }}
                  onUnset={() => {
                    req.spec!.repository!.authentication = undefined;
                    updateReq();
                  }}
                >
                  {req.spec!.repository!.authentication &&
                    req.spec!.repository!.authentication.type.oneofKind ===
                      "http" &&
                    req.spec!.repository!.authentication.type.http && (
                      <Group grow>
                        <Field
                          val={
                            req.spec!.repository!.authentication!.type.http!
                              .username
                          }
                          label="Auth username"
                          isRequired
                          placeholder={`user-1234`}
                          onChange={(v) => {
                            let f = req.spec!.repository!.authentication!
                              .type as {
                              oneofKind: "http";
                              http: WsPB.Workspace_Spec_Repository_Authentication_HTTP;
                            };
                            f.http.username = v as string;
                            updateReq();
                          }}
                        />

                        {qrySpace.isSuccess &&
                          qrySecret.isSuccess &&
                          req.spec!.repository!.authentication!.type.http!
                            .password &&
                          req.spec!.repository!.authentication!.type.http!
                            .password!.type.oneofKind === `fromSecret` && (
                            <Select
                              label="Password Secret"
                              required
                              data={qrySecret.data!.items.map((x) => ({
                                label: x.metadata!.name.split(".").at(0) ?? "",
                                value: x.metadata!.name,
                              }))}
                              defaultValue={
                                req.spec!.repository!.authentication!.type.http!
                                  .password!.type!.fromSecret ?? ""
                              }
                              onChange={(val) => {
                                if (!val) {
                                  return;
                                }
                                let f = req.spec!.repository!.authentication!
                                  .type as {
                                  oneofKind: "http";
                                  http: WsPB.Workspace_Spec_Repository_Authentication_HTTP;
                                };
                                let g = f.http.password!.type as {
                                  oneofKind: "fromSecret";
                                  fromSecret: string;
                                };
                                g.fromSecret = val;
                                updateReq();
                              }}
                            />
                          )}
                      </Group>
                    )}
                </EditItem>
              )}
          </>
        )}
      </EditItem>

      <EditItem
        title="Docker Image"
        description="Set a Docker image for the Workspace (e.g. Ubuntu)"
        obj={req.spec!.image ? {} : undefined}
        onSet={() => {
          req.spec!.image = WsPB.Workspace_Spec_Image.create();
          updateReq();
        }}
        onUnset={() => {
          req.spec!.image = undefined;
          updateReq();
        }}
      >
        {req.spec!.image && (
          <>
            <EditItem
              title="Registry"
              description="Use an image from a container registry"
              obj={
                req.spec!.image.type.oneofKind === `registry` ? {} : undefined
              }
              onSet={() => {
                req.spec!.image!.type = {
                  oneofKind: "registry",
                  registry: WsPB.Workspace_Spec_Image_Registry.create(),
                };
                updateReq();
              }}
              onUnset={() => {
                req.spec!.image = undefined;
                updateReq();
              }}
            >
              {req.spec!.image!.type.oneofKind === "registry" && (
                <div>
                  <Field
                    val={req.spec!.image!.type.registry.url}
                    label="Workspace Container Image URL"
                    placeholder={`e.g. "ubuntu:jammy", "mcr.microsoft.com/devcontainers/universal:linux"`}
                    onChange={(v) => {
                      let f = req.spec!.image!.type as {
                        oneofKind: "registry";
                        registry: WsPB.Workspace_Spec_Image_Registry;
                      };

                      f.registry.url = v as string;
                      updateReq();
                    }}
                  />

                  {qrySecret.isSuccess &&
                    qrySecret.data.listResponseMeta &&
                    qrySecret.data.listResponseMeta?.totalCount > 0 && (
                      <EditItem
                        title="Authentication"
                        description="Container registry authentication info"
                        obj={
                          req.spec!.image!.type.registry.authentication
                            ? {}
                            : undefined
                        }
                        onSet={() => {
                          {
                            let f = req.spec!.image!.type as {
                              oneofKind: "registry";
                              registry: WsPB.Workspace_Spec_Image_Registry;
                            };
                            f.registry.authentication =
                              WsPB.Workspace_Spec_Image_Registry_Authentication.create(
                                {
                                  password: {
                                    type: {
                                      oneofKind: "fromSecret",
                                      fromSecret: "",
                                    },
                                  },
                                },
                              );
                          }

                          updateReq();
                        }}
                        onUnset={() => {
                          let f = req.spec!.image!.type as {
                            oneofKind: "registry";
                            registry: WsPB.Workspace_Spec_Image_Registry;
                          };
                          f.registry.authentication = undefined;
                          updateReq();
                        }}
                      >
                        {req.spec!.image!.type.registry.authentication && (
                          <Group grow>
                            <Field
                              val={
                                req.spec!.image!.type.registry.authentication!
                                  .username
                              }
                              label={`Auth Username`}
                              placeholder="user-1234"
                              onChange={(v) => {
                                let f = req.spec!.image!.type as {
                                  oneofKind: "registry";
                                  registry: WsPB.Workspace_Spec_Image_Registry;
                                };
                                f.registry.authentication!.username =
                                  v as string;
                                updateReq();
                              }}
                            />

                            {qrySpace.isSuccess &&
                              qrySecret.isSuccess &&
                              req.spec!.image!.type.registry.authentication
                                .password &&
                              req.spec!.image!.type.registry.authentication
                                .password!.type.oneofKind === `fromSecret` && (
                                <Select
                                  label="Password Secret"
                                  data={qrySecret.data!.items.map((x) => ({
                                    label:
                                      x.metadata!.name.split(".").at(0) ?? "",
                                    value: x.metadata!.name,
                                  }))}
                                  defaultValue={
                                    req.spec!.image!.type.registry
                                      .authentication.password!.type!
                                      .fromSecret ?? ""
                                  }
                                  onChange={(val) => {
                                    if (!val) {
                                      return;
                                    }
                                    let f = req.spec!.image!.type as {
                                      oneofKind: "registry";
                                      registry: WsPB.Workspace_Spec_Image_Registry;
                                    };
                                    let g = f.registry.authentication!.password!
                                      .type as {
                                      oneofKind: "fromSecret";
                                      fromSecret: string;
                                    };
                                    g.fromSecret = val;
                                    updateReq();
                                  }}
                                />
                              )}
                          </Group>
                        )}
                      </EditItem>
                    )}
                </div>
              )}
            </EditItem>
            <Divider>OR</Divider>
            <EditItem
              title="Dockerfile"
              description="Build an image from a Dockerfile"
              obj={
                req.spec!.image.type.oneofKind === `dockerfile` ? {} : undefined
              }
              onSet={() => {
                req.spec!.image!.type = {
                  oneofKind: "dockerfile",
                  dockerfile: WsPB.Workspace_Spec_Image_Dockerfile.create(),
                };
                updateReq();
              }}
              onUnset={() => {
                req.spec!.image = undefined;
                updateReq();
              }}
            >
              {req.spec!.image!.type.oneofKind === "dockerfile" && (
                <div>
                  <EditItem
                    title="Inline"
                    description="Write your own Dockerfile"
                    obj={
                      req.spec!.image.type.dockerfile.type.oneofKind ===
                      `inline`
                        ? {}
                        : undefined
                    }
                    onSet={() => {
                      let f = req.spec!.image!.type as {
                        oneofKind: "dockerfile";
                        dockerfile: WsPB.Workspace_Spec_Image_Dockerfile;
                      };
                      f.dockerfile.type = {
                        oneofKind: "inline",
                        inline: "",
                      };

                      updateReq();
                    }}
                    onUnset={() => {
                      req.spec!.image = undefined;
                      updateReq();
                    }}
                  >
                    {req.spec!.image!.type.dockerfile.type.oneofKind ===
                      `inline` && (
                      <>
                        <Editor
                          value={req.spec!.image!.type.dockerfile.type.inline}
                          mode="dockerfile"
                          onChange={(v) => {
                            let a = req.spec!.image!.type as {
                              oneofKind: "dockerfile";
                              dockerfile: WsPB.Workspace_Spec_Image_Dockerfile;
                            };
                            let f = a.dockerfile.type as {
                              oneofKind: "inline";
                              inline: string;
                            };

                            f.inline = v as string;

                            updateReq();
                          }}
                        />
                      </>
                    )}
                  </EditItem>
                  <Divider>OR</Divider>
                  <EditItem
                    title="URL"
                    description="Get the Dockerfile from a URL"
                    obj={
                      req.spec!.image.type.dockerfile.type.oneofKind === `url`
                        ? {}
                        : undefined
                    }
                    onSet={() => {
                      let f = req.spec!.image!.type as {
                        oneofKind: "dockerfile";
                        dockerfile: WsPB.Workspace_Spec_Image_Dockerfile;
                      };
                      f.dockerfile.type = {
                        oneofKind: "url",
                        url: "",
                      };

                      updateReq();
                    }}
                    onUnset={() => {
                      req.spec!.image = undefined;
                      updateReq();
                    }}
                  >
                    {req.spec!.image!.type.dockerfile.type.oneofKind ===
                      `url` && (
                      <>
                        <Field
                          label="Dockerfile URL"
                          placeholder="https://raw.githubusercontent.com/alpinelinux/docker-alpine/master/Dockerfile"
                          val={req.spec!.image!.type.dockerfile.type.url}
                          onChange={(v) => {
                            let a = req.spec!.image!.type as {
                              oneofKind: "dockerfile";
                              dockerfile: WsPB.Workspace_Spec_Image_Dockerfile;
                            };
                            let f = a.dockerfile.type as {
                              oneofKind: "url";
                              url: string;
                            };

                            f.url = v as string;

                            updateReq();
                          }}
                        />
                      </>
                    )}
                  </EditItem>
                </div>
              )}
            </EditItem>
          </>
        )}
      </EditItem>

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
                          placeholder="KEY-1"
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

            <Group grow>
              <Field
                val={req.spec!.runtime!.cmd}
                label="Override Container Command"
                placeholder="/bin/script-init"
                multiLine
                maxRows={7}
                onChange={(v) => {
                  req.spec!.runtime!.cmd = v as string;
                  updateReq();
                }}
              />

              <Field
                val={req.spec!.runtime!.entrypoint}
                label="Override Container Entrypoint"
                placeholder="/bin/init"
                multiLine
                maxRows={7}
                onChange={(v) => {
                  req.spec!.runtime!.entrypoint = v as string;
                  updateReq();
                }}
              />

              <Switch
                label="Disable Init Process"
                description="Disable the default init process"
                val={req.spec!.runtime.disableInit}
                onChange={(v) => {
                  req.spec!.runtime!.disableInit = v;
                  updateReq();
                }}
              />
            </Group>

            <EditItem
              title="Devcontainers"
              description="Set devcontainers (Development Containers) options"
              obj={req.spec!.runtime.devcontainers}
              onSet={() => {
                req.spec!.runtime!.devcontainers =
                  WsPB.Workspace_Spec_Runtime_Devcontainers.create();
                updateReq();
              }}
              onUnset={() => {
                req.spec!.runtime!.devcontainers = undefined;
                updateReq();
              }}
            >
              {req.spec!.runtime!.devcontainers && (
                <>
                  <EditItem
                    title="Features"
                    description="Install and run devcontainers features"
                    isList
                    obj={req.spec!.runtime!.devcontainers!.features}
                    onSet={() => {
                      req.spec!.runtime!.devcontainers!.features.push(
                        WsPB.Workspace_Spec_Runtime_Devcontainers_Feature.create(
                          {},
                        ),
                      );
                      updateReq();
                    }}
                    onAddListItem={() => {
                      req.spec!.runtime!.devcontainers!.features.push(
                        WsPB.Workspace_Spec_Runtime_Devcontainers_Feature.create(
                          {},
                        ),
                      );
                      updateReq();
                    }}
                    onUnset={() => {
                      req.spec!.runtime!.devcontainers!.features = [];
                      updateReq();
                    }}
                  >
                    {req.spec!.runtime!.devcontainers!.features.map(
                      (ftr, idxFtr, ftrArray) => (
                        <EditItem
                          obj={ftrArray[idxFtr]}
                          onUnset={() => {
                            ftrArray.splice(idxFtr, 1);
                            updateReq();
                          }}
                        >
                          <Field
                            val={ftrArray[idxFtr].reference}
                            label="Reference"
                            placeholder="ghcr.io/devcontainers/features/aws-cli:1"
                            onChange={(v) => {
                              ftrArray[idxFtr].reference = v as string;
                              updateReq();
                            }}
                          />

                          {ftrArray[idxFtr].options && (
                            <>
                              <EditItem
                                title="Options"
                                isList
                                obj={ftrArray[idxFtr].options}
                                onSet={() => {
                                  ftrArray[idxFtr].options.push(
                                    WsPB.Workspace_Spec_Runtime_Devcontainers_Feature_Option.create(),
                                  );
                                  updateReq();
                                }}
                                onAddListItem={() => {
                                  ftrArray[idxFtr].options.push(
                                    WsPB.Workspace_Spec_Runtime_Devcontainers_Feature_Option.create(),
                                  );
                                  updateReq();
                                }}
                                onUnset={() => {
                                  ftrArray[idxFtr].options = [];
                                  updateReq();
                                }}
                              >
                                {ftrArray[idxFtr].options.map(
                                  (option, idxOption, optionArray) => (
                                    <EditItem
                                      obj={optionArray[idxOption]}
                                      onUnset={() => {
                                        optionArray.splice(idxOption, 1);
                                        updateReq();
                                      }}
                                    >
                                      <Group grow>
                                        <Field
                                          val={optionArray[idxOption].key}
                                          label="Key"
                                          placeholder="version"
                                          onChange={(v) => {
                                            optionArray[idxOption].key =
                                              v as string;
                                            updateReq();
                                          }}
                                        />

                                        <Field
                                          val={optionArray[idxOption].value}
                                          label="Value"
                                          placeholder="latest"
                                          onChange={(v) => {
                                            optionArray[idxOption].value =
                                              v as string;
                                            updateReq();
                                          }}
                                        />
                                      </Group>
                                    </EditItem>
                                  ),
                                )}
                              </EditItem>
                            </>
                          )}
                        </EditItem>
                      ),
                    )}
                  </EditItem>
                </>
              )}
            </EditItem>
          </>
        )}
      </EditItem>

      {isWorkspace() && (
        <EditItem
          title="Applications"
          description="Expose and share the ports of Workspace services"
          isList
          obj={(req as WsPB.Workspace).spec!.applications}
          onSet={() => {
            (req as WsPB.Workspace).spec!.applications.push(
              WsPB.Workspace_Spec_Application.create({}),
            );
            updateReq();
          }}
          onAddListItem={() => {
            (req as WsPB.Workspace).spec!.applications.push(
              WsPB.Workspace_Spec_Application.create({}),
            );
            updateReq();
          }}
          onUnset={() => {
            (req as WsPB.Workspace).spec!.applications = [];
            updateReq();
          }}
        >
          {(req as WsPB.Workspace).spec!.applications.map(
            (application, idxApplications, applicationsArr) => (
              <EditItem
                obj={applicationsArr[idxApplications]}
                onUnset={() => {
                  applicationsArr.splice(idxApplications, 1);
                  updateReq();
                }}
              >
                <Group grow>
                  <Field
                    val={applicationsArr[idxApplications].name}
                    label="Name"
                    placeholder="my-app"
                    isRequired
                    onChange={(v) => {
                      applicationsArr[idxApplications].name = v as string;
                      updateReq();
                    }}
                  />

                  <Field
                    val={applicationsArr[idxApplications].displayName}
                    label="Display Name"
                    placeholder="My App"
                    onChange={(v) => {
                      applicationsArr[idxApplications].displayName =
                        v as string;
                      updateReq();
                    }}
                  />

                  <Field
                    val={applicationsArr[idxApplications].port}
                    label="Port"
                    isNumber
                    onChange={(v) => {
                      applicationsArr[idxApplications].port = v as number;

                      updateReq();
                    }}
                  />

                  <Switch
                    label="Default Application"
                    val={applicationsArr[idxApplications].isDefault}
                    onChange={(v) => {
                      applicationsArr[idxApplications].isDefault = v;
                      updateReq();
                    }}
                  />
                </Group>

                <div className="flex flex-row items-center">
                  <div className="w-full"></div>
                </div>
              </EditItem>
            ),
          )}
        </EditItem>
      )}
      <EditItem
        title="Additional Repositories"
        description="Add additional git repos"
        isList
        obj={req.spec!.additionalRepositories}
        onSet={() => {
          req.spec!.additionalRepositories.push(
            WsPB.Workspace_Spec_AdditionalRepository.create({}),
          );
          updateReq();
        }}
        onAddListItem={() => {
          req.spec!.additionalRepositories.push(
            WsPB.Workspace_Spec_AdditionalRepository.create({}),
          );
          updateReq();
        }}
        onUnset={() => {
          req.spec!.additionalRepositories = [];
          updateReq();
        }}
      >
        {req.spec!.additionalRepositories.map((repo, idxRepo, repoArr) => (
          <EditItem
            obj={repo}
            onUnset={() => {
              repoArr.splice(idxRepo, 1);
              updateReq();
            }}
          >
            <Group grow>
              <Field
                val={repo.name}
                label="Name"
                placeholder="linux-repo"
                isRequired
                onChange={(v) => {
                  repo.name = v as string;
                  updateReq();
                }}
              />

              <Field
                val={repo.clonePath}
                label="Clone Path"
                placeholder="/home/ubuntu/custom/directory"
                onChange={(v) => {
                  repo.clonePath = v as string;
                  updateReq();
                }}
              />
            </Group>

            <EditItem
              title="Git Repository"
              description="Set the git repo URL of your environment"
              onUnset={() => {
                repo.repository = undefined;
                updateReq();
              }}
              obj={repo.repository ? {} : undefined}
              onSet={() => {
                repo.repository = WsPB.Workspace_Spec_Repository.create({});
                updateReq();
              }}
            >
              {repo && repo.repository && (
                <>
                  <Field
                    val={repo.repository!.url}
                    label="Repo URL"
                    placeholder="https://github.com/torvalds/linux"
                    isRequired
                    onChange={(v) => {
                      repo.repository!.url = v as string;

                      updateReq();
                    }}
                  />

                  <EditItem
                    title="Clone Options"
                    description="Set the clone options for the Repository"
                    obj={repo.repository!.cloneOptions ? {} : undefined}
                    onSet={() => {
                      repoArr[idxRepo].repository!.cloneOptions =
                        WsPB.Workspace_Spec_Repository_CloneOptions.create({});

                      updateReq();
                    }}
                    onUnset={() => {
                      repoArr[idxRepo].repository!.cloneOptions = undefined;
                      updateReq();
                    }}
                  >
                    {repo &&
                      repo.repository &&
                      repo.repository.cloneOptions && (
                        <Group grow>
                          <Field
                            val={repo.repository!.cloneOptions!.branch}
                            label="Branch"
                            placeholder="dev"
                            onChange={(v) => {
                              repoArr[
                                idxRepo
                              ].repository!.cloneOptions!.branch = v as string;

                              updateReq();
                            }}
                          />

                          <Field
                            val={repo.repository!.cloneOptions!.depth}
                            label="Depth"
                            placeholder="dev"
                            isNumber
                            onChange={(v) => {
                              repoArr[idxRepo].repository!.cloneOptions!.depth =
                                v as number;

                              updateReq();
                            }}
                          />

                          <Field
                            val={repo.repository!.cloneOptions!.checkout}
                            label="Checkout"
                            placeholder="dev"
                            onChange={(v) => {
                              repoArr[
                                idxRepo
                              ].repository!.cloneOptions!.checkout =
                                v as string;

                              updateReq();
                            }}
                          />

                          <Switch
                            label="Single Branch"
                            val={repo.repository!.cloneOptions!.singleBranch}
                            onChange={(v) => {
                              repoArr[
                                idxRepo
                              ].repository!.cloneOptions!.singleBranch = v;
                              updateReq();
                            }}
                          />

                          <Switch
                            label="Shallow Submodules"
                            val={
                              repo.repository!.cloneOptions!.shallowSubmodules
                            }
                            onChange={(v) => {
                              repoArr[
                                idxRepo
                              ].repository!.cloneOptions!.shallowSubmodules = v;
                              updateReq();
                            }}
                          />
                        </Group>
                      )}
                  </EditItem>

                  <EditItem
                    title="HTTP Authentication"
                    description="Use username/password to authenticate to the private repo"
                    obj={repo.repository!.authentication ? {} : undefined}
                    onSet={() => {
                      {
                        repoArr[idxRepo].repository!.authentication =
                          WsPB.Workspace_Spec_Repository_Authentication.create({
                            type: {
                              oneofKind: "http",
                              http: {
                                password: {
                                  type: {
                                    oneofKind: "fromSecret",
                                    fromSecret: "",
                                  },
                                },
                              } as WsPB.Workspace_Spec_Repository_Authentication_HTTP,
                            },
                          });
                      }

                      updateReq();
                    }}
                    onUnset={() => {
                      repoArr[idxRepo].repository!.authentication = undefined;
                      updateReq();
                    }}
                  >
                    {repo.repository!.authentication &&
                      repo.repository!.authentication!.type.oneofKind ===
                        "http" &&
                      repo.repository!.authentication!.type.http && (
                        <Group grow>
                          <Field
                            val={
                              repo.repository!.authentication!.type.http!
                                .username
                            }
                            label={`Username`}
                            placeholder="user-1234"
                            description="The username of the basic authentication"
                            onChange={(v) => {
                              let f = repo.repository!.authentication!.type as {
                                oneofKind: "http";
                                http: WsPB.Workspace_Spec_Repository_Authentication_HTTP;
                              };
                              f.http.username = v as string;
                              updateReq();
                            }}
                          />

                          {qrySpace.isSuccess &&
                            qrySecret.isSuccess &&
                            repo.repository!.authentication!.type.http!
                              .password &&
                            repo.repository!.authentication!.type.http!
                              .password!.type.oneofKind === `fromSecret` && (
                              <Select
                                label="Password Secret"
                                data={qrySecret.data!.items.map((x) => ({
                                  label:
                                    x.metadata!.name.split(".").at(0) ?? "",
                                  value: x.metadata!.name,
                                }))}
                                defaultValue={
                                  repo.repository!.authentication!.type.http!
                                    .password!.type!.fromSecret ?? ""
                                }
                                onChange={(val) => {
                                  if (!val) {
                                    return;
                                  }
                                  let f = repo.repository!.authentication!
                                    .type as {
                                    oneofKind: "http";
                                    http: WsPB.Workspace_Spec_Repository_Authentication_HTTP;
                                  };
                                  let g = f.http.password!.type as {
                                    oneofKind: "fromSecret";
                                    fromSecret: string;
                                  };
                                  g.fromSecret = val;
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
          </EditItem>
        ))}
      </EditItem>

      {isTemplate() &&
        qryGitProvider.isSuccess &&
        qryGitProvider.data.listResponseMeta &&
        qryGitProvider.data.listResponseMeta?.totalCount > 0 && (
          <Select
            label="Git Provider"
            data={qryGitProvider.data!.items.map((x) => x.metadata!.name)}
            defaultValue={(req as WsPB.Template).spec!.gitProvider}
            onChange={(val) => {
              if (!val) {
                return;
              }
              (req as WsPB.Template).spec!.gitProvider = val;
              updateReq();
            }}
          />
        )}

      {isTemplate() && (
        <EditItem
          title="Limit"
          description="Set Workspace resource limits"
          onUnset={() => {
            (req as WsPB.Template).spec!.limit = undefined;
            updateReq();
          }}
          obj={(req as WsPB.Template).spec!.limit ? {} : undefined}
          onSet={() => {
            (req as WsPB.Template).spec!.limit =
              WsPB.Workspace_Spec_Limit.create();
            updateReq();
          }}
        >
          {(req as WsPB.Template).spec!.limit && (
            <>
              <EditItem
                title="CPU"
                description="Set CPU limits in millicores"
                obj={(req as WsPB.Template).spec!.limit!.cpu ? {} : undefined}
                onSet={() => {
                  (req as WsPB.Template).spec!.limit!.cpu =
                    WsPB.Workspace_Spec_Limit_CPU.create();
                  updateReq();
                }}
                onUnset={() => {
                  (req as WsPB.Template).spec!.limit!.cpu = undefined;
                  updateReq();
                }}
              >
                {(req as WsPB.Template).spec!.limit!.cpu && (
                  <Field
                    val={(req as WsPB.Template).spec!.limit!.cpu!.millicores}
                    label="Millicores"
                    isNumber
                    onChange={(v) => {
                      (req as WsPB.Template).spec!.limit!.cpu!.millicores =
                        v as number;
                      updateReq();
                    }}
                  />
                )}
              </EditItem>

              <EditItem
                title="Memory"
                description="Set RAM limits in Megabytes"
                obj={
                  (req as WsPB.Template).spec!.limit!.memory ? {} : undefined
                }
                onSet={() => {
                  (req as WsPB.Template).spec!.limit!.memory =
                    WsPB.Workspace_Spec_Limit_Memory.create();
                  updateReq();
                }}
                onUnset={() => {
                  (req as WsPB.Template).spec!.limit!.memory = undefined;
                  updateReq();
                }}
              >
                {(req as WsPB.Template).spec!.limit!.memory && (
                  <Field
                    val={(req as WsPB.Template).spec!.limit!.memory!.megabytes}
                    label="Megabytes"
                    isNumber
                    onChange={(v) => {
                      (req as WsPB.Template).spec!.limit!.memory!.megabytes =
                        v as number;
                      updateReq();
                    }}
                  />
                )}
              </EditItem>

              <EditItem
                title="Storage"
                description="Set Storage limits in Megabytes"
                obj={
                  (req as WsPB.Template).spec!.limit!.storage ? {} : undefined
                }
                onSet={() => {
                  (req as WsPB.Template).spec!.limit!.storage =
                    WsPB.Workspace_Spec_Limit_Storage.create();
                  updateReq();
                }}
                onUnset={() => {
                  (req as WsPB.Template).spec!.limit!.storage = undefined;
                  updateReq();
                }}
              >
                {(req as WsPB.Template).spec!.limit!.storage && (
                  <Field
                    val={(req as WsPB.Template).spec!.limit!.storage!.megabytes}
                    label="Megabytes"
                    isNumber
                    onChange={(v) => {
                      (req as WsPB.Template).spec!.limit!.storage!.megabytes =
                        v as number;
                      updateReq();
                    }}
                  />
                )}
              </EditItem>
            </>
          )}
        </EditItem>
      )}
    </div>
  );
};

export default EditSpec;
