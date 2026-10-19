export const consoleToolbarButtonClass =
  "text-ink-soft transition-colors duration-150 hover:bg-surface-strong hover:text-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus disabled:text-ink-faint";

export const consoleToolbarButtonVars = () => ({
  root: {
    "--ai-bg": "transparent",
    "--ai-hover": "var(--app-surface-strong)",
    "--ai-color": "var(--app-ink-soft)",
    "--ai-hover-color": "var(--app-ink)",
    "--ai-bd": "1px solid transparent",
  },
});
