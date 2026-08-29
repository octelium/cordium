import CodeEditor from "@/components/CodeEditor";
import OptionalBlock from "@/components/OptionalBlock";
import SecretSelect from "@/components/SecretSelect";
import { SegmentedControl, Stack, Text, TextInput } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { CommonSpec, SectionProps } from "./types";

type ImageKind = "none" | "registry" | "dockerfile" | "git" | "repository";

const kindOf = (image?: WsPB.Workspace_Spec_Image): ImageKind => {
  switch (image?.type.oneofKind) {
    case "registry":
      return "registry";
    case "dockerfile":
      return "dockerfile";
    case "git":
      return "git";
    case "repository":
      return "repository";
    default:
      return "none";
  }
};

const hints: Record<ImageKind, string> = {
  none: "No image is set, so the Space or Cluster default image is used.",
  registry: "Pull a prebuilt image from a container registry.",
  dockerfile: "Build the image from a Dockerfile you provide inline or by URL.",
  git: "Build the image from a Dockerfile inside another git repository.",
  repository:
    "Build the image from the primary repository — either its devcontainer or a Dockerfile it contains.",
};

const registryOf = (d: CommonSpec) =>
  d.image?.type.oneofKind === "registry" ? d.image.type.registry : undefined;

const dockerfileOf = (d: CommonSpec) =>
  d.image?.type.oneofKind === "dockerfile" ? d.image.type.dockerfile : undefined;

const gitOf = (d: CommonSpec) =>
  d.image?.type.oneofKind === "git" ? d.image.type.git : undefined;

const repoOf = (d: CommonSpec) =>
  d.image?.type.oneofKind === "repository" ? d.image.type.repository : undefined;

