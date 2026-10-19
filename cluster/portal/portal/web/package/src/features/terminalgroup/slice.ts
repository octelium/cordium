import Terminal from "@/utils/types/terminal";
import { createSlice, PayloadAction } from "@reduxjs/toolkit";

interface State {
  workspaceID?: string;
  activeTerminal?: string;
  lastArray: string[];
  terminals: Terminal[];
}

export const slice = createSlice({
  name: "terminalgroup",
  initialState: {
    lastArray: [],
    terminals: [],
  } as State,
  reducers: {
    clearTerminalGroup: (
      state,
      _action: PayloadAction<Record<string, never>>,
    ) => {
      state.lastArray = [];
      state.terminals = [];
      state.activeTerminal = undefined;
    },
    initTerminalGroup: (
      state,
      action: PayloadAction<{ workspaceID: string; termList: Terminal[] }>,
    ) => {
      if (state.workspaceID !== action.payload.workspaceID) {
        state.workspaceID = action.payload.workspaceID;
        state.terminals = [];
        state.lastArray = [];
        state.activeTerminal = undefined;
      }
      const incoming = new Map(
        action.payload.termList.map((terminal) => [terminal.id, terminal]),
      );
      const existing = new Set(state.terminals.map((terminal) => terminal.id));
      state.terminals = [
        ...state.terminals.filter((terminal) => incoming.has(terminal.id)),
        ...action.payload.termList.filter(
          (terminal) => !existing.has(terminal.id),
        ),
      ].sort((a, b) => Number(!!b.pinned) - Number(!!a.pinned));
      state.lastArray = state.lastArray.filter((id) => incoming.has(id));
      if (state.terminals.length > 0) {
        if (
          state.terminals.findIndex((x) => x.id === state.activeTerminal) < 0
        ) {
          state.activeTerminal = state.terminals[0].id;
        }
      } else {
        state.activeTerminal = undefined;
      }
    },
    setActiveTerminal: (state, action: PayloadAction<{ id: string }>) => {
      if (
        state.activeTerminal === action.payload.id ||
        !state.terminals.some((terminal) => terminal.id === action.payload.id)
      )
        return;
      state.lastArray = state.lastArray.filter(
        (id) => id !== action.payload.id && id !== state.activeTerminal,
      );

      if (state.activeTerminal) {
        state.lastArray.push(state.activeTerminal);
      }

      state.activeTerminal = action.payload.id;
    },

    addTerminal: (state, action: PayloadAction<{ id: string }>) => {
      if (state.terminals.some((terminal) => terminal.id === action.payload.id))
        return;
      state.terminals.push({
        id: action.payload.id,
        title: "Terminal",
      } as Terminal);
    },

    removeTerminal: (state, action: PayloadAction<{ id: string }>) => {
      const idx = state.terminals.findIndex((x) => x.id === action.payload.id);
      if (idx < 0) {
        return;
      }

      state.lastArray = state.lastArray.filter((x) => x !== action.payload.id);

      if (state.activeTerminal === action.payload.id) {
        state.activeTerminal = state.lastArray.pop();
      }

      state.terminals.splice(idx, 1);
      if (!state.activeTerminal && state.terminals.length > 0) {
        state.activeTerminal = state.terminals[state.terminals.length - 1].id;
      }
    },

    setTerminalPinned: (
      state,
      action: PayloadAction<{ id: string; pinned: boolean }>,
    ) => {
      const index = state.terminals.findIndex(
        (terminal) => terminal.id === action.payload.id,
      );
      if (
        index < 0 ||
        !!state.terminals[index].pinned === action.payload.pinned
      )
        return;
      const [terminal] = state.terminals.splice(index, 1);
      terminal.pinned = action.payload.pinned;
      const boundary = state.terminals.filter((item) => item.pinned).length;
      state.terminals.splice(boundary, 0, terminal);
    },

    moveTerminal: (
      state,
      action: PayloadAction<{
        id: string;
        targetId: string;
        position: "before" | "after";
      }>,
    ) => {
      const { id, targetId, position } = action.payload;
      if (id === targetId) return;
      const index = state.terminals.findIndex((terminal) => terminal.id === id);
      if (
        index < 0 ||
        !state.terminals.some((terminal) => terminal.id === targetId)
      )
        return;
      const [terminal] = state.terminals.splice(index, 1);
      const target = state.terminals.findIndex((item) => item.id === targetId);
      const boundary = state.terminals.filter((item) => item.pinned).length;
      const desired = target + (position === "after" ? 1 : 0);
      const destination = terminal.pinned
        ? Math.min(desired, boundary)
        : Math.max(desired, boundary);
      state.terminals.splice(destination, 0, terminal);
    },

    setTerminalTitle: (
      state,
      action: PayloadAction<{ id: string; title: string }>,
    ) => {
      const idx = state.terminals.findIndex((x) => x.id === action.payload.id);
      if (idx >= 0) {
        state.terminals[idx].title = action.payload.title;
      }
    },
  },
});

export const {
  clearTerminalGroup,
  setActiveTerminal,
  setTerminalTitle,
  addTerminal,
  removeTerminal,
  initTerminalGroup,
  setTerminalPinned,
  moveTerminal,
} = slice.actions;

export default slice.reducer;
