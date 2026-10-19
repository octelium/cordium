import { WebglAddon } from "@xterm/addon-webgl";
import type { Terminal } from "@xterm/xterm";

export const loadWebglRenderer = (terminal: Terminal, refresh: () => void) => {
  const addon = new WebglAddon();
  const contextLoss = addon.onContextLoss(() => {
    contextLoss.dispose();
    addon.dispose();
    refresh();
  });
  try {
    terminal.loadAddon(addon);
  } catch {
    contextLoss.dispose();
    addon.dispose();
  }
  return contextLoss;
};
