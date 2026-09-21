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

const ToolbarDivider = () => (
  <span className="mx-1.5 h-5 w-px shrink-0 bg-zinc-700/80" />
);

export const consoleToolbarButtonClass =
  "border border-zinc-600/80 bg-zinc-800/90 text-zinc-300 shadow-sm transition-all duration-150 hover:border-zinc-500 hover:bg-zinc-700 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400/70 focus-visible:ring-offset-1 focus-visible:ring-offset-console-chrome disabled:border-zinc-700 disabled:bg-zinc-800/50 disabled:text-zinc-600 disabled:shadow-none";

export const consoleToolbarButtonVars = () => ({
  root: {
    "--ai-bg": "rgb(30 41 59 / 0.95)",
    "--ai-hover": "rgb(51 65 85)",
    "--ai-color": "rgb(203 213 225)",
    "--ai-hover-color": "rgb(248 250 252)",
    "--ai-bd": "1px solid rgb(71 85 105 / 0.8)",
  },
});

const consoleToolbarActiveButtonVars = () => ({
  root: {
    "--ai-bg": "rgb(16 185 129 / 0.12)",
    "--ai-hover": "rgb(16 185 129 / 0.2)",
    "--ai-color": "rgb(167 243 208)",
    "--ai-hover-color": "rgb(209 250 229)",
    "--ai-bd": "1px solid rgb(52 211 153 / 0.5)",
  },
});

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

  useHotkeys([
    ["Escape", () => fullscreen && dispatch(setTerminalFullscreen({ value: false }))],
  ]);

  React.useEffect(() => {
    return () => {
      dispatch(setTerminalFullscreen({ value: false }));
    };
  }, [dispatch]);

  const toolbar = (
    <div className="flex items-center gap-2 border-b border-zinc-800 bg-console-chrome px-3 py-2">
      <div className="flex min-w-0 flex-1 items-center">{props.tabs}</div>

      <div className="flex shrink-0 items-center gap-1 border-l border-zinc-700/80 pl-3">
        {props.actions}

        {props.withFontControls !== false && (
          <>
            {props.actions && <ToolbarDivider />}
            <div className="flex items-center gap-0.5 rounded-lg border border-zinc-700 bg-zinc-900/60 p-0.5">
              <Tooltip label="Decrease font size">
                <ActionIcon
                  size={27}
                  variant="transparent"
                  aria-label="Decrease font size"
                  className={consoleToolbarButtonClass}
                  vars={consoleToolbarButtonVars}
                  disabled={fontSize <= TERMINAL_FONT_SIZE_MIN}
                  onClick={() =>
                    dispatch(setTerminalFontSize({ value: fontSize - 1 }))
                  }
                >
                  <IconMinus size={13} stroke={2.25} />
                </ActionIcon>
              </Tooltip>
              <span className="min-w-7 px-1 text-center font-mono text-[0.7rem] font-semibold tabular-nums text-zinc-200">
                {fontSize}
              </span>
              <Tooltip label="Increase font size">
                <ActionIcon
                  size={27}
                  variant="transparent"
                  aria-label="Increase font size"
                  className={consoleToolbarButtonClass}
                  vars={consoleToolbarButtonVars}
                  disabled={fontSize >= TERMINAL_FONT_SIZE_MAX}
                  onClick={() =>
                    dispatch(setTerminalFontSize({ value: fontSize + 1 }))
                  }
                >
                  <IconPlus size={13} stroke={2.25} />
                </ActionIcon>
              </Tooltip>
            </div>
          </>
        )}

        <ToolbarDivider />

        <div className="flex items-center gap-1 rounded-lg border border-zinc-700 bg-zinc-900/60 p-0.5">
          <Tooltip
            label={fullscreen ? "Exit full screen (Esc)" : "Full screen"}
          >
            <ActionIcon
              size={27}
              variant="transparent"
              aria-label="Toggle full screen"
              vars={
                fullscreen
                  ? consoleToolbarActiveButtonVars
                  : consoleToolbarButtonVars
              }
              className={twMerge(
                consoleToolbarButtonClass,
                fullscreen &&
                  "border-emerald-400/50 bg-emerald-400/10 text-emerald-200 hover:bg-emerald-400/20",
              )}
              onClick={() =>
                dispatch(setTerminalFullscreen({ value: !fullscreen }))
              }
            >
              {fullscreen ? (
                <IconArrowsMinimize size={14} />
              ) : (
                <IconArrowsMaximize size={14} />
              )}
            </ActionIcon>
          </Tooltip>
        </div>
      </div>
    </div>
  );

  if (fullscreen) {
    return (
      <RemoveScroll>
        <div className="console-surface fixed inset-0 z-[400] flex flex-col bg-console">
          {toolbar}
          <div className="min-h-0 flex-1 px-2 py-2">{props.children}</div>
        </div>
      </RemoveScroll>
    );
  }

  return (
    <div
      className={twMerge(
        "console-surface overflow-hidden rounded-xl border border-zinc-800 bg-console",
        "shadow-console",
      )}
    >
      {toolbar}
      <div
        className="px-2 py-2"
        style={{ height: props.height ?? 520 }}
      >
        {props.children}
      </div>
    </div>
  );
};

export default ConsoleShell;
