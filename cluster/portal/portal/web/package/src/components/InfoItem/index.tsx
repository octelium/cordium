import * as React from "react";

export const InfoItem = (props: {
  children?: React.ReactNode;
  title: string;
}) => {
  return (
    <div className="w-full flex text-sm font-bold mb-1 items-center">
      <div className="flex text-black min-w-[100px]">{props.title}</div>
      <div className="ml-2 text-gray-600">{props.children}</div>
    </div>
  );
};

export default InfoItem;
