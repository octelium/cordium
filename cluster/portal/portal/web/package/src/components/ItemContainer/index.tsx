import React from "react";
import { twMerge } from "tailwind-merge";

const ItemContainer = (props: {
  children?: React.ReactNode;
  title: string;
  isHorizontal?: boolean;
}) => {
  return (
    <div
      className={twMerge(
        "mt-4 flex w-full",
        props.isHorizontal ? "flex-row items-center" : "flex-col items-start",
      )}
    >
      <div className="mr-2">
        <div className="font-semibold text-black text-sm inline-flex">
          {props.title}
        </div>
      </div>
      <div className="flex-grow w-full">{props.children}</div>
    </div>
  );
};

export default ItemContainer;
