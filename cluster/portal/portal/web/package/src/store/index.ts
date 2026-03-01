import { configureStore } from "@reduxjs/toolkit";
import connReducer from "../features/conn/slice";
import terminalGroupReducer from "../features/terminalgroup/slice";
import ConnMiddleware from "../middlewares/conn";
import settingsReducer from "../features/settings/slice";

const store = configureStore({
  reducer: {
    terminalGroup: terminalGroupReducer,
    conn: connReducer,
    settings: settingsReducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(ConnMiddleware()),
});

export default store;

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
