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
import {
  IconBolt,
  IconDeviceDesktop,
  IconGitBranch,
  IconLayoutGrid,
  IconLock,
  IconSettings,
  IconTemplate,
  IconUserPlus,
  IconUsers,
} from "@tabler/icons-react";
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
        DeleteOptions.create({ uid: data?.metadata?.uid }),
      );
      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({ queryKey: ["workspace/listSpace", 0] });
      navigate("/spaces");
    },
    onError,
  });

  if (!ctx.space.isSuccess || !data) return null;

  const isAdmin =
    ctx.membership.isSuccess && isMemberAdmin(ctx.membership.data);
  const isOwner =
    ctx.membership.isSuccess &&
    ctx.membership.data.spec!.role === Membership_Spec_Role.OWNER;
  const isOrg = data?.status?.type === Space_Status_Type.ORGANIZATION;

  const loc = useLocation();

  const activeTab = match(
    loc.pathname
      .replace(/(\/templates|secrets|gitproviders|memberships)\/.*$/, "$1")
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
    .with("memberships", () => "members")
    .otherwise(() => "main");

  const tabItems: Array<{
    value: string;
    label: string;
    icon: React.ReactNode;
    path: string;
    hidden?: boolean;
  }> = [
    {
      value: "main",
      label: "Overview",
      icon: <IconLayoutGrid size={14} />,
      path: "./",
    },
    {
      value: "edit",
      label: "Config",
      icon: <IconSettings size={14} />,
      path: "./edit",
    },
    {
      value: "templates",
      label: "Templates",
      icon: <IconTemplate size={14} />,
      path: "./templates",
    },
    {
      value: "secrets",
      label: "Secrets",
      icon: <IconLock size={14} />,
      path: "./secrets",
    },
    {
      value: "workspaces",
      label: "Workspaces",
      icon: <IconDeviceDesktop size={14} />,
      path: "./workspaces",
    },
    {
      value: "gitproviders",
      label: "Git providers",
      icon: <IconGitBranch size={14} />,
      path: "./gitproviders",
    },
    {
      value: "members",
      label: "Members",
      icon: <IconUsers size={14} />,
      path: "./memberships",
      hidden: !isOrg,
    },
    {
      value: "actions",
      label: "Actions",
      icon: <IconBolt size={14} />,
      path: "./actions",
    },
  ];

  return (
    <PageWrap qry={ctx.space}>
      <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
        <Tabs value={activeTab}>
          <Tabs.List
            style={{
              background: "white",
              borderRadius: "10px 10px 0 0",
              border: "1px solid #e2e8f0",
              borderBottom: "none",
              padding: "0 8px",
            }}
          >
            {tabItems
              .filter((t) => !t.hidden)
              .map((t) => (
                <Tabs.Tab
                  key={t.value}
                  value={t.value}
                  leftSection={t.icon}
                  onClick={() => navigate(t.path)}
                  style={{ fontSize: 13 }}
                >
                  {t.label}
                </Tabs.Tab>
              ))}

            {(isAdmin || isOwner) && (
              <div
                style={{
                  marginLeft: "auto",
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  paddingRight: 4,
                }}
              >
                {isOrg && isAdmin && (
                  <Button
                    size="xs"
                    variant="light"
                    leftSection={<IconUserPlus size={13} />}
                    onClick={() =>
                      navigate(
                        `/memberships/create?spaceUID=${data.metadata?.uid}`,
                      )
                    }
                  >
                    Add member
                  </Button>
                )}
                {isOwner && isOrg && (
                  <DeleteResource onDelete={() => mutationDelete.mutate()} />
                )}
              </div>
            )}
          </Tabs.List>

          <div
            style={{
              background: "white",
              border: "1px solid #e2e8f0",
              borderTop: "none",
              borderRadius: "0 0 10px 10px",
              padding: "20px",
              minHeight: 200,
            }}
          >
            <Outlet context={[data]} />
          </div>
        </Tabs>
      </div>
    </PageWrap>
  );
};

export default Page;
