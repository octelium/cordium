import Settings from "@/utils/types/settings";
import { PayloadAction, createSlice } from "@reduxjs/toolkit";

import * as UserPB from "@octelium/apis/main/userv1";

export const TERMINAL_FONT_SIZE_MIN = 10;
export const TERMINAL_FONT_SIZE_MAX = 26;

const clampFontSize = (v: number) =>
  Math.min(TERMINAL_FONT_SIZE_MAX, Math.max(TERMINAL_FONT_SIZE_MIN, v));

export const slice = createSlice({
  name: "settings",
  initialState: {
    terminalWide: false,
    terminalFullscreen: false,
    terminalFontSize: 15,
    itemsPerPage: 10,
    navCollapsed: false,
  } as Settings,
  reducers: {
    setTerminalWide: (state, action: PayloadAction<{ value: boolean }>) => {
      state.terminalWide = action.payload.value;
    },

    setTerminalFullscreen: (
      state,
      action: PayloadAction<{ value: boolean }>,
    ) => {
      state.terminalFullscreen = action.payload.value;
    },

    setTerminalFontSize: (state, action: PayloadAction<{ value: number }>) => {
      state.terminalFontSize = clampFontSize(action.payload.value);
    },

    setItemsPerPage: (
      state,
      action: PayloadAction<{ itemsPerPage: number }>,
    ) => {
      state.itemsPerPage = action.payload.itemsPerPage;
    },

    setNavCollapsed: (state, action: PayloadAction<{ value: boolean }>) => {
      state.navCollapsed = action.payload.value;
    },

    setStatus: (
      state,
      action: PayloadAction<{ status: UserPB.GetStatusResponse }>,
    ) => {
      state.status = action.payload.status;
    },
  },
});

export const {
  setTerminalWide,
  setTerminalFullscreen,
  setTerminalFontSize,
  setItemsPerPage,
  setNavCollapsed,
  setStatus,
} = slice.actions;

export default slice.reducer;
