import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import Field from "@/components/Field";
import MetadataEdit from "@/components/MetadataEdit";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { cloneResource, getResourceRef } from "@/utils/pb";
import { Button, Group, Select, Tabs } from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateGitProvider = () => {
  const ctx = useContextSpace();

  if (!ctx.space.isSuccess) {
    return <></>;
  }

  let [req, setReq] = React.useState(
    WsPB.GitProvider.create({
      apiVersion: "workspace/v1",
      kind: "GitProvider",
      metadata: {},
      spec: {
        type: {
          oneofKind: `github`,
          github: {
            clientID: "",

            clientSecret: {
              type: {
                oneofKind: "fromSecret",
                fromSecret: "",
              },
            },
          } as WsPB.GitProvider_Spec_Github,
        },
      },
      status: {},
    }),
  );

  const updateReq = () => {
    const clone = cloneResource(req) as WsPB.GitProvider;
    setReq(clone);
  };

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
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

      toast.success("GitProvider created");
      navigate(`${getPathSpace(ctx.space.data!)}/gitproviders`);
    },
    onError: onError,
  });

  let qrySecret = useQuery({
    queryKey: ["workspace/listSecret/"],
    queryFn: () => {
      const { response } = client.listSecret(
        WsPB.ListSecretOptions.create({
          spaceRef: getResourceRef(ctx.space.data!),
          common: {
            itemsPerPage: 500,
          },
        }),
      );
      return response;
    },
  });

  if (!qrySecret.isSuccess) {
    return <></>;
  }

  return (
    <div>
      <div className="w-full">
        <MetadataEdit
          metadata={req.metadata!}
          onUpdate={(itm) => {
            req.metadata = itm;
            updateReq();
          }}
          parentName={ctx.space.data.metadata?.name}
        />

        <Tabs className="mt-12" defaultValue="github">
          <Tabs.List className="mb-2">
            <Tabs.Tab
              value="github"
              onClick={() => {
                req.spec!.type = {
                  oneofKind: "github",
                  github: {
                    clientID: "",
                    clientSecret: {
                      type: {
                        oneofKind: "fromSecret",
                        fromSecret: "",
                      },
                    },
                    scopes: [],
                  },
                };
                updateReq();
              }}
            >
              Github
            </Tabs.Tab>
            <Tabs.Tab
              value="gitlab"
              onClick={() => {
                req.spec!.type = {
                  oneofKind: "gitlab",
                  gitlab: {
                    clientID: "",
                    clientSecret: {
                      type: {
                        oneofKind: "fromSecret",
                        fromSecret: "",
                      },
                    },
                    scopes: [],
                  },
                };
                updateReq();
              }}
            >
              Gitlab
            </Tabs.Tab>
            <Tabs.Tab
              value="oauth2"
              onClick={() => {
                req.spec!.type = {
                  oneofKind: "oauth2",
                  oauth2: {
                    clientID: "",
                    authURL: "",
                    tokenURL: "",
                    scopes: [],
                    clientSecret: {
                      type: {
                        oneofKind: "fromSecret",
                        fromSecret: "",
                      },
                    },
                  },
                };
                updateReq();
              }}
            >
              Generic OAuth2 Provider
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="github">
            {req.spec!.type.oneofKind === `github` && (
              <Group grow>
                <Field
                  val={req.spec!.type.github.clientID}
                  label={`Client ID`}
                  placeholder="abcdefg123456"
                  isRequired
                  onChange={(v) => {
                    let f = req.spec!.type as {
                      oneofKind: "github";
                      github: WsPB.GitProvider_Spec_Github;
                    };

                    f.github.clientID = v as string;
                    updateReq();
                  }}
                />

                {req.spec!.type.github.clientSecret?.type.oneofKind ===
                  "fromSecret" && (
                  <>
                    {req.spec!.type.github.clientSecret.type.oneofKind ===
                      "fromSecret" && (
                      <Select
                        label="Client Secret"
                        required
                        data={qrySecret.data!.items.map((x) => ({
                          value: x.metadata!.name,
                          label: x.metadata!.name.split(".").at(0) ?? "",
                        }))}
                        defaultValue={
                          req.spec!.type.github.clientSecret.type.fromSecret ??
                          ""
                        }
                        onChange={(val) => {
                          if (!val) {
                            return;
                          }
                          let f = req.spec!.type as {
                            oneofKind: "github";
                            github: WsPB.GitProvider_Spec_Github;
                          };

                          let g = f.github.clientSecret?.type as {
                            oneofKind: "fromSecret";
                            fromSecret: string;
                          };
                          g.fromSecret = val;
                          updateReq();
                        }}
                      />
                    )}
                  </>
                )}
              </Group>
            )}
          </Tabs.Panel>
          <Tabs.Panel value="gitlab">
            {req.spec!.type.oneofKind === `gitlab` && (
              <Group grow>
                <Field
                  val={req.spec!.type.gitlab.clientID}
                  label={`Client ID`}
                  placeholder="abcdefg123456"
                  isRequired
                  onChange={(v) => {
                    let f = req.spec!.type as {
                      oneofKind: "gitlab";
                      gitlab: WsPB.GitProvider_Spec_Gitlab;
                    };

                    f.gitlab.clientID = v as string;
                    updateReq();
                  }}
                />

                {req.spec!.type.gitlab.clientSecret?.type.oneofKind ===
                  "fromSecret" && (
                  <>
                    {req.spec!.type.gitlab.clientSecret.type.oneofKind ===
                      "fromSecret" && (
                      <Select
                        label="Client Secret"
                        required
                        data={qrySecret.data!.items.map((x) => ({
                          value: x.metadata!.name,
                          label: x.metadata!.name.split(".").at(0) ?? "",
                        }))}
                        defaultValue={
                          req.spec!.type.gitlab.clientSecret.type.fromSecret ??
                          ""
                        }
                        onChange={(val) => {
                          if (!val) {
                            return;
                          }
                          let f = req.spec!.type as {
                            oneofKind: "gitlab";
                            gitlab: WsPB.GitProvider_Spec_Gitlab;
                          };

                          let g = f.gitlab.clientSecret?.type as {
                            oneofKind: "fromSecret";
                            fromSecret: string;
                          };
                          g.fromSecret = val;
                          updateReq();
                        }}
                      />
                    )}
                  </>
                )}
              </Group>
            )}
          </Tabs.Panel>
          <Tabs.Panel value="oauth2">
            {req.spec!.type.oneofKind === `oauth2` && (
              <Group grow>
                <Field
                  val={req.spec!.type.oauth2.clientID}
                  label={`Client ID`}
                  placeholder="abcdefg123456"
                  isRequired
                  onChange={(v) => {
                    let f = req.spec!.type as {
                      oneofKind: "oauth2";
                      oauth2: WsPB.GitProvider_Spec_OAuth2;
                    };

                    f.oauth2.clientID = v as string;
                    updateReq();
                  }}
                />

                {req.spec!.type.oauth2.clientSecret?.type.oneofKind ===
                  "fromSecret" && (
                  <>
                    {req.spec!.type.oauth2.clientSecret.type.oneofKind ===
                      "fromSecret" && (
                      <Select
                        label="Client Secret"
                        required
                        data={qrySecret.data!.items.map((x) => ({
                          value: x.metadata!.name,
                          label: x.metadata!.name.split(".").at(0) ?? "",
                        }))}
                        defaultValue={
                          req.spec!.type.oauth2.clientSecret.type.fromSecret ??
                          ""
                        }
                        onChange={(val) => {
                          if (!val) {
                            return;
                          }
                          let f = req.spec!.type as {
                            oneofKind: "oauth2";
                            oauth2: WsPB.GitProvider_Spec_OAuth2;
                          };

                          let g = f.oauth2.clientSecret?.type as {
                            oneofKind: "fromSecret";
                            fromSecret: string;
                          };
                          g.fromSecret = val;
                          updateReq();
                        }}
                      />
                    )}
                  </>
                )}

                <Field
                  val={req.spec!.type.oauth2.authURL}
                  label={`Auth URL`}
                  isRequired
                  placeholder="https://example.com/auth"
                  onChange={(v) => {
                    let f = req.spec!.type as {
                      oneofKind: "oauth2";
                      oauth2: WsPB.GitProvider_Spec_OAuth2;
                    };

                    f.oauth2.authURL = v as string;
                    updateReq();
                  }}
                />

                <Field
                  val={req.spec!.type.oauth2.tokenURL}
                  label={`Token URL`}
                  isRequired
                  placeholder="https://example.com/oauth/oauth20/token"
                  onChange={(v) => {
                    let f = req.spec!.type as {
                      oneofKind: "oauth2";
                      oauth2: WsPB.GitProvider_Spec_OAuth2;
                    };

                    f.oauth2.tokenURL = v as string;
                    updateReq();
                  }}
                />
              </Group>
            )}
          </Tabs.Panel>
        </Tabs>

        {/*
        <div className="mt-16">
          <ItemContainer title="">
            <EditItem
              title="GitHub"
              obj={req.spec!.type.oneofKind === `github` ? {} : undefined}
              onSet={() => {
                req.spec!.type = {
                  oneofKind: "github",
                  github: {
                    clientID: "",
                    clientSecret: {
                      type: {
                        oneofKind: "fromSecret",
                        fromSecret: "",
                      },
                    },
                    scopes: [],
                  },
                };
                updateReq();
              }}
              onUnset={() => {
                req.spec!.type = {
                  oneofKind: undefined,
                };
                updateReq();
              }}
            >
              {req.spec!.type.oneofKind === `github` && (
                <Group grow>
                  <Field
                    val={req.spec!.type.github.clientID}
                    label={`Client ID`}
                    placeholder="abcdefg123456"
                    isRequired
                    onChange={(v) => {
                      let f = req.spec!.type as {
                        oneofKind: "github";
                        github: WsPB.GitProvider_Spec_Github;
                      };

                      f.github.clientID = v as string;
                      updateReq();
                    }}
                  />

                  {req.spec!.type.github.clientSecret?.type.oneofKind ===
                    "fromSecret" && (
                    <>
                      {req.spec!.type.github.clientSecret.type.oneofKind ===
                        "fromSecret" && (
                        <Select
                          label="Client Secret"
                          required
                          data={qrySecret.data!.items.map((x) => ({
                            value: x.metadata!.name,
                            label: x.metadata!.name.split(".").at(0) ?? "",
                          }))}
                          defaultValue={
                            req.spec!.type.github.clientSecret.type
                              .fromSecret ?? ""
                          }
                          onChange={(val) => {
                            if (!val) {
                              return;
                            }
                            let f = req.spec!.type as {
                              oneofKind: "github";
                              github: WsPB.GitProvider_Spec_Github;
                            };

                            let g = f.github.clientSecret?.type as {
                              oneofKind: "fromSecret";
                              fromSecret: string;
                            };
                            g.fromSecret = val;
                            updateReq();
                          }}
                        />
                      )}
                    </>
                  )}
                </Group>
              )}
            </EditItem>
            <Divider>OR</Divider>
            <EditItem
              title="Gitlab"
              obj={req.spec!.type.oneofKind === `gitlab` ? {} : undefined}
              onSet={() => {
                req.spec!.type = {
                  oneofKind: "gitlab",
                  gitlab: {
                    clientID: "",
                    clientSecret: {
                      type: {
                        oneofKind: "fromSecret",
                        fromSecret: "",
                      },
                    },
                    scopes: [],
                  },
                };
                updateReq();
              }}
              onUnset={() => {
                req.spec!.type = {
                  oneofKind: undefined,
                };
                updateReq();
              }}
            >
              {req.spec!.type.oneofKind === `gitlab` && (
                <Group grow>
                  <Field
                    val={req.spec!.type.gitlab.clientID}
                    label={`Client ID`}
                    placeholder="abcdefg123456"
                    isRequired
                    onChange={(v) => {
                      let f = req.spec!.type as {
                        oneofKind: "gitlab";
                        gitlab: WsPB.GitProvider_Spec_Gitlab;
                      };

                      f.gitlab.clientID = v as string;
                      updateReq();
                    }}
                  />

                  {req.spec!.type.gitlab.clientSecret?.type.oneofKind ===
                    "fromSecret" && (
                    <>
                      {req.spec!.type.gitlab.clientSecret.type.oneofKind ===
                        "fromSecret" && (
                        <Select
                          label="Client Secret"
                          required
                          data={qrySecret.data!.items.map((x) => ({
                            value: x.metadata!.name,
                            label: x.metadata!.name.split(".").at(0) ?? "",
                          }))}
                          defaultValue={
                            req.spec!.type.gitlab.clientSecret.type
                              .fromSecret ?? ""
                          }
                          onChange={(val) => {
                            if (!val) {
                              return;
                            }
                            let f = req.spec!.type as {
                              oneofKind: "gitlab";
                              gitlab: WsPB.GitProvider_Spec_Gitlab;
                            };

                            let g = f.gitlab.clientSecret?.type as {
                              oneofKind: "fromSecret";
                              fromSecret: string;
                            };
                            g.fromSecret = val;
                            updateReq();
                          }}
                        />
                      )}
                    </>
                  )}
                </Group>
              )}
            </EditItem>

            <Divider>OR</Divider>

            <EditItem
              title="Generic OAuth2 Provider"
              obj={req.spec!.type.oneofKind === `oauth2` ? {} : undefined}
              onSet={() => {
                req.spec!.type = {
                  oneofKind: "oauth2",
                  oauth2: {
                    clientID: "",
                    authURL: "",
                    tokenURL: "",
                    scopes: [],
                    clientSecret: {
                      type: {
                        oneofKind: "fromSecret",
                        fromSecret: "",
                      },
                    },
                  },
                };
                updateReq();
              }}
              onUnset={() => {
                req.spec!.type = {
                  oneofKind: undefined,
                };
                updateReq();
              }}
            >
              {req.spec!.type.oneofKind === `oauth2` && (
                <Group grow>
                  <Field
                    val={req.spec!.type.oauth2.clientID}
                    label={`Client ID`}
                    placeholder="abcdefg123456"
                    isRequired
                    onChange={(v) => {
                      let f = req.spec!.type as {
                        oneofKind: "oauth2";
                        oauth2: WsPB.GitProvider_Spec_OAuth2;
                      };

                      f.oauth2.clientID = v as string;
                      updateReq();
                    }}
                  />

                  {req.spec!.type.oauth2.clientSecret?.type.oneofKind ===
                    "fromSecret" && (
                    <>
                      {req.spec!.type.oauth2.clientSecret.type.oneofKind ===
                        "fromSecret" && (
                        <Select
                          label="Client Secret"
                          required
                          data={qrySecret.data!.items.map((x) => ({
                            value: x.metadata!.name,
                            label: x.metadata!.name.split(".").at(0) ?? "",
                          }))}
                          defaultValue={
                            req.spec!.type.oauth2.clientSecret.type
                              .fromSecret ?? ""
                          }
                          onChange={(val) => {
                            if (!val) {
                              return;
                            }
                            let f = req.spec!.type as {
                              oneofKind: "oauth2";
                              oauth2: WsPB.GitProvider_Spec_OAuth2;
                            };

                            let g = f.oauth2.clientSecret?.type as {
                              oneofKind: "fromSecret";
                              fromSecret: string;
                            };
                            g.fromSecret = val;
                            updateReq();
                          }}
                        />
                      )}
                    </>
                  )}

                  <Field
                    val={req.spec!.type.oauth2.authURL}
                    label={`Auth URL`}
                    placeholder="https://example.com/auth"
                    onChange={(v) => {
                      let f = req.spec!.type as {
                        oneofKind: "oauth2";
                        oauth2: WsPB.GitProvider_Spec_OAuth2;
                      };

                      f.oauth2.authURL = v as string;
                      updateReq();
                    }}
                  />

                  <Field
                    val={req.spec!.type.oauth2.tokenURL}
                    label={`Token URL`}
                    placeholder="https://example.com/oauth/oauth20/token"
                    onChange={(v) => {
                      let f = req.spec!.type as {
                        oneofKind: "oauth2";
                        oauth2: WsPB.GitProvider_Spec_OAuth2;
                      };

                      f.oauth2.tokenURL = v as string;
                      updateReq();
                    }}
                  />
                </Group>
              )}
            </EditItem>
          </ItemContainer>
        </div> 
        */}
      </div>
      <div className="flex items-center justify-end mt-12">
        <Button
          variant="outline"
          size="lg"
          onClick={() => {
            navigate(-1);
          }}
        >
          Cancel
        </Button>
        <Button
          size="lg"
          className="ml-2"
          onClick={() => {
            mutation.mutate();
          }}
        >
          Create
        </Button>
      </div>
    </div>
  );
};

export default CreateGitProvider;
