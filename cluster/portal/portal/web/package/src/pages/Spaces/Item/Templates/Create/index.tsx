import Meta from "@/components/Meta";
import MetadataEdit from "@/components/MetadataEdit";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import SpecEditor from "@/components/SpecEditor";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getPathSpace, getPathTemplate, invalidateTemplate } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Button, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconSettings2, IconTemplate } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateForm = (props: { space: WsPB.Space }) => {
  const { space } = props;
  const client = getClientWorkspace();
  const navigate = useNavigate();

  const [req, setReq] = React.useState(() =>
    WsPB.Template.create({
      apiVersion: "cordium/v1",
      kind: "Template",
      metadata: {},
      spec: {},
      status: { spaceRef: getResourceRef(space) },
    }),
  );

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createTemplate(req);
      return response;
    },
    onSuccess: (data) => {
      invalidateTemplate(data);
      toast.success("Template created");
      navigate(getPathTemplate(data));
    },
    onError,
  });

  return (
    <>
      <Meta title="New Template" />
      <PageHeader
        title="New Template"
        crumbs={[
          { label: "Spaces", to: "/spaces" },
          { label: getShortName(space), to: getPathSpace(space) },
          { label: "Templates", to: `${getPathSpace(space)}/templates` },
          { label: "New" },
        ]}
        description={`Blueprint for Workspaces in ${getShortName(space)}.`}
      />

      <Stack gap="lg">
        <Panel>
          <PanelHeader
            icon={<IconTemplate size={16} />}
            title="Identity"
            description="How this Template appears in the portal and the CLI."
          />
          <PanelBody>
            <MetadataEdit
              metadata={req.metadata!}
              parentName={space.metadata?.name}
              withDescription
              onChange={(md) => {
                const next = WsPB.Template.clone(req);
                next.metadata = md;
                setReq(next);
              }}
            />
          </PanelBody>
        </Panel>

        <Panel>
          <PanelHeader
            icon={<IconSettings2 size={16} />}
            title="Configuration"
            description="Defaults inherited by every Workspace created from this Template."
          />
          <PanelBody>
            <SpecEditor
              kind="Template"
              item={req}
              spaceRef={getResourceRef(space)}
              onChange={(next) => setReq(next as WsPB.Template)}
            />
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
              Create Template
            </Button>
          </PanelFooter>
        </Panel>
      </Stack>
    </>
  );
};

const CreateTemplate = () => {
  const ctx = useContextSpace();
  const space = ctx.space.data;

  if (!space) return null;

  return <CreateForm key={space.metadata!.uid} space={space} />;
};

export default CreateTemplate;
