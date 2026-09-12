import * as React from "react";
import { twMerge } from "tailwind-merge";

export const Panel = (props: {
  children?: React.ReactNode;
  className?: string;
}) => (
  <section
    className={twMerge(
      "bg-surface border border-line rounded-xl overflow-hidden",
      "shadow-panel",
      props.className,
    )}
  >
    {props.children}
  </section>
);

export const PanelHeader = (props: {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  icon?: React.ReactNode;
}) => (
  <header className="flex items-start gap-3 px-5 py-3.5 border-b border-line-subtle bg-surface-subtle">
    {props.icon && (
      <span className="mt-0.5 text-ink-subtle shrink-0">{props.icon}</span>
    )}
    <div className="flex-1 min-w-0">
      <h2 className="text-[0.72rem] font-bold uppercase tracking-[0.07em] text-ink-muted">
        {props.title}
      </h2>
      {props.description && (
        <p className="mt-1 text-[0.78rem] font-medium text-ink-muted">
          {props.description}
        </p>
      )}
    </div>
    {props.actions && (
      <div className="flex items-center gap-2 shrink-0">{props.actions}</div>
    )}
  </header>
);

export const PanelBody = (props: {
  children?: React.ReactNode;
  className?: string;
}) => (
  <div className={twMerge("p-5", props.className)}>{props.children}</div>
);

export const PanelFooter = (props: { children?: React.ReactNode }) => (
  <footer className="flex items-center justify-end gap-2 px-5 py-3 border-t border-line-subtle bg-surface-subtle">
    {props.children}
  </footer>
);

export default Panel;
