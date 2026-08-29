import Meta from "@/components/Meta";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getPathSpace, invalidateMemberships } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import {
  Button,
  SegmentedControl,
  Select,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { ObjectReference } from "@octelium/apis/main/metav1";
import { IconUserPlus } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const Role = WsPB.CreateMembershipRequest_Role;

const AddMember = () => {
  const ctx = useContextSpace();
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const space = ctx.space.data;

  const [mode, setMode] = React.useState<"email" | "user">("email");
  const [identifier, setIdentifier] = React.useState("");
  const [role, setRole] = React.useState<WsPB.CreateMembershipRequest_Role>(
    Role.USER,
  );

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createMembership(
        WsPB.CreateMembershipRequest.create({
          role,
          spaceRef: getResourceRef(space!),
          userType:
            mode === "email"
              ? { oneofKind: "email", email: identifier }
              : {
                  oneofKind: "userRef",
                  userRef: ObjectReference.create({ name: identifier }),
                },
        }),
      );
      return response;
    },
    onSuccess: () => {
      invalidateMemberships();
      toast.success("Member added");
      navigate(`${getPathSpace(space!)}/memberships`);
    },
    onError,
  });

  if (!space) return null;

  return (
    <>
      <Meta title="Add member" />
      <PageHeader
        title="Add member"
        crumbs={[
          { label: "Spaces", to: "/spaces" },
          { label: getShortName(space), to: getPathSpace(space) },
          { label: "Members", to: `${getPathSpace(space)}/memberships` },
          { label: "Add" },
        ]}
        description={`Grant access to ${getShortName(space)}.`}
      />

      <div className="max-w-2xl">
        <Panel>
          <PanelHeader
            icon={<IconUserPlus size={16} />}
            title="New member"
            description="The user must already exist in the Cluster."
          />
          <PanelBody>
            <Stack gap="lg">
              <div>
                <Text size="sm" fw={700} mb={6}>
                  Identify by
                </Text>
                <SegmentedControl
                  value={mode}
                  onChange={(v) => {
                    setMode(v as "email" | "user");
                    setIdentifier("");
                  }}
                  data={[
                    { label: "Email", value: "email" },
                    { label: "Username", value: "user" },
                  ]}
                />
              </div>

              <TextInput
                label={mode === "email" ? "Email address" : "Username"}
                description={
                  mode === "email"
                    ? "Matched against the user's Cluster email."
                    : "The Cluster User name, e.g. jane."
                }
                placeholder={mode === "email" ? "jane@example.com" : "jane"}
                required
                value={identifier}
                onChange={(e) => setIdentifier(e.currentTarget.value)}
              />

              <Select
                label="Role"
                description="Admins manage Space resources; owners can also delete the Space."
                allowDeselect={false}
                data={[
                  {
                    value: Role[Role.USER],
                    label: "Member — can create Workspaces",
                  },
                  {
                    value: Role[Role.ADMIN],
                    label: "Admin — can manage Space resources",
                  },
                  { value: Role[Role.OWNER], label: "Owner — full control" },
                ]}
                value={Role[role]}
                onChange={(val) => {
                  if (!val) return;
                  setRole(
                    Role[
                      val as keyof typeof Role
                    ] as WsPB.CreateMembershipRequest_Role,
                  );
                }}
              />
            </Stack>
          </PanelBody>
          <PanelFooter>
            <Button variant="default" onClick={() => navigate(-1)}>
              Cancel
            </Button>
            <Button
              loading={mutation.isPending}
              disabled={!identifier}
              onClick={() => mutation.mutate()}
            >
              Add member
            </Button>
          </PanelFooter>
        </Panel>
      </div>
    </>
  );
};

export default AddMember;
