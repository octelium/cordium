import CodeEditor from "@/components/CodeEditor";
import ConfirmAction from "@/components/ConfirmAction";
import Facts, { Fact } from "@/components/Facts";
import OptionalBlock from "@/components/OptionalBlock";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import SecretSelect from "@/components/SecretSelect";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppSelector } from "@/utils/hooks";
import { invalidateSpace, invalidateSpaces } from "@/utils/octelium";
import { getResourceRef, getShortName, isOrgSpace } from "@/utils/pb";
import {
  Alert,
  Button,
  NumberInput,
  SegmentedControl,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { DeleteOptions } from "@octelium/apis/main/metav1";
import {
  IconAlertTriangle,
  IconDoorExit,
  IconLock,
  IconTrash,
} from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";
import { useContextSpace } from "../../utils";

const T = WsPB.Workspace_Spec_Runtime_Task_Type;

const LimitFields = (props: {
  title: string;
  description: string;
  limit?: WsPB.Workspace_Spec_Limit;
  onEnable: () => void;
  onDisable: () => void;
  onChange: (fn: (l: WsPB.Workspace_Spec_Limit) => void) => void;
}) => (
  <OptionalBlock
    title={props.title}
    description={props.description}
    enabled={!!props.limit}
    onEnable={props.onEnable}
    onDisable={props.onDisable}
  >
    {props.limit && (
      <div className="grid gap-4 md:grid-cols-3">
        <NumberInput
          label="CPU"
          description="Millicores. 1000 = 1 vCPU."
          placeholder="2000"
          min={0}
          step={500}
          value={props.limit.cpu?.millicores ?? 0}
          onChange={(v) =>
            props.onChange((l) => {
              l.cpu = WsPB.Workspace_Spec_Limit_CPU.create({
                millicores: typeof v === "number" ? v : Number(v) || 0,
              });
            })
          }
        />
        <NumberInput
          label="Memory"
          description="Megabytes of RAM."
          placeholder="4096"
          min={0}
          step={512}
          value={props.limit.memory?.megabytes ?? 0}
          onChange={(v) =>
            props.onChange((l) => {
              l.memory = WsPB.Workspace_Spec_Limit_Memory.create({
                megabytes: typeof v === "number" ? v : Number(v) || 0,
              });
            })
          }
        />
        <NumberInput
          label="Storage"
          description="Megabytes of persistent disk."
          placeholder="20000"
          min={0}
          step={1000}
          value={props.limit.storage?.megabytes ?? 0}
          onChange={(v) =>
            props.onChange((l) => {
              l.storage = WsPB.Workspace_Spec_Limit_Storage.create({
                megabytes: typeof v === "number" ? v : Number(v) || 0,
              });
            })
          }
        />
      </div>
    )}
  </OptionalBlock>
);

const SettingsForm = (props: { data: WsPB.Space }) => {
  const { data } = props;
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const userUID = useAppSelector(
    (s) => s.settings.status?.user?.metadata?.uid,
  );

  const [req, setReq] = React.useState(() => WsPB.Space.clone(data));

  const patch = (fn: (draft: WsPB.Space) => void) => {
    const next = WsPB.Space.clone(req);
    fn(next);
    setReq(next);
  };

  const mutationUpdate = useMutation({
    mutationFn: async () => {
      const { response } = await client.updateSpace(req);
      return response;
    },
    onSuccess: (response) => {
      invalidateSpace(response);
      toast.success("Space updated");
    },
    onError,
  });

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteSpace(
        DeleteOptions.create({ uid: data.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateSpaces();
      toast.success("Space deleted");
      navigate("/spaces");
    },
    onError,
  });

  const mutationLeave = useMutation({
    mutationFn: async () => {
      await client.leaveSpace(
        WsPB.LeaveSpaceRequest.create({ spaceRef: getResourceRef(data) }),
      );
    },
    onSuccess: () => {
      invalidateSpaces();
      toast.success("You left the Space");
      navigate("/spaces");
    },
    onError,
  });

  const runtime = req.spec?.runtime;
  const org = isOrgSpace(data);
  const isCreator = data.status?.userRef?.uid === userUID;
  const spaceRef = getResourceRef(data);

  return (
    <Stack gap="lg" className="max-w-4xl">
      <Panel>
        <PanelHeader
          title="Shared runtime"
          description="Applied to every Workspace in this Space, on top of what its Template defines."
        />
        <PanelBody>
          <Stack gap="lg">
            <OptionalBlock
              title="Runtime defaults"
              description="Environment variables and lifecycle tasks shared by all Workspaces here."
              enabled={!!runtime}
              onEnable={() =>
                patch((d) => {
                  d.spec!.runtime = WsPB.Space_Spec_Runtime.create();
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.spec!.runtime = undefined;
                })
              }
            >
              {runtime && (
                <Stack gap="lg">
                  <RepeatBlock
                    title="Environment variables"
                    description="Injected into every Workspace in this Space."
                    addLabel="Add variable"
                    emptyHint="No shared environment variables."
                    count={runtime.envVars.length}
                    onAdd={() =>
                      patch((d) => {
                        d.spec!.runtime!.envVars.push(
                          WsPB.Workspace_Spec_Runtime_EnvVar.create({
                            key: "",
                            type: { oneofKind: "value", value: "" },
                          }),
                        );
                      })
                    }
                  >
                    {runtime.envVars.map((envVar, idx) => (
                      <RepeatItem
                        key={idx}
                        index={idx}
                        label={envVar.key}
                        onRemove={() =>
                          patch((d) => {
                            d.spec!.runtime!.envVars.splice(idx, 1);
                          })
                        }
                      >
                        <div className="grid gap-4 md:grid-cols-[1fr_auto_1.4fr] md:items-end">
                          <TextInput
                            label="Key"
                            placeholder="REGISTRY_HOST"
                            required
                            value={envVar.key}
                            onChange={(e) => {
                              const v = e.currentTarget.value;
                              patch((d) => {
                                d.spec!.runtime!.envVars[idx].key = v;
                              });
                            }}
                          />
                          <div>
                            <Text size="sm" fw={500} mb={6}>
                              Source
                            </Text>
                            <SegmentedControl
                              size="xs"
                              value={
                                envVar.type.oneofKind === "fromSecret"
                                  ? "fromSecret"
                                  : "value"
                              }
                              onChange={(v) =>
                                patch((d) => {
                                  d.spec!.runtime!.envVars[idx].type =
                                    v === "fromSecret"
                                      ? {
                                          oneofKind: "fromSecret",
                                          fromSecret: "",
                                        }
                                      : { oneofKind: "value", value: "" };
                                })
                              }
                              data={[
                                { label: "Literal", value: "value" },
                                {
                                  label: "Secret",
                                  value: "fromSecret",
                                  disabled: !org,
                                },
                              ]}
                            />
                          </div>

                          {envVar.type.oneofKind === "value" && (
                            <TextInput
                              label="Value"
                              placeholder="registry.internal"
                              value={envVar.type.value}
                              onChange={(e) => {
                                const v = e.currentTarget.value;
                                patch((d) => {
                                  d.spec!.runtime!.envVars[idx].type = {
                                    oneofKind: "value",
                                    value: v,
                                  };
                                });
                              }}
                            />
                          )}

                          {envVar.type.oneofKind === "fromSecret" && (
                            <SecretSelect
                              spaceRef={spaceRef}
                              required
                              value={envVar.type.fromSecret}
                              onChange={(val) =>
                                patch((d) => {
                                  d.spec!.runtime!.envVars[idx].type = {
                                    oneofKind: "fromSecret",
                                    fromSecret: val,
                                  };
                                })
                              }
                            />
                          )}
                        </div>
                      </RepeatItem>
                    ))}
                  </RepeatBlock>

                  {!org && (
                    <Alert color="gray" variant="light">
                      Secret-backed variables are only available in
                      Organization Spaces.
                    </Alert>
                  )}

                  <RepeatBlock
                    title="Tasks"
                    description="Lifecycle scripts run in every Workspace in this Space."
                    addLabel="Add task"
                    emptyHint="No shared tasks."
                    count={runtime.tasks.length}
                    onAdd={() =>
                      patch((d) => {
                        d.spec!.runtime!.tasks.push(
                          WsPB.Workspace_Spec_Runtime_Task.create({
                            type: T.POST_START,
                          }),
                        );
                      })
                    }
                  >
                    {runtime.tasks.map((task, idx) => (
                      <RepeatItem
                        key={idx}
                        index={idx}
                        label={task.name}
                        onRemove={() =>
                          patch((d) => {
                            d.spec!.runtime!.tasks.splice(idx, 1);
                          })
                        }
                      >
                        <Stack gap="md">
                          <div className="grid gap-4 md:grid-cols-3">
                            <TextInput
                              label="Name"
                              placeholder="mount-cache"
                              required
                              value={task.name}
                              onChange={(e) => {
                                const v = e.currentTarget.value;
                                patch((d) => {
                                  d.spec!.runtime!.tasks[idx].name = v;
                                });
                              }}
                            />
                            <TextInput
                              label="Working directory"
                              placeholder="/workspace"
                              value={task.workingDir}
                              onChange={(e) => {
                                const v = e.currentTarget.value;
                                patch((d) => {
                                  d.spec!.runtime!.tasks[idx].workingDir = v;
                                });
                              }}
                            />
                            <Select
                              label="Trigger"
                              required
                              allowDeselect={false}
                              data={[
                                {
                                  value: T[T.ON_CREATE],
                                  label: "On create — first run only",
                                },
                                {
                                  value: T[T.POST_START],
                                  label: "Post start — every run",
                                },
                                {
                                  value: T[T.PRE_STOP],
                                  label: "Pre stop — before shutdown",
                                },
                              ]}
                              value={T[task.type]}
                              onChange={(val) => {
                                if (!val) return;
                                patch((d) => {
                                  d.spec!.runtime!.tasks[idx].type = T[
                                    val as keyof typeof T
                                  ] as WsPB.Workspace_Spec_Runtime_Task_Type;
                                });
                              }}
                            />
                          </div>

                          <div className="flex flex-wrap gap-6">
                            <Switch
                              size="sm"
                              label="Run in background"
                              checked={task.isBackground}
                              onChange={(e) => {
                                const v = e.currentTarget.checked;
                                patch((d) => {
                                  d.spec!.runtime!.tasks[idx].isBackground = v;
                                });
                              }}
                            />
                            <Switch
                              size="sm"
                              label="Run as root"
                              checked={task.runAsRoot}
                              onChange={(e) => {
                                const v = e.currentTarget.checked;
                                patch((d) => {
                                  d.spec!.runtime!.tasks[idx].runAsRoot = v;
                                });
                              }}
                            />
                          </div>

                          <div>
                            <Text size="sm" fw={500} mb={6}>
                              Script
                            </Text>
                            <CodeEditor
                              mode="shell"
                              value={task.run}
                              minHeight="140px"
                              maxHeight="320px"
                              onChange={(v) =>
                                patch((d) => {
                                  d.spec!.runtime!.tasks[idx].run = v;
                                })
                              }
                            />
                          </div>
                        </Stack>
                      </RepeatItem>
                    ))}
                  </RepeatBlock>
                </Stack>
              )}
            </OptionalBlock>

            <OptionalBlock
              icon={<IconLock size={16} />}
              title="Authorization"
              description="Access rules applied to every Workspace in this Space."
              enabled={!!req.spec?.authorization}
              onEnable={() =>
                patch((d) => {
                  d.spec!.authorization =
                    WsPB.Space_Spec_Authorization.create();
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.spec!.authorization = undefined;
                })
              }
            >
              {req.spec?.authorization && (
                <Switch
                  size="sm"
                  label="Disable SSH"
                  description="Blocks `cordium ssh` and SSH-based tooling for Workspaces in this Space."
                  checked={req.spec.authorization.disableSSH}
                  onChange={(e) => {
                    const v = e.currentTarget.checked;
                    patch((d) => {
                      d.spec!.authorization!.disableSSH = v;
                    });
                  }}
                />
              )}
            </OptionalBlock>

            {org && (
              <OptionalBlock
                title="Resource limits"
                description="Defaults and ceilings for Workspaces created in this Space."
                enabled={!!req.spec?.limit}
                onEnable={() =>
                  patch((d) => {
                    d.spec!.limit = WsPB.Space_Spec_Limit.create();
                  })
                }
                onDisable={() =>
                  patch((d) => {
                    d.spec!.limit = undefined;
                  })
                }
              >
                {req.spec?.limit && (
                  <Stack gap="md">
                    <LimitFields
                      title="Default limits"
                      description="Used when a Workspace does not set its own limits."
                      limit={req.spec.limit.defaultLimit}
                      onEnable={() =>
                        patch((d) => {
                          d.spec!.limit!.defaultLimit =
                            WsPB.Workspace_Spec_Limit.create({
                              cpu: { millicores: 2000 },
                              memory: { megabytes: 4096 },
                              storage: { megabytes: 20000 },
                            });
                        })
                      }
                      onDisable={() =>
                        patch((d) => {
                          d.spec!.limit!.defaultLimit = undefined;
                        })
                      }
                      onChange={(fn) =>
                        patch((d) => fn(d.spec!.limit!.defaultLimit!))
                      }
                    />
                    <LimitFields
                      title="Maximum limits"
                      description="Workspaces requesting more than this are rejected."
                      limit={req.spec.limit.maxLimit}
                      onEnable={() =>
                        patch((d) => {
                          d.spec!.limit!.maxLimit =
                            WsPB.Workspace_Spec_Limit.create({
                              cpu: { millicores: 8000 },
                              memory: { megabytes: 16384 },
                              storage: { megabytes: 50000 },
                            });
                        })
                      }
                      onDisable={() =>
                        patch((d) => {
                          d.spec!.limit!.maxLimit = undefined;
                        })
                      }
                      onChange={(fn) =>
                        patch((d) => fn(d.spec!.limit!.maxLimit!))
                      }
                    />
                  </Stack>
                )}
              </OptionalBlock>
            )}
          </Stack>
        </PanelBody>
        <PanelFooter>
          <Button
            variant="default"
            onClick={() => setReq(WsPB.Space.clone(data))}
          >
            Reset
          </Button>
          <Button
            loading={mutationUpdate.isPending}
            onClick={() => mutationUpdate.mutate()}
          >
            Save changes
          </Button>
        </PanelFooter>
      </Panel>

      <Panel className="border-rose-200">
        <PanelHeader
          icon={<IconAlertTriangle size={16} />}
          title="Danger zone"
          description="These actions cannot be undone."
        />
        <PanelBody>
          <Stack gap="md">
            <Facts>
              <Fact label="Space">
                <span className="font-mono">{data.metadata?.name}</span>
              </Fact>
            </Facts>

            <div className="flex flex-wrap gap-2">
              {!isCreator && (
                <ConfirmAction
                  triggerLabel="Leave Space"
                  triggerIcon={<IconDoorExit size={14} />}
                  color="orange"
                  size="sm"
                  title="Leave this Space?"
                  confirmLabel="Leave Space"
                  description="You will lose access to its Templates, Secrets and Workspaces. An admin can invite you back."
                  loading={mutationLeave.isPending}
                  onConfirm={() => mutationLeave.mutate()}
                />
              )}

              {isCreator && (
                <ConfirmAction
                  triggerLabel="Delete Space"
                  triggerIcon={<IconTrash size={14} />}
                  size="sm"
                  title="Delete this Space?"
                  confirmLabel="Delete Space"
                  description="All Templates, Secrets, Git providers and memberships in this Space are deleted with it."
                  details={
                    <Facts>
                      <Fact label="Name">
                        <span className="font-mono">{getShortName(data)}</span>
                      </Fact>
                      <Fact label="UID">
                        <span className="font-mono text-[0.8em]">
                          {data.metadata?.uid}
                        </span>
                      </Fact>
                    </Facts>
                  }
                  loading={mutationDelete.isPending}
                  onConfirm={() => mutationDelete.mutate()}
                />
              )}
            </div>
          </Stack>
        </PanelBody>
      </Panel>
    </Stack>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const data = ctx.space.data;

  if (!data) return null;

  return (
    <SettingsForm
      key={`${data.metadata!.uid}:${data.metadata?.updatedAt?.seconds ?? 0}`}
      data={data}
    />
  );
};

export default Page;
