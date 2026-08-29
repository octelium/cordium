import { ActionIcon, Tooltip } from "@mantine/core";
import { useClipboard } from "@mantine/hooks";
import { IconCheck, IconCopy } from "@tabler/icons-react";
import { twMerge } from "tailwind-merge";
import truncate from "truncate-utf8-bytes";

const CopyText = (props: {
  value?: string;
  truncate?: number;
  mono?: boolean;
  className?: string;
}) => {
  const clipboard = useClipboard({ timeout: 1500 });
  const { value } = props;

  if (!value) return null;

  const display =
    props.truncate && props.truncate < value.length
      ? `${truncate(value, props.truncate)}…`
      : value;

  return (
    <span className={twMerge("inline-flex items-center gap-1", props.className)}>
      <span
        className={twMerge(
          "break-all",
          props.mono !== false && "font-mono text-[0.82em]",
        )}
      >
        {display}
      </span>
      <Tooltip label={clipboard.copied ? "Copied" : "Copy"}>
        <ActionIcon
          size="xs"
          variant="subtle"
          color={clipboard.copied ? "teal" : "gray"}
          aria-label="Copy to clipboard"
          onClick={() => clipboard.copy(value)}
        >
          {clipboard.copied ? <IconCheck size={12} /> : <IconCopy size={12} />}
        </ActionIcon>
      </Tooltip>
    </span>
  );
};

export default CopyText;
