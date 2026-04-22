import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import MetadataEdit from "@/components/MetadataEdit";
import PageWrap from "@/components/PageWrap";
import WorkspaceEdit from "@/components/WorkspaceEdit";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError, queryClient } from "@/utils";
import { getPathTemplate } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import { Button, Divider, Group, Stack, Text, ThemeIcon } from "@mantine/core";
import { useMutation } from "@tanstack/react-query";
import { LayoutTemplate, Settings2 } from "lucide-react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateTemplate = () => {
  const ctx = useContextSpace();

  const [req, setReq] = React.useState(
    WsPB.Template.create({
      apiVersion: "workspace/v1",
      kind: "Template",
      metadata: {},
      spec: {},
      status: {
        spaceRef: ctx.space.isSuccess
          ? getResourceRef(ctx.space.data)
          : undefined,
      },
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createTemplate(req);
      return response;
    },
    onSuccess: (data) => {
      navigate(getPathTemplate(data));
      queryClient.invalidateQueries({
        queryKey: ["workspace/listTemplate", data.status?.spaceRef?.uid, 0],
      });
      toast.success(`Template ${data.metadata?.name} created`);
    },
    onError,
  });

  if (!ctx.space.isSuccess) return null;

  const data = ctx.space.data;

  return (
    <PageWrap qry={ctx.space} title="Create a Template">
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
              <LayoutTemplate size={13} />
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
              setReq(WsPB.Template.clone(req));
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
              Configuration
            </Text>
          </Group>
          <WorkspaceEdit
            spaceRef={getResourceRef(data)}
            item={req}
            onUpdate={(itm) => {
              const item = itm as WsPB.Template;
              req.spec = item.spec;
              setReq(WsPB.Template.clone(req));
            }}
          />
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
            Create template
          </Button>
        </Group>
      </Stack>
    </PageWrap>
  );
};

export default CreateTemplate;
