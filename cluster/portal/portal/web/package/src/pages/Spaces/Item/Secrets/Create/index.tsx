import Meta from "@/components/Meta";
import MetadataEdit from "@/components/MetadataEdit";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import SecretValueInput from "@/components/SecretValueInput";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getPathSpace, invalidateSecrets } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Button, Stack } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { IconKey, IconLock } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateSecret = () => {
  const ctx = useContextSpace();
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const space = ctx.space.data;

  const [req, setReq] = React.useState(
    WsPB.Secret.create({
      apiVersion: "cordium/v1",
      kind: "Secret",
      metadata: {},
      spec: {},
      status: {},
      data: { type: { oneofKind: "value", value: "" } },
    }),
  );

  const value = req.data?.type.oneofKind === "value" ? req.data.type.value : "";

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = WsPB.Secret.clone(req);
      payload.status!.spaceRef = getResourceRef(space!);
      const { response } = await client.createSecret(payload);
      return response;
    },
    onSuccess: () => {
      invalidateSecrets();
      toast.success("Secret created");
      navigate(`${getPathSpace(space!)}/secrets`);
    },
    onError,
  });

  if (!space) return null;

  return (
    <>
      <Meta title="New Secret" />
      <PageHeader
        title="New Secret"
        crumbs={[
          { label: "Spaces", to: "/spaces" },
          { label: getShortName(space), to: getPathSpace(space) },
          { label: "Secrets", to: `${getPathSpace(space)}/secrets` },
          { label: "New" },
        ]}
        description={`Available to every Template and Workspace in ${getShortName(space)}.`}
      />

      <div className="max-w-3xl">
        <Stack gap="lg">
          <Panel>
            <PanelHeader
              icon={<IconKey size={16} />}
              title="Identity"
              description="Referenced by this name from specs, e.g. as an env var source."
            />
            <PanelBody>
              <MetadataEdit
                metadata={req.metadata!}
                parentName={space.metadata?.name}
                onChange={(md) => {
                  const next = WsPB.Secret.clone(req);
                  next.metadata = md;
                  setReq(next);
                }}
              />
            </PanelBody>
          </Panel>

          <Panel>
            <PanelHeader
              icon={<IconLock size={16} />}
              title="Value"
              description="Write-only. You can replace it later but never read it back."
            />
            <PanelBody>
              <SecretValueInput
                value={value}
                onChange={(v) => {
                  const next = WsPB.Secret.clone(req);
                  next.data!.type = { oneofKind: "value", value: v };
                  setReq(next);
                }}
              />
            </PanelBody>
            <PanelFooter>
              <Button variant="default" onClick={() => navigate(-1)}>
                Cancel
              </Button>
              <Button
                loading={mutation.isPending}
                disabled={!value || !req.metadata?.name}
                onClick={() => mutation.mutate()}
              >
                Create Secret
              </Button>
            </PanelFooter>
          </Panel>
        </Stack>
      </div>
    </>
  );
};

export default CreateSecret;
