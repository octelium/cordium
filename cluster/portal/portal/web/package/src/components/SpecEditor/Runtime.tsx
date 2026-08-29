import CodeEditor from "@/components/CodeEditor";
import OptionalBlock from "@/components/OptionalBlock";
import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import SecretSelect from "@/components/SecretSelect";
import {
  SegmentedControl,
  Select,
  Stack,
  Switch,
  TagsInput,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconPlugConnected, IconShieldLock } from "@tabler/icons-react";
import { SectionProps } from "./types";

const T = WsPB.Workspace_Spec_Runtime_Task_Type;
const OnFailure = WsPB.Workspace_Spec_Runtime_Task_OnFailure;

const taskTypeData = [
  { value: T[T.ON_CREATE], label: "On create — first run only" },
  { value: T[T.POST_START], label: "Post start — every run" },
  { value: T[T.PRE_STOP], label: "Pre stop — before shutdown" },
];

const onFailureData = [
  { value: OnFailure[OnFailure.UNSET], label: "Default" },
  { value: OnFailure[OnFailure.ABORT], label: "Abort startup" },
  { value: OnFailure[OnFailure.CONTINUE], label: "Continue anyway" },
];

const RuntimeSection = (props: SectionProps) => {
  const { spec, patch } = props;
  const runtime = spec.runtime;

  return (
    <Stack gap="lg">
      <OptionalBlock
        title="Runtime configuration"
        description="Environment variables, startup tasks, container overrides and sandbox settings."
        enabled={!!runtime}
        onEnable={() =>
          patch((d) => {
            d.runtime = WsPB.Workspace_Spec_Runtime.create();
          })
        }
        onDisable={() =>
          patch((d) => {
            d.runtime = undefined;
          })
        }
      >
        {runtime && (
          <Stack gap="lg">
            <RepeatBlock
              title="Environment variables"
              description="Injected into every process in the Workspace. Up to 128."
              addLabel="Add variable"
              emptyHint="No environment variables defined."
              count={runtime.envVars.length}
              onAdd={() =>
                patch((d) => {
                  d.runtime!.envVars.push(
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
                      d.runtime!.envVars.splice(idx, 1);
                    })
                  }
                >
                  <div className="grid gap-4 md:grid-cols-[1fr_auto_1.4fr] md:items-end">
                    <TextInput
                      label="Key"
                      description="Variable name."
                      placeholder="DATABASE_URL"
                      required
                      value={envVar.key}
                      onChange={(e) => {
                        const v = e.currentTarget.value;
                        patch((d) => {
                          d.runtime!.envVars[idx].key = v;
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
                            d.runtime!.envVars[idx].type =
                              v === "fromSecret"
                                ? { oneofKind: "fromSecret", fromSecret: "" }
                                : { oneofKind: "value", value: "" };
                          })
                        }
                        data={[
                          { label: "Literal", value: "value" },
                          { label: "Secret", value: "fromSecret" },
                        ]}
                      />
                    </div>

                    {envVar.type.oneofKind === "value" && (
                      <TextInput
                        label="Value"
                        description="Stored in plain text in the spec."
                        placeholder="postgres://localhost:5432/app"
                        value={envVar.type.value}
                        onChange={(e) => {
                          const v = e.currentTarget.value;
                          patch((d) => {
                            d.runtime!.envVars[idx].type = {
                              oneofKind: "value",
                              value: v,
                            };
                          });
                        }}
                      />
                    )}

                    {envVar.type.oneofKind === "fromSecret" && (
                      <SecretSelect
                        spaceRef={props.spaceRef}
                        required
                        label="Secret"
                        description="Resolved at start; the value never enters the spec."
                        value={envVar.type.fromSecret}
                        onChange={(val) =>
                          patch((d) => {
                            d.runtime!.envVars[idx].type = {
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

            <RepeatBlock
              title="Tasks"
              description="Shell scripts run at lifecycle events. Up to 128."
              addLabel="Add task"
              emptyHint="No tasks defined."
              count={runtime.tasks.length}
              onAdd={() =>
                patch((d) => {
                  d.runtime!.tasks.push(
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
                      d.runtime!.tasks.splice(idx, 1);
                    })
                  }
                >
                  <Stack gap="md">
                    <div className="grid gap-4 md:grid-cols-2">
                      <TextInput
                        label="Name"
                        description="Unique name for this task."
                        placeholder="install-deps"
                        required
                        value={task.name}
                        onChange={(e) => {
                          const v = e.currentTarget.value;
                          patch((d) => {
                            d.runtime!.tasks[idx].name = v;
                          });
                        }}
                      />
                      <TextInput
                        label="Working directory"
                        description="Directory the script runs in."
                        placeholder="/workspace/repo"
                        value={task.workingDir}
                        onChange={(e) => {
                          const v = e.currentTarget.value;
                          patch((d) => {
                            d.runtime!.tasks[idx].workingDir = v;
                          });
                        }}
                      />
                      <Select
                        label="Trigger"
                        description="When this task runs."
                        required
                        allowDeselect={false}
                        data={taskTypeData}
                        value={T[task.type]}
                        onChange={(val) => {
                          if (!val) return;
                          patch((d) => {
                            d.runtime!.tasks[idx].type = T[
                              val as keyof typeof T
                            ] as WsPB.Workspace_Spec_Runtime_Task_Type;
                          });
                        }}
                      />
                      <Select
                        label="On failure"
                        description="What happens if the script exits non-zero."
                        allowDeselect={false}
                        data={onFailureData}
                        value={OnFailure[task.onFailure]}
                        onChange={(val) => {
                          if (!val) return;
                          patch((d) => {
                            d.runtime!.tasks[idx].onFailure = OnFailure[
                              val as keyof typeof OnFailure
                            ] as WsPB.Workspace_Spec_Runtime_Task_OnFailure;
                          });
                        }}
                      />
                    </div>

                    <div className="flex flex-wrap gap-6">
                      <Switch
                        size="sm"
                        label="Run in background"
                        description="Keep it running as a service instead of waiting for it."
                        checked={task.isBackground}
                        onChange={(e) => {
                          const v = e.currentTarget.checked;
                          patch((d) => {
                            d.runtime!.tasks[idx].isBackground = v;
                          });
                        }}
                      />
                      <Switch
                        size="sm"
                        label="Run as root"
                        description="Execute as root instead of the Workspace user."
                        checked={task.runAsRoot}
                        onChange={(e) => {
                          const v = e.currentTarget.checked;
                          patch((d) => {
                            d.runtime!.tasks[idx].runAsRoot = v;
                          });
                        }}
                      />
                    </div>

                    <div>
                      <Text size="sm" fw={500} mb={2}>
                        Script
                      </Text>
                      <Text size="xs" c="dimmed" mb={8}>
                        Runs with the Workspace shell. Maximum 5000 characters.
                      </Text>
                      <CodeEditor
                        mode="shell"
                        value={task.run}
                        minHeight="140px"
                        maxHeight="320px"
                        onChange={(v) =>
                          patch((d) => {
                            d.runtime!.tasks[idx].run = v;
                          })
                        }
                      />
                    </div>
                  </Stack>
                </RepeatItem>
              ))}
            </RepeatBlock>

            <div className="grid gap-4 md:grid-cols-2">
              <Textarea
                label="Command"
                description="Overrides the container CMD. Leave empty to keep the image default."
                placeholder="/bin/bash -lc 'sleep infinity'"
                autosize
                minRows={2}
                maxRows={6}
                value={runtime.cmd}
                onChange={(e) => {
                  const v = e.currentTarget.value;
                  patch((d) => {
                    d.runtime!.cmd = v;
                  });
                }}
              />
              <Textarea
                label="Entrypoint"
                description="Overrides the container ENTRYPOINT."
                placeholder="/bin/init"
                autosize
                minRows={2}
                maxRows={6}
                value={runtime.entrypoint}
                onChange={(e) => {
                  const v = e.currentTarget.value;
                  patch((d) => {
                    d.runtime!.entrypoint = v;
                  });
                }}
              />
            </div>

            <div className="flex flex-wrap gap-6">
              <Switch
                size="sm"
                label="Disable init process"
                description="Skip the default PID 1 init supervisor."
                checked={runtime.disableInit}
                onChange={(e) => {
                  const v = e.currentTarget.checked;
                  patch((d) => {
                    d.runtime!.disableInit = v;
                  });
                }}
              />
              <Switch
                size="sm"
                label="Auto stop"
                description="Stop the Workspace once all POST_START tasks finish. Useful for CI-style runs."
                checked={runtime.autoStop}
                onChange={(e) => {
                  const v = e.currentTarget.checked;
                  patch((d) => {
                    d.runtime!.autoStop = v;
                  });
                }}
              />
            </div>

            <OptionalBlock
              title="Devcontainer features"
              description="Install Development Container features on top of the base image."
              enabled={!!runtime.devcontainers}
              onEnable={() =>
                patch((d) => {
                  d.runtime!.devcontainers =
                    WsPB.Workspace_Spec_Runtime_Devcontainers.create();
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.runtime!.devcontainers = undefined;
                })
              }
            >
              {runtime.devcontainers && (
                <RepeatBlock
                  title="Features"
                  description="OCI references to devcontainer features. Up to 100."
                  addLabel="Add feature"
                  emptyHint="No features selected."
                  count={runtime.devcontainers.features.length}
                  onAdd={() =>
                    patch((d) => {
                      d.runtime!.devcontainers!.features.push(
                        WsPB.Workspace_Spec_Runtime_Devcontainers_Feature.create(),
                      );
                    })
                  }
                >
                  {runtime.devcontainers.features.map((ftr, fIdx) => (
                    <RepeatItem
                      key={fIdx}
                      index={fIdx}
                      label={ftr.reference}
                      onRemove={() =>
                        patch((d) => {
                          d.runtime!.devcontainers!.features.splice(fIdx, 1);
                        })
                      }
                    >
                      <Stack gap="md">
                        <TextInput
                          label="Reference"
                          description="Feature image reference."
                          placeholder="ghcr.io/devcontainers/features/aws-cli:1"
                          required
                          value={ftr.reference}
                          onChange={(e) => {
                            const v = e.currentTarget.value;
                            patch((d) => {
                              d.runtime!.devcontainers!.features[
                                fIdx
                              ].reference = v;
                            });
                          }}
                        />

                        <RepeatBlock
                          title="Options"
                          description="Key/value options passed to the feature."
                          addLabel="Add option"
                          emptyHint="Using the feature defaults."
                          count={ftr.options.length}
                          onAdd={() =>
                            patch((d) => {
                              d.runtime!.devcontainers!.features[
                                fIdx
                              ].options.push(
                                WsPB.Workspace_Spec_Runtime_Devcontainers_Feature_Option.create(),
                              );
                            })
                          }
                        >
                          {ftr.options.map((opt, oIdx) => (
                            <RepeatItem
                              key={oIdx}
                              index={oIdx}
                              label={opt.key}
                              onRemove={() =>
                                patch((d) => {
                                  d.runtime!.devcontainers!.features[
                                    fIdx
                                  ].options.splice(oIdx, 1);
                                })
                              }
                            >
                              <div className="grid gap-4 md:grid-cols-2">
                                <TextInput
                                  label="Key"
                                  placeholder="version"
                                  required
                                  value={opt.key}
                                  onChange={(e) => {
                                    const v = e.currentTarget.value;
                                    patch((d) => {
                                      d.runtime!.devcontainers!.features[
                                        fIdx
                                      ].options[oIdx].key = v;
                                    });
                                  }}
                                />
                                <TextInput
                                  label="Value"
                                  placeholder="latest"
                                  required
                                  value={opt.value}
                                  onChange={(e) => {
                                    const v = e.currentTarget.value;
                                    patch((d) => {
                                      d.runtime!.devcontainers!.features[
                                        fIdx
                                      ].options[oIdx].value = v;
                                    });
                                  }}
                                />
                              </div>
                            </RepeatItem>
                          ))}
                        </RepeatBlock>
                      </Stack>
                    </RepeatItem>
                  ))}
                </RepeatBlock>
              )}
            </OptionalBlock>

            <OptionalBlock
              icon={<IconPlugConnected size={16} />}
              title="Octelium Services"
              description="Expose the Octelium Services assigned to you inside the Workspace network."
              enabled={!!runtime.octelium}
              onEnable={() =>
                patch((d) => {
                  d.runtime!.octelium =
                    WsPB.Workspace_Spec_Runtime_Octelium.create();
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.runtime!.octelium = undefined;
                })
              }
            >
              {runtime.octelium && (
                <Stack gap="md">
                  <Switch
                    size="sm"
                    label="Serve all Services"
                    description="Serve every Service assigned to your User."
                    checked={runtime.octelium.serveAll}
                    onChange={(e) => {
                      const v = e.currentTarget.checked;
                      patch((d) => {
                        d.runtime!.octelium!.serveAll = v;
                      });
                    }}
                  />
                  {!runtime.octelium.serveAll && (
                    <TagsInput
                      label="Service names"
                      description="Specific Service names to serve. Press Enter after each. Up to 128."
                      placeholder="postgres.prod"
                      value={runtime.octelium.serveServices}
                      onChange={(v) =>
                        patch((d) => {
                          d.runtime!.octelium!.serveServices = v;
                        })
                      }
                    />
                  )}
                </Stack>
              )}
            </OptionalBlock>

            <OptionalBlock
              icon={<IconShieldLock size={16} />}
              title="Linux capabilities"
              description="Fine-tune the container's kernel capabilities. Up to 100 each."
              enabled={!!runtime.capabilities}
              onEnable={() =>
                patch((d) => {
                  d.runtime!.capabilities =
                    WsPB.Workspace_Spec_Runtime_Capabilities.create();
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.runtime!.capabilities = undefined;
                })
              }
            >
              {runtime.capabilities && (
                <div className="grid gap-4 md:grid-cols-2">
                  <TagsInput
                    label="Add"
                    description="Uppercase names without the CAP_ prefix, e.g. NET_ADMIN."
                    placeholder="NET_ADMIN"
                    value={runtime.capabilities.add}
                    onChange={(v) =>
                      patch((d) => {
                        d.runtime!.capabilities!.add = v;
                      })
                    }
                  />
                  <TagsInput
                    label="Drop"
                    description='Capabilities to remove. Use "ALL" to drop everything.'
                    placeholder="NET_RAW"
                    value={runtime.capabilities.drop}
                    onChange={(v) =>
                      patch((d) => {
                        d.runtime!.capabilities!.drop = v;
                      })
                    }
                  />
                </div>
              )}
            </OptionalBlock>

            <OptionalBlock
              title="Read-only root filesystem"
              description="Mount the container root as read-only. Only /workspace stays writable."
              enabled={!!runtime.filesystem}
              onEnable={() =>
                patch((d) => {
                  d.runtime!.filesystem =
                    WsPB.Workspace_Spec_Runtime_Filesystem.create({
                      readOnly: true,
                    });
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.runtime!.filesystem = undefined;
                })
              }
            >
              {runtime.filesystem && (
                <Switch
                  size="sm"
                  label="Read-only root filesystem"
                  checked={runtime.filesystem.readOnly}
                  onChange={(e) => {
                    const v = e.currentTarget.checked;
                    patch((d) => {
                      d.runtime!.filesystem!.readOnly = v;
                    });
                  }}
                />
              )}
            </OptionalBlock>

            <OptionalBlock
              title="Inactivity timeout"
              description="Overrides how the Cluster auto-stops idle Workspaces."
              enabled={!!runtime.timeout}
              onEnable={() =>
                patch((d) => {
                  d.runtime!.timeout =
                    WsPB.Workspace_Spec_Runtime_Timeout.create({
                      mode: WsPB.Workspace_Spec_Runtime_Timeout_Mode.DEFAULT,
                    });
                })
              }
              onDisable={() =>
                patch((d) => {
                  d.runtime!.timeout = undefined;
                })
              }
            >
              {runtime.timeout && (
                <SegmentedControl
                  size="xs"
                  className="w-fit"
                  value={
                    runtime.timeout.mode ===
                    WsPB.Workspace_Spec_Runtime_Timeout_Mode.DISABLED
                      ? "disabled"
                      : "default"
                  }
                  onChange={(v) =>
                    patch((d) => {
                      d.runtime!.timeout!.mode =
                        v === "disabled"
                          ? WsPB.Workspace_Spec_Runtime_Timeout_Mode.DISABLED
                          : WsPB.Workspace_Spec_Runtime_Timeout_Mode.DEFAULT;
                    })
                  }
                  data={[
                    { label: "Cluster default", value: "default" },
                    { label: "Never time out", value: "disabled" },
                  ]}
                />
              )}
            </OptionalBlock>
          </Stack>
        )}
      </OptionalBlock>
    </Stack>
  );
};

export default RuntimeSection;
