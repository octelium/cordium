import Terminal from "@/utils/types/terminal";
import { createSlice, PayloadAction } from "@reduxjs/toolkit";

interface State {
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
    clearTerminalGroup: (state, action: PayloadAction<{}>) => {
      // state.activeTerminal = undefined;
      state.lastArray = [];
      state.terminals = [];
    },
    initTerminalGroup: (
      state,
      action: PayloadAction<{ termList: Terminal[] }>,
    ) => {
      state.lastArray = [];
      state.terminals = action.payload.termList;
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
      state.lastArray = state.lastArray.filter((x) => x !== action.payload.id);

      if (state.activeTerminal) {
        state.lastArray.push(state.activeTerminal);
      }
      console.log("lastArr after push", state.lastArray);

      state.activeTerminal = action.payload.id;
    },

    /*
    setLastActiveTerminal: (state, action: PayloadAction<{ id: string }>) => {
      state.lastArray = state.lastArray.filter((x) => x !== action.payload.id);
      console.log("curr array", state.lastArray);
      state.activeTerminal = state.lastArray.pop();
      console.log("last now active", state.activeTerminal);
    },

    deleteLastActive: (state, action: PayloadAction<{ id: string }>) => {
      state.lastArray = state.lastArray.filter((x) => x !== action.payload.id);
    },
    */

    addTerminal: (state, action: PayloadAction<{ id: string }>) => {
      state.terminals.push({
        id: action.payload.id,
        title: "Terminal",
      } as Terminal);
    },

    removeTerminal: (state, action: PayloadAction<{ id: string }>) => {
      console.log("Deleting terminal", action.payload.id);
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
} = slice.actions;

export default slice.reducer;
