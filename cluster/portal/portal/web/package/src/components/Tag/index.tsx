import * as React from "react";
import { twMerge } from "tailwind-merge";

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

const tones: Record<Tone, string> = {
  neutral: "bg-surface-muted text-ink-soft border-line",
  accent: "bg-inverted text-on-inverted border-inverted",
  success: "bg-hue-emerald-soft text-hue-emerald border-hue-emerald-line",
  warning: "bg-hue-amber-soft text-hue-amber border-hue-amber-line",
  danger: "bg-hue-rose-soft text-hue-rose border-hue-rose-line",
  info: "bg-hue-sky-soft text-hue-sky border-hue-sky-line",
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
      <span className="text-ink-subtle font-medium">{props.label}</span>
    )}
    <span className={props.mono ? "font-mono" : undefined}>
      {props.children}
    </span>
  </span>
);

export default Tag;
