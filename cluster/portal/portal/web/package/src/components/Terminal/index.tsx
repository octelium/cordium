import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
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
import { loadWebglRenderer } from "./renderer";
import { terminalTheme } from "./theme";

interface Props extends PropsFromRedux {
  id: string;
  isActive: boolean;
}

class Terminal extends React.Component<Props> {
  t!: XTerm;
  fitAddon!: FitAddon;
  containerRef = React.createRef<HTMLDivElement>();
  disposables: IDisposable[] = [];
  resizeObserver: ResizeObserver | null = null;
  mounted = false;
  awaitingReplay = true;

  handleMessage(evt: WsPB.ServerMessage_ListenTerminalEvent) {
    const msg = evt.listenTerminalResponse;
    if (!msg) return;
    switch (msg.type.oneofKind) {
      case "close":
        this.props.removeTerminal({ id: this.props.id });
        break;
      case "windowSize": {
        if (!this.awaitingReplay) break;
        this.awaitingReplay = false;
        this.t.reset();
        const { rows, cols } = msg.type.windowSize;
        if (rows > 0 && cols > 0) this.t.resize(cols, rows);
        this.doResize();
        break;
      }
      case "stdout":
        this.t.write(msg.type.stdout.data);
        break;
    }
  }

  hasSize(): boolean {
    const element = this.containerRef.current;
    return !!element && element.clientWidth > 0 && element.clientHeight > 0;
  }

  doResize = debounce(() => {
    if (!this.mounted || !this.hasSize()) return;
    this.fitAddon.fit();
    this.t.refresh(0, this.t.rows - 1);
    this.props.sendSetTerminalSize({
      uid: this.props.id,
      rows: this.t.rows,
      cols: this.t.cols,
    });
  }, 80);

  resume = () => {
    if (document.visibilityState !== "hidden" && this.props.isActive)
      this.doResize();
  };

  componentDidMount() {
    this.mounted = true;
    this.awaitingReplay = true;
    this.init();
    const unsubscribe = emitter.on(
      `tg-${this.props.id}`,
      (message: WsPB.ServerMessage_ListenTerminalEvent) =>
        this.handleMessage(message),
    );
    const reconnect = emitter.on("terminal-reconnect", () => {
      this.awaitingReplay = true;
      this.doResize();
    });
    this.disposables.push({ dispose: unsubscribe }, { dispose: reconnect });
    this.resizeObserver = new ResizeObserver(() => this.doResize());
    if (this.containerRef.current)
      this.resizeObserver.observe(this.containerRef.current);
    window.addEventListener("focus", this.resume);
    window.addEventListener("pageshow", this.resume);
    document.addEventListener("visibilitychange", this.resume);
    document.fonts.ready.then(() => {
      if (this.mounted) this.doResize();
    });
    this.props.sendListenTerminal({ id: this.props.id });
  }

  componentDidUpdate(prevProps: Props) {
    if (prevProps.fontSize !== this.props.fontSize) {
      this.t.options.fontSize = this.props.fontSize;
      this.doResize();
    }
    if (prevProps.connection !== this.props.connection) {
      this.t.options.disableStdin =
        !isDev() && this.props.connection !== "connected";
      if (this.props.connection === "connected") this.doResize();
    }
    if (!prevProps.isActive && this.props.isActive) {
      this.doResize();
      this.t.focus();
    }
  }

  componentWillUnmount() {
    this.mounted = false;
    this.props.sendListenTerminalEnd({ id: this.props.id });
    this.resizeObserver?.disconnect();
    this.doResize.cancel();
    window.removeEventListener("focus", this.resume);
    window.removeEventListener("pageshow", this.resume);
    document.removeEventListener("visibilitychange", this.resume);
    this.disposables.forEach((disposable) => disposable.dispose());
    this.disposables = [];
    this.t.dispose();
  }

  init() {
    this.t = new XTerm({
      convertEol: true,
      fontFamily: '"Ubuntu Mono", ui-monospace, Menlo, Consolas, monospace',
      fontWeight: 400,
      fontWeightBold: 700,
      cursorBlink: true,
      scrollback: 5000,
      allowProposedApi: true,
      disableStdin: !isDev() && this.props.connection !== "connected",
      fontSize: this.props.fontSize,
      theme: terminalTheme,
    });
    this.fitAddon = new FitAddon();
    this.t.loadAddon(this.fitAddon);
    this.t.loadAddon(new WebLinksAddon());
    this.t.open(this.containerRef.current!);
    if (isWebgl2Supported)
      this.disposables.push(loadWebglRenderer(this.t, () => this.doResize()));
    this.doResize();
    if (this.props.isActive) this.t.focus();
    if (isDev()) {
      this.t.writeln("Cordium development terminal");
      this.t.writeln("Terminal I/O is simulated locally in npm run dev.");
      this.t.write("$ ");
    }
    this.disposables.push(
      this.t.onData((data) => {
        if (isDev()) this.t.write(data);
        this.props.sendTerminalData({ uid: this.props.id, data });
      }),
      this.t.onTitleChange((title) =>
        this.props.setTerminalTitle({ id: this.props.id, title }),
      ),
    );
  }

  render() {
    return <div className="terminal-host" ref={this.containerRef} />;
  }
}

const mapState = (state: RootState) => ({
  fontSize: state.settings.terminalFontSize,
  connection: state.conn.status,
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
