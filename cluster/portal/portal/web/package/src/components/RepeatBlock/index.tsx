import { ActionIcon, Button, Tooltip } from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import * as React from "react";

export const RepeatItem = (props: {
  index: number;
  label?: string;
  onRemove: () => void;
  children?: React.ReactNode;
}) => (
  <div className="rounded-lg border border-line bg-surface">
    <div className="flex items-center gap-2 border-b border-line-subtle bg-surface-subtle px-3 py-2">
      <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-md bg-surface-strong px-1.5 text-[0.7rem] font-bold text-ink-soft">
        {props.index + 1}
      </span>
      <span className="flex-1 truncate text-[0.78rem] font-semibold text-ink-muted">
        {props.label}
      </span>
      <Tooltip label="Remove">
        <ActionIcon
          size="sm"
          color="red"
          variant="subtle"
          aria-label="Remove item"
          onClick={props.onRemove}
        >
          <IconTrash size={14} />
        </ActionIcon>
      </Tooltip>
    </div>
    <div className="px-3 py-3">{props.children}</div>
  </div>
);

const RepeatBlock = (props: {
  title: string;
  description?: React.ReactNode;
  count: number;
  addLabel?: string;
  onAdd: () => void;
  emptyHint?: string;
  children?: React.ReactNode;
}) => (
  <div className="rounded-xl border border-line bg-surface-subtle">
    <div className="flex items-start gap-3 px-4 py-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-bold text-ink-strong">
            {props.title}
          </span>
          {props.count > 0 && (
            <span className="rounded-md bg-surface-strong px-1.5 text-[0.7rem] font-bold text-ink-soft">
              {props.count}
            </span>
          )}
        </div>
        {props.description && (
          <p className="mt-0.5 text-[0.78rem] font-medium text-ink-muted">
            {props.description}
          </p>
        )}
      </div>
      <Button
        size="compact-xs"
        variant="default"
        leftSection={<IconPlus size={13} />}
        onClick={props.onAdd}
      >
        {props.addLabel ?? "Add"}
      </Button>
    </div>

    {props.count > 0 ? (
      <div className="flex flex-col gap-3 border-t border-line px-4 py-4">
        {props.children}
      </div>
    ) : (
      props.emptyHint && (
        <p className="border-t border-line px-4 py-3 text-[0.78rem] font-medium text-ink-subtle">
          {props.emptyHint}
        </p>
      )
    )}
  </div>
);

export default RepeatBlock;
