import * as React from "react";
import { Link } from "react-router-dom";
import { twMerge } from "tailwind-merge";

const StatTile = (props: {
  label: string;
  value: React.ReactNode;
  hint?: React.ReactNode;
  icon?: React.ReactNode;
  to?: string;
}) => {
  const body = (
    <>
      <div className="flex items-center justify-between gap-2">
        <span className="text-[0.7rem] font-bold uppercase tracking-[0.07em] text-ink-subtle">
          {props.label}
        </span>
        {props.icon && <span className="text-ink-faint">{props.icon}</span>}
      </div>
      <div className="mt-2 text-2xl font-bold tabular-nums text-ink">
        {props.value}
      </div>
      {props.hint && (
        <div className="mt-0.5 text-[0.75rem] font-medium text-ink-subtle">
          {props.hint}
        </div>
      )}
    </>
  );

  const className = twMerge(
    "block bg-surface border border-line rounded-xl px-4 py-3.5",
    "shadow-panel transition-colors duration-150",
    props.to && "hover:border-line-strong hover:bg-surface-hover",
  );

  if (props.to) {
    return (
      <Link to={props.to} className={className}>
        {body}
      </Link>
    );
  }

  return <div className={className}>{body}</div>;
};

export default StatTile;
