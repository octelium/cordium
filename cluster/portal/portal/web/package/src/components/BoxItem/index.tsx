import { twMerge } from "tailwind-merge";

export default (props: { children?: React.ReactNode; title?: string }) => {
  return (
    <div className="w-full my-8 border-[2px] border-gray-200 rounded-lg p-2 shadow-sm">
      {props.title && (
        <div className="mb-2 font-bold text-lg text-black">{props.title}</div>
      )}
      <div className={twMerge(props.title ? `ml-1` : undefined)}>
        {props.children}
      </div>
    </div>
  );
};
