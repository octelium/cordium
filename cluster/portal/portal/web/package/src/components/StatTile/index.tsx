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
        <span className="text-[0.7rem] font-bold uppercase tracking-[0.07em] text-slate-400">
          {props.label}
        </span>
        {props.icon && <span className="text-slate-300">{props.icon}</span>}
      </div>
      <div className="mt-2 text-2xl font-bold tabular-nums text-slate-900">
        {props.value}
      </div>
      {props.hint && (
        <div className="mt-0.5 text-[0.75rem] font-medium text-slate-400">
          {props.hint}
        </div>
      )}
    </>
  );

  const className = twMerge(
    "block bg-white border border-slate-200 rounded-xl px-4 py-3.5",
    "shadow-[0_1px_3px_rgba(15,23,42,0.05)] transition-colors duration-150",
    props.to && "hover:border-slate-300 hover:bg-slate-50/60",
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
