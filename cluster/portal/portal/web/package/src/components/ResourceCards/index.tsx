import * as React from "react";
import { useNavigate } from "react-router-dom";
import { twMerge } from "tailwind-merge";

export const CardGrid = (props: {
  children?: React.ReactNode;
  columns?: 1 | 2 | 3;
}) => (
  <div
    className={twMerge(
      "grid gap-3",
      props.columns === 3
        ? "sm:grid-cols-2 xl:grid-cols-3"
        : props.columns === 2
          ? "sm:grid-cols-2"
          : "grid-cols-1",
    )}
  >
    {props.children}
  </div>
);

export const CardList = (props: { children?: React.ReactNode }) => (
  <div className="flex flex-col gap-2.5">{props.children}</div>
);

export const ClickableCard = (props: {
  to?: string;
  children?: React.ReactNode;
  className?: string;
}) => {
  const navigate = useNavigate();
  const { to } = props;

  return (
    <div
      role={to ? "link" : undefined}
      tabIndex={to ? 0 : undefined}
      onClick={to ? () => navigate(to) : undefined}
      onKeyDown={
        to
          ? (e) => {
              if (e.key === "Enter") navigate(to);
            }
          : undefined
      }
      className={twMerge(
        "bg-white border border-slate-200 rounded-xl px-4 py-3.5",
        "shadow-[0_1px_3px_rgba(15,23,42,0.05)] transition-colors duration-150",
        to &&
          "cursor-pointer hover:border-slate-300 hover:bg-slate-50/60 focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-400",
        props.className,
      )}
    >
      {props.children}
    </div>
  );
};

export const CardTitle = (props: {
  name: string;
  displayName?: string;
  meta?: React.ReactNode;
}) => (
  <div className="min-w-0">
    <div className="flex flex-wrap items-baseline gap-x-2">
      <span className="truncate text-sm font-bold text-slate-800">
        {props.displayName || props.name}
      </span>
      {props.displayName && props.displayName !== props.name && (
        <span className="truncate font-mono text-[0.72rem] font-medium text-slate-400">
          {props.name}
        </span>
      )}
    </div>
    {props.meta && (
      <div className="mt-0.5 text-[0.72rem] font-medium text-slate-400">
        {props.meta}
      </div>
    )}
  </div>
);
