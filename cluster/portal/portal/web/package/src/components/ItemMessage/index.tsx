import { Badge, Collapse } from "@mantine/core";
import { ChevronDown, ChevronUp, Plus } from "lucide-react";
import * as React from "react";
import { twMerge } from "tailwind-merge";

interface Props {
  children?: React.ReactNode;
  title: string;
  obj?: object | Array<any>;
  onSet?: () => void;
  isList?: boolean;
  onAddListItem?: () => void;
}

const ItemMessage = (props: Props) => {
  const arr = props.isList ? (props.obj as Array<any> | undefined) : undefined;
  const arrLen = arr?.length ?? 0;
  const isObjNull = !props.obj;

  const [isExpanded, setIsExpanded] = React.useState(false);

  const handleHeaderClick = () => {
    if (isObjNull) {
      props.onSet?.();
      setIsExpanded(true);
    } else {
      setIsExpanded((v) => !v);
    }
  };

  const handleAddItem = (e: React.MouseEvent) => {
    e.stopPropagation();
    props.onAddListItem?.();
    setIsExpanded(true);
  };

  return (
    <div className="mt-6">
      <button
        type="button"
        aria-expanded={isExpanded}
        className={twMerge(
          "w-full flex items-center gap-2 cursor-pointer",
          "pb-2 border-b border-gray-300 hover:border-gray-400",
          "transition-colors duration-150 group",
        )}
        onClick={handleHeaderClick}
      >
        <span className="text-gray-500 group-hover:text-gray-700 transition-colors duration-150 flex-shrink-0">
          {isExpanded ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
        </span>

        <span
          className={twMerge(
            "font-bold text-sm flex-1 text-left transition-colors duration-150",
            isObjNull
              ? "text-gray-500 group-hover:text-gray-700"
              : "text-gray-900",
          )}
        >
          {props.title}
        </span>

        {props.isList && (
          <Badge size="sm" variant="light" color="gray">
            {arrLen === 1 ? "1 item" : `${arrLen} items`}
          </Badge>
        )}

        {props.isList && props.onAddListItem && (
          <button
            type="button"
            aria-label={`Add item to ${props.title}`}
            onClick={handleAddItem}
            className={twMerge(
              "inline-flex items-center gap-1",
              "px-2 py-0.5 rounded-full text-xs font-bold",
              "bg-gray-900 text-white",
              "hover:bg-black transition-colors duration-150",
            )}
          >
            <Plus size={11} />
            Add
          </button>
        )}
      </button>

      {props.obj && (
        <Collapse expanded={isExpanded}>
          <div className="ml-4 mt-3 mb-4">{props.children}</div>
        </Collapse>
      )}
    </div>
  );
};

export default ItemMessage;
