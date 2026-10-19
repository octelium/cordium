import * as WsPB from "@octelium/apis/main/cordiumv1";

import type { ConnectionStatus } from "@/features/conn/slice";
import emitter from "@/utils/event";
import { invalidateWorkspace } from "@/utils/octelium";
import { Emitter } from "nanoevents";
import ReconnectingWebSocket from "reconnecting-websocket";

const TERMINAL_DATA_CHUNK_SIZE = 4096;

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
  terminals = new Set<string>();
  terminalSizes = new Map<string, { rows: number; cols: number }>();

  constructor(onStateChange: (status: ConnectionStatus) => void = () => {}) {
    this.state = State.OPENING;
    this.emitter = emitter;
    this.ws = this.doConnect();

    this.ws.onopen = () => {
      this.state = State.OPEN;
      this.emitter.emit("terminal-reconnect");
      for (const id of this.terminals) {
        this.sendListenTerminal(id);
        const size = this.terminalSizes.get(id);
        if (size) this.sendMsgResizeTerminal(id, size.rows, size.cols);
      }
      onStateChange("connected");
    };
    this.ws.onclose = () => {
      this.state = State.CLOSED;
      onStateChange("reconnecting");
    };
    this.ws.onmessage = (e) => {
      const payload = new Uint8Array(e.data);

      this.handleMessage(payload);
    };
    this.ws.onerror = () => {
      this.state = State.ERROR;
      onStateChange("reconnecting");
    };
  }

  doConnect() {
    const scheme = location.protocol === "https:" ? "wss" : "ws";

    const ret = new ReconnectingWebSocket(
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
    for (
      let offset = 0;
      offset < data.length;
      offset += TERMINAL_DATA_CHUNK_SIZE
    ) {
      const msg = WsPB.ClientMessage.create({
        type: {
          oneofKind: "writeTerminalDataRequest",
          writeTerminalDataRequest: {
            id,
            data: data.subarray(offset, offset + TERMINAL_DATA_CHUNK_SIZE),
          },
        },
      });
      this.sendMsg(msg);
    }
  }

  sendListenTerminal(id: string) {
    this.terminals.add(id);
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
    this.terminalSizes.delete(id);
    if (!this.terminals.delete(id)) return;
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
    this.terminalSizes.set(id, { rows, cols });
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
        if (
          msg.type.listenTerminalEvent.listenTerminalResponse?.type
            .oneofKind === "close"
        ) {
          this.terminals.delete(msg.type.listenTerminalEvent.id);
          this.terminalSizes.delete(msg.type.listenTerminalEvent.id);
        }
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
