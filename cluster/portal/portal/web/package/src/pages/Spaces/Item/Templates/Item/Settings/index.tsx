import ConfirmAction from "@/components/ConfirmAction";
import Facts, { Fact } from "@/components/Facts";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import SpecEditor from "@/components/SpecEditor";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getPathSpace, invalidateTemplate, invalidateTemplates } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Button, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { DeleteOptions } from "@octelium/apis/main/metav1";
import { IconAlertTriangle, IconTrash } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const SettingsForm = (props: { data: WsPB.Template; space: WsPB.Space }) => {
  const { data, space } = props;
  const client = getClientWorkspace();
  const navigate = useNavigate();

  const [req, setReq] = React.useState(() => WsPB.Template.clone(data));

  const mutationUpdate = useMutation({
    mutationFn: async () => {
      const { response } = await client.updateTemplate(req!);
      return response;
    },
    onSuccess: (response) => {
      invalidateTemplate(response);
      toast.success("Template updated");
    },
    onError,
  });

  const mutationDelete = useMutation({
    mutationFn: async () => {
      await client.deleteTemplate(
        DeleteOptions.create({ uid: data!.metadata!.uid }),
      );
    },
    onSuccess: () => {
      invalidateTemplates();
      toast.success("Template deleted");
      navigate(`${getPathSpace(space!)}/templates`);
    },
    onError,
  });


  return (
    <Stack gap="lg">
      <Panel>
        <PanelHeader
          title="Configuration"
          description="Changes apply to Workspaces created from this Template afterwards; existing Workspaces keep their own spec."
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
          <Button
            variant="default"
            onClick={() => setReq(WsPB.Template.clone(data))}
          >
            Reset
          </Button>
          <Button
            loading={mutationUpdate.isPending}
            onClick={() => mutationUpdate.mutate()}
          >
            Save changes
          </Button>
        </PanelFooter>
      </Panel>

      <Panel className="border-rose-200">
        <PanelHeader
          icon={<IconAlertTriangle size={16} />}
          title="Danger zone"
          description="Deleting a Template does not delete Workspaces already created from it."
        />
        <PanelBody>
          <ConfirmAction
            triggerLabel="Delete Template"
            triggerIcon={<IconTrash size={14} />}
            size="sm"
            title="Delete this Template?"
            confirmLabel="Delete Template"
            description="New Workspaces can no longer be created from it. This cannot be undone."
            details={
              <Facts>
                <Fact label="Name">
                  <span className="font-mono">{getShortName(data)}</span>
                </Fact>
                <Fact label="Space">
                  <span className="font-mono">{getShortName(space)}</span>
                </Fact>
              </Facts>
            }
            loading={mutationDelete.isPending}
            onConfirm={() => mutationDelete.mutate()}
          />
        </PanelBody>
      </Panel>
    </Stack>
  );
};

const Page = () => {
  const ctx = useContextSpace();
  const data = ctx.template.data;
  const space = ctx.space.data;

  if (!data || !space) return null;

  return (
    <SettingsForm
      key={`${data.metadata!.uid}:${data.metadata?.updatedAt?.seconds ?? 0}`}
      data={data}
      space={space}
    />
  );
};

export default Page;
