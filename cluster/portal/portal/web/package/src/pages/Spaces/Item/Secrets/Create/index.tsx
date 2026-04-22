import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import MetadataEdit from "@/components/MetadataEdit";
import PageWrap from "@/components/PageWrap";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef } from "@/utils/pb";
import {
  ActionIcon,
  Button,
  Divider,
  Group,
  PasswordInput,
  SegmentedControl,
  Stack,
  Text,
  Textarea,
  ThemeIcon,
} from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff, FileText, KeyRound, Upload } from "lucide-react";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

type InputMode = "value" | "file";

const CreateSecret = () => {
  const ctx = useContextSpace();

  const [req, setReq] = React.useState(
    WsPB.Secret.create({
      apiVersion: "workspace/v1",
      kind: "Secret",
      metadata: {},
      spec: {},
      status: {},
      data: { type: { oneofKind: "value", value: "" } },
    }),
  );

  const [inputMode, setInputMode] = React.useState<InputMode>("value");
  const [revealed, setRevealed] = React.useState(false);
  const [fileName, setFileName] = React.useState<string | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  if (!ctx.space.isSuccess) return null;
  const data = ctx.space.data;

  const mutation = useMutation({
    mutationFn: async () => {
      req.status!.spaceRef = getResourceRef(data!);
      const { response } = await client.createSecret(req);
      return response;
    },
    onSuccess: () => {
      queryClient.refetchQueries({
        queryKey: ["workspace/listSecret", data?.metadata?.uid, 0],
      });
      toast.success("Secret created");
      navigate(getPathSpace(data));
    },
    onError,
  });

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    const reader = new FileReader();
    reader.onload = (ev) => {
      const text = ev.target?.result as string;
      req.data!.type = { oneofKind: "value", value: text };
      setReq(WsPB.Secret.clone(req));
    };
    reader.readAsText(file);
  };

  const currentValue =
    req.data?.type.oneofKind === "value" ? req.data.type.value : "";

  return (
    <PageWrap qry={ctx.space} title="Create a Secret">
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
              <KeyRound size={13} />
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
              setReq(WsPB.Secret.clone(req));
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
              <FileText size={13} />
            </ThemeIcon>
            <Text
              size="xs"
              fw={600}
              tt="uppercase"
              style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
            >
              Secret value
            </Text>
          </Group>

          <Stack gap="md">
            <SegmentedControl
              size="xs"
              value={inputMode}
              onChange={(v) => {
                setInputMode(v as InputMode);
                setFileName(null);
                req.data!.type = { oneofKind: "value", value: "" };
                setReq(WsPB.Secret.clone(req));
              }}
              data={[
                { label: "Enter value", value: "value" },
                { label: "Upload from file", value: "file" },
              ]}
              style={{ width: "fit-content" }}
            />

            {inputMode === "value" && (
              <div style={{ position: "relative" }}>
                {revealed ? (
                  <Textarea
                    placeholder="Enter secret value…"
                    required
                    autosize
                    minRows={3}
                    maxRows={8}
                    value={currentValue}
                    onChange={(e) => {
                      req.data!.type = {
                        oneofKind: "value",
                        value: e.currentTarget.value,
                      };
                      setReq(WsPB.Secret.clone(req));
                    }}
                    rightSection={
                      <ActionIcon
                        variant="subtle"
                        color="gray"
                        onClick={() => setRevealed(false)}
                        style={{ alignSelf: "flex-start", marginTop: 6 }}
                      >
                        <EyeOff size={14} />
                      </ActionIcon>
                    }
                    rightSectionProps={{
                      style: { alignItems: "flex-start", paddingTop: 6 },
                    }}
                  />
                ) : (
                  <PasswordInput
                    placeholder="Enter secret value…"
                    required
                    value={currentValue}
                    onChange={(e) => {
                      req.data!.type = {
                        oneofKind: "value",
                        value: e.currentTarget.value,
                      };
                      setReq(WsPB.Secret.clone(req));
                    }}
                    visibilityToggleIcon={({ reveal }) =>
                      reveal ? <EyeOff size={14} /> : <Eye size={14} />
                    }
                    onVisibilityChange={setRevealed}
                  />
                )}
                <Text size="xs" c="dimmed" mt={6}>
                  Value is masked by default. Click the eye icon to reveal.
                </Text>
              </div>
            )}

            {inputMode === "file" && (
              <Stack gap="xs">
                <input
                  ref={fileInputRef}
                  type="file"
                  style={{ display: "none" }}
                  onChange={handleFileChange}
                />
                <Button
                  variant="default"
                  size="sm"
                  leftSection={<Upload size={14} />}
                  onClick={() => fileInputRef.current?.click()}
                  style={{ width: "fit-content" }}
                >
                  {fileName ? "Replace file" : "Choose file"}
                </Button>
                {fileName && (
                  <Group gap="xs">
                    <FileText size={13} style={{ color: "#64748b" }} />
                    <Text size="xs" c="dimmed">
                      {fileName}
                    </Text>
                    {currentValue && (
                      <Text size="xs" c="teal.6" fw={500}>
                        ✓ Loaded ({currentValue.length} bytes)
                      </Text>
                    )}
                  </Group>
                )}
                <Text size="xs" c="dimmed">
                  File contents will be stored as the secret value.
                </Text>
              </Stack>
            )}
          </Stack>
        </div>

        <Divider />

        <Group justify="flex-end" gap="sm">
          <Button variant="default" size="sm" onClick={() => navigate(-1)}>
            Cancel
          </Button>
          <Button
            size="sm"
            loading={mutation.isPending}
            disabled={!currentValue}
            onClick={() => mutation.mutate()}
          >
            Create secret
          </Button>
        </Group>
      </Stack>
    </PageWrap>
  );
};

export default CreateSecret;
