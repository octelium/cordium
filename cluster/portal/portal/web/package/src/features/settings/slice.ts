import Settings from "@/utils/types/settings";
import { PayloadAction, createSlice } from "@reduxjs/toolkit";

import * as UserPB from "@/apis/userv1/userv1";

export const slice = createSlice({
  name: "settings",
  initialState: {
    wideTerminal: false,
    itemsPerPage: 10,
    // itemsPerPageNavigator: 5,
  } as Settings,
  reducers: {
    setWideTerminal: (
      state,
      action: PayloadAction<{ wideTerminal: boolean }>,
    ) => {
      state.wideTerminal = action.payload.wideTerminal;
    },

    setItemsPerPage: (
      state,
      action: PayloadAction<{ itemsPerPage: number }>,
    ) => {
      state.itemsPerPage = action.payload.itemsPerPage;
    },

    setStatus: (
      state,
      action: PayloadAction<{ status: UserPB.GetStatusResponse }>,
    ) => {
      state.status = action.payload.status;
    },

    setPersonalSpaceUID: (state, action: PayloadAction<{ uid: string }>) => {
      state.personalSpaceUID = action.payload.uid;
    },
  },
});

export const {
  setWideTerminal,
  setItemsPerPage,
  setStatus,
  setPersonalSpaceUID,
} = slice.actions;

export default slice.reducer;
