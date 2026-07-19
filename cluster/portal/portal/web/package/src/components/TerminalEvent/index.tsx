import { FitAddon } from "@xterm/addon-fit";
import { IDisposable, Terminal as XTerm } from "@xterm/xterm";
import * as React from "react";

import { bindActionCreators } from "@reduxjs/toolkit";
import { SerializeAddon } from "@xterm/addon-serialize";
import { WebglAddon } from "@xterm/addon-webgl";
import { debounce } from "lodash";
import { connect, ConnectedProps } from "react-redux";
import { AppDispatch, RootState } from "../../store";

import { getClientWorkspaceSvc } from "@/utils/client";
import { getResourceRef } from "@/utils/pb";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { twMerge } from "tailwind-merge";

import {
  sendCloseTerminal,
  sendCreateTerminal,
  sendSetTerminalSize,
  sendTerminalData,
} from "../../features/conn/slice";
import { setWideTerminal } from "../../features/settings/slice";
import { isWebgl2Supported } from "../../utils";

interface Props extends PropsFromRedux {
  item: WsPB.Workspace;
}

interface State {}

export class TerminalEvent extends React.Component<Props, State> {
  t: XTerm;
  ref: React.RefObject<HTMLInputElement | null>;

  uid!: string;
  fitAddon: FitAddon;
  serializeAddon: SerializeAddon;
  disposables: IDisposable[];

  c: WsPB.WorkspaceServiceClient;

  constructor(props: Props) {
    super(props);
    this.t = new XTerm({
      convertEol: true,
      fontFamily: "Ubuntu Mono",
      fontWeight: 600,
      fontWeightBold: 700,
      cursorBlink: false,
      scrollback: 3000,

      fontSize: props.state.settings.terminalFontSize ?? 18,
    });

    this.ref = React.createRef<HTMLInputElement>();
    this.c = getClientWorkspaceSvc(this.props.item.status?.regionRef);

    this.disposables = [];

    this.fitAddon = new FitAddon();
    this.serializeAddon = new SerializeAddon();
    this.state = {};

    const doResize = debounce(() => {
      this.fitAddon.fit();
    }, 300);

    window.addEventListener("resize", doResize);

    this.disposables.push({
      dispose: () => {
        window.removeEventListener("resize", doResize);
      },
    });

    this.disposables.push(this.fitAddon);
    this.disposables.push(this.t);
  }

  componentDidMount() {
    this.init();
    this.fitAddon.fit();
    setTimeout(() => {
      this.fitAddon.fit();
    }, 500);

    const strm = this.c.listenLog(
      WsPB.ListenLogRequest.create({
        workspaceRef: getResourceRef(this.props.item),
      }),
    );

    strm.responses.onMessage((msg) => {
      this.t.write(msg.data);
      this.t.write("\r\n");
    });
  }

  onActive() {
    this.fitAddon.fit();
  }

  componentDidUpdate() {
    this.fitAddon.fit();
  }

  componentWillUnmount() {
    this.close();
  }

  init() {
    this.t.loadAddon(this.fitAddon);
    this.t.loadAddon(this.serializeAddon);

    this.t.open(this.ref.current!);

    if (isWebgl2Supported) {
      console.log("WebGL2 is supported");
      this.t.loadAddon(new WebglAddon());
    }

    this.fitAddon.fit();

    this.t.focus();

    this.t.onData((arg) => {});

    this.t.onResize(({ rows, cols }) => {});

    const onActive = () => {
      this.onActive();
    };

    this.t.textarea?.addEventListener("focus", onActive);
    this.disposables.push({
      dispose: () => {
        this.t.textarea?.removeEventListener("focus", onActive);
      },
    });

    this.t.write("\r\n");
  }

  close() {
    this.disposables.map((x) => x.dispose());
    this.disposables = [];
  }

  handleClose() {
    this.close();
  }

  render() {
    return (
      <div
        className={twMerge(
          "w-full",
          "rounded-md overflow-hidden shadow-2xl",
          "flex items-center justify-center transition-all duration-100",
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
      sendCloseTerminal,

      sendCreateTerminal,
      setWideTerminal,
    },
    dispatch,
  );
};
const connector = connect(mapState, mapDispatch);
type PropsFromRedux = ConnectedProps<typeof connector>;
export default connector(TerminalEvent);
