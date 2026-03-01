import { twMerge } from "tailwind-merge";

const Divider = (props: { children?: React.ReactNode }) => {
  return (
    <div className="relative flex py-1 items-center">
      <div className="flex-grow border-t border-gray-400"></div>
      <span
        className={twMerge(
          "flex-shrink text-gray-400 font-bold",
          props.children ? "mx-4" : undefined,
        )}
      >
        {props.children}
      </span>
      <div className="flex-grow border-t border-gray-400"></div>
    </div>
  );
};

export default Divider;
