import { ActionIcon, Button, Modal, Tooltip } from "@mantine/core";
import {
  IconCheck,
  IconCopy,
  IconFile,
  IconFileCode,
  IconFileSpreadsheet,
  IconFileText,
  IconFileZip,
  IconPhoto,
  IconSparkles,
  IconTrash,
} from "@tabler/icons-react";
import * as React from "react";
import { toast } from "react-hot-toast";
import { twMerge } from "tailwind-merge";
import { copyText, fileKind } from "./utils";

const fileIcons = {
  image: IconPhoto,
  spreadsheet: IconFileSpreadsheet,
  archive: IconFileZip,
  code: IconFileCode,
  text: IconFileText,
  file: IconFile,
};

export const FileIcon = (props: {
  mimeType: string;
  name?: string;
  size: number;
}) => {
  const Icon = fileIcons[fileKind(props.mimeType, props.name)];
  return <Icon size={props.size} />;
};

export const Kbd = (props: {
  children: React.ReactNode;
  className?: string;
}) => (
  <kbd
    className={twMerge(
      "inline-flex h-5 min-w-5 items-center justify-center rounded-md border border-line bg-surface px-1 font-sans text-[0.7rem]/4 font-semibold text-ink-muted",
      props.className,
    )}
  >
    {props.children}
  </kbd>
);

export const Shimmer = (props: {
  children: React.ReactNode;
  className?: string;
}) => (
  <span
    className={twMerge(
      "animate-shimmer bg-[linear-gradient(90deg,var(--app-ink-muted)_40%,var(--app-ink)_50%,var(--app-ink-muted)_60%)] bg-[length:250%_100%] bg-clip-text text-transparent motion-reduce:animate-none",
      props.className,
    )}
  >
    {props.children}
  </span>
);

const dotTones = {
  green: "bg-hue-emerald-solid",
  blue: "bg-hue-blue-solid",
  amber: "bg-hue-amber-solid",
  red: "bg-hue-rose-solid",
  gray: "bg-ink-subtle",
};

export type DotTone = keyof typeof dotTones;

export const StatusDot = (props: {
  tone: DotTone;
  pulse?: boolean;
  className?: string;
}) => (
  <span
    className={twMerge(
      "relative inline-flex h-2 w-2 shrink-0",
      props.className,
    )}
  >
    {props.pulse && (
      <span
        className={twMerge(
          "absolute inline-flex h-full w-full animate-ping rounded-full opacity-60 motion-reduce:animate-none",
          dotTones[props.tone],
        )}
      />
    )}
    <span
      className={twMerge(
        "relative inline-flex h-2 w-2 rounded-full",
        dotTones[props.tone],
      )}
    />
  </span>
);

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

export const Highlight = (props: { text: string; terms: string[] }) => {
  const parts = React.useMemo(() => {
    if (props.terms.length === 0) return [props.text];
    return props.text.split(
      new RegExp(`(${props.terms.map(escapeRegExp).join("|")})`, "gi"),
    );
  }, [props.text, props.terms]);

  return (
    <>
      {parts.map((part, idx) =>
        idx % 2 === 1 ? (
          <mark
            key={idx}
            className="rounded-[3px] bg-hue-amber-line px-px text-inherit"
          >
            {part}
          </mark>
        ) : (
          part
        ),
      )}
    </>
  );
};

export const CopyButton = (props: {
  value: () => string;
  label?: string;
  dark?: boolean;
}) => {
  const [copied, setCopied] = React.useState(false);
  const label = props.label ?? "Copy";

  React.useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = async () => setCopied(await copyText(props.value()));

  return (
    <Tooltip label={copied ? "Copied" : label}>
      {props.dark ? (
        <button
          type="button"
          aria-label={label}
          onClick={() => void copy()}
          className="flex h-[22px] w-[22px] cursor-pointer items-center justify-center rounded-md text-zinc-500 transition-colors duration-150 hover:bg-white/10 hover:text-zinc-200"
        >
          {copied ? (
            <IconCheck size={13} stroke={2.5} className="text-emerald-400" />
          ) : (
            <IconCopy size={13} />
          )}
        </button>
      ) : (
        <ActionIcon
          size="sm"
          variant="subtle"
          color="gray"
          aria-label={label}
          onClick={() => void copy()}
        >
          {copied ? (
            <IconCheck size={13} stroke={2.5} className="text-hue-emerald" />
          ) : (
            <IconCopy size={13} />
          )}
        </ActionIcon>
      )}
    </Tooltip>
  );
};

export const ConfirmModal = (props: {
  opened: boolean;
  onClose: () => void;
  title: string;
  confirmLabel: string;
  onConfirm: () => Promise<unknown> | void;
  icon?: React.ReactNode;
  children?: React.ReactNode;
}) => {
  const [pending, setPending] = React.useState(false);

  const confirm = async () => {
    setPending(true);
    try {
      await props.onConfirm();
      props.onClose();
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setPending(false);
    }
  };

  return (
    <Modal
      opened={props.opened}
      onClose={() => !pending && props.onClose()}
      size="sm"
      withCloseButton={false}
      padding={0}
      overlayProps={{ backgroundOpacity: 0.3, blur: 1 }}
      transitionProps={{ transition: "pop", duration: 200 }}
      styles={{
        content: {
          border: "1px solid var(--app-line)",
          borderRadius: "14px",
          boxShadow: "var(--app-shadow-overlay)",
          overflow: "hidden",
        },
      }}
    >
      <div className="flex flex-col">
        <div className="flex items-center gap-3 border-b border-line bg-surface-subtle px-5 py-3.5">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-hue-rose-solid text-white shadow-panel">
            {props.icon ?? <IconTrash size={15} stroke={2.25} />}
          </span>
          <span className="min-w-0 truncate text-[0.82rem]/5 font-bold text-ink">
            {props.title}
          </span>
        </div>
        <div className="px-5 py-4 text-[0.82rem]/6 font-medium text-ink-soft">
          {props.children}
        </div>
        <div className="flex items-center justify-end gap-2 border-t border-line bg-surface-subtle px-5 py-3">
          <Button
            variant="default"
            size="sm"
            disabled={pending}
            onClick={props.onClose}
          >
            Cancel
          </Button>
          <Button
            color="red"
            size="sm"
            loading={pending}
            data-autofocus
            onClick={() => void confirm()}
          >
            {props.confirmLabel}
          </Button>
        </div>
      </div>
    </Modal>
  );
};

export const AgentMark = (props: {
  live?: boolean;
  size?: "sm" | "lg";
  className?: string;
}) => {
  const large = props.size === "lg";
  return (
    <span
      className={twMerge(
        "relative flex shrink-0 items-center justify-center bg-inverted text-on-inverted shadow-panel",
        large ? "h-11 w-11 rounded-2xl" : "h-7 w-7 rounded-lg",
        props.className,
      )}
    >
      {props.live && (
        <span
          className={twMerge(
            "absolute -inset-1 border border-line-strong motion-safe:animate-pulse",
            large ? "rounded-[20px]" : "rounded-[11px]",
          )}
        />
      )}
      <IconSparkles size={large ? 20 : 14} stroke={2.2} />
    </span>
  );
};
