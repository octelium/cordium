import {
  Button,
  PasswordInput,
  SegmentedControl,
  Stack,
  Text,
  Textarea,
} from "@mantine/core";
import { IconFileText, IconUpload } from "@tabler/icons-react";
import * as React from "react";

type InputMode = "value" | "multiline" | "file";

const SecretValueInput = (props: {
  value: string;
  onChange: (value: string) => void;
}) => {
  const [mode, setMode] = React.useState<InputMode>("value");
  const [fileName, setFileName] = React.useState<string | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    const reader = new FileReader();
    reader.onload = (ev) => props.onChange((ev.target?.result as string) ?? "");
    reader.readAsText(file);
    e.target.value = "";
  };

  return (
    <Stack gap="md">
      <SegmentedControl
        size="xs"
        className="w-fit"
        value={mode}
        onChange={(v) => {
          setMode(v as InputMode);
          setFileName(null);
          props.onChange("");
        }}
        data={[
          { label: "Single line", value: "value" },
          { label: "Multi-line", value: "multiline" },
          { label: "From file", value: "file" },
        ]}
      />

      {mode === "value" && (
        <PasswordInput
          label="Value"
          description="Masked by default. Stored encrypted and never returned by the API."
          placeholder="Enter the secret value"
          required
          autoComplete="new-password"
          value={props.value}
          onChange={(e) => props.onChange(e.currentTarget.value)}
        />
      )}

      {mode === "multiline" && (
        <Textarea
          label="Value"
          description="For keys, certificates or JSON blobs. Stored verbatim."
          placeholder="-----BEGIN PRIVATE KEY-----"
          required
          autosize
          minRows={5}
          maxRows={14}
          value={props.value}
          onChange={(e) => props.onChange(e.currentTarget.value)}
        />
      )}

      {mode === "file" && (
        <Stack gap="xs">
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            onChange={handleFileChange}
          />
          <Button
            variant="default"
            size="sm"
            className="w-fit"
            leftSection={<IconUpload size={14} />}
            onClick={() => fileInputRef.current?.click()}
          >
            {fileName ? "Replace file" : "Choose file"}
          </Button>
          {fileName && (
            <div className="flex items-center gap-2">
              <IconFileText size={13} className="shrink-0 text-slate-400" />
              <Text size="xs" c="dimmed">
                {fileName}
              </Text>
              {props.value && (
                <Text size="xs" fw={600} c="teal.7">
                  {props.value.length} bytes loaded
                </Text>
              )}
            </div>
          )}
          <Text size="xs" c="dimmed">
            The file contents become the secret value. The file itself is not
            uploaded anywhere else.
          </Text>
        </Stack>
      )}
    </Stack>
  );
};

export default SecretValueInput;
