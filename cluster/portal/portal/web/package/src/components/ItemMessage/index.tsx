import * as React from "react";

import { MdExpandLess, MdExpandMore } from "react-icons/md";

import { IoMdAdd } from "react-icons/io";

import { Chip, Collapse } from "@mantine/core";
import { twMerge } from "tailwind-merge";

interface Props {
  children?: React.ReactNode;
  title: string;
  obj?: object | Array<any> | undefined;
  onSet?: () => void;
  isList?: boolean;
  onAddListItem?: () => void;
}

const ItemMessage = (props: Props) => {
  let [isExpanded, setIsExpanded] = React.useState(false);

  let arrLen = 0;
  if (props.isList) {
    let arr = props.obj as Array<any>;
    arrLen = arr.length;
  }

  const isObjNull = !props.obj;

  return (
    <div className="mt-6">
      <div
        className="font-bold text-sm text-black mb-4 border-b-[1px] border-b-gray-300 cursor-pointer flex items-center"
        onClick={() => {
          if (isObjNull) {
            if (props.onSet) {
              props.onSet();
              setIsExpanded(true);
            }
          } else {
            setIsExpanded(!isExpanded);
          }
        }}
      >
        <button
          className="text-gray-600 font-bold text-sm hover:text-gray-800 mr-2 transition-all duration-300"
          onClick={() => {
            setIsExpanded(!isExpanded);
          }}
        >
          {isExpanded ? <MdExpandLess /> : <MdExpandMore />}
        </button>
        <span
          className={twMerge(
            isObjNull ? "text-gray-600" : "text-black",
            "mr-2",
          )}
        >
          {props.title}
        </span>{" "}
        {props.isList && (
          <Chip size="sm" className="mx-3">{`List (${
            arrLen == 1 ? "1 Item" : `${arrLen} Items`
          })`}</Chip>
        )}
        {props.isList && props.onAddListItem && (
          <button
            className={twMerge(
              "inline-flex items-center mx-2",
              "bg-black text-white font-bold p-1 rounded-full text-xs",
              "shadow-xl",
              "transition-all duration-300",
              "hover:bg-gray-800",
              "mr-2",
            )}
            onClick={(e) => {
              e.stopPropagation();
              if (props.onAddListItem) {
                props.onAddListItem();
                setIsExpanded(true);
              }
            }}
          >
            <IoMdAdd />
          </button>
        )}
        {/*
        {!props.obj && (
          <Button
            onClick={(e) => {
              e.stopPropagation();
              if (props.onSet) {
                props.onSet();
                setIsExpanded(true);
              }
            }}
          >
            Not set (Set!)
          </Button>
        )}
        */}
      </div>

      {props.obj && (
        <Collapse in={isExpanded}>
          <div className="ml-4 mb-4">
            <div>{props.children}</div>
          </div>
        </Collapse>
      )}
    </div>
  );
};

export default ItemMessage;
