import { FitAddon } from "@xterm/addon-fit";
import { IDisposable, Terminal as XTerm } from "@xterm/xterm";
import * as React from "react";
import TerminalT from "../../utils/types/terminal";

import { bindActionCreators } from "@reduxjs/toolkit";
import { WebglAddon } from "@xterm/addon-webgl";
import { debounce } from "lodash";
import { connect, ConnectedProps } from "react-redux";
import { AppDispatch, RootState } from "../../store";

import { getClientWorkspaceSvc } from "@/utils/client";
import { twMerge } from "tailwind-merge";
import * as WsPB from "../../apis/cordiumv1/cordiumv1";
import * as WsGRPC from "../../apis/cordiumv1/cordiumv1.client";
import {
  sendListenTerminal,
  sendListenTerminalEnd,
  sendSetTerminalSize,
  sendTerminalData,
} from "../../features/conn/slice";
import { setWideTerminal } from "../../features/settings/slice";
import {
  removeTerminal,
  setTerminalTitle,
} from "../../features/terminalgroup/slice";
import { isDev, isWebgl2Supported } from "../../utils";

import emitter from "../../utils/event";

interface Props extends PropsFromRedux {
  item: TerminalT;
}

interface State {
  title?: string;
}

export class Terminal extends React.Component<Props, State> {
  t: XTerm;
  ref: React.RefObject<HTMLInputElement | null>;

  fitAddon: FitAddon;
  // serializeAddon: SerializeAddon;
  disposables: IDisposable[];
  c: WsGRPC.WorkspaceServiceClient;

  constructor(props: Props) {
    super(props);
    console.log("Starting term constructor");
    this.t = new XTerm({
      convertEol: true,
      fontFamily: "Ubuntu Mono",
      fontWeight: 600,
      fontWeightBold: 700,
      cursorBlink: true,
      scrollback: 1000,
      fontSize: props.state.settings.terminalFontSize ?? 18,
    });

    this.ref = React.createRef<HTMLInputElement>();

    this.c = getClientWorkspaceSvc(undefined);

    this.disposables = [];

    this.fitAddon = new FitAddon();
    // this.serializeAddon = new SerializeAddon();
    this.state = {};

    const unsub = emitter.on(
      `tg-${this.props.item.id}`,
      (msg: WsPB.ServerMessage_ListenTerminalEvent) => {
        this.handleMessage(msg);
      },
    );

    const doResize = debounce(() => {
      console.log("Resizing terminal: ", this.props.item.id);
      this.doResize();
    }, 300);

    window.addEventListener("resize", doResize);

    this.disposables.push({
      dispose: () => {
        window.removeEventListener("resize", doResize);
      },
    });

    this.disposables.push({
      dispose: () => {
        unsub();
      },
    });

    this.disposables.push(this.fitAddon);
    this.disposables.push(this.t);
    console.log("Successfully done term constructor");
  }

  handleMessage(evt: WsPB.ServerMessage_ListenTerminalEvent) {
    const msg = evt.listenTerminalResponse!;
    switch (msg.type.oneofKind) {
      case "close": {
        this.props.removeTerminal({
          id: this.props.item.id,
        });
        break;
      }
      case "stdout": {
        this.t.write(msg.type.stdout.data);
        break;
      }
      case "windowSize": {
        break;
      }
    }
  }

  doResize() {
    this.fitAddon.fit();
    const termSize = this.fitAddon.proposeDimensions();
    if (termSize === undefined) {
      console.log("Undefined term size. Skipping");
      return;
    }
    if (isNaN(termSize.rows) || isNaN(termSize.cols)) {
      console.log("NaN rows or cols. Skipping...");
      return;
    }

    console.log(
      "Fitting and sending terminal size",
      this.props.item.id,
      termSize,
    );
    this.props.sendSetTerminalSize({
      uid: this.props.item.id,
      rows: termSize.rows,
      cols: termSize.cols,
    });
  }

