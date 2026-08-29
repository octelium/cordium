import Meta from "@/components/Meta";
import MetadataEdit from "@/components/MetadataEdit";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import SecretSelect from "@/components/SecretSelect";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getPathSpace, invalidateGitProviders } from "@/utils/octelium";
import { cloneResource, getResourceRef, getShortName } from "@/utils/pb";
import {
  Alert,
  Button,
  SegmentedControl,
  Stack,
  TagsInput,
  Text,
  TextInput,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconGitBranch, IconSettings2 } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

type ProviderKind = "github" | "gitlab" | "oauth2";

const emptySecret = () => ({
  type: { oneofKind: "fromSecret" as const, fromSecret: "" },
});

const makeSpec = (kind: ProviderKind): WsPB.GitProvider["spec"] => {
  switch (kind) {
    case "github":
      return {
        type: {
          oneofKind: "github",
          github: {
            clientID: "",
            clientSecret: emptySecret(),
            scopes: ["repo"],
          },
        },
      };
    case "gitlab":
      return {
        type: {
          oneofKind: "gitlab",
          gitlab: {
            clientID: "",
            clientSecret: emptySecret(),
            scopes: ["read_repository"],
          },
        },
      };
    case "oauth2":
      return {
        type: {
          oneofKind: "oauth2",
          oauth2: {
            clientID: "",
            clientSecret: emptySecret(),
            authURL: "",
            tokenURL: "",
            scopes: [],
          },
        },
      };
  }
};

