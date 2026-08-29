import OptionalBlock from "@/components/OptionalBlock";
import RepeatBlock, { RepeatItem } from "@/components/RepeatBlock";
import SecretSelect from "@/components/SecretSelect";
import { NumberInput, Stack, Switch, TextInput } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { IconGitFork } from "@tabler/icons-react";
import { CommonSpec, SectionProps } from "./types";

type RepoSelector = (draft: CommonSpec) => WsPB.Workspace_Spec_Repository;

const emptyHttpAuth = () =>
  WsPB.Workspace_Spec_Repository_Authentication.create({
    type: {
      oneofKind: "http",
      http: WsPB.Workspace_Spec_Repository_Authentication_HTTP.create({
        password: { type: { oneofKind: "fromSecret", fromSecret: "" } },
      }),
    },
  });

const RepositoryFields = (props: {
  repo: WsPB.Workspace_Spec_Repository;
  select: RepoSelector;
  spaceRef: MetaPB.ObjectReference;
  requireUrl?: boolean;
  patch: (fn: (draft: CommonSpec) => void) => void;
}) => {
  const { repo, select, patch } = props;
  const auth =
    repo.authentication?.type.oneofKind === "http"
      ? repo.authentication.type.http
      : undefined;

  return (
    <Stack gap="md">
      <TextInput
        label="Repository URL"
        description="HTTPS clone URL. Cloned into /workspace/repo when the Workspace starts."
        placeholder="https://github.com/octelium/octelium"
        required={props.requireUrl}
        value={repo.url}
        onChange={(e) => {
          const v = e.currentTarget.value;
          patch((d) => {
            select(d).url = v;
          });
        }}
      />

      <OptionalBlock
        title="Clone options"
        description="Control what git fetches. Leave off to do a full clone of the default branch."
        enabled={!!repo.cloneOptions}
        onEnable={() =>
          patch((d) => {
            select(d).cloneOptions =
              WsPB.Workspace_Spec_Repository_CloneOptions.create();
          })
        }
        onDisable={() =>
          patch((d) => {
            select(d).cloneOptions = undefined;
          })
        }
      >
        {repo.cloneOptions && (
          <Stack gap="md">
            <div className="grid gap-4 md:grid-cols-3">
              <TextInput
                label="Branch"
                description="Branch to clone. Defaults to the repository's default branch."
                placeholder="main"
                value={repo.cloneOptions.branch}
                onChange={(e) => {
                  const v = e.currentTarget.value;
                  patch((d) => {
                    select(d).cloneOptions!.branch = v;
                  });
                }}
              />
              <TextInput
                label="Checkout"
                description="Commit, tag or ref to check out after cloning."
                placeholder="v1.4.0"
                value={repo.cloneOptions.checkout}
                onChange={(e) => {
                  const v = e.currentTarget.value;
                  patch((d) => {
                    select(d).cloneOptions!.checkout = v;
                  });
                }}
              />
              <NumberInput
                label="Depth"
                description="Shallow clone depth. 0 clones the full history."
                placeholder="1"
                min={0}
                value={repo.cloneOptions.depth}
                onChange={(v) => {
                  const n = typeof v === "number" ? v : Number(v) || 0;
                  patch((d) => {
                    select(d).cloneOptions!.depth = n;
                  });
                }}
              />
            </div>

            <div className="grid gap-3 md:grid-cols-3">
              <Switch
                size="sm"
                label="Single branch"
                description="Only fetch the history leading to the tip of one branch."
                checked={repo.cloneOptions.singleBranch}
                onChange={(e) => {
                  const v = e.currentTarget.checked;
                  patch((d) => {
                    select(d).cloneOptions!.singleBranch = v;
                  });
                }}
              />
              <Switch
                size="sm"
                label="Shallow submodules"
                description="Clone submodules with a depth of 1."
                checked={repo.cloneOptions.shallowSubmodules}
                onChange={(e) => {
                  const v = e.currentTarget.checked;
                  patch((d) => {
                    select(d).cloneOptions!.shallowSubmodules = v;
                  });
                }}
              />
              <Switch
                size="sm"
                label="Disable lazy unshallow"
                description="Keep the clone shallow instead of fetching the rest on demand."
                checked={repo.cloneOptions.disableLazyUnshallow}
                onChange={(e) => {
                  const v = e.currentTarget.checked;
                  patch((d) => {
                    select(d).cloneOptions!.disableLazyUnshallow = v;
                  });
                }}
              />
            </div>
          </Stack>
        )}
      </OptionalBlock>

      <OptionalBlock
        title="HTTP authentication"
        description="Required for private repositories. The password is read from a Space Secret."
        enabled={!!repo.authentication}
        onEnable={() =>
          patch((d) => {
            select(d).authentication = emptyHttpAuth();
          })
        }
        onDisable={() =>
          patch((d) => {
            select(d).authentication = undefined;
          })
        }
      >
        {auth && (
          <div className="grid gap-4 md:grid-cols-2">
            <TextInput
              label="Username"
              description="Git username or token holder (e.g. your GitHub handle)."
              placeholder="octelium-bot"
              required
              value={auth.username}
              onChange={(e) => {
                const v = e.currentTarget.value;
                patch((d) => {
                  const t = select(d).authentication!.type;
                  if (t.oneofKind === "http") t.http.username = v;
                });
              }}
            />
            <SecretSelect
              spaceRef={props.spaceRef}
              required
              label="Password Secret"
              description="Secret holding the password or personal access token."
              value={
                auth.password?.type.oneofKind === "fromSecret"
                  ? auth.password.type.fromSecret
                  : ""
              }
              onChange={(val) =>
                patch((d) => {
                  const t = select(d).authentication!.type;
                  if (t.oneofKind === "http") {
                    t.http.password = {
                      type: { oneofKind: "fromSecret", fromSecret: val },
                    };
                  }
                })
              }
            />
          </div>
        )}
      </OptionalBlock>
    </Stack>
  );
};