  componentDidMount() {
    console.log("mounted terminal: ", this.props.item.id);
    this.init();
    this.doResize();
    setTimeout(() => {
      this.doResize();
    }, 500);

    this.props.sendListenTerminal({
      id: this.props.item.id,
    });
    /*
    const strm = this.c.listenTerminal(
      WsPB.ListenTerminalRequest.create({
        id: this.props.item.id,
      })
    );
    strm.responses.onMessage((msg) => {
      console.log("Got listenTerminal msg", this.props.item.id, msg);
      switch (msg.type.oneofKind) {
        case "close": {
          this.props.removeTerminal({
            id: this.props.item.id,
          });
          break;
        }
        case "stdout": {
          this.t.write(msg.type.stdout.data);
          break;
        }
        case "windowSize": {
          break;
        }
      }
    });
    */
  }

  onActive() {
    console.log("TERM ACTIVE: ", this.props.item.id);
    this.doResize();
  }

  componentDidUpdate() {
    console.log("DID UPDATE: ", this.props.item.id);

    this.doResize();
  }

  componentWillUnmount() {
    console.log("Unmounting terminal: ", this.props.item.id);
    this.props.sendListenTerminalEnd({
      id: this.props.item.id,
    });
    this.close();
  }

  init() {
    console.log("Initializing terminal: ", this.props.item.id);

    this.t.loadAddon(this.fitAddon);
    // this.t.loadAddon(this.serializeAddon);

    /*
    if (this.props.item.buffer) {
      this.t.write(this.props.item.buffer);
    }
    */
    this.t.open(this.ref.current!);

    if (isWebgl2Supported) {
      console.log("WebGL2 is supported");
      this.t.loadAddon(new WebglAddon());
    }

    this.fitAddon.fit();

    this.t.focus();

    this.t.onData((arg) => {
      console.log("onData", arg);
      this.handleOnData(arg);
    });

    this.t.onTitleChange((title) => {
      console.log("New title = ", title);
      this.setState({
        title,
      });
      this.props.setTerminalTitle({ id: this.props.item.id, title });
    });

    this.t.onResize(({ rows, cols }) => {
      console.log("Sending setTerminal size: ", rows, cols);
      this.props.sendSetTerminalSize({
        uid: this.props.item.id,
        rows,
        cols,
      });
    });

    const onActive = () => {
      this.onActive();
    };

    this.t.textarea?.addEventListener("focus", onActive);
    this.disposables.push({
      dispose: () => {
        this.t.textarea?.removeEventListener("focus", onActive);
      },
    });
  }

  close() {
    console.log("closing terminal: ", this.props.item.id);
    /*
    this.props.serializeTerminalBuffer({
      uid: this.props.item.id,
      buffer: this.serializeAddon.serialize(),
    });
    */
    this.disposables.map((x) => x.dispose());
    this.disposables = [];
  }

  handleClose() {
    this.close();
  }

  handleOnData(arg: string) {
    console.log("WRITING", arg);
    if (isDev()) {
      this.t.write(arg);
      return;
    }

    this.props.sendTerminalData({
      uid: this.props.item.id,
      data: arg,
    });
  }

  render() {
    const wideTerminal = this.props.state.settings.wideTerminal;

    return (
      <div
        className={twMerge(
          "rounded-md overflow-hidden shadow-2xl",
          "flex items-center justify-center transition-all duration-100",
          wideTerminal ? `md:w-[96vw] md:ml-[calc(-48vw+50%)]` : undefined,
        )}
        ref={this.ref}
      ></div>
    );
  }
}

const mapState = (state: RootState) => ({
  state,
});
const mapDispatch = (dispatch: AppDispatch) => {
  return bindActionCreators(
    {
      sendTerminalData,
      sendSetTerminalSize,
      setTerminalTitle,
      setWideTerminal,
      removeTerminal,
      sendListenTerminal,
      sendListenTerminalEnd,
    },
    dispatch,
  );
};
const connector = connect(mapState, mapDispatch);
type PropsFromRedux = ConnectedProps<typeof connector>;
export default connector(Terminal);
