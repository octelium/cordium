import { getClientWorkspaceSvc } from "@/utils/client";
import { getResourceRef } from "@/utils/pb";
import { FitAddon } from "@xterm/addon-fit";
import { IDisposable, Terminal as XTerm } from "@xterm/xterm";
import * as React from "react";

import { debounce } from "lodash";
import { connect, ConnectedProps } from "react-redux";

import * as WsPB from "@octelium/apis/main/cordiumv1";
import { RootState } from "../../store";
import { isWebgl2Supported } from "../../utils";
import { loadWebglRenderer } from "../Terminal/renderer";
import { terminalTheme } from "../Terminal/theme";

interface Props extends PropsFromRedux {
  item: WsPB.Workspace;
  clearToken: number;
}

class LogConsole extends React.Component<Props> {
  t!: XTerm;
  containerRef = React.createRef<HTMLDivElement>();
  fitAddon!: FitAddon;
  disposables: IDisposable[] = [];
  resizeObserver: ResizeObserver | null = null;
  streamController: AbortController | null = null;

  hasSize(): boolean {
    const el = this.containerRef.current;
    return !!el && el.clientWidth > 0 && el.clientHeight > 0;
  }

  doResize = debounce(() => {
    if (this.hasSize()) {
      this.fitAddon.fit();
      this.t.refresh(0, this.t.rows - 1);
    }
  }, 80);

  componentDidMount() {
    this.t = new XTerm({
      convertEol: true,
      fontFamily: '"Ubuntu Mono", ui-monospace, Menlo, Consolas, monospace',
      fontWeight: 400,
      fontWeightBold: 700,
      cursorBlink: false,
      disableStdin: true,
      scrollback: 5000,
      fontSize: this.props.fontSize,
      theme: terminalTheme,
    });
    this.fitAddon = new FitAddon();
    this.t.loadAddon(this.fitAddon);
    this.t.open(this.containerRef.current!);

    if (isWebgl2Supported) {
      this.disposables.push(loadWebglRenderer(this.t, () => this.doResize()));
    }

    this.doResize();

    this.resizeObserver = new ResizeObserver(() => this.doResize());
    if (this.containerRef.current) {
      this.resizeObserver.observe(this.containerRef.current);
    }

    const client = getClientWorkspaceSvc(this.props.item.status?.regionRef);
    this.streamController = new AbortController();
    const strm = client.listenLog(
      WsPB.ListenLogRequest.create({
        workspaceRef: getResourceRef(this.props.item),
      }),
      { abort: this.streamController.signal },
    );

    const unsubscribe = strm.responses.onMessage((msg) => {
      this.t.write(msg.data);
      this.t.write("\r\n");
    });
    this.disposables.push({ dispose: unsubscribe });
  }

  componentDidUpdate(prevProps: Props) {
    if (prevProps.fontSize !== this.props.fontSize) {
      this.t.options.fontSize = this.props.fontSize;
      this.doResize();
    }

    if (prevProps.clearToken !== this.props.clearToken) {
      this.t.clear();
    }
  }

  componentWillUnmount() {
    this.streamController?.abort();
    this.resizeObserver?.disconnect();
    this.doResize.cancel();
    this.disposables.forEach((x) => x.dispose());
    this.disposables = [];
    this.t.dispose();
  }

  render() {
    return <div className="terminal-host" ref={this.containerRef} />;
  }
}

const mapState = (state: RootState) => ({
  fontSize: state.settings.terminalFontSize,
});

const connector = connect(mapState);
type PropsFromRedux = ConnectedProps<typeof connector>;
export default connector(LogConsole);
