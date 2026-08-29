import { getClientWorkspaceSvc } from "@/utils/client";
import { getResourceRef } from "@/utils/pb";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import { IDisposable, Terminal as XTerm } from "@xterm/xterm";
import * as React from "react";

import { debounce } from "lodash";
import { connect, ConnectedProps } from "react-redux";

import * as WsPB from "@octelium/apis/main/cordiumv1";
import { RootState } from "../../store";
import { isWebgl2Supported } from "../../utils";
import { terminalTheme } from "../Terminal/theme";

interface Props extends PropsFromRedux {
  item: WsPB.Workspace;
  clearToken: number;
}

class LogConsole extends React.Component<Props> {
  t: XTerm;
  containerRef: React.RefObject<HTMLDivElement | null>;
  fitAddon: FitAddon;
  disposables: IDisposable[];
  resizeObserver: ResizeObserver | null = null;
  stream: { requests?: { abort?: () => void } } | null = null;

  constructor(props: Props) {
    super(props);

    this.t = new XTerm({
      convertEol: true,
      fontFamily: '"Ubuntu Mono", ui-monospace, Menlo, Consolas, monospace',
      fontWeight: 400,
      fontWeightBold: 700,
      cursorBlink: false,
      disableStdin: true,
      scrollback: 5000,
      fontSize: props.fontSize,
      theme: terminalTheme,
    });

    this.containerRef = React.createRef<HTMLDivElement>();
    this.fitAddon = new FitAddon();
    this.disposables = [this.fitAddon, this.t];
  }

  hasSize(): boolean {
    const el = this.containerRef.current;
    return !!el && el.clientWidth > 0 && el.clientHeight > 0;
  }

  doResize = debounce(() => {
    if (this.hasSize()) {
      this.fitAddon.fit();
    }
  }, 80);

  componentDidMount() {
    this.t.loadAddon(this.fitAddon);
    this.t.open(this.containerRef.current!);

    if (isWebgl2Supported) {
      try {
        this.t.loadAddon(new WebglAddon());
      } catch {
        /* falls back to the DOM renderer */
      }
    }

    this.doResize();

    this.resizeObserver = new ResizeObserver(() => this.doResize());
    if (this.containerRef.current) {
      this.resizeObserver.observe(this.containerRef.current);
    }

    const client = getClientWorkspaceSvc(this.props.item.status?.regionRef);
    const strm = client.listenLog(
      WsPB.ListenLogRequest.create({
        workspaceRef: getResourceRef(this.props.item),
      }),
    );

    strm.responses.onMessage((msg) => {
      this.t.write(msg.data);
      this.t.write("\r\n");
    });
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
    this.resizeObserver?.disconnect();
    this.doResize.cancel();
    this.disposables.forEach((x) => x.dispose());
    this.disposables = [];
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
