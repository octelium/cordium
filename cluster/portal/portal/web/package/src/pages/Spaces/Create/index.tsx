import MetadataEdit from "@/components/MetadataEdit";
import { onError } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { getPathSpace, invalidateSpace } from "@/utils/octelium";
import { Button, Divider, Group, Stack, Text, ThemeIcon } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { useMutation } from "@tanstack/react-query";
import { Layers } from "lucide-react";
import * as React from "react";
import { useNavigate } from "react-router-dom";
import { getClientWorkspace } from "../../../utils/client";

const CreateSpace = () => {
  const [req, setReq] = React.useState(
    WsPB.Space.create({
      metadata: {},
      spec: {},
      status: { type: WsPB.Space_Status_Type.USER },
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const user = useAppSelector((a) => a.settings.status?.user);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createSpace(req);
      return response;
    },
    onSuccess: (data) => {
      invalidateSpace(data);
      navigate(getPathSpace(data));
    },
    onError,
  });

  const parentName =
    req.status?.type === WsPB.Space_Status_Type.USER
      ? user?.metadata?.name
      : "cordium";

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
            <Layers size={13} />
          </ThemeIcon>
          <Text
            size="xs"
            fw={700}
            tt="uppercase"
            style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
          >
            Space details
          </Text>
        </Group>
        <MetadataEdit
          metadata={req.metadata!}
          onUpdate={(itm) => {
            req.metadata = itm;
            setReq(WsPB.Space.clone(req));
          }}
          parentName={parentName}
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
          Create space
        </Button>
      </Group>
    </Stack>
  );
};

export default CreateSpace;
