import { Middleware } from "redux";
import {
  sendListenTerminal,
  sendListenTerminalEnd,
  sendSetTerminalSize,
  sendTerminalData,
} from "../../features/conn/slice";

// import { addWorkspace, setWorkspaces } from "../../features/workspaces/slice";

import WebSocketCtl from "./websocket";

export default (): Middleware => {
  const ws = new WebSocketCtl();

  return (store) => (next) => (action) => {
    if (sendTerminalData.match(action)) {
      const dataBytes = Uint8Array.from(action.payload.data, (x) =>
        x.charCodeAt(0),
      );

      ws.sendMsgData(action.payload.uid, dataBytes);
    } else if (sendSetTerminalSize.match(action)) {
      console.log("Sending set terminal size", action.payload);
      ws.sendMsgResizeTerminal(
        action.payload.uid,
        action.payload.rows,
        action.payload.cols,
      );
    } else if (sendListenTerminal.match(action)) {
      console.log("Sending listenTerminal", action.payload);
      ws.sendListenTerminal(action.payload.id);
    } else if (sendListenTerminalEnd.match(action)) {
      console.log("Sending listenTerminalEnd", action.payload);
      ws.sendListenTerminalEnd(action.payload.id);
    }

    next(action);
  };
};
