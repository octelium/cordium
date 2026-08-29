import { createSlice, PayloadAction } from "@reduxjs/toolkit";

type State = Record<string, never>;

const noop = () => {};

export const slice = createSlice({
  name: "conn",
  initialState: {} as State,
  reducers: {
    sendTerminalData: (
      _state,
      _action: PayloadAction<{ uid: string; data: string }>,
    ) => noop(),
    sendCreateTerminal: (
      _state,
      _action: PayloadAction<{ userID: string; workspaceUID: string }>,
    ) => noop(),
    sendSetTerminalSize: (
      _state,
      _action: PayloadAction<{ uid: string; rows: number; cols: number }>,
    ) => noop(),
    sendCloseTerminal: (_state, _action: PayloadAction<{ uid: string }>) =>
      noop(),
    sendListenTerminal: (_state, _action: PayloadAction<{ id: string }>) =>
      noop(),
    sendListenTerminalEnd: (_state, _action: PayloadAction<{ id: string }>) =>
      noop(),
  },
});

export const {
  sendTerminalData,
  sendCreateTerminal,
  sendSetTerminalSize,
  sendCloseTerminal,
  sendListenTerminal,
  sendListenTerminalEnd,
} = slice.actions;

export default slice.reducer;
