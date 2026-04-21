import {
  Membership_Spec_Role,
  Space_Status_Type,
} from "@/apis/cordiumv1/cordiumv1";
import { DeleteOptions } from "@/apis/metav1/metav1";
import { getClientWorkspace } from "@/utils/client";
import { isMemberAdmin } from "@/utils/pb";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Outlet, useLocation, useNavigate } from "react-router-dom";

import DeleteResource from "@/components/DeleteResource";
import PageWrap from "@/components/PageWrap";
import { onError } from "@/utils";
import { Button, Tabs } from "@mantine/core";
import { match } from "ts-pattern";
import { useContextSpace } from "../utils";

const Page = () => {
  const client = getClientWorkspace();
  const navigate = useNavigate();

  const ctx = useContextSpace();

  const data = ctx.space.data;

  const queryClient = useQueryClient();

  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteSpace(
        DeleteOptions.create({
          uid: data?.metadata?.uid,
        }),
      );

      return response;
    },
    onSuccess: (data) => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listSpace", 0],
      });
      navigate(`/spaces`);
    },
    onError: onError,
  });

  if (!ctx.space.isSuccess || !data) {
    return <></>;
  }

  const isAdmin =
    ctx.membership.isSuccess && isMemberAdmin(ctx.membership.data);

  const isOwner =
    ctx.membership.isSuccess &&
    ctx.membership.data.spec!.role == Membership_Spec_Role.OWNER;

  const isOrg = data?.status?.type == Space_Status_Type.ORGANIZATION;
  const isPersonal = data?.status?.type == Space_Status_Type.USER;

  const loc = useLocation();

  return (
    <PageWrap qry={ctx.space}>
      <div>
        <Tabs
          defaultValue="main"
          className="font-bold text-xl"
          value={match(
            loc.pathname
              .replace(
                /(\/templates|secrets|gitproviders|memberships)\/.*$/,
                "$1",
              )
              .split("/")
              .reverse()
              .at(0),
          )
            .with("edit", (v) => v)
            .with("actions", (v) => v)
            .with("secrets", (v) => v)
            .with("templates", (v) => v)
            .with("workspaces", (v) => v)
            .with("gitproviders", (v) => v)
            .otherwise(() => "main")}
        >
          <Tabs.List className="mb-2">
            <Tabs.Tab
              value="main"
              onClick={() => {
                navigate(`./`);
              }}
            >
              Main
            </Tabs.Tab>
            <Tabs.Tab
              value="edit"
              onClick={() => {
                navigate(`./edit`);
              }}
            >
              Config
            </Tabs.Tab>

            <Tabs.Tab
              value="templates"
              onClick={() => {
                navigate(`./templates`);
              }}
            >
              Templates
            </Tabs.Tab>
            <Tabs.Tab
              value="secrets"
              onClick={() => {
                navigate(`./secrets`);
              }}
            >
              Secrets
            </Tabs.Tab>

            <Tabs.Tab
              value="workspaces"
              onClick={() => {
                navigate(`./workspaces`);
              }}
            >
              Your Workspaces
            </Tabs.Tab>
            <Tabs.Tab
              value="gitproviders"
              onClick={() => {
                navigate(`./gitproviders`);
              }}
            >
              Git Providers
            </Tabs.Tab>
            {isOrg && (
              <Tabs.Tab
                value="members"
                onClick={() => {
                  navigate(`./memberships`);
                }}
              >
                Members
              </Tabs.Tab>
            )}
            <Tabs.Tab
              value="actions"
              onClick={() => {
                navigate("./actions");
              }}
            >
              Actions
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="action" className="mt-2">
            <div>
              <div className="flex items-center">
                <div className="flex items-center flex-1">
                  {isOrg && isAdmin && (
                    <Button
                      size="small"
                      variant="outline"
                      onClick={() => {
                        navigate(
                          `/memberships/create?spaceUID=${data.metadata?.uid}`,
                        );
                      }}
                    >
                      Add a Member
                    </Button>
                  )}
                </div>
                <div className="flex items-center flex-none">
                  {ctx.membership.isSuccess && isOwner && isOrg && (
                    <DeleteResource
                      onDelete={() => {
                        mutationDelete.mutate();
                      }}
                    />
                  )}
                </div>
              </div>
            </div>
          </Tabs.Panel>
        </Tabs>
      </div>
      <div>
        <div>
          <Outlet context={[data]} />

          <div className="w-full"></div>
          <div className="mb-4"></div>

          <div className="mb-4"></div>
        </div>
      </div>
    </PageWrap>
  );
};

export default Page;