const RepositorySection = (props: SectionProps) => {
  const { spec, patch } = props;

  return (
    <Stack gap="lg">
      <OptionalBlock
        icon={<IconGitFork size={16} />}
        title="Primary repository"
        description="Cloned into /workspace/repo before startup tasks run."
        enabled={!!spec.repository}
        onEnable={() =>
          patch((d) => {
            d.repository = WsPB.Workspace_Spec_Repository.create();
          })
        }
        onDisable={() =>
          patch((d) => {
            d.repository = undefined;
          })
        }
      >
        {spec.repository && (
          <RepositoryFields
            repo={spec.repository}
            select={(d) => d.repository!}
            spaceRef={props.spaceRef}
            patch={patch}
          />
        )}
      </OptionalBlock>

      <RepeatBlock
        title="Additional repositories"
        description="Extra repos cloned alongside the primary one. Up to 32."
        addLabel="Add repository"
        emptyHint="No additional repositories. They are cloned into /workspace/additional-repos/<name> unless you override the path."
        count={spec.additionalRepositories.length}
        onAdd={() =>
          patch((d) => {
            d.additionalRepositories.push(
              WsPB.Workspace_Spec_AdditionalRepository.create({
                repository: WsPB.Workspace_Spec_Repository.create(),
              }),
            );
          })
        }
      >
        {spec.additionalRepositories.map((repo, idx) => (
          <RepeatItem
            key={idx}
            index={idx}
            label={repo.name || repo.repository?.url}
            onRemove={() =>
              patch((d) => {
                d.additionalRepositories.splice(idx, 1);
              })
            }
          >
            <Stack gap="md">
              <div className="grid gap-4 md:grid-cols-2">
                <TextInput
                  label="Name"
                  description="Unique identifier for this repository."
                  placeholder="shared-lib"
                  required
                  value={repo.name}
                  onChange={(e) => {
                    const v = e.currentTarget.value;
                    patch((d) => {
                      d.additionalRepositories[idx].name = v;
                    });
                  }}
                />
                <TextInput
                  label="Clone path"
                  description="Overrides the default /workspace/additional-repos/<name>."
                  placeholder="/workspace/libs/shared"
                  value={repo.clonePath}
                  onChange={(e) => {
                    const v = e.currentTarget.value;
                    patch((d) => {
                      d.additionalRepositories[idx].clonePath = v;
                    });
                  }}
                />
              </div>

              {repo.repository && (
                <RepositoryFields
                  repo={repo.repository}
                  select={(d) => d.additionalRepositories[idx].repository!}
                  spaceRef={props.spaceRef}
                  requireUrl
                  patch={patch}
                />
              )}
            </Stack>
          </RepeatItem>
        ))}
      </RepeatBlock>
    </Stack>
  );
};

export default RepositorySection;