const CreateGitProvider = () => {
  const ctx = useContextSpace();
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const space = ctx.space.data;

  const [kind, setKind] = React.useState<ProviderKind>("github");
  const [req, setReq] = React.useState(
    WsPB.GitProvider.create({
      apiVersion: "cordium/v1",
      kind: "GitProvider",
      metadata: {},
      spec: makeSpec("github"),
      status: {},
    }),
  );

  const patch = (fn: (draft: WsPB.GitProvider) => void) => {
    const next = cloneResource(req);
    fn(next);
    setReq(next);
  };

  const innerOf = (draft: WsPB.GitProvider) => {
    const t = draft.spec!.type;
    if (t.oneofKind === "github") return t.github;
    if (t.oneofKind === "gitlab") return t.gitlab;
    if (t.oneofKind === "oauth2") return t.oauth2;
    return undefined;
  };

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = cloneResource(req);
      payload.status!.spaceRef = getResourceRef(space!);
      const { response } = await client.createGitProvider(payload);
      return response;
    },
    onSuccess: () => {
      invalidateGitProviders();
      toast.success("Git provider created");
      navigate(`${getPathSpace(space!)}/gitproviders`);
    },
    onError,
  });

  if (!space) return null;

  const t = req.spec!.type;
  const inner =
    t.oneofKind === "github"
      ? t.github
      : t.oneofKind === "gitlab"
        ? t.gitlab
        : t.oneofKind === "oauth2"
          ? t.oauth2
          : undefined;

  const clientSecret =
    inner?.clientSecret?.type.oneofKind === "fromSecret"
      ? inner.clientSecret.type.fromSecret
      : "";

  return (
    <>
      <Meta title="New Git provider" />
      <PageHeader
        title="New Git provider"
        crumbs={[
          { label: "Spaces", to: "/spaces" },
          { label: getShortName(space), to: getPathSpace(space) },
          { label: "Git providers", to: `${getPathSpace(space)}/gitproviders` },
          { label: "New" },
        ]}
        description="Register an OAuth application so members can authorise Cordium to clone their private repositories."
      />

      <div className="max-w-3xl">
        <Stack gap="lg">
          <Panel>
            <PanelHeader
              icon={<IconGitBranch size={16} />}
              title="Identity"
              description="Templates reference this provider by name."
            />
            <PanelBody>
              <MetadataEdit
                metadata={req.metadata!}
                parentName={space.metadata?.name}
                onChange={(md) =>
                  patch((d) => {
                    d.metadata = md;
                  })
                }
              />
            </PanelBody>
          </Panel>

          <Panel>
            <PanelHeader
              icon={<IconSettings2 size={16} />}
              title="OAuth application"
              description="Create the app with your provider first, then paste its credentials here."
            />
            <PanelBody>
              <Stack gap="lg">
                <div>
                  <Text size="sm" fw={700} mb={6}>
                    Provider
                  </Text>
                  <SegmentedControl
                    value={kind}
                    onChange={(v) => {
                      const next = v as ProviderKind;
                      setKind(next);
                      patch((d) => {
                        d.spec = makeSpec(next);
                      });
                    }}
                    data={[
                      { label: "GitHub", value: "github" },
                      { label: "GitLab", value: "gitlab" },
                      { label: "Generic OAuth2", value: "oauth2" },
                    ]}
                  />
                </div>

                <Alert color="gray" variant="light">
                  The client secret is never entered here directly — store it as
                  a Secret in this Space first, then select it below.
                </Alert>

                <div className="grid gap-4 md:grid-cols-2">
                  <TextInput
                    label="Client ID"
                    description="Public identifier of the OAuth application."
                    placeholder="Iv1.a1b2c3d4e5f6"
                    required
                    value={inner?.clientID ?? ""}
                    onChange={(e) => {
                      const v = e.currentTarget.value;
                      patch((d) => {
                        const target = innerOf(d);
                        if (target) target.clientID = v;
                      });
                    }}
                  />
                  <SecretSelect
                    spaceRef={getResourceRef(space)}
                    required
                    label="Client secret"
                    description="Space Secret holding the OAuth client secret."
                    value={clientSecret}
                    onChange={(val) =>
                      patch((d) => {
                        const target = innerOf(d);
                        if (target) {
                          target.clientSecret = {
                            type: { oneofKind: "fromSecret", fromSecret: val },
                          };
                        }
                      })
                    }
                  />
                </div>

                {t.oneofKind === "oauth2" && (
                  <div className="grid gap-4 md:grid-cols-2">
                    <TextInput
                      label="Authorization URL"
                      description="Endpoint users are redirected to for consent."
                      placeholder="https://git.example.com/oauth/authorize"
                      required
                      value={t.oauth2.authURL}
                      onChange={(e) => {
                        const v = e.currentTarget.value;
                        patch((d) => {
                          const dt = d.spec!.type;
                          if (dt.oneofKind === "oauth2") dt.oauth2.authURL = v;
                        });
                      }}
                    />
                    <TextInput
                      label="Token URL"
                      description="Endpoint that exchanges the code for an access token."
                      placeholder="https://git.example.com/oauth/token"
                      required
                      value={t.oauth2.tokenURL}
                      onChange={(e) => {
                        const v = e.currentTarget.value;
                        patch((d) => {
                          const dt = d.spec!.type;
                          if (dt.oneofKind === "oauth2") dt.oauth2.tokenURL = v;
                        });
                      }}
                    />
                  </div>
                )}

                <TagsInput
                  label="Scopes"
                  description="OAuth scopes requested at sign-in. Press Enter after each."
                  placeholder="repo"
                  value={inner?.scopes ?? []}
                  onChange={(v) =>
                    patch((d) => {
                      const target = innerOf(d);
                      if (target) target.scopes = v;
                    })
                  }
                />
              </Stack>
            </PanelBody>
            <PanelFooter>
              <Button variant="default" onClick={() => navigate(-1)}>
                Cancel
              </Button>
              <Button
                loading={mutation.isPending}
                disabled={!req.metadata?.name || !inner?.clientID}
                onClick={() => mutation.mutate()}
              >
                Create provider
              </Button>
            </PanelFooter>
          </Panel>
        </Stack>
      </div>
    </>
  );
};

export default CreateGitProvider;
