import { FitAddon } from "@xterm/addon-fit";
import { IDisposable, Terminal as XTerm } from "@xterm/xterm";
import * as React from "react";
import TerminalT from "../../utils/types/terminal";

import { bindActionCreators } from "@reduxjs/toolkit";
import { WebglAddon } from "@xterm/addon-webgl";
import { debounce } from "lodash";
import { connect, ConnectedProps } from "react-redux";
import { AppDispatch, RootState } from "../../store";

import * as WsPB from "../../apis/cordiumv1/cordiumv1";
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
  containerRef: React.RefObject<HTMLDivElement | null>;

  fitAddon: FitAddon;
  disposables: IDisposable[];
  resizeObserver: ResizeObserver | null = null;

  constructor(props: Props) {
    super(props);

    this.t = new XTerm({
      convertEol: true,
      fontFamily: "Ubuntu Mono",
      fontWeight: 600,
      fontWeightBold: 700,
      cursorBlink: true,
      scrollback: 5000,
      fontSize: props.state.settings.terminalFontSize ?? 18,
      theme: {
        background: "#0f172a",
        foreground: "#e2e8f0",
        cursor: "#94a3b8",
        selectionBackground: "#334155",
        black: "#1e293b",
        brightBlack: "#475569",
        blue: "#60a5fa",
        brightBlue: "#93c5fd",
        cyan: "#22d3ee",
        brightCyan: "#67e8f9",
        green: "#4ade80",
        brightGreen: "#86efac",
        red: "#f87171",
        brightRed: "#fca5a5",
        yellow: "#facc15",
        brightYellow: "#fde047",
        magenta: "#c084fc",
        brightMagenta: "#d8b4fe",
        white: "#cbd5e1",
        brightWhite: "#f1f5f9",
      },
    });

    this.containerRef = React.createRef<HTMLDivElement>();
    this.disposables = [];
    this.fitAddon = new FitAddon();
    this.state = {};

    const unsub = emitter.on(
      `tg-${this.props.item.id}`,
      (msg: WsPB.ServerMessage_ListenTerminalEvent) => {
        this.handleMessage(msg);
      },
    );

    this.disposables.push({ dispose: () => unsub() });
    this.disposables.push(this.fitAddon);
    this.disposables.push(this.t);
  }

  handleMessage(evt: WsPB.ServerMessage_ListenTerminalEvent) {
    const msg = evt.listenTerminalResponse!;
    switch (msg.type.oneofKind) {
      case "close":
        this.props.removeTerminal({ id: this.props.item.id });
        break;
      case "stdout":
        this.t.write(msg.type.stdout.data);
        break;
    }
  }

  doResize = debounce(() => {
    this.fitAddon.fit();
    const termSize = this.fitAddon.proposeDimensions();
    if (!termSize || isNaN(termSize.rows) || isNaN(termSize.cols)) return;

    this.props.sendSetTerminalSize({
      uid: this.props.item.id,
      rows: termSize.rows,
      cols: termSize.cols,
    });
  }, 100);

  componentDidMount() {
    this.init();

    this.resizeObserver = new ResizeObserver(() => {
      this.doResize();
    });

    if (this.containerRef.current) {
      this.resizeObserver.observe(this.containerRef.current);
    }

    this.props.sendListenTerminal({ id: this.props.item.id });
  }

  componentDidUpdate(prevProps: Props) {
    const prevFontSize = prevProps.state.settings.terminalFontSize;
    const nextFontSize = this.props.state.settings.terminalFontSize;

    if (prevFontSize !== nextFontSize && nextFontSize) {
      this.t.options.fontSize = nextFontSize;
      this.doResize();
    }

    const prevWide = prevProps.state.settings.wideTerminal;
    const nextWide = this.props.state.settings.wideTerminal;
    if (prevWide !== nextWide) {
      this.doResize();
    }
  }

  componentWillUnmount() {
    this.props.sendListenTerminalEnd({ id: this.props.item.id });
    this.resizeObserver?.disconnect();
    this.doResize.cancel();
    this.close();
  }

  init() {
    this.t.loadAddon(this.fitAddon);
    this.t.open(this.containerRef.current!);

    if (isWebgl2Supported) {
      try {
        this.t.loadAddon(new WebglAddon());
      } catch {}
    }

    this.fitAddon.fit();
    this.t.focus();

    this.t.onData((data) => this.handleOnData(data));

    this.t.onTitleChange((title) => {
      this.setState({ title });
      this.props.setTerminalTitle({ id: this.props.item.id, title });
    });

    this.t.onResize(({ rows, cols }) => {
      this.props.sendSetTerminalSize({
        uid: this.props.item.id,
        rows,
        cols,
      });
    });

    const onFocus = () => this.doResize();
    this.t.textarea?.addEventListener("focus", onFocus);
    this.disposables.push({
      dispose: () => this.t.textarea?.removeEventListener("focus", onFocus),
    });
  }

  close() {
    this.disposables.forEach((x) => x.dispose());
    this.disposables = [];
  }

  handleOnData(data: string) {
    if (isDev()) {
      this.t.write(data);
    }
    this.props.sendTerminalData({
      uid: this.props.item.id,
      data,
    });
  }

  render() {
    const wideTerminal = this.props.state.settings.wideTerminal;

    return (
      <div
        style={{
          width: wideTerminal ? "96vw" : "100%",
          marginLeft: wideTerminal ? "calc(-48vw + 50%)" : undefined,
          padding: "8px 4px 4px",
          boxSizing: "border-box",
          transition: "width 150ms ease, margin-left 150ms ease",
        }}
        ref={this.containerRef}
      />
    );
  }
}

const mapState = (state: RootState) => ({ state });
const mapDispatch = (dispatch: AppDispatch) =>
  bindActionCreators(
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

const connector = connect(mapState, mapDispatch);
type PropsFromRedux = ConnectedProps<typeof connector>;
export default connector(Terminal);
