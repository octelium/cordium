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
        "bg-surface border border-line rounded-xl px-4 py-3.5",
        "shadow-panel transition-colors duration-150",
        to &&
          "cursor-pointer hover:border-line-strong hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-focus",
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
      <span className="truncate text-sm font-bold text-ink-strong">
        {props.displayName || props.name}
      </span>
      {props.displayName && props.displayName !== props.name && (
        <span className="truncate font-mono text-[0.72rem] font-medium text-ink-subtle">
          {props.name}
        </span>
      )}
    </div>
    {props.meta && (
      <div className="mt-0.5 text-[0.72rem] font-medium text-ink-subtle">
        {props.meta}
      </div>
    )}
  </div>
);
