import * as React from "react";
import { twMerge } from "tailwind-merge";

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

const tones: Record<Tone, string> = {
  neutral: "bg-slate-100 text-slate-600 border-slate-200",
  accent: "bg-slate-800 text-white border-slate-800",
  success: "bg-emerald-50 text-emerald-700 border-emerald-200",
  warning: "bg-amber-50 text-amber-700 border-amber-200",
  danger: "bg-rose-50 text-rose-700 border-rose-200",
  info: "bg-sky-50 text-sky-700 border-sky-200",
};

const Tag = (props: {
  children?: React.ReactNode;
  tone?: Tone;
  icon?: React.ReactNode;
  label?: string;
  mono?: boolean;
  className?: string;
}) => (
  <span
    className={twMerge(
      "inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5",
      "text-[0.72rem] font-semibold leading-5 whitespace-nowrap",
      tones[props.tone ?? "neutral"],
      props.className,
    )}
  >
    {props.icon}
    {props.label && (
      <span className="text-slate-400 font-medium">{props.label}</span>
    )}
    <span className={props.mono ? "font-mono" : undefined}>
      {props.children}
    </span>
  </span>
);

export default Tag;
