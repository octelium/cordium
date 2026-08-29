import { ActionIcon, Button, Tooltip } from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import * as React from "react";

export const RepeatItem = (props: {
  index: number;
  label?: string;
  onRemove: () => void;
  children?: React.ReactNode;
}) => (
  <div className="rounded-lg border border-slate-200 bg-white">
    <div className="flex items-center gap-2 border-b border-slate-100 bg-slate-50/70 px-3 py-2">
      <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-md bg-slate-200 px-1.5 text-[0.7rem] font-bold text-slate-600">
        {props.index + 1}
      </span>
      <span className="flex-1 truncate text-[0.78rem] font-semibold text-slate-500">
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
  <div className="rounded-xl border border-slate-200 bg-slate-50/60">
    <div className="flex items-start gap-3 px-4 py-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-bold text-slate-800">
            {props.title}
          </span>
          {props.count > 0 && (
            <span className="rounded-md bg-slate-200 px-1.5 text-[0.7rem] font-bold text-slate-600">
              {props.count}
            </span>
          )}
        </div>
        {props.description && (
          <p className="mt-0.5 text-[0.78rem] font-medium text-slate-500">
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
      <div className="flex flex-col gap-3 border-t border-slate-200 px-4 py-4">
        {props.children}
      </div>
    ) : (
      props.emptyHint && (
        <p className="border-t border-slate-200 px-4 py-3 text-[0.78rem] font-medium text-slate-400">
          {props.emptyHint}
        </p>
      )
    )}
  </div>
);

export default RepeatBlock;
