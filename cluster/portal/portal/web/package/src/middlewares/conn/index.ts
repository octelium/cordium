import { Middleware } from "redux";
import {
  sendListenTerminal,
  sendListenTerminalEnd,
  sendSetTerminalSize,
  sendTerminalData,
} from "../../features/conn/slice";
import { isDev } from "../../utils";

import WebSocketCtl from "./websocket";

export default (): Middleware => {
  if (isDev()) {
    return () => (next) => (action) => next(action);
  }

  const ws = new WebSocketCtl();

  return () => (next) => (action) => {
    if (sendTerminalData.match(action)) {
      const dataBytes = Uint8Array.from(action.payload.data, (x) =>
        x.charCodeAt(0),
      );

      ws.sendMsgData(action.payload.uid, dataBytes);
    } else if (sendSetTerminalSize.match(action)) {
      ws.sendMsgResizeTerminal(
        action.payload.uid,
        action.payload.rows,
        action.payload.cols,
      );
    } else if (sendListenTerminal.match(action)) {
      ws.sendListenTerminal(action.payload.id);
    } else if (sendListenTerminalEnd.match(action)) {
      ws.sendListenTerminalEnd(action.payload.id);
    }

    next(action);
  };
};
