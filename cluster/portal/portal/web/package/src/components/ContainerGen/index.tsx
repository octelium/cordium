import * as React from "react";
import { twMerge } from "tailwind-merge";

const ContainerGen = (props: {
  children?: React.ReactNode;
  title?: React.ReactNode;
}) => {
  return (
    <div className="w-full border-[1px] border-gray-200 p-2 mb-2 rounded-lg">
      {props.title && (
        <div className="w-full font-bold text-sm text-gray-600 mb-2">
          {props.title}
        </div>
      )}
      <div className={twMerge(props.title ? `ml-4` : undefined)}>
        {props.children}
      </div>
    </div>
  );
};

export default ContainerGen;
