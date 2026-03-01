import * as React from "react";

import { Button, Collapse } from "@mantine/core";

import { twJoin, twMerge } from "tailwind-merge";

import { MdAdd, MdDelete } from "react-icons/md";

interface Props {
  children?: React.ReactNode;
  title?: string;
  description?: string;
  obj?: object | Array<any> | undefined;
  onSet?: () => void;
  onUnset: () => void;
  isList?: boolean;
  onAddListItem?: () => void;
  noDelete?: boolean;
}

const EditItem = (props: Props) => {
  // let [isExpanded, setIsExpanded] = React.useState(false);

  let arrLen = 0;
  if (props.isList) {
    let arr = props.obj as Array<any>;
    arrLen = arr.length;
  }

  const isExpanded = props.isList ? arrLen > 0 : props.obj !== undefined;

  return (
    <div
      className={twJoin(
        "mt-4 pl-2 border-l-4",
        "transition-all duration-200",
        isExpanded ? "border-l-gray-800" : "border-l-gray-400",
        isExpanded ? undefined : "hover:bg-white hover:bg-opacity-50",
      )}
    >
      <div className="w-full flex items-center">
        <div
          className={twJoin(
            "font-bold text-sm  flex items-center",
            "transition-all duration-200",
            "flex-1 flex-grow w-full",
            isExpanded ? "text-black" : "text-gray-600",
            isExpanded ? undefined : "cursor-pointer",
          )}
          onClick={() => {
            if (!isExpanded) {
              if (props.onSet) {
                props.onSet();
              }
            }
          }}
        >
          {props.title && (
            <span className={twMerge("text-sm", "mr-2")}>{props.title}</span>
          )}
          {props.description && (
            <span
              className={twMerge(
                isExpanded ? "text-gray-600" : "text-gray-500",
                "mr-2",
                "transition-all duration-200",
              )}
            >
              {props.description}
            </span>
          )}

          {props.isList && props.onAddListItem && (
            <Button
              size="xs"
              // variant="light"
              className="!shadow transition-all duration-500"
              onClick={(e) => {
                e.stopPropagation();
                if (props.onAddListItem) {
                  props.onAddListItem();
                }
              }}
            >
              Add Item {`(${arrLen})`} <MdAdd />
            </Button>
          )}
        </div>
        {!props.noDelete && (
          <div>
            <button
              className={twJoin(
                "text-gray-600 font-bold text-xl p-1 cursor-pointer",
                "hover:text-gray-800 transition-all duration-200",
                isExpanded ? "visible" : "invisible",
              )}
              onClick={() => {
                props.onUnset();
              }}
            >
              <MdDelete />
            </button>
          </div>
        )}
      </div>

      <div>
        <Collapse in={isExpanded}>
          <div>
            <div className="ml-2">{props.children}</div>
            {props.isList && props.onAddListItem && (
              <div className="flex justify-center items-center my-4">
                <Button
                  size="xs"
                  // variant="light"
                  className="!shadow transition-all duration-500"
                  onClick={() => {
                    props.onAddListItem!();
                  }}
                >
                  Add Item {`(${arrLen})`} <MdAdd />
                </Button>
              </div>
            )}
          </div>
        </Collapse>
      </div>
    </div>
  );
};

export default EditItem;
