import CodeEditor from "@/components/CodeEditor";
import Meta from "@/components/Meta";
import OptionalBlock from "@/components/OptionalBlock";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import QueryBoundary from "@/components/QueryBoundary";
import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import UserSecretSelect from "@/components/UserSecretSelect";
import {
  setItemsPerPage,
  setTerminalFontSize,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
} from "@/features/settings/slice";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import {
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
import {
  IconAdjustments,
  IconFileCode,
  IconTerminal2,
  IconUser,
} from "@tabler/icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";

const T = WsPB.Workspace_Spec_Runtime_Task_Type;

const LocalPreferences = () => {
  const dispatch = useAppDispatch();
  const fontSize = useAppSelector((s) => s.settings.terminalFontSize);
  const itemsPerPage = useAppSelector((s) => s.settings.itemsPerPage);

  return (
    <Panel>
      <PanelHeader
        icon={<IconAdjustments size={16} />}
        title="Portal preferences"
        description="Stored in this browser session only."
      />
      <PanelBody>
        <div className="grid gap-4 md:grid-cols-2">
          <NumberInput
            label="Terminal font size"
            description="Also adjustable from the terminal toolbar."
            min={TERMINAL_FONT_SIZE_MIN}
            max={TERMINAL_FONT_SIZE_MAX}
            leftSection={<IconTerminal2 size={15} />}
            value={fontSize}
            onChange={(v) =>
              dispatch(
                setTerminalFontSize({
                  value: typeof v === "number" ? v : Number(v) || 15,
                }),
              )
            }
          />
          <div>
            <Text size="sm" fw={500} mb={2}>
              Items per page
            </Text>
            <Text size="xs" c="dimmed" mb={8}>
              Applied to every list in the portal.
            </Text>
            <SegmentedControl
              value={String(itemsPerPage)}
              onChange={(v) =>
                dispatch(setItemsPerPage({ itemsPerPage: Number(v) }))
              }
              data={["10", "20", "50"].map((v) => ({ label: v, value: v }))}
            />
          </div>
        </div>
      </PanelBody>
    </Panel>
  );
};

const UserConfigForm = (props: { userConfig: WsPB.UserConfig }) => {
  const client = getClientWorkspace();
  const [req, setReq] = React.useState(
    WsPB.UserConfig.clone(props.userConfig),
  );

  const patch = (fn: (draft: WsPB.UserConfig) => void) => {
    const next = WsPB.UserConfig.clone(req);
    fn(next);
    setReq(next);
  };

  const qryRegions = useQuery({
    queryKey: ["workspace/listRegion"],
    queryFn: () => {
      const { response } = client.listRegion(
        WsPB.ListRegionOptions.create({}),
      );
      return response;
    },
  });

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.updateUserConfig(req);
      return response;
    },
    onSuccess: () => {
      toast.success("Settings saved");
    },
    onError,
  });

  const regions = qryRegions.data?.items ?? [];
  const dotfiles = req.spec?.dotfiles;
  const dotfilesAuth =
    dotfiles?.authentication?.type.oneofKind === "http"
      ? dotfiles.authentication.type.http
      : undefined;

  return (
    <Panel>
      <PanelHeader
        icon={<IconUser size={16} />}
        title="Workspace defaults"
        description="Applied to every Workspace you create, in every Space."
      />
      <PanelBody>
        <Stack gap="lg">
          {regions.length > 1 && (
            <Select
              label="Preferred region"
              description="Used when starting a Workspace without an explicit region."
              placeholder="Cluster default"
              clearable
              data={regions.map((x) => ({
                value: x.metadata!.name,
                label: [x.metadata!.name, x.status?.city, x.status?.country]
                  .filter(Boolean)
                  .join(" · "),
              }))}
              value={req.spec?.preferredRegion || null}
              onChange={(val) =>
                patch((d) => {
                  d.spec!.preferredRegion = val ?? "";
                })
              }
            />
          )}

          <OptionalBlock
            icon={<IconFileCode size={16} />}
            title="Dotfiles"
            description="Cloned and applied on the first start of each Workspace, so your shell and editor config follow you."
            enabled={!!dotfiles}
            onEnable={() =>
              patch((d) => {
                d.spec!.dotfiles = WsPB.UserConfig_Spec_Dotfiles.create();
              })
            }
            onDisable={() =>
              patch((d) => {
                d.spec!.dotfiles = undefined;
              })
            }
          >
            {dotfiles && (
              <Stack gap="md">
                <div className="grid gap-4 md:grid-cols-2">
                  <TextInput
                    label="Repository URL"
                    description="HTTPS URL of your dotfiles repository."
                    placeholder="https://github.com/you/dotfiles"
                    required
                    value={dotfiles.url}
                    onChange={(e) => {
                      const v = e.currentTarget.value;
                      patch((d) => {
                        d.spec!.dotfiles!.url = v;
                      });
                    }}
                  />
                  <TextInput
                    label="Branch"
                    description="Defaults to the repository's default branch."
                    placeholder="main"
                    value={dotfiles.branch}
                    onChange={(e) => {
                      const v = e.currentTarget.value;
                      patch((d) => {
                        d.spec!.dotfiles!.branch = v;
                      });
                    }}
                  />
                </div>

                <OptionalBlock
                  title="Private repository"
                  description="Authenticate with a username and one of your User Secrets."
                  enabled={!!dotfiles.authentication}
                  onEnable={() =>
                    patch((d) => {
                      d.spec!.dotfiles!.authentication =
                        WsPB.UserConfig_Spec_Dotfiles_Authentication.create({
                          type: {
                            oneofKind: "http",
                            http: WsPB.UserConfig_Spec_Dotfiles_Authentication_HTTP.create(
                              {
                                password: {
                                  type: {
                                    oneofKind: "fromUserSecret",
                                    fromUserSecret: "",
                                  },
                                },
                              },
                            ),
                          },
                        });
                    })
                  }
                  onDisable={() =>
                    patch((d) => {
                      d.spec!.dotfiles!.authentication = undefined;
                    })
                  }
                >
                  {dotfilesAuth && (
                    <div className="grid gap-4 md:grid-cols-2">
                      <TextInput
                        label="Username"
                        placeholder="your-git-handle"
                        required
                        value={dotfilesAuth.username}
                        onChange={(e) => {
                          const v = e.currentTarget.value;
                          patch((d) => {
                            const t = d.spec!.dotfiles!.authentication!.type;
                            if (t.oneofKind === "http") t.http.username = v;
                          });
                        }}
                      />
                      <UserSecretSelect
                        label="Password Secret"
                        description="User Secret holding the token or password."
                        value={
                          dotfilesAuth.password?.type.oneofKind ===
                          "fromUserSecret"
                            ? dotfilesAuth.password.type.fromUserSecret
                            : ""
                        }
                        onChange={(val) =>
                          patch((d) => {
                            const t = d.spec!.dotfiles!.authentication!.type;
                            if (t.oneofKind === "http") {
                              t.http.password = {
                                type: {
                                  oneofKind: "fromUserSecret",
                                  fromUserSecret: val,
                                },
                              };
                            }
                          })
                        }
                      />
                    </div>
                  )}
                </OptionalBlock>
              </Stack>
            )}
          </OptionalBlock>

          <RepeatBlock
            title="Environment variables"
            description="Injected into every Workspace you own."
            addLabel="Add variable"
            emptyHint="No personal environment variables."
            count={req.spec?.envVars.length ?? 0}
            onAdd={() =>
              patch((d) => {
                d.spec!.envVars.push(
                  WsPB.UserConfig_Spec_EnvVar.create({
                    key: "",
                    type: { oneofKind: "value", value: "" },
                  }),
                );
              })
            }
          >
            {(req.spec?.envVars ?? []).map((envVar, idx) => (
              <RepeatItem
                key={idx}
                index={idx}
                label={envVar.key}
                onRemove={() =>
                  patch((d) => {
                    d.spec!.envVars.splice(idx, 1);
                  })
                }
              >
                <div className="grid gap-4 md:grid-cols-[1fr_auto_1.4fr] md:items-end">
                  <TextInput
                    label="Key"
                    placeholder="EDITOR"
                    required
                    value={envVar.key}
                    onChange={(e) => {
                      const v = e.currentTarget.value;
                      patch((d) => {
                        d.spec!.envVars[idx].key = v;
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
                        envVar.type.oneofKind === "fromUserSecret"
                          ? "fromUserSecret"
                          : "value"
                      }
                      onChange={(v) =>
                        patch((d) => {
                          d.spec!.envVars[idx].type =
                            v === "fromUserSecret"
                              ? {
                                  oneofKind: "fromUserSecret",
                                  fromUserSecret: "",
                                }
                              : { oneofKind: "value", value: "" };
                        })
                      }
                      data={[
                        { label: "Literal", value: "value" },
                        { label: "Secret", value: "fromUserSecret" },
                      ]}
                    />
                  </div>

                  {envVar.type.oneofKind === "value" && (
                    <TextInput
                      label="Value"
                      placeholder="nvim"
                      value={envVar.type.value}
                      onChange={(e) => {
                        const v = e.currentTarget.value;
                        patch((d) => {
                          d.spec!.envVars[idx].type = {
                            oneofKind: "value",
                            value: v,
                          };
                        });
                      }}
                    />
                  )}

                  {envVar.type.oneofKind === "fromUserSecret" && (
                    <UserSecretSelect
                      value={envVar.type.fromUserSecret}
                      onChange={(val) =>
                        patch((d) => {
                          d.spec!.envVars[idx].type = {
                            oneofKind: "fromUserSecret",
                            fromUserSecret: val,
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
            title="Startup tasks"
            description="Personal scripts run in every Workspace you own, after the Space and Template tasks."
            addLabel="Add task"
            emptyHint="No personal startup tasks."
            count={req.spec?.tasks.length ?? 0}
            onAdd={() =>
              patch((d) => {
                d.spec!.tasks.push(
                  WsPB.Workspace_Spec_Runtime_Task.create({
                    type: T.POST_START,
                  }),
                );
              })
            }
          >
            {(req.spec?.tasks ?? []).map((task, idx) => (
              <RepeatItem
                key={idx}
                index={idx}
                label={task.name}
                onRemove={() =>
                  patch((d) => {
                    d.spec!.tasks.splice(idx, 1);
                  })
                }
              >
                <Stack gap="md">
                  <div className="grid gap-4 md:grid-cols-3">
                    <TextInput
                      label="Name"
                      placeholder="setup-shell"
                      required
                      value={task.name}
                      onChange={(e) => {
                        const v = e.currentTarget.value;
                        patch((d) => {
                          d.spec!.tasks[idx].name = v;
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
                          d.spec!.tasks[idx].workingDir = v;
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
                          d.spec!.tasks[idx].type = T[
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
                          d.spec!.tasks[idx].isBackground = v;
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
                          d.spec!.tasks[idx].runAsRoot = v;
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
                          d.spec!.tasks[idx].run = v;
                        })
                      }
                    />
                  </div>
                </Stack>
              </RepeatItem>
            ))}
          </RepeatBlock>
        </Stack>
      </PanelBody>
      <PanelFooter>
        <Button
          variant="default"
          onClick={() => setReq(WsPB.UserConfig.clone(props.userConfig))}
        >
          Reset
        </Button>
        <Button
          loading={mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          Save settings
        </Button>
      </PanelFooter>
    </Panel>
  );
};

const Page = () => {
  const client = getClientWorkspace();

  const qry = useQuery({
    queryKey: ["workspace/getUserConfig"],
    queryFn: () => {
      const { response } = client.getUserConfig({});
      return response;
    },
  });

  return (
    <>
      <Meta title="Settings" />
      <PageHeader
        title="Settings"
        description="Defaults applied to every Workspace you create, plus portal preferences."
      />

      <div className="max-w-4xl">
        <Stack gap="lg">
          <QueryBoundary query={qry}>
            {qry.data && (
              <UserConfigForm key={qry.data.metadata?.uid} userConfig={qry.data} />
            )}
          </QueryBoundary>
          <LocalPreferences />
        </Stack>
      </div>
    </>
  );
};

export default Page;
