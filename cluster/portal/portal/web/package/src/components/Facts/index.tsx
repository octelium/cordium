import * as React from "react";

export const Facts = (props: { children?: React.ReactNode }) => (
  <dl className="divide-y divide-slate-100">{props.children}</dl>
);

export const Fact = (props: {
  label: string;
  children?: React.ReactNode;
  stacked?: boolean;
}) => {
  if (props.stacked) {
    return (
      <div className="py-2.5">
        <dt className="text-[0.7rem] font-bold uppercase tracking-[0.06em] text-slate-400">
          {props.label}
        </dt>
        <dd className="mt-1 text-sm font-medium text-slate-700 break-words">
          {props.children}
        </dd>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-[minmax(0,9rem)_1fr] items-baseline gap-x-4 py-2.5">
      <dt className="text-[0.7rem] font-bold uppercase tracking-[0.06em] text-slate-400">
        {props.label}
      </dt>
      <dd className="text-sm font-medium text-slate-700 break-words min-w-0">
        {props.children}
      </dd>
    </div>
  );
};

export default Facts;