const ImageSection = (props: SectionProps) => {
  const { spec, patch } = props;
  const kind = kindOf(spec.image);

  const setKind = (next: ImageKind) => {
    patch((d) => {
      if (next === "none") {
        d.image = undefined;
        return;
      }

      const created = WsPB.Workspace_Spec_Image.create();
      switch (next) {
        case "registry":
          created.type = {
            oneofKind: "registry",
            registry: WsPB.Workspace_Spec_Image_Registry.create(),
          };
          break;
        case "dockerfile":
          created.type = {
            oneofKind: "dockerfile",
            dockerfile: WsPB.Workspace_Spec_Image_Dockerfile.create({
              type: { oneofKind: "inline", inline: "" },
            }),
          };
          break;
        case "git":
          created.type = {
            oneofKind: "git",
            git: WsPB.Workspace_Spec_Image_Git.create(),
          };
          break;
        case "repository":
          created.type = {
            oneofKind: "repository",
            repository: WsPB.Workspace_Spec_Image_Repository.create({
              type: {
                oneofKind: "devcontainer",
                devcontainer:
                  WsPB.Workspace_Spec_Image_Repository_Devcontainer.create({
                    dirPath: "/workspace/repo/.devcontainer",
                  }),
              },
            }),
          };
          break;
      }
      d.image = created;
    });
  };

  const registry = registryOf(spec);
  const dockerfile = dockerfileOf(spec);
  const git = gitOf(spec);
  const repository = repoOf(spec);

  return (
    <Stack gap="lg">
      <div>
        <Text size="sm" fw={700} mb={4}>
          Image source
        </Text>
        <SegmentedControl
          fullWidth
          value={kind}
          onChange={(v) => setKind(v as ImageKind)}
          data={[
            { label: "Default", value: "none" },
            { label: "Registry", value: "registry" },
            { label: "Dockerfile", value: "dockerfile" },
            { label: "Git", value: "git" },
            { label: "From repo", value: "repository" },
          ]}
        />
        <Text size="xs" c="dimmed" mt={6}>
          {hints[kind]}
        </Text>
      </div>

      {registry && (
        <Stack gap="md">
          <TextInput
            label="Image URL"
            description='Registry reference, e.g. "ubuntu:24.04" or "mcr.microsoft.com/devcontainers/universal:linux".'
            placeholder="ubuntu:24.04"
            required
            value={registry.url}
            onChange={(e) => {
              const v = e.currentTarget.value;
              patch((d) => {
                registryOf(d)!.url = v;
              });
            }}
          />

          <OptionalBlock
            title="Registry authentication"
            description="Needed for private registries. The password comes from a Space Secret."
            enabled={!!registry.authentication}
            onEnable={() =>
              patch((d) => {
                registryOf(d)!.authentication =
                  WsPB.Workspace_Spec_Image_Registry_Authentication.create({
                    password: {
                      type: { oneofKind: "fromSecret", fromSecret: "" },
                    },
                  });
              })
            }
            onDisable={() =>
              patch((d) => {
                registryOf(d)!.authentication = undefined;
              })
            }
          >
            {registry.authentication && (
              <div className="grid gap-4 md:grid-cols-2">
                <TextInput
                  label="Username"
                  description="Registry username."
                  placeholder="octelium-bot"
                  required
                  value={registry.authentication.username}
                  onChange={(e) => {
                    const v = e.currentTarget.value;
                    patch((d) => {
                      registryOf(d)!.authentication!.username = v;
                    });
                  }}
                />
                <SecretSelect
                  spaceRef={props.spaceRef}
                  required
                  label="Password Secret"
                  description="Secret holding the registry password or token."
                  value={
                    registry.authentication.password?.type.oneofKind ===
                    "fromSecret"
                      ? registry.authentication.password.type.fromSecret
                      : ""
                  }
                  onChange={(val) =>
                    patch((d) => {
                      registryOf(d)!.authentication!.password = {
                        type: { oneofKind: "fromSecret", fromSecret: val },
                      };
                    })
                  }
                />
              </div>
            )}
          </OptionalBlock>
        </Stack>
      )}

      {dockerfile && (
        <Stack gap="md">
          <SegmentedControl
            size="xs"
            className="w-fit"
            value={dockerfile.type.oneofKind === "url" ? "url" : "inline"}
            onChange={(v) =>
              patch((d) => {
                dockerfileOf(d)!.type =
                  v === "url"
                    ? { oneofKind: "url", url: "" }
                    : { oneofKind: "inline", inline: "" };
              })
            }
            data={[
              { label: "Write inline", value: "inline" },
              { label: "Fetch from URL", value: "url" },
            ]}
          />

          {dockerfile.type.oneofKind === "inline" && (
            <div>
              <Text size="sm" fw={500} mb={2}>
                Dockerfile
              </Text>
              <Text size="xs" c="dimmed" mb={8}>
                Built when the Workspace starts. Maximum 5000 characters. COPY
                and ADD from a local context are not supported.
              </Text>
              <CodeEditor
                mode="dockerfile"
                value={dockerfile.type.inline}
                minHeight="220px"
                onChange={(v) =>
                  patch((d) => {
                    dockerfileOf(d)!.type = { oneofKind: "inline", inline: v };
                  })
                }
              />
            </div>
          )}

          {dockerfile.type.oneofKind === "url" && (
            <TextInput
              label="Dockerfile URL"
              description="Publicly reachable URL that returns a Dockerfile."
              placeholder="https://raw.githubusercontent.com/org/repo/main/Dockerfile"
              required
              value={dockerfile.type.url}
              onChange={(e) => {
                const v = e.currentTarget.value;
                patch((d) => {
                  dockerfileOf(d)!.type = { oneofKind: "url", url: v };
                });
              }}
            />
          )}
        </Stack>
      )}

      {git && (
        <div className="grid gap-4 md:grid-cols-2">
          <TextInput
            label="Repository URL"
            description="Git repository that contains the Dockerfile."
            placeholder="https://github.com/org/images"
            required
            value={git.url}
            onChange={(e) => {
              const v = e.currentTarget.value;
              patch((d) => {
                gitOf(d)!.url = v;
              });
            }}
          />
          <TextInput
            label="Checkout"
            description="Branch, tag or commit to build from."
            placeholder="main"
            value={git.checkout}
            onChange={(e) => {
              const v = e.currentTarget.value;
              patch((d) => {
                gitOf(d)!.checkout = v;
              });
            }}
          />
          <TextInput
            label="Dockerfile path"
            description="Path to the Dockerfile inside that repository."
            placeholder="/docker/Dockerfile"
            value={git.dockerfile}
            onChange={(e) => {
              const v = e.currentTarget.value;
              patch((d) => {
                gitOf(d)!.dockerfile = v;
              });
            }}
          />
          <TextInput
            label="Build context"
            description="Directory used as the build context. Defaults to the repository root."
            placeholder="/"
            value={git.context}
            onChange={(e) => {
              const v = e.currentTarget.value;
              patch((d) => {
                gitOf(d)!.context = v;
              });
            }}
          />
        </div>
      )}

      {repository && (
        <Stack gap="md">
          <SegmentedControl
            size="xs"
            className="w-fit"
            value={
              repository.type.oneofKind === "dockerfile"
                ? "dockerfile"
                : "devcontainer"
            }
            onChange={(v) =>
              patch((d) => {
                repoOf(d)!.type =
                  v === "dockerfile"
                    ? {
                        oneofKind: "dockerfile",
                        dockerfile:
                          WsPB.Workspace_Spec_Image_Repository_Dockerfile.create(),
                      }
                    : {
                        oneofKind: "devcontainer",
                        devcontainer:
                          WsPB.Workspace_Spec_Image_Repository_Devcontainer.create(
                            { dirPath: "/workspace/repo/.devcontainer" },
                          ),
                      };
              })
            }
            data={[
              { label: "Devcontainer", value: "devcontainer" },
              { label: "Dockerfile", value: "dockerfile" },
            ]}
          />

          {repository.type.oneofKind === "devcontainer" && (
            <TextInput
              label="Devcontainer directory"
              description="Directory in the primary repository holding devcontainer.json."
              placeholder="/workspace/repo/.devcontainer"
              required
              value={repository.type.devcontainer.dirPath}
              onChange={(e) => {
                const v = e.currentTarget.value;
                patch((d) => {
                  const t = repoOf(d)!.type;
                  if (t.oneofKind === "devcontainer") {
                    t.devcontainer.dirPath = v;
                  }
                });
              }}
            />
          )}

          {repository.type.oneofKind === "dockerfile" && (
            <div className="grid gap-4 md:grid-cols-2">
              <TextInput
                label="Dockerfile path"
                description="Path to the Dockerfile inside the primary repository."
                placeholder="/workspace/repo/Dockerfile"
                required
                value={repository.type.dockerfile.path}
                onChange={(e) => {
                  const v = e.currentTarget.value;
                  patch((d) => {
                    const t = repoOf(d)!.type;
                    if (t.oneofKind === "dockerfile") t.dockerfile.path = v;
                  });
                }}
              />
              <TextInput
                label="Build context"
                description="Directory used as the build context. Defaults to the repository root."
                placeholder="/workspace/repo"
                value={repository.type.dockerfile.context}
                onChange={(e) => {
                  const v = e.currentTarget.value;
                  patch((d) => {
                    const t = repoOf(d)!.type;
                    if (t.oneofKind === "dockerfile") t.dockerfile.context = v;
                  });
                }}
              />
            </div>
          )}
        </Stack>
      )}
    </Stack>
  );
};

export default ImageSection;
