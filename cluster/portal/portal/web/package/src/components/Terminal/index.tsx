import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import { IDisposable, Terminal as XTerm } from "@xterm/xterm";
import * as React from "react";

import { bindActionCreators } from "@reduxjs/toolkit";
import { debounce } from "lodash";
import { connect, ConnectedProps } from "react-redux";

import * as WsPB from "@octelium/apis/main/cordiumv1";
import {
  sendListenTerminal,
  sendListenTerminalEnd,
  sendSetTerminalSize,
  sendTerminalData,
} from "../../features/conn/slice";
import {
  removeTerminal,
  setTerminalTitle,
} from "../../features/terminalgroup/slice";
import { AppDispatch, RootState } from "../../store";
import { isDev, isWebgl2Supported } from "../../utils";

import emitter from "../../utils/event";
import { terminalTheme } from "./theme";

interface Props extends PropsFromRedux {
  id: string;
  isActive: boolean;
}

class Terminal extends React.Component<Props> {
  t: XTerm;
  containerRef: React.RefObject<HTMLDivElement | null>;

  fitAddon: FitAddon;
  disposables: IDisposable[];
  resizeObserver: ResizeObserver | null = null;

  constructor(props: Props) {
    super(props);

    this.t = new XTerm({
      convertEol: true,
      fontFamily: '"Ubuntu Mono", ui-monospace, Menlo, Consolas, monospace',
      fontWeight: 400,
      fontWeightBold: 700,
      cursorBlink: true,
      scrollback: 5000,
      allowProposedApi: true,
      fontSize: props.fontSize,
      theme: terminalTheme,
    });

    this.containerRef = React.createRef<HTMLDivElement>();
    this.disposables = [];
    this.fitAddon = new FitAddon();

    const unsub = emitter.on(
      `tg-${this.props.id}`,
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
        this.props.removeTerminal({ id: this.props.id });
        break;
      case "stdout":
        this.t.write(msg.type.stdout.data);
        break;
    }
  }

  hasSize(): boolean {
    const el = this.containerRef.current;
    return !!el && el.clientWidth > 0 && el.clientHeight > 0;
  }

  doResize = debounce(() => {
    if (!this.hasSize()) return;

    this.fitAddon.fit();
    const termSize = this.fitAddon.proposeDimensions();
    if (!termSize || isNaN(termSize.rows) || isNaN(termSize.cols)) return;

    this.props.sendSetTerminalSize({
      uid: this.props.id,
      rows: termSize.rows,
      cols: termSize.cols,
    });
  }, 80);

  componentDidMount() {
    this.init();

    this.resizeObserver = new ResizeObserver(() => this.doResize());
    if (this.containerRef.current) {
      this.resizeObserver.observe(this.containerRef.current);
    }

    this.props.sendListenTerminal({ id: this.props.id });
  }

  componentDidUpdate(prevProps: Props) {
    if (prevProps.fontSize !== this.props.fontSize) {
      this.t.options.fontSize = this.props.fontSize;
      this.doResize();
    }

    if (!prevProps.isActive && this.props.isActive) {
      this.doResize();
      this.t.focus();
    }
  }

  componentWillUnmount() {
    this.props.sendListenTerminalEnd({ id: this.props.id });
    this.resizeObserver?.disconnect();
    this.doResize.cancel();
    this.close();
  }

  init() {
    this.t.loadAddon(this.fitAddon);
    this.t.loadAddon(new WebLinksAddon());
    this.t.open(this.containerRef.current!);

    if (isWebgl2Supported) {
      try {
        this.t.loadAddon(new WebglAddon());
      } catch {
        /* falls back to the DOM renderer */
      }
    }

    if (this.hasSize()) {
      this.fitAddon.fit();
    }

    if (this.props.isActive) {
      this.t.focus();
    }

    this.t.onData((data) => this.handleOnData(data));

    this.t.onTitleChange((title) => {
      this.props.setTerminalTitle({ id: this.props.id, title });
    });

    this.t.onResize(({ rows, cols }) => {
      this.props.sendSetTerminalSize({ uid: this.props.id, rows, cols });
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
    this.props.sendTerminalData({ uid: this.props.id, data });
  }

  render() {
    return <div className="terminal-host" ref={this.containerRef} />;
  }
}

const mapState = (state: RootState) => ({
  fontSize: state.settings.terminalFontSize,
});

const mapDispatch = (dispatch: AppDispatch) =>
  bindActionCreators(
    {
      sendTerminalData,
      sendSetTerminalSize,
      setTerminalTitle,
      removeTerminal,
      sendListenTerminal,
      sendListenTerminalEnd,
    },
    dispatch,
  );

const connector = connect(mapState, mapDispatch);
type PropsFromRedux = ConnectedProps<typeof connector>;
export default connector(Terminal);
