import type { Terminal } from "@xterm/xterm";

export const terminalFontFamily =
  '"JetBrains Mono", ui-monospace, Menlo, Consolas, monospace';

export const loadTerminalFont = () =>
  Promise.allSettled([
    document.fonts.load('400 1em "JetBrains Mono"'),
    document.fonts.load('700 1em "JetBrains Mono"'),
  ]);

export const refreshTerminalFont = (terminal: Terminal) => {
  terminal.options.fontFamily = "monospace";
  terminal.options.fontFamily = terminalFontFamily;
};

export const terminalTheme = {
  background: "#0b0b0c",
  foreground: "#e4e4e7",
  cursor: "#a1a1aa",
  cursorAccent: "#0b0b0c",
  selectionBackground: "#3f3f46",
  black: "#27272a",
  brightBlack: "#52525b",
  blue: "#60a5fa",
  brightBlue: "#93c5fd",
  cyan: "#22d3ee",
  brightCyan: "#67e8f9",
  green: "#4ade80",
  brightGreen: "#86efac",
  red: "#f87171",
  brightRed: "#fca5a5",
  yellow: "#facc15",
  brightYellow: "#fde047",
  magenta: "#c084fc",
  brightMagenta: "#d8b4fe",
  white: "#d4d4d8",
  brightWhite: "#f4f4f5",
};
