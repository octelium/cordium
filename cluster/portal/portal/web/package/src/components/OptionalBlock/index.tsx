import { Collapse, Switch } from "@mantine/core";
import * as React from "react";
import { twMerge } from "tailwind-merge";

const OptionalBlock = (props: {
  title: string;
  description?: React.ReactNode;
  enabled: boolean;
  onEnable: () => void;
  onDisable: () => void;
  children?: React.ReactNode;
  icon?: React.ReactNode;
}) => (
  <div
    className={twMerge(
      "rounded-xl border transition-colors duration-150",
      props.enabled
        ? "border-slate-300 bg-white"
        : "border-slate-200 bg-slate-50/60",
    )}
  >
    <div className="flex items-start gap-3 px-4 py-3">
      {props.icon && (
        <span
          className={twMerge(
            "mt-0.5 shrink-0",
            props.enabled ? "text-slate-500" : "text-slate-400",
          )}
        >
          {props.icon}
        </span>
      )}
      <div className="flex-1 min-w-0">
        <div
          className={twMerge(
            "text-sm font-bold",
            props.enabled ? "text-slate-800" : "text-slate-600",
          )}
        >
          {props.title}
        </div>
        {props.description && (
          <p className="mt-0.5 text-[0.78rem] font-medium text-slate-500">
            {props.description}
          </p>
        )}
      </div>
      <Switch
        size="sm"
        checked={props.enabled}
        aria-label={props.title}
        onChange={(e) =>
          e.currentTarget.checked ? props.onEnable() : props.onDisable()
        }
      />
    </div>

    <Collapse expanded={props.enabled}>
      <div className="border-t border-slate-100 px-4 py-4">
        {props.children}
      </div>
    </Collapse>
  </div>
);

export default OptionalBlock;
