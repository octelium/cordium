import {
  setTerminalFontSize,
  setTerminalFullscreen,
  setTerminalWide,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
} from "@/features/settings/slice";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";
import { ActionIcon, RemoveScroll, Tooltip } from "@mantine/core";
import { useHotkeys } from "@mantine/hooks";
import {
  IconArrowsDiagonal,
  IconArrowsDiagonalMinimize2,
  IconArrowsMaximize,
  IconArrowsMinimize,
  IconMinus,
  IconPlus,
} from "@tabler/icons-react";
import * as React from "react";
import { twMerge } from "tailwind-merge";

const ToolbarDivider = () => (
  <span className="mx-1.5 h-5 w-px shrink-0 bg-slate-700/80" />
);

export const consoleToolbarButtonClass =
  "border border-slate-600/80 bg-slate-800/90 text-slate-300 shadow-sm transition-all duration-150 hover:border-slate-500 hover:bg-slate-700 hover:text-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400/70 focus-visible:ring-offset-1 focus-visible:ring-offset-[var(--console-chrome)] disabled:border-slate-700 disabled:bg-slate-800/50 disabled:text-slate-600 disabled:shadow-none";

const ConsoleShell = (props: {
  tabs?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  height?: number;
  withFontControls?: boolean;
}) => {
  const dispatch = useAppDispatch();
  const wide = useAppSelector((s) => s.settings.terminalWide);
  const fullscreen = useAppSelector((s) => s.settings.terminalFullscreen);
  const fontSize = useAppSelector((s) => s.settings.terminalFontSize);

  useHotkeys([
    ["Escape", () => fullscreen && dispatch(setTerminalFullscreen({ value: false }))],
  ]);

  React.useEffect(() => {
    return () => {
      dispatch(setTerminalFullscreen({ value: false }));
      dispatch(setTerminalWide({ value: false }));
    };
  }, [dispatch]);

  const toolbar = (
    <div className="flex items-center gap-2 border-b border-slate-800 bg-[var(--console-chrome)] px-3 py-2">
      <div className="flex min-w-0 flex-1 items-center">{props.tabs}</div>

      <div className="flex shrink-0 items-center gap-1">
        {props.actions}

        {props.withFontControls !== false && (
          <>
            {props.actions && <ToolbarDivider />}
            <div className="flex items-center gap-0.5 rounded-lg border border-slate-700 bg-slate-900/60 p-0.5">
              <Tooltip label="Decrease font size">
                <ActionIcon
                  size={27}
                  variant="transparent"
                  aria-label="Decrease font size"
                  className={consoleToolbarButtonClass}
                  disabled={fontSize <= TERMINAL_FONT_SIZE_MIN}
                  onClick={() =>
                    dispatch(setTerminalFontSize({ value: fontSize - 1 }))
                  }
                >
                  <IconMinus size={13} stroke={2.25} />
                </ActionIcon>
              </Tooltip>
              <span className="min-w-7 px-1 text-center font-mono text-[0.7rem] font-semibold tabular-nums text-slate-200">
                {fontSize}
              </span>
              <Tooltip label="Increase font size">
                <ActionIcon
                  size={27}
                  variant="transparent"
                  aria-label="Increase font size"
                  className={consoleToolbarButtonClass}
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

        <div className="flex items-center gap-1 rounded-lg border border-slate-700 bg-slate-900/60 p-0.5">
          {!fullscreen && (
            <Tooltip label={wide ? "Exit wide mode" : "Wide mode"}>
              <ActionIcon
                size={27}
                variant="transparent"
                aria-label="Toggle wide mode"
                className={twMerge(
                  consoleToolbarButtonClass,
                  wide &&
                    "border-emerald-400/50 bg-emerald-400/10 text-emerald-200 hover:bg-emerald-400/20",
                )}
                onClick={() => dispatch(setTerminalWide({ value: !wide }))}
              >
                {wide ? (
                  <IconArrowsDiagonalMinimize2 size={14} />
                ) : (
                  <IconArrowsDiagonal size={14} />
                )}
              </ActionIcon>
            </Tooltip>
          )}

          <Tooltip
            label={fullscreen ? "Exit full screen (Esc)" : "Full screen"}
          >
            <ActionIcon
              size={27}
              variant="transparent"
              aria-label="Toggle full screen"
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
        <div className="console-surface fixed inset-0 z-[400] flex flex-col bg-[var(--console-bg)]">
          {toolbar}
          <div className="min-h-0 flex-1 px-2 py-2">{props.children}</div>
        </div>
      </RemoveScroll>
    );
  }

  return (
    <div
      className={twMerge(
        "console-surface overflow-hidden rounded-xl border border-slate-800 bg-[var(--console-bg)]",
        "shadow-[0_8px_24px_rgba(15,23,42,0.16)]",
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
