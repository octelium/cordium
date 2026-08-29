import Meta from "@/components/Meta";
import MetadataEdit from "@/components/MetadataEdit";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import { onError } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateSpace } from "@/utils/octelium";
import { Button, SegmentedControl, Stack, Text } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconStack2 } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";
import { getClientWorkspace } from "../../../utils/client";

const CreateSpace = () => {
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const user = useAppSelector((a) => a.settings.status?.user);

  const [req, setReq] = React.useState(
    WsPB.Space.create({
      apiVersion: "cordium/v1",
      kind: "Space",
      metadata: {},
      spec: {},
      status: { type: WsPB.Space_Status_Type.USER },
    }),
  );

  const isOrg = req.status?.type === WsPB.Space_Status_Type.ORGANIZATION;
  const parentName = isOrg ? "cordium" : user?.metadata?.name;

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createSpace(req);
      return response;
    },
    onSuccess: (data) => {
      invalidateSpace(data);
      toast.success("Space created");
      navigate(getPathSpace(data));
    },
    onError,
  });

  const setType = (type: WsPB.Space_Status_Type) => {
    const next = WsPB.Space.clone(req);
    next.status!.type = type;
    const short = next.metadata!.name.split(".").at(0) ?? "";
    const parent =
      type === WsPB.Space_Status_Type.ORGANIZATION
        ? "cordium"
        : user?.metadata?.name;
    next.metadata!.name = parent && short ? `${short}.${parent}` : short;
    setReq(next);
  };

  return (
    <>
      <Meta title="New Space" />
      <PageHeader
        title="New Space"
        crumbs={[{ label: "Spaces", to: "/spaces" }, { label: "New" }]}
        description="Spaces scope Templates, Secrets, Git providers and Workspaces."
      />

      <div className="max-w-3xl">
        <Panel>
          <PanelHeader
            icon={<IconStack2 size={16} />}
            title="Space details"
            description="You can change the display name later; the name is permanent."
          />
          <PanelBody>
            <Stack gap="lg">
              <div>
                <Text size="sm" fw={700} mb={2}>
                  Visibility
                </Text>
                <Text size="xs" c="dimmed" mb={8}>
                  Personal Spaces belong only to you. Organization Spaces can be
                  shared with other members and support Secrets and resource
                  limits.
                </Text>
                <SegmentedControl
                  value={isOrg ? "org" : "user"}
                  onChange={(v) =>
                    setType(
                      v === "org"
                        ? WsPB.Space_Status_Type.ORGANIZATION
                        : WsPB.Space_Status_Type.USER,
                    )
                  }
                  data={[
                    { label: "Personal", value: "user" },
                    { label: "Organization", value: "org" },
                  ]}
                />
              </div>

              <MetadataEdit
                metadata={req.metadata!}
                parentName={parentName}
                withDescription
                nameDescription="Unique within your account. Used in URLs and the CLI."
                onChange={(md) => {
                  const next = WsPB.Space.clone(req);
                  next.metadata = md;
                  setReq(next);
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
              disabled={!req.metadata?.name}
              onClick={() => mutation.mutate()}
            >
              Create Space
            </Button>
          </PanelFooter>
        </Panel>
      </div>
    </>
  );
};

export default CreateSpace;
