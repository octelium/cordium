import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import MetadataEdit from "@/components/MetadataEdit";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { cloneResource, getResourceRef } from "@/utils/pb";
import {
  Button,
  Divider,
  Group,
  Select,
  Stack,
  Tabs,
  Text,
  TextInput,
  ThemeIcon,
} from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { GitBranch, Settings2 } from "lucide-react";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

type ProviderKind = "github" | "gitlab" | "oauth2";

const PROVIDER_TABS: { value: ProviderKind; label: string }[] = [
  { value: "github", label: "GitHub" },
  { value: "gitlab", label: "GitLab" },
  { value: "oauth2", label: "Generic OAuth2" },
];

const makeProviderSpec = (kind: ProviderKind): WsPB.GitProvider["spec"] => {
  const secret = { type: { oneofKind: "fromSecret" as const, fromSecret: "" } };
  switch (kind) {
    case "github":
      return {
        type: {
          oneofKind: "github",
          github: { clientID: "", clientSecret: secret, scopes: [] },
        },
      };
    case "gitlab":
      return {
        type: {
          oneofKind: "gitlab",
          gitlab: { clientID: "", clientSecret: secret, scopes: [] },
        },
      };
    case "oauth2":
      return {
        type: {
          oneofKind: "oauth2",
          oauth2: {
            clientID: "",
            authURL: "",
            tokenURL: "",
            scopes: [],
            clientSecret: secret,
          },
        },
      };
  }
};

