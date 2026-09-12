import * as React from "react";
import { NavLink } from "react-router-dom";
import { twMerge } from "tailwind-merge";

export interface TabItem {
  label: string;
  to: string;
  icon?: React.ReactNode;
  end?: boolean;
  count?: number;
  hidden?: boolean;
}

const TabNav = (props: { items: TabItem[]; actions?: React.ReactNode }) => (
  <div className="mb-6 flex items-end gap-4 border-b border-line">
    <div className="scrollbar-none -mb-px flex flex-1 gap-1 overflow-x-auto">
      {props.items
        .filter((t) => !t.hidden)
        .map((t) => (
          <NavLink
            key={t.to}
            to={t.to}
            end={t.end}
            className={({ isActive }) =>
              twMerge(
                "flex shrink-0 items-center gap-1.5 whitespace-nowrap border-b-2 px-3 py-2.5",
                "text-[0.82rem] font-semibold transition-colors duration-150",
                isActive
                  ? "border-inverted text-ink"
                  : "border-transparent text-ink-muted hover:border-line-strong hover:text-ink-body",
              )
            }
          >
            {t.icon}
            {t.label}
            {typeof t.count === "number" && t.count > 0 && (
              <span className="rounded-md bg-surface-muted px-1.5 text-[0.68rem] font-bold text-ink-muted">
                {t.count}
              </span>
            )}
          </NavLink>
        ))}
    </div>

    {props.actions && (
      <div className="flex shrink-0 items-center gap-2 pb-2">
        {props.actions}
      </div>
    )}
  </div>
);

export default TabNav;
