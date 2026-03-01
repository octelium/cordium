import { createSlice, PayloadAction } from "@reduxjs/toolkit";

interface State {}

export const slice = createSlice({
  name: "conn",
  initialState: {} as State,
  reducers: {
    sendTerminalData: (
      state,
      action: PayloadAction<{ uid: string; data: string }>,
    ) => {},
    sendCreateTerminal: (
      state,
      action: PayloadAction<{
        userID: string;
        workspaceUID: string;
      }>,
    ) => {},
    sendSetTerminalSize: (
      state,
      action: PayloadAction<{ uid: string; rows: number; cols: number }>,
    ) => {},
    sendCloseTerminal: (state, action: PayloadAction<{ uid: string }>) => {},

    sendListenTerminal: (state, action: PayloadAction<{ id: string }>) => {},
    sendListenTerminalEnd: (state, action: PayloadAction<{ id: string }>) => {},
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