const CreateGitProvider = () => {
  const ctx = useContextSpace();
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [req, setReq] = React.useState(
    WsPB.GitProvider.create({
      apiVersion: "cordium/v1",
      kind: "GitProvider",
      metadata: {},
      spec: makeProviderSpec("github"),
      status: {},
    }),
  );

  const [activeTab, setActiveTab] = React.useState<ProviderKind>("github");

  const updateReq = () => setReq(cloneResource(req) as WsPB.GitProvider);

  const mutation = useMutation({
    mutationFn: async () => {
      req.status!.spaceRef = getResourceRef(ctx.space.data!);
      const { response } = await client.createGitProvider(req);
      return response;
    },
    onSuccess: (data) => {
      queryClient.refetchQueries({
        queryKey: [
          "workspace/listGitProvider",
          ctx.space.data?.metadata?.uid,
          0,
        ],
      });
      toast.success("Git provider created");
      navigate(`${getPathSpace(ctx.space.data!)}/gitproviders`);
    },
    onError,
  });

  const qrySecret = useQuery({
    queryKey: ["workspace/listSecret/", ctx.space.data?.metadata?.uid],
    queryFn: () => {
      const { response } = client.listSecret(
        WsPB.ListSecretOptions.create({
          spaceRef: getResourceRef(ctx.space.data!),
          common: { itemsPerPage: 500 },
        }),
      );
      return response;
    },
    enabled: ctx.space.isSuccess,
  });

  if (!ctx.space.isSuccess || !qrySecret.isSuccess) return null;

  const data = ctx.space.data;

  const secretOptions = qrySecret.data.items.map((x) => ({
    value: x.metadata!.name,
    label: x.metadata!.name.split(".").at(0) ?? x.metadata!.name,
  }));

  const getClientSecretValue = (): string => {
    const t = req.spec!.type;
    if (t.oneofKind === "github")
      return t.github.clientSecret?.type.oneofKind === "fromSecret"
        ? t.github.clientSecret.type.fromSecret
        : "";
    if (t.oneofKind === "gitlab")
      return t.gitlab.clientSecret?.type.oneofKind === "fromSecret"
        ? t.gitlab.clientSecret.type.fromSecret
        : "";
    if (t.oneofKind === "oauth2")
      return t.oauth2.clientSecret?.type.oneofKind === "fromSecret"
        ? t.oauth2.clientSecret.type.fromSecret
        : "";
    return "";
  };

  const setClientID = (v: string) => {
    const t = req.spec!.type;
    if (t.oneofKind === "github") t.github.clientID = v;
    else if (t.oneofKind === "gitlab") t.gitlab.clientID = v;
    else if (t.oneofKind === "oauth2") t.oauth2.clientID = v;
    updateReq();
  };

  const setClientSecret = (val: string) => {
    const t = req.spec!.type;
    const patch = { oneofKind: "fromSecret" as const, fromSecret: val };
    if (t.oneofKind === "github" && t.github.clientSecret)
      t.github.clientSecret.type = patch;
    else if (t.oneofKind === "gitlab" && t.gitlab.clientSecret)
      t.gitlab.clientSecret.type = patch;
    else if (t.oneofKind === "oauth2" && t.oauth2.clientSecret)
      t.oauth2.clientSecret.type = patch;
    updateReq();
  };

  const getClientID = (): string => {
    const t = req.spec!.type;
    if (t.oneofKind === "github") return t.github.clientID;
    if (t.oneofKind === "gitlab") return t.gitlab.clientID;
    if (t.oneofKind === "oauth2") return t.oauth2.clientID;
    return "";
  };

  return (
    <Stack gap="xl">
      <div
        style={{
          background: "#f8fafc",
          border: "1px solid #e2e8f0",
          borderRadius: 10,
          padding: "16px 20px",
        }}
      >
        <Group gap="xs" mb="md">
          <ThemeIcon size="sm" variant="light" color="blue" radius="md">
            <GitBranch size={13} />
          </ThemeIcon>
          <Text
            size="xs"
            fw={600}
            tt="uppercase"
            style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
          >
            Metadata
          </Text>
        </Group>
        <MetadataEdit
          metadata={req.metadata!}
          onUpdate={(itm) => {
            req.metadata = itm;
            updateReq();
          }}
          parentName={data.metadata?.name}
        />
      </div>

      <div
        style={{
          background: "#f8fafc",
          border: "1px solid #e2e8f0",
          borderRadius: 10,
          padding: "16px 20px",
        }}
      >
        <Group gap="xs" mb="md">
          <ThemeIcon size="sm" variant="light" color="violet" radius="md">
            <Settings2 size={13} />
          </ThemeIcon>
          <Text
            size="xs"
            fw={600}
            tt="uppercase"
            style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
          >
            Spec
          </Text>
        </Group>

        <Tabs
          value={activeTab}
          onChange={(v) => {
            const kind = v as ProviderKind;
            setActiveTab(kind);
            req.spec = makeProviderSpec(kind);
            updateReq();
          }}
        >
          <Tabs.List mb="md">
            {PROVIDER_TABS.map((t) => (
              <Tabs.Tab key={t.value} value={t.value} style={{ fontSize: 13 }}>
                {t.label}
              </Tabs.Tab>
            ))}
          </Tabs.List>

          {PROVIDER_TABS.map((t) => (
            <Tabs.Panel key={t.value} value={t.value}>
              <Stack gap="md">
                <Group grow align="flex-start">
                  <TextInput
                    label="Client ID"
                    placeholder="abcdefg123456"
                    required
                    value={getClientID()}
                    onChange={(e) => setClientID(e.currentTarget.value)}
                  />
                  <Select
                    label="Client secret"
                    placeholder="Select a secret…"
                    required
                    data={secretOptions}
                    value={getClientSecretValue() || null}
                    onChange={(val) => val && setClientSecret(val)}
                  />
                </Group>

                {t.value === "oauth2" &&
                  req.spec!.type.oneofKind === "oauth2" && (
                    <Group grow align="flex-start">
                      <TextInput
                        label="Auth URL"
                        placeholder="https://example.com/oauth/authorize"
                        required
                        value={req.spec!.type.oauth2.authURL}
                        onChange={(e) => {
                          (
                            req.spec!.type as {
                              oneofKind: "oauth2";
                              oauth2: WsPB.GitProvider_Spec_OAuth2;
                            }
                          ).oauth2.authURL = e.currentTarget.value;
                          updateReq();
                        }}
                      />
                      <TextInput
                        label="Token URL"
                        placeholder="https://example.com/oauth/token"
                        required
                        value={req.spec!.type.oauth2.tokenURL}
                        onChange={(e) => {
                          (
                            req.spec!.type as {
                              oneofKind: "oauth2";
                              oauth2: WsPB.GitProvider_Spec_OAuth2;
                            }
                          ).oauth2.tokenURL = e.currentTarget.value;
                          updateReq();
                        }}
                      />
                    </Group>
                  )}
              </Stack>
            </Tabs.Panel>
          ))}
        </Tabs>
      </div>

      <Divider />

      <Group justify="flex-end" gap="sm">
        <Button variant="default" size="sm" onClick={() => navigate(-1)}>
          Cancel
        </Button>
        <Button
          size="sm"
          loading={mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          Create provider
        </Button>
      </Group>
    </Stack>
  );
};

export default CreateGitProvider;
