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
  <span className="mx-1 h-4 w-px shrink-0 bg-slate-700" />
);

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
    <div className="flex items-center gap-2 border-b border-slate-900 bg-[var(--console-chrome)] px-2 py-1.5">
      <div className="flex min-w-0 flex-1 items-center">{props.tabs}</div>

      <div className="flex shrink-0 items-center gap-1">
        {props.actions}

        {props.withFontControls !== false && (
          <>
            {props.actions && <ToolbarDivider />}
            <Tooltip label="Decrease font size">
              <ActionIcon
                size={26}
                variant="subtle"
                color="gray"
                aria-label="Decrease font size"
                disabled={fontSize <= TERMINAL_FONT_SIZE_MIN}
                onClick={() =>
                  dispatch(setTerminalFontSize({ value: fontSize - 1 }))
                }
              >
                <IconMinus size={13} />
              </ActionIcon>
            </Tooltip>
            <span className="w-6 text-center font-mono text-[0.68rem] font-semibold text-slate-400">
              {fontSize}
            </span>
            <Tooltip label="Increase font size">
              <ActionIcon
                size={26}
                variant="subtle"
                color="gray"
                aria-label="Increase font size"
                disabled={fontSize >= TERMINAL_FONT_SIZE_MAX}
                onClick={() =>
                  dispatch(setTerminalFontSize({ value: fontSize + 1 }))
                }
              >
                <IconPlus size={13} />
              </ActionIcon>
            </Tooltip>
          </>
        )}

        <ToolbarDivider />

        {!fullscreen && (
          <Tooltip label={wide ? "Exit wide mode" : "Wide mode"}>
            <ActionIcon
              size={26}
              variant={wide ? "light" : "subtle"}
              color="gray"
              aria-label="Toggle wide mode"
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
            size={26}
            variant={fullscreen ? "light" : "subtle"}
            color="gray"
            aria-label="Toggle full screen"
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
