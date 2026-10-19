import type Terminal from "@/utils/types/terminal";

const storageKey = (workspaceID: string) =>
  `cordium-terminal-tabs:${workspaceID}`;

export const restoreTerminalPreferences = (
  workspaceID: string,
  terminals: Terminal[],
): Terminal[] => {
  try {
    const stored: unknown = JSON.parse(
      localStorage.getItem(storageKey(workspaceID)) ?? "[]",
    );
    if (!Array.isArray(stored)) return terminals;
    const remaining = new Map(
      terminals.map((terminal) => [terminal.id, terminal]),
    );
    const ordered: Terminal[] = [];
    for (const item of stored) {
      if (
        !item ||
        typeof item.id !== "string" ||
        typeof item.pinned !== "boolean"
      )
        continue;
      const terminal = remaining.get(item.id);
      if (!terminal) continue;
      ordered.push({ ...terminal, pinned: item.pinned });
      remaining.delete(item.id);
    }
    return [...ordered, ...remaining.values()].sort(
      (a, b) => Number(!!b.pinned) - Number(!!a.pinned),
    );
  } catch {
    return terminals;
  }
};

export const saveTerminalPreferences = (
  workspaceID: string,
  terminals: Terminal[],
) => {
  try {
    localStorage.setItem(
      storageKey(workspaceID),
      JSON.stringify(
        terminals.map(({ id, pinned }) => ({ id, pinned: !!pinned })),
      ),
    );
  } catch {
    return;
  }
};
