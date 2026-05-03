import * as WsPB from "@/apis/cordiumv1/cordiumv1";

import { clearTerminalGroup } from "@/features/terminalgroup/slice";
import store from "@/store";
import { isDev } from "@/utils";
import emitter from "@/utils/event";
import { invalidateWorkspace } from "@/utils/octelium";
import { Emitter } from "nanoevents";
import toast from "react-hot-toast";
import ReconnectingWebSocket from "reconnecting-websocket";

export enum State {
  OPENING = 1,
  OPEN,
  CLOSED,
  ERROR,
}

export class WebSocketCtl {
  ws: ReconnectingWebSocket;
  state: State;
  emitter: Emitter;
  // curCreatedTerminalUserID?: string;
  // isCreatingTerminal!: boolean;

  constructor() {
    this.state = State.OPENING;
    this.ws = this.doConnect();
    this.emitter = emitter;

    this.ws.onopen = () => {
      console.log("ws conn is now open");
      this.state = State.OPEN;
      toast.remove("ws-err-term-group");
    };
    this.ws.onclose = () => {
      console.log("ws conn is now closed");
      this.state = State.CLOSED;
    };
    this.ws.onmessage = (e) => {
      const payload = new Uint8Array(e.data);

      this.handleMessage(payload);
    };
    this.ws.onerror = (err) => {
      console.log("ws conn error", err);
      this.state = State.ERROR;
      store.dispatch(clearTerminalGroup({}));
      if (!isDev()) {
        toast.error("Websocket disconnected. Please restart the page", {
          id: "ws-err-term-group",
          duration: Infinity,
        });
      }
    };
  }

  doConnect() {
    const scheme = location.protocol === "https:" ? "wss" : "ws";

    let ret = new ReconnectingWebSocket(
      `${scheme}://${window.location.host}/connect`,
      [],
      {
        debug: false,
      },
    );

    ret.binaryType = "arraybuffer";

    return ret;
  }

  sendMsg(msg: WsPB.ClientMessage) {
    if (this.state != State.OPEN) {
      return;
    }

    this.ws.send(WsPB.ClientMessage.toBinary(msg));
  }

  sendMsgData(id: string, data: Uint8Array) {
    const msg = WsPB.ClientMessage.create({
      type: {
        oneofKind: "writeTerminalDataRequest",
        writeTerminalDataRequest: {
          id,
          data,
        },
      },
    });
    this.sendMsg(msg);
  }

  sendListenTerminal(id: string) {
    const msg = WsPB.ClientMessage.create({
      type: {
        oneofKind: "listenTerminalRequest",
        listenTerminalRequest: {
          id,
        },
      },
    });
    this.sendMsg(msg);
  }

  sendListenTerminalEnd(id: string) {
    const msg = WsPB.ClientMessage.create({
      type: {
        oneofKind: "listenTerminalEndRequest",
        listenTerminalEndRequest: {
          id,
        },
      },
    });
    this.sendMsg(msg);
  }

  sendMsgResizeTerminal(id: string, rows: number, cols: number) {
    const msg = WsPB.ClientMessage.create({
      type: {
        oneofKind: "setTerminalWindowSizeRequest",
        setTerminalWindowSizeRequest: {
          id,
          cols,
          rows,
        },
      },
    });
    this.sendMsg(msg);
  }

  handleMessage(data: Uint8Array) {
    const msg = WsPB.ServerMessage.fromBinary(data);

    switch (msg.type.oneofKind) {
      case "workspaceUpdate": {
        const workspace = msg.type.workspaceUpdate.workspace!;
        invalidateWorkspace(workspace);

        break;
      }
      case "listenTerminalEvent": {
        this.emitter.emit(
          `tg-${msg.type.listenTerminalEvent.id}`,
          msg.type.listenTerminalEvent,
        );
        break;
      }
    }
  }
}

export default WebSocketCtl;
