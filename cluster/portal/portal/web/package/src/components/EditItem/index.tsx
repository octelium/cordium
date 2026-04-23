import { Button, Collapse } from "@mantine/core";
import { Plus, Trash2 } from "lucide-react";
import * as React from "react";
import { twJoin, twMerge } from "tailwind-merge";

interface Props {
  children?: React.ReactNode;
  title?: string;
  description?: string;
  obj?: object | Array<any>;
  onSet?: () => void;
  onUnset: () => void;
  isList?: boolean;
  onAddListItem?: () => void;
  noDelete?: boolean;
}

const EditItem = (props: Props) => {
  const arr = props.isList ? (props.obj as Array<any> | undefined) : undefined;
  const arrLen = arr?.length ?? 0;
  const isExpanded = props.isList ? arrLen > 0 : props.obj !== undefined;

  const handleAddItem = (e: React.MouseEvent) => {
    e.stopPropagation();
    props.onAddListItem?.();
  };

  return (
    <div
      className={twJoin(
        "mt-4 pl-3 border-l-4",
        "transition-colors duration-200",
        isExpanded ? "border-l-gray-800" : "border-l-gray-300",
        !isExpanded && "hover:border-l-gray-500",
      )}
    >
      <div className="w-full flex items-center gap-2 min-h-[28px]">
        <div
          className={twJoin(
            "flex items-center gap-2 flex-1 min-w-0",
            !isExpanded && "cursor-pointer",
          )}
          onClick={() => {
            if (!isExpanded) props.onSet?.();
          }}
        >
          {props.title && (
            <span
              className={twMerge(
                "font-bold text-sm transition-colors duration-200 shrink-0",
                isExpanded ? "text-gray-900" : "text-gray-500",
              )}
            >
              {props.title}
            </span>
          )}

          {props.description && (
            <span
              className={twMerge(
                "text-xs transition-colors duration-200 truncate",
                isExpanded ? "text-gray-500" : "text-gray-400",
              )}
            >
              {props.description}
            </span>
          )}

          {props.isList && props.onAddListItem && (
            <Button
              size="xs"
              variant="light"
              leftSection={<Plus size={12} />}
              onClick={handleAddItem}
            >
              Add item {arrLen > 0 && `(${arrLen})`}
            </Button>
          )}
        </div>

        {!props.noDelete && isExpanded && (
          <button
            type="button"
            aria-label={`Remove ${props.title ?? "item"}`}
            className="text-gray-400 hover:text-red-500 transition-colors duration-150 p-1 cursor-pointer flex-shrink-0"
            onClick={() => props.onUnset()}
          >
            <Trash2 size={14} />
          </button>
        )}
      </div>

      <Collapse expanded={isExpanded}>
        <div className="ml-2 mt-1">
          {props.children}

          {props.isList && props.onAddListItem && arrLen > 0 && (
            <div className="flex justify-start items-center mt-3 mb-1">
              <Button
                size="xs"
                variant="light"
                leftSection={<Plus size={12} />}
                onClick={(e) => {
                  e.stopPropagation();
                  props.onAddListItem!();
                }}
              >
                Add another item ({arrLen})
              </Button>
            </div>
          )}
        </div>
      </Collapse>
    </div>
  );
};

export default EditItem;
