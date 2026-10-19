import {
  setTerminalFontSize,
  setTerminalFullscreen,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
} from "@/features/settings/slice";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { ActionIcon, RemoveScroll, Tooltip } from "@mantine/core";
import { useHotkeys } from "@mantine/hooks";
import {
  IconArrowsMaximize,
  IconArrowsMinimize,
  IconMinus,
  IconPlus,
} from "@tabler/icons-react";
import * as React from "react";
import { twMerge } from "tailwind-merge";

import { consoleToolbarButtonClass, consoleToolbarButtonVars } from "./toolbar";

const ConsoleShell = (props: {
  tabs?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  height?: number;
  withFontControls?: boolean;
}) => {
  const dispatch = useAppDispatch();
  const fullscreen = useAppSelector((s) => s.settings.terminalFullscreen);
  const fontSize = useAppSelector((s) => s.settings.terminalFontSize);
  const shellRef = React.useRef<HTMLDivElement>(null);

  useHotkeys([
    [
      "Escape",
      () => fullscreen && dispatch(setTerminalFullscreen({ value: false })),
    ],
  ]);

  React.useEffect(
    () => () => {
      dispatch(setTerminalFullscreen({ value: false }));
    },
    [dispatch],
  );

  React.useEffect(() => {
    const shell = shellRef.current;
    if (!shell) return;
    const onWheel = (event: WheelEvent) => {
      let node = event.target as HTMLElement | null;
      while (node && node !== shell) {
        if (node.scrollHeight > node.clientHeight) {
          const overflowY = getComputedStyle(node).overflowY;
          if (overflowY === "auto" || overflowY === "scroll") return;
        }
        node = node.parentElement;
      }
      event.preventDefault();
    };
    shell.addEventListener("wheel", onWheel, { passive: false });
    return () => shell.removeEventListener("wheel", onWheel);
  }, []);

  return (
    <RemoveScroll enabled={fullscreen} forwardProps>
      <div
        ref={shellRef}
        className={twMerge(
          "console-surface flex min-w-0 flex-col overflow-hidden border border-line bg-console shadow-console",
          fullscreen
            ? "fixed inset-0 z-[400]"
            : "sticky top-[calc(var(--app-header-height)+0.75rem)] z-10 rounded-xl",
        )}
      >
        <div className="flex flex-col gap-2 border-b border-line bg-surface-subtle p-2 sm:flex-row sm:items-center">
          <div className="flex min-w-0 flex-1 items-center">{props.tabs}</div>
          <div
            role="group"
            aria-label="Console controls"
            className="flex shrink-0 items-center gap-1 self-end sm:self-auto sm:border-l sm:border-line sm:pl-2"
          >
            {props.actions}
            {props.withFontControls !== false && (
              <div className="flex items-center gap-0.5">
                <Tooltip label="Decrease font size">
                  <ActionIcon
                    size={28}
                    variant="transparent"
                    aria-label="Decrease font size"
                    className={consoleToolbarButtonClass}
                    vars={consoleToolbarButtonVars}
                    disabled={fontSize <= TERMINAL_FONT_SIZE_MIN}
                    onClick={() =>
                      dispatch(setTerminalFontSize({ value: fontSize - 1 }))
                    }
                  >
                    <IconMinus size={14} />
                  </ActionIcon>
                </Tooltip>
                <span
                  className="min-w-6 text-center text-xs font-medium tabular-nums text-ink-muted"
                  aria-label={`Font size ${fontSize}`}
                >
                  {fontSize}
                </span>
                <Tooltip label="Increase font size">
                  <ActionIcon
                    size={28}
                    variant="transparent"
                    aria-label="Increase font size"
                    className={consoleToolbarButtonClass}
                    vars={consoleToolbarButtonVars}
                    disabled={fontSize >= TERMINAL_FONT_SIZE_MAX}
                    onClick={() =>
                      dispatch(setTerminalFontSize({ value: fontSize + 1 }))
                    }
                  >
                    <IconPlus size={14} />
                  </ActionIcon>
                </Tooltip>
              </div>
            )}
            <span className="mx-1 h-4 w-px bg-line" aria-hidden />
            <Tooltip
              label={fullscreen ? "Exit full screen (Esc)" : "Full screen"}
            >
              <ActionIcon
                size={28}
                variant="transparent"
                aria-label="Toggle full screen"
                aria-pressed={fullscreen}
                vars={consoleToolbarButtonVars}
                className={consoleToolbarButtonClass}
                onClick={() =>
                  dispatch(setTerminalFullscreen({ value: !fullscreen }))
                }
              >
                {fullscreen ? (
                  <IconArrowsMinimize size={16} />
                ) : (
                  <IconArrowsMaximize size={16} />
                )}
              </ActionIcon>
            </Tooltip>
          </div>
        </div>
        <div
          className={twMerge("min-h-0 px-2 py-2", fullscreen && "flex-1")}
          style={
            fullscreen
              ? undefined
              : {
                  height: `clamp(240px, calc(100dvh - 22rem), ${props.height ?? 520}px)`,
                }
          }
        >
          {props.children}
        </div>
      </div>
    </RemoveScroll>
  );
};

export default ConsoleShell;
